package share

import (
	"context"
	"database/sql"
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

// errNotAvailable is every public route's 404, so no answer tells a
// stranger which case they hit.
func errNotAvailable() error { return httpserver.PublicNotFound(notAvailable) }

// PublicImage is a photo of a publicly shared recipe. Its files are served at
// /public-images/{token}/{id}/{thumb|detail|original}.jpg.
type PublicImage struct {
	ID     string `json:"id"`
	Width  int    `json:"width" doc:"Width of the original variant in pixels"`
	Height int    `json:"height" doc:"Height of the original variant in pixels"`
}

// PublicRecipe is what a public link shows: the recipe's content and nothing
// about the household it lives in - no ids, people, permissions, favorites or
// timestamps - plus whether the owner wants Rezepte named at the page's foot,
// since the page has no session to read the settings with. It is its own type, built field by field, never recipe.Recipe:
// a field added there must not reach a stranger by accident.
type PublicRecipe struct {
	Title            string                   `json:"title"`
	Description      string                   `json:"description"`
	Servings         int                      `json:"servings"`
	PrepMinutes      *int                     `json:"prepMinutes" nullable:"true"`
	CookMinutes      *int                     `json:"cookMinutes" nullable:"true"`
	SourceURL        *string                  `json:"sourceUrl" nullable:"true"`
	SourceName       *string                  `json:"sourceName" nullable:"true"`
	Tags             []string                 `json:"tags" nullable:"false"`
	IngredientGroups []recipe.IngredientGroup `json:"ingredientGroups" nullable:"false"`
	Steps            []recipe.Step            `json:"steps" nullable:"false"`
	Images           []PublicImage            `json:"images" nullable:"false"`
	CoverImageID     *string                  `json:"coverImageId" nullable:"true"`
	Attribution      bool                     `json:"attribution" doc:"Whether the page names Rezepte, with a link to the project, at its foot"`
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
		SourceName:       r.SourceName,
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

type publicImageInput struct {
	Token   string `path:"token" doc:"The token of the public link, the last segment of /s/{token}"`
	ImageID string `path:"imageId"`
	File    string `path:"file" doc:"thumb.jpg, detail.jpg or original.jpg"`
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
func RegisterPublic(api huma.API, svc *Service, recipes *recipe.Service, images *image.Service) {
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
		Middlewares: huma.Middlewares{withPublicHeaders},
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
		st, err := svc.settings.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("get settings: %w", err)
		}
		body := toPublic(r)
		body.Attribution = st.PublicShareAttribution
		return &publicOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-public-image-file",
		Method:      http.MethodGet,
		Path:        "/public-images/{token}/{imageId}/{file}",
		Summary:     "Read a photo of the recipe behind a public link",
		Description: "No session needed, like get-public-recipe. A photo of another recipe, one removed from the recipe, an unknown size and a link that serves nothing answer 404 alike.",
		Tags:        []string{"public"},
		Errors:      []int{404},
		Responses:   image.JPEGResponses,
		Middlewares: huma.Middlewares{withPublicHeaders},
	}, func(ctx context.Context, in *publicImageInput) (*huma.StreamResponse, error) {
		f, err := svc.openPublicImage(ctx, images, in.Token, in.ImageID, in.File)
		if err != nil {
			if !errors.Is(err, errNotServed) {
				svc.logger.Warn("public image", "err", err)
			}
			return nil, errNotAvailable()
		}
		// Cache-Control is publicHeaders' own.
		return image.JPEG(f, ""), nil
	})
}

// withPublicHeaders sets publicHeaders on every answer of a public
// operation. A middleware rather than output headers, so the 404 carries
// them too.
func withPublicHeaders(ctx huma.Context, next func(huma.Context)) {
	for _, kv := range publicHeaders {
		ctx.SetHeader(kv[0], kv[1])
	}
	next(ctx)
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
