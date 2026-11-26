//go:build unix

package pdf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/i18n"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/pdf/typstbin"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

// productName is instance.product: the product, never an operator's name.
const productName = "Rezepte"

// PDFRequest is the body of create-pdf: what the browser formats, never
// author, photo, URLs, instance or generator, which the server adds.
type PDFRequest struct { //nolint:revive // the prefix keeps the global API schema name specific
	Origin  string         `json:"origin" format:"uri" pattern:"^https?://[^/?#\\s]+$" doc:"The browser's location.origin; REZEPTE_PUBLIC_URL wins when set"`
	Locale  string         `json:"locale" doc:"One of the instance's interface languages"`
	Labels  Labels         `json:"labels"`
	Options Options        `json:"options"`
	Book    *Book          `json:"book" doc:"Title, recipe count and date of a cookbook; required for bundle book, ignored otherwise"`
	Recipes []ClientRecipe `json:"recipes" minItems:"1" maxItems:"500" nullable:"false"`
}

type createInput struct {
	Body PDFRequest
}

// Service turns a request into a rendered file. It also owns whether PDFs
// work at all, which the settings response reports.
type Service struct {
	r         *Renderer
	recipes   *recipe.Service
	images    *image.Service
	settings  *settings.Service
	publicURL string
	version   string
	log       *slog.Logger
	available atomic.Bool
}

// Available reports whether a typst exists: embedded, or on PATH. It does
// not run it; an extraction or exec failure shows on the first render.
func Available() bool {
	if typstbin.Embedded() {
		return true
	}
	_, err := exec.LookPath("typst")
	return err == nil
}

// NewService serves PDFs with r, after LoadTemplates. publicURL is
// REZEPTE_PUBLIC_URL ("" when unset), available what startup found
// (Available and the template probe).
func NewService(r *Renderer, recipes *recipe.Service, images *image.Service, instance *settings.Service, publicURL, version string, available bool, log *slog.Logger) *Service {
	s := &Service{r: r, recipes: recipes, images: images, settings: instance, publicURL: publicURL, version: version, log: log}
	s.available.Store(available)
	return s
}

// Info is the pdf part of the settings response.
func (s *Service) Info() settings.PDFInfo {
	return settings.PDFInfo{Available: s.available.Load()}
}

// Register installs create-pdf, the file to keep, and preview-pdf, the
// image of its page 1 that the PDF tab shows. Both render the same request.
func Register(api huma.API, s *Service) {
	binary := &huma.Schema{Type: "string", Format: "binary"}
	op := func(id, path, summary, description, answer string, content map[string]*huma.MediaType) huma.Operation {
		return huma.Operation{
			OperationID:  id,
			Method:       http.MethodPost,
			Path:         path,
			Summary:      summary,
			Description:  description,
			Tags:         []string{"recipes"},
			Security:     auth.Protected(auth.ScopeRecipesRead),
			MaxBodyBytes: 16 << 20,
			Errors:       []int{403, 404, 422, 503},
			Responses:    map[string]*huma.Response{"200": {Description: answer, Content: content}},
		}
	}
	huma.Register(api, op("create-pdf", "/api/v1/pdf",
		"Render recipes as a PDF, a cookbook or a zip of PDFs",
		"Bundle single takes exactly one recipe and is open to every member; book and zip are for administrators.",
		"A PDF for single and book, a zip of one PDF per recipe for zip, as a download",
		map[string]*huma.MediaType{"application/pdf": {Schema: binary}, "application/zip": {Schema: binary}},
	), func(ctx context.Context, in *createInput) (*huma.StreamResponse, error) {
		res, name, err := s.create(ctx, &in.Body, false)
		if err != nil {
			return nil, err
		}
		return serve(res, contentDisposition(name)), nil
	})
	huma.Register(api, op("preview-pdf", "/api/v1/pdf/preview",
		"Render page 1 of a recipe's PDF as an image",
		"Takes the same body as create-pdf with bundle single and renders the same PDF, for a preview of it.",
		"Page 1 of the PDF as a PNG at 96 ppi",
		map[string]*huma.MediaType{"image/png": {Schema: binary}},
	), func(ctx context.Context, in *createInput) (*huma.StreamResponse, error) {
		res, _, err := s.create(ctx, &in.Body, true)
		if err != nil {
			return nil, err
		}
		return serve(res, ""), nil
	})
}

// serve streams res and removes it afterwards; disposition "" sends none.
func serve(res *Result, disposition string) *huma.StreamResponse {
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		defer res.Close()
		f, err := os.Open(res.Path)
		if err != nil {
			hctx.SetStatus(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		r, w := humago.Unwrap(hctx)
		w.Header().Set("Content-Type", res.ContentType)
		if disposition != "" {
			w.Header().Set("Content-Disposition", disposition)
		}
		// ServeContent sets the exact Content-Length from the file.
		http.ServeContent(w, r, "", time.Time{}, f)
	}}
}

// create checks the request, loads every recipe before rendering anything,
// adds the server-owned data and renders it: the file, or with preview the
// image of its page 1.
func (s *Service) create(ctx context.Context, req *PDFRequest, preview bool) (*Result, string, error) {
	if !s.available.Load() {
		return nil, "", huma.Error503ServiceUnavailable("PDF rendering is unavailable on this instance")
	}
	bundle := req.Options.Bundle
	if bundle == BundleSingle && len(req.Recipes) != 1 {
		return nil, "", huma.Error422UnprocessableEntity("bundle single takes exactly one recipe")
	}
	if preview && bundle != BundleSingle {
		return nil, "", huma.Error422UnprocessableEntity("a preview takes bundle single")
	}
	if bundle != BundleSingle {
		if _, err := auth.RequireRole(ctx, user.RoleAdmin); err != nil {
			return nil, "", err
		}
	}
	if bundle == BundleBook && req.Book == nil {
		return nil, "", huma.Error422UnprocessableEntity("bundle book needs a book")
	}
	if bundle != BundleBook {
		req.Book = nil
	}
	if !slices.Contains(i18n.Locales(), req.Locale) {
		return nil, "", huma.Error422UnprocessableEntity("unknown locale " + strconv.Quote(req.Locale))
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return nil, "", err
	}
	base := req.Origin
	if s.publicURL != "" {
		base = s.publicURL
	}
	d := Data{
		SchemaVersion: SchemaVersion,
		Generator:     Generator{Name: "rezepte", Version: s.version},
		GeneratedAt:   time.Now().UTC(),
		Locale:        req.Locale,
		Options:       req.Options,
		Instance:      Instance{Product: productName, URL: base, Attribution: st.PublicShareAttribution},
		Labels:        req.Labels,
		Book:          req.Book,
		Recipes:       make([]Recipe, 0, len(req.Recipes)),
	}
	photos := map[string]string{}
	for i, c := range req.Recipes {
		rec, err := s.recipes.ByID(ctx, c.ID)
		if errors.Is(err, recipe.ErrNotFound) {
			return nil, "", huma.Error404NotFound("recipe " + c.ID + " not found")
		}
		if err != nil {
			return nil, "", err
		}
		photo, src, err := s.cover(rec, i)
		if err != nil {
			return nil, "", err
		}
		if photo != nil {
			photos[photo.Path] = src
		}
		d.Recipes = append(d.Recipes, Merge(c, ServerRecipe{
			Slug: rec.Slug, URL: base + "/recipes/" + rec.Slug,
			Author:    Author{DisplayName: rec.CreatedBy.DisplayName},
			CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt, Photo: photo,
		}))
	}
	render := s.r.Render
	if preview {
		render = s.r.Preview
	}
	res, err := render(ctx, d, photos)
	if errors.Is(err, ErrCannotRun) {
		if s.available.CompareAndSwap(true, false) {
			s.log.Warn("pdf: typst unavailable, PDF downloads are off until a restart", "err", err)
		}
		return nil, "", huma.Error503ServiceUnavailable("PDF rendering is unavailable on this instance")
	}
	if err != nil {
		return nil, "", fmt.Errorf("render pdf: %w", err)
	}
	day := time.Now().Format(time.DateOnly)
	switch bundle {
	case BundleBook:
		return res, "rezepte-" + day + ".pdf", nil
	case BundleZip:
		return res, "rezepte-pdf-" + day + ".zip", nil
	default:
		return res, FileName(d.Recipes[0].Title, d.Labels.RecipeFallback, map[string]int{}), nil
	}
}

// cover returns the recipe's cover (detail variant) as photos/<i>.jpg with
// its source file, or nil when it has none. The template gets it whenever
// one exists; options.photo says whether to show it.
func (s *Service) cover(rec recipe.Recipe, i int) (*Photo, string, error) {
	if rec.CoverImageID == nil {
		return nil, "", nil
	}
	f, err := s.images.Open(rec.ID, *rec.CoverImageID, "detail")
	if errors.Is(err, image.ErrNotFound) {
		return nil, "", nil // a lost file prints without a photo
	}
	if err != nil {
		return nil, "", err
	}
	_ = f.Close()
	p := &Photo{Path: "photos/" + strconv.Itoa(i) + ".jpg"}
	for _, im := range rec.Images {
		if im.ID == *rec.CoverImageID {
			p.Width, p.Height, p.Focus = im.Width, im.Height, Focus{X: im.Focus.X, Y: im.Focus.Y}
		}
	}
	return p, f.Name(), nil
}

// contentDisposition names the download: a quoted ASCII filename every
// client reads, plus RFC 6266's UTF-8 filename* when the name needs it.
func contentDisposition(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	v := `attachment; filename="` + ascii + `"`
	if ascii != name {
		v += strings.TrimPrefix(mime.FormatMediaType("attachment", map[string]string{"filename": name}), "attachment")
	}
	return v
}
