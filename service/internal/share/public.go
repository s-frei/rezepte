package share

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/preview"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

// notAvailable is the one answer every public route gives for a link that
// serves nothing - unknown, revoked, paused, limited or expired alike - so a
// stranger holding a token learns nothing about why.
const notAvailable = "not available"

// notAvailableError is huma's ErrorModel under a type of its own. huma's
// schema link transformer recognizes a response body by its Go type, and a
// bare ErrorModel would get a "$schema" key and a Link header pointing at a
// schema route that needs a session - for the same reason get-public-recipe
// gives its 200 body inline. Its own type keeps the 404 identical to the one
// writeNotAvailable sends for a photo.
type notAvailableError struct{ huma.ErrorModel }

func errNotAvailable() error {
	return &notAvailableError{huma.ErrorModel{
		Title:  http.StatusText(http.StatusNotFound),
		Status: http.StatusNotFound,
		Detail: notAvailable,
	}}
}

// PublicImage is a photo of a publicly shared recipe. Its files are served at
// /public-images/{token}/{id}/{thumb|detail|original}.jpg.
type PublicImage struct {
	ID     string `json:"id"`
	Width  int    `json:"width" doc:"Width of the original variant in pixels"`
	Height int    `json:"height" doc:"Height of the original variant in pixels"`
}

// PublicRecipe is what a public link shows: the recipe's content and nothing
// about the household it lives in - no ids, people, permissions, favorites or
// timestamps. It is its own type, built field by field, never recipe.Recipe:
// a field added there must not reach a stranger by accident.
type PublicRecipe struct {
	Title            string                   `json:"title"`
	Description      string                   `json:"description"`
	Servings         int                      `json:"servings"`
	PrepMinutes      *int                     `json:"prepMinutes" nullable:"true"`
	CookMinutes      *int                     `json:"cookMinutes" nullable:"true"`
	SourceURL        *string                  `json:"sourceUrl" nullable:"true"`
	Tags             []string                 `json:"tags" nullable:"false"`
	IngredientGroups []recipe.IngredientGroup `json:"ingredientGroups" nullable:"false"`
	Steps            []recipe.Step            `json:"steps" nullable:"false"`
	Images           []PublicImage            `json:"images" nullable:"false"`
	CoverImageID     *string                  `json:"coverImageId" nullable:"true"`
}

// toPublic copies the recipe's content into a PublicRecipe.
func toPublic(r recipe.Recipe) PublicRecipe {
	images := make([]PublicImage, 0, len(r.Images))
	for _, im := range r.Images {
		images = append(images, PublicImage{ID: im.ID, Width: im.Width, Height: im.Height})
	}
	tags := r.Tags
	if tags == nil {
		tags = []string{}
	}
	groups := r.IngredientGroups
	if groups == nil {
		groups = []recipe.IngredientGroup{}
	}
	steps := r.Steps
	if steps == nil {
		steps = []recipe.Step{}
	}
	return PublicRecipe{
		Title:            r.Title,
		Description:      r.Description,
		Servings:         r.Servings,
		PrepMinutes:      r.PrepMinutes,
		CookMinutes:      r.CookMinutes,
		SourceURL:        r.SourceURL,
		Tags:             tags,
		IngredientGroups: groups,
		Steps:            steps,
		Images:           images,
		CoverImageID:     r.CoverImageID,
	}
}

type publicInput struct {
	Token string `path:"token" doc:"The token of the public link, the last segment of /s/{token}"`
}

type publicOutput struct {
	Body PublicRecipe
}

// publicHeaders is what every public route answers with, success or not: no
// Referer leaves the page (the token is in its address), search engines do
// not index it, and nothing caches it without asking again - a revoked link
// must stop showing the recipe at once.
var publicHeaders = [...][2]string{
	{"Referrer-Policy", "no-referrer"},
	{"X-Robots-Tag", "noindex"},
	{"Cache-Control", "private, no-cache"},
}

func setPublicHeaders(h http.Header) {
	for _, kv := range publicHeaders {
		h.Set(kv[0], kv[1])
	}
}

// ShellHeaders is the httpserver.ShellHeaderFunc for the public share page:
// the app shell for /s/... gets the public headers.
func ShellHeaders(r *http.Request, h http.Header) {
	if r.URL.Path == "/s" || strings.HasPrefix(r.URL.Path, "/s/") {
		setPublicHeaders(h)
	}
}

// RegisterPublic installs get-public-recipe, the recipe behind a public link.
// It declares no Security, so auth.Middleware lets it through without a
// session; the token is the whole authorization.
func RegisterPublic(api huma.API, svc *Service, recipes *recipe.Service) {
	// The body schema is given inline rather than as a $ref: huma's schema
	// link transformer adds a "$schema" field and a Link header to every
	// response whose schema is a $ref, which would put a key into the public
	// response that PublicRecipe does not declare, pointing at a schema
	// route that needs a session.
	body := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[PublicRecipe](), false, "")
	huma.Register(api, huma.Operation{
		OperationID: "get-public-recipe",
		Method:      http.MethodGet,
		Path:        "/api/v1/public/shares/{token}",
		Summary:     "Read the recipe behind a public link",
		Description: "No session needed. A link that is unknown, revoked, paused, limited by the maximum lifetime or expired answers 404 alike.",
		Tags:        []string{"public"},
		Errors:      []int{404},
		Responses: map[string]*huma.Response{
			"200": {Content: map[string]*huma.MediaType{"application/json": {Schema: body}}},
		},
		// A middleware rather than output headers, so the 404 carries them too.
		Middlewares: huma.Middlewares{func(ctx huma.Context, next func(huma.Context)) {
			for _, kv := range publicHeaders {
				ctx.SetHeader(kv[0], kv[1])
			}
			next(ctx)
		}},
	}, func(ctx context.Context, in *publicInput) (*publicOutput, error) {
		recipeID, ok, err := svc.Resolve(ctx, in.Token)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errNotAvailable()
		}
		r, err := recipes.ByID(ctx, recipeID)
		if errors.Is(err, recipe.ErrNotFound) {
			return nil, errNotAvailable()
		}
		if err != nil {
			return nil, err
		}
		return &publicOutput{Body: toPublic(r)}, nil
	})
}

// ImageHandler serves GET /public-images/{token}/{imageId}/{file}, where file
// is thumb.jpg, detail.jpg or original.jpg: a photo of the recipe a public
// link shows, while the link serves - without a session, since the stranger
// asking has none. A photo of another recipe, one removed from the recipe,
// an unknown variant and a link that serves nothing are the same 404.
func (s *Service) ImageHandler(images *image.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setPublicHeaders(w.Header())
		f, err := s.openPublicImage(r.Context(), images, r.PathValue("token"), r.PathValue("imageId"), r.PathValue("file"))
		if err != nil {
			if !errors.Is(err, errNotServed) {
				s.logger.Warn("public image", "err", err)
			}
			writeNotAvailable(w)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			writeNotAvailable(w)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeContent(w, r, "", info.ModTime(), f)
	})
}

// errNotServed is openPublicImage's answer for every request the link does
// not cover, as opposed to a failure worth logging.
var errNotServed = errors.New("not served")

func (s *Service) openPublicImage(ctx context.Context, images *image.Service, token, imageID, file string) (*os.File, error) {
	variant, ok := strings.CutSuffix(file, ".jpg")
	if !ok {
		return nil, errNotServed
	}
	recipeID, ok, err := s.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNotServed
	}
	// The row, not only the file: a photo removed from the recipe is gone
	// from the link even if its files have not been cleaned up yet.
	if _, err := s.q.GetImage(ctx, sqlc.GetImageParams{RecipeID: recipeID, ID: imageID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errNotServed
		}
		return nil, fmt.Errorf("get image %s: %w", imageID, err)
	}
	f, err := images.Open(recipeID, imageID, variant)
	if errors.Is(err, image.ErrNotFound) {
		return nil, errNotServed
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// writeNotAvailable answers with the problem+json 404 get-public-recipe
// gives, so every public route fails the same way.
func writeNotAvailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(errNotAvailable())
}

// ForRequest returns the link preview for the public share page /s/{token},
// or nil for any other page and for a link that serves nothing. It is the
// httpserver.PreviewFunc main.go puts ahead of the internal link previews; a
// failure falls back to the app's own preview and is logged.
func (s *Service) ForRequest(r *http.Request) *httpserver.LinkPreview {
	token, ok := strings.CutPrefix(r.URL.Path, "/s/")
	if !ok || token == "" || strings.Contains(token, "/") {
		return nil
	}
	p, err := s.previewFor(r.Context(), token)
	if err != nil {
		s.logger.Warn("public share link preview", "err", err)
		return nil
	}
	return p
}

func (s *Service) previewFor(ctx context.Context, token string) (*httpserver.LinkPreview, error) {
	recipeID, ok, err := s.Resolve(ctx, token)
	if err != nil || !ok {
		return nil, err
	}
	rec, err := s.q.GetRecipe(ctx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	p := &httpserver.LinkPreview{
		URL:         "/s/" + url.PathEscape(token),
		Title:       rec.Title,
		Description: preview.Summary(rec.Description),
	}
	if rec.CoverImageID == nil {
		return p, nil
	}
	img, err := s.q.GetImage(ctx, sqlc.GetImageParams{RecipeID: rec.ID, ID: *rec.CoverImageID})
	if err != nil {
		return nil, fmt.Errorf("get cover of %s: %w", rec.ID, err)
	}
	p.Image = "/public-images/" + url.PathEscape(token) + "/" + url.PathEscape(img.ID) + "/thumb.jpg"
	p.ImageWidth, p.ImageHeight = image.ThumbSize(int(img.Width), int(img.Height))
	p.ImageType = "image/jpeg"
	p.ImageAlt = rec.Title
	return p, nil
}
