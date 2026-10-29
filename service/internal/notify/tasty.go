package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// tastyDelay is how long a tasty mark waits before its author hears of it,
// so a mark taken back at once tells nobody. A variable so a test can
// shorten it.
var tastyDelay = time.Minute

// pendingTasty holds "<recipeID>:<markerID>" while its notification waits,
// so marking, clearing and marking again within the delay tells once.
var pendingTasty sync.Map

// TastyMarked tells the author of recipeID, by mail and push, that markerID
// finds it tasty, tastyDelay later and only if the mark is still there. It
// implements recipe.TastyNotifier.
// ponytail: pending tasty notifications live in memory; a restart within the delay drops them.
func (s *Service) TastyMarked(ctx context.Context, markerID, recipeID string) {
	key := recipeID + ":" + markerID
	if _, waiting := pendingTasty.LoadOrStore(key, struct{}{}); waiting {
		return
	}
	time.AfterFunc(tastyDelay, func() {
		pendingTasty.Delete(key)
		detach(ctx, "tasty notification", func(ctx context.Context) { s.sendTasty(ctx, markerID, recipeID) })
	})
}

// sendTasty mails and pushes the author of recipeID that markerID finds it
// tasty, if the mark still exists and the author is somebody else.
func (s *Service) sendTasty(ctx context.Context, markerID, recipeID string) {
	marker, author, r, ok, err := s.tastyFacts(ctx, markerID, recipeID)
	if err != nil {
		slog.WarnContext(ctx, "tasty notification failed", "recipe", recipeID, "err", err)
	}
	if !ok {
		return
	}
	s.mailTasty(ctx, marker, author, r)
	if s.pusher != nil {
		s.pushTasty(ctx, marker, author, r)
	}
}

// tastyFacts reads the marker, the recipe and its author; ok is false when
// the mark was taken back, the recipe or either account went, or the
// author is the marker.
func (s *Service) tastyFacts(ctx context.Context, markerID, recipeID string) (marker, author user.User, r sqlc.Recipe, ok bool, err error) {
	ids, err := json.Marshal([]string{recipeID})
	if err != nil {
		return marker, author, r, false, fmt.Errorf("marshal recipe id: %w", err)
	}
	marks, err := s.q.ListTastyRecipeIDs(ctx, sqlc.ListTastyRecipeIDsParams{UserID: markerID, RecipeIds: string(ids)})
	if err != nil {
		return marker, author, r, false, fmt.Errorf("list tasty marks: %w", err)
	}
	if len(marks) == 0 {
		return marker, author, r, false, nil
	}
	r, err = s.q.GetRecipe(ctx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		return marker, author, r, false, nil
	}
	if err != nil {
		return marker, author, r, false, fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	if r.CreatedBy == markerID {
		return marker, author, r, false, nil
	}
	for _, p := range []struct {
		u  *user.User
		id string
	}{{&marker, markerID}, {&author, r.CreatedBy}} {
		*p.u, err = s.users.ByID(ctx, p.id)
		if errors.Is(err, user.ErrNotFound) {
			return marker, author, r, false, nil
		}
		if err != nil {
			return marker, author, r, false, fmt.Errorf("get user %s: %w", p.id, err)
		}
	}
	return marker, author, r, true, nil
}

func (s *Service) mailTasty(ctx context.Context, marker, author user.User, r sqlc.Recipe) {
	if !mailable(author.Email, author.EmailVerified) {
		return
	}
	if !s.wanted(ctx, author.ID, KindTasty, ChannelMail) {
		return
	}
	sctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	err := s.mailer.SendTasty(sctx, mail.Tasty{
		To: author.Email, RecipeTitle: r.Title, RecipeURLPath: "/recipes/" + url.PathEscape(r.Slug),
		MarkerName: marker.DisplayName, MarkerColor: marker.Color, MarkerAvatar: s.picture(ctx, marker),
		Unsubscribe: s.UnsubscribeLinks(author.ID, KindTasty), Locale: author.Locale,
	})
	mail.LogUnsent(ctx, "tasty mail failed", author.ID, err)
}

func (s *Service) pushTasty(ctx context.Context, marker, author user.User, r sqlc.Recipe) {
	if !s.wanted(ctx, author.ID, KindTasty, ChannelPush) {
		return
	}
	title, err1 := mail.Text(author.Locale, "mail_push_tasty_title", nil)
	body, err2 := mail.Text(author.Locale, "mail_push_tasty_body", map[string]string{"name": marker.DisplayName, "title": r.Title})
	if err := errors.Join(err1, err2); err != nil {
		slog.WarnContext(ctx, "tasty push failed", "err", err)
		return
	}
	s.pusher.SendToUser(ctx, author.ID, push.Message{
		Kind: string(KindTasty), Title: title, Body: body,
		URL: "/recipes/" + url.PathEscape(r.Slug), Tag: "tasty:" + r.ID + ":" + marker.ID,
	})
}
