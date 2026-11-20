package mealplan

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/httpserver"
)

// feedDays is how far back the calendar feed reaches.
const feedDays = 365

// Feeds hands out each person's secret calendar feed and answers it. The
// secret is not an API token: anyone signed in gets one, it reads nothing
// but the feed, and only its SHA-256 is stored.
type Feeds struct {
	q         *sqlc.Queries
	publicURL string
	host      string
	now       func() time.Time
}

// NewFeeds returns Feeds backed by conn; publicURL starts the feed and
// recipe addresses and names the host of each event's UID.
func NewFeeds(conn *sql.DB, publicURL string) *Feeds {
	host := "rezepte"
	if u, err := url.Parse(publicURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return &Feeds{q: sqlc.New(conn), publicURL: publicURL, host: host, now: time.Now}
}

func feedHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Create makes a new secret for the person, revoking the one before, and
// returns the feed's address.
func (f *Feeds) Create(ctx context.Context, userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("feed secret: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	if err := f.q.UpsertCalendarFeed(ctx, sqlc.UpsertCalendarFeedParams{UserID: userID, TokenHash: feedHash(secret), CreatedAt: db.FormatTime(f.now())}); err != nil {
		return "", fmt.Errorf("upsert calendar feed: %w", err)
	}
	return f.publicURL + "/calendar/" + secret + ".ics", nil
}

// Since reports when the person's current feed was made; nil while they
// have none. The secret itself is never stored, so it cannot be shown again.
func (f *Feeds) Since(ctx context.Context, userID string) (*time.Time, error) {
	at, err := f.q.GetCalendarFeedCreated(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get calendar feed: %w", err)
	}
	t, err := db.ParseTime(at)
	if err != nil {
		return nil, fmt.Errorf("parse calendar feed time: %w", err)
	}
	return &t, nil
}

// Delete revokes the person's feed.
func (f *Feeds) Delete(ctx context.Context, userID string) error {
	if err := f.q.DeleteCalendarFeed(ctx, userID); err != nil {
		return fmt.Errorf("delete calendar feed: %w", err)
	}
	return nil
}

// Write writes the feed behind secret: every live dated entry of every plan
// its person can access, from feedDays ago on. ErrNotFound for an unknown
// secret.
func (f *Feeds) Write(ctx context.Context, w io.Writer, secret string) error {
	userID, err := f.q.GetCalendarFeedUser(ctx, feedHash(secret))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get calendar feed: %w", err)
	}
	now := f.now()
	from := now.AddDate(0, 0, -feedDays).Format(dayLayout)
	rows, err := f.q.ListCalendarFeedEntries(ctx, sqlc.ListCalendarFeedEntriesParams{FromDay: &from, UserID: userID})
	if err != nil {
		return fmt.Errorf("list calendar feed entries: %w", err)
	}
	events := make([]feedEvent, 0, len(rows))
	for _, r := range rows {
		e := feedEvent{UID: r.ID, Day: *r.Day, Summary: r.Title}
		if r.RecipeSlug != nil && f.publicURL != "" {
			e.URL = f.publicURL + "/recipes/" + *r.RecipeSlug
		}
		events = append(events, e)
	}
	return writeICal(w, f.host, events, now)
}

type (
	// FeedURL is the response body of create-calendar-feed.
	FeedURL struct {
		URL string `json:"url" doc:"The feed's secret address; anyone holding it reads the caller's plans"`
	}
	feedOut struct{ Body FeedURL }
	// FeedState is the response body of get-calendar-feed-state.
	FeedState struct {
		Active    bool       `json:"active" doc:"Whether the caller has a feed address that works"`
		CreatedAt *time.Time `json:"createdAt" doc:"When that address was made; null without one"`
	}
	feedStateOut struct{ Body FeedState }
	feedFile     struct {
		File string `path:"file" doc:"The secret followed by .ics"`
	}
	icsOut struct {
		ContentType  string `header:"Content-Type"`
		CacheControl string `header:"Cache-Control"`
		Body         []byte
	}
)

// RegisterFeed installs the calendar feed: creating and revoking the secret
// with a session only, and the feed itself for anyone holding it.
func RegisterFeed(api huma.API, feeds *Feeds) {
	caller := func(ctx context.Context) (string, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return "", huma.Error401Unauthorized("authentication required")
		}
		return u.ID, nil
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-calendar-feed-state", Method: http.MethodGet, Path: "/api/v1/calendar-feed",
		Summary:     "Tell whether the caller has a calendar feed",
		Description: "Whether the caller's feed address works and since when; the address itself is shown only once, by create-calendar-feed. Session only.",
		Tags:        []string{"meal-plans"}, Security: auth.SessionSecurity,
	}, func(ctx context.Context, _ *struct{}) (*feedStateOut, error) {
		uid, err := caller(ctx)
		if err != nil {
			return nil, err
		}
		at, err := feeds.Since(ctx, uid)
		if err != nil {
			return nil, err
		}
		return &feedStateOut{Body: FeedState{Active: at != nil, CreatedAt: at}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-calendar-feed", Method: http.MethodPost, Path: "/api/v1/calendar-feed",
		Summary:     "Create or replace the caller's calendar feed",
		Description: "A new secret address for the caller's meal plans as an iCalendar feed; the address before stops working. Session only.",
		Tags:        []string{"meal-plans"}, Security: auth.SessionSecurity,
	}, func(ctx context.Context, _ *struct{}) (*feedOut, error) {
		uid, err := caller(ctx)
		if err != nil {
			return nil, err
		}
		u, err := feeds.Create(ctx, uid)
		if err != nil {
			return nil, err
		}
		return &feedOut{Body: FeedURL{URL: u}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-calendar-feed", Method: http.MethodDelete, Path: "/api/v1/calendar-feed",
		Summary: "Revoke the caller's calendar feed", Tags: []string{"meal-plans"}, Security: auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		uid, err := caller(ctx)
		if err != nil {
			return nil, err
		}
		return &struct{}{}, feeds.Delete(ctx, uid)
	})

	// ServeMux cannot match "{secret}.ics", so the segment is taken whole.
	huma.Register(api, huma.Operation{
		OperationID: "get-calendar-feed", Method: http.MethodGet, Path: "/calendar/{file}",
		Summary:     "Read a calendar feed",
		Description: "No session needed: a calendar app subscribes to the address create-calendar-feed returned. One all-day event per planned day entry of every plan the feed's owner can access, from a year ago on. An unknown or revoked secret answers 404.",
		Tags:        []string{"public"}, Errors: []int{404},
		Responses: map[string]*huma.Response{"200": {
			Description: "The iCalendar file",
			Content:     map[string]*huma.MediaType{"text/calendar": {Schema: &huma.Schema{Type: "string"}}},
		}},
	}, func(ctx context.Context, in *feedFile) (*icsOut, error) {
		secret, ok := strings.CutSuffix(in.File, ".ics")
		if !ok {
			return nil, httpserver.PublicNotFound("not found")
		}
		var b strings.Builder
		if err := feeds.Write(ctx, &b, secret); errors.Is(err, ErrNotFound) {
			return nil, httpserver.PublicNotFound("not found")
		} else if err != nil {
			return nil, err
		}
		return &icsOut{ContentType: "text/calendar; charset=utf-8", CacheControl: "private, max-age=900", Body: []byte(b.String())}, nil
	})
}
