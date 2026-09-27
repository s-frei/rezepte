// Package preview decides what the link preview of a recipe link shared from
// the app shows to a crawler that is not signed in: the recipe's title,
// description and cover, for as long as the link's token lasts, or the app's
// own card. The owner's LinkPreviews setting switches it on and
// LinkPreviewMinutes sets the lifetime.
// See docs/memory/content/features/link-previews.mdx.
package preview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/settings"
)

// ErrNotFound is returned for a recipe that does not exist.
var ErrNotFound = errors.New("recipe not found")

// shareParam is the query parameter a share link carries its token in.
const shareParam = "share"

// descriptionRunes caps og:description: previews show two or three lines,
// and a crawler has no use for a whole recipe introduction.
const descriptionRunes = 200

// Service answers for link previews.
type Service struct {
	q        *sqlc.Queries
	settings *settings.Service
	images   *image.Service
	signer   *Signer
	logger   *slog.Logger
}

// NewService returns a Service signing with the stored key. It fails when
// the key is missing, so the service refuses to start rather than sign with
// an empty one.
func NewService(ctx context.Context, conn *sql.DB, st *settings.Service, images *image.Service, logger *slog.Logger) (*Service, error) {
	key, err := st.LinkPreviewKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("link previews: %w", err)
	}
	return &Service{q: sqlc.New(conn), settings: st, images: images, signer: NewSigner(key), logger: logger}, nil
}

// Link is the address to share a recipe by.
type Link struct {
	// Path is the recipe's path, with a token when link previews are on.
	Path string `json:"path" doc:"The recipe's path, carrying a token for its link preview when link previews are on"`
	// ExpiresAt is when the token stops showing the recipe; nil when the
	// path carries none.
	ExpiresAt *time.Time `json:"expiresAt" nullable:"true" doc:"When the share token stops showing the recipe in previews; null when the path carries none"`
}

// ShareLink returns the address to share the recipe recipeID by.
func (s *Service) ShareLink(ctx context.Context, recipeID string) (Link, error) {
	rec, err := s.q.GetRecipe(ctx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("get recipe: %w", err)
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return Link{}, err
	}
	link := Link{Path: recipePath(rec.Slug)}
	if st.LinkPreviews {
		token, expires := s.signer.Sign(rec.ID, time.Duration(st.LinkPreviewMinutes)*time.Minute)
		link.Path += "?" + shareParam + "=" + token
		link.ExpiresAt = &expires
	}
	return link, nil
}

// ForRequest returns the link preview for a recipe's page, or nil when the
// page is not a recipe, the recipe does not exist or the household does not
// show it to this request. It is the httpserver.PreviewFunc the app shell
// calls; a failure falls back to the app's own preview and is logged.
func (s *Service) ForRequest(r *http.Request) *httpserver.LinkPreview {
	slug, ok := strings.CutPrefix(r.URL.Path, "/recipes/")
	if !ok || slug == "" || strings.Contains(slug, "/") {
		return nil
	}
	p, err := s.forSlug(r.Context(), slug, r.URL.Query().Get(shareParam))
	if err != nil {
		s.logger.Warn("link preview", "path", r.URL.Path, "err", err)
		return nil
	}
	return p
}

func (s *Service) forSlug(ctx context.Context, slug, token string) (*httpserver.LinkPreview, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	if !st.LinkPreviews {
		return nil, nil
	}
	rec, err := s.q.GetRecipeBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get recipe: %w", err)
	}
	if !s.signer.Verify(rec.ID, token) {
		return nil, nil
	}
	query := "?" + shareParam + "=" + url.QueryEscape(token)
	p := &httpserver.LinkPreview{
		URL:         recipePath(rec.Slug) + query,
		Title:       rec.Title,
		Description: summary(rec.Description),
	}
	if rec.CoverImageID == nil {
		return p, nil
	}
	img, err := s.q.GetImage(ctx, sqlc.GetImageParams{RecipeID: rec.ID, ID: *rec.CoverImageID})
	if err != nil {
		return nil, fmt.Errorf("get cover: %w", err)
	}
	p.Image = coverPath(rec.ID, img.ID) + query
	p.ImageWidth, p.ImageHeight = image.ThumbSize(int(img.Width), int(img.Height))
	p.ImageType = "image/jpeg"
	p.ImageAlt = rec.Title
	return p, nil
}

// CoverHandler serves GET /link-preview/{recipeId}/{imageId}: the thumb of a
// recipe's cover, to a request carrying a valid token while link previews
// are on - without a session, since the crawler asking has none. Anything else is a 404, the
// same answer as for a recipe that does not exist, so the route tells a
// stranger nothing. Only the cover is served, never another photo.
func (s *Service) CoverHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recipeID, imageID := r.PathValue("recipeId"), r.PathValue("imageId")
		if !s.showsCover(r.Context(), recipeID, imageID, r.URL.Query().Get(shareParam)) {
			http.NotFound(w, r)
			return
		}
		f, err := s.images.Open(recipeID, imageID, "thumb")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		// A crawler fetches it once; nobody else should keep it around.
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeContent(w, r, "", info.ModTime(), f)
	})
}

func (s *Service) showsCover(ctx context.Context, recipeID, imageID, token string) bool {
	st, err := s.settings.Get(ctx)
	if err != nil {
		s.logger.Warn("link preview cover", "err", err)
		return false
	}
	if !st.LinkPreviews || !s.signer.Verify(recipeID, token) {
		return false
	}
	rec, err := s.q.GetRecipe(ctx, recipeID)
	if err != nil {
		return false
	}
	return rec.CoverImageID != nil && *rec.CoverImageID == imageID
}

func recipePath(slug string) string { return "/recipes/" + url.PathEscape(slug) }

func coverPath(recipeID, imageID string) string {
	return "/link-preview/" + url.PathEscape(recipeID) + "/" + url.PathEscape(imageID)
}

// summary folds a description onto one line and cuts it to descriptionRunes
// at a word boundary.
func summary(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= descriptionRunes {
		return s
	}
	cut := string([]rune(s)[:descriptionRunes])
	if i := strings.LastIndex(cut, " "); i > descriptionRunes/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}
