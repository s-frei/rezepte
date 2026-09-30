package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

type exportInput struct {
	Body struct {
		RecipeIDs []string `json:"recipeIds" minItems:"1" maxItems:"500" uniqueItems:"true" nullable:"false"`
	}
}

type importOutput struct {
	Body struct {
		Created []Created `json:"created"`
	}
}

type spooledKey struct{}

// InputValidator validates against the schema huma registered for
// recipe.Input, so an import enforces the same rules as POST /recipes.
// It must be called after recipe.Register, which registers that schema.
func InputValidator(api huma.API) Validator {
	registry := api.OpenAPI().Components.Schemas
	schema := registry.Schema(reflect.TypeFor[recipe.Input](), true, "")
	return func(raw map[string]any) []*huma.ErrorDetail {
		res := &huma.ValidateResult{}
		huma.Validate(registry, schema, huma.NewPathBuffer([]byte("body"), len("body")), huma.ModeWriteToServer, raw, res)
		out := make([]*huma.ErrorDetail, 0, len(res.Errors))
		for _, e := range res.Errors {
			var d *huma.ErrorDetail
			if errors.As(e, &d) {
				out = append(out, d)
			}
		}
		return out
	}
}

// Register adds the export and import operations. Both are for
// administrators only, whether the caller holds a session or a token.
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "export-recipes",
		Method:      http.MethodPost,
		Path:        "/api/v1/export",
		Summary:     "Export recipes with their photos as a zip",
		Tags:        []string{"transfer"},
		Security:    auth.Protected(auth.ScopeRecipesRead),
		Errors:      []int{403, 404, 422},
		Responses: map[string]*huma.Response{"200": {
			Description: "The zip, one folder per recipe",
			Content:     map[string]*huma.MediaType{"application/zip": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
		}},
	}, func(ctx context.Context, in *exportInput) (*huma.StreamResponse, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		f, err := svc.Export(ctx, in.Body.RecipeIDs)
		if errors.Is(err, recipe.ErrNotFound) {
			return nil, huma.Error404NotFound("recipe not found")
		}
		if err != nil {
			return nil, err
		}
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			defer os.Remove(f.Name())
			defer f.Close()
			r, w := humago.Unwrap(hctx)
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="rezepte-%s.zip"`, time.Now().Format(time.DateOnly)))
			// ServeContent sets the exact Content-Length from the file.
			http.ServeContent(w, r, "", time.Time{}, f)
		}}, nil
	})

	// The import input has neither Body nor RawBody, and the request body's
	// schema is a binary string, so huma never reads the body (verified in
	// v2.39.1: processInputType leaves hasInputBody false). spoolBody streams
	// it to a temp file instead of holding up to 1 GiB in memory.
	huma.Register(api, huma.Operation{
		OperationID:   "import-recipes",
		Method:        http.MethodPost,
		Path:          "/api/v1/import",
		Summary:       "Import recipes from an export zip (at most 1 GiB)",
		Tags:          []string{"transfer"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusCreated,
		RequestBody: &huma.RequestBody{
			Required: true,
			Content:  map[string]*huma.MediaType{"application/zip": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
		},
		Middlewares: huma.Middlewares{spoolBody(api, svc.tmpDir)},
		Errors:      []int{403, 413, 422},
	}, func(ctx context.Context, _ *struct{}) (*importOutput, error) {
		u, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		f, ok := ctx.Value(spooledKey{}).(*os.File)
		if !ok {
			return nil, huma.Error422UnprocessableEntity("empty body")
		}
		info, err := f.Stat()
		if err != nil {
			return nil, fmt.Errorf("stat import: %w", err)
		}
		created, err := svc.Import(ctx, u, f, info.Size())
		// errors.As, not a type switch: a failed rollback joins its delete
		// error to the *Error, and the file is still what was wrong.
		var te *Error
		if errors.As(err, &te) {
			return nil, huma.Error422UnprocessableEntity("the file cannot be imported", &huma.ErrorDetail{Location: te.Location, Message: te.Msg})
		}
		if err != nil {
			return nil, err
		}
		out := &importOutput{}
		out.Body.Created = created
		return out, nil
	})
}

// spoolBody copies the request body into a temp file, which archive/zip
// needs to seek in, and removes it after the operation. A declared length
// above the cap is refused before reading; http.MaxBytesReader stops a
// chunked or lying client.
func spoolBody(api huma.API, dir string) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// Refuse a non-admin before reading a byte, so a member (or a
		// member's token) cannot make the server spool 1 GiB. The handler
		// checks again; this is the cheap early exit. The API middleware
		// (auth.Middleware) has already run and put the caller in the context.
		u, ok := auth.UserFrom(ctx.Context())
		if !ok {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
			return
		}
		if !u.Role.IsAdmin() {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "admin role required")
			return
		}
		r, w := humago.Unwrap(ctx)
		if r.ContentLength > maxImportBytes {
			_ = huma.WriteErr(api, ctx, http.StatusRequestEntityTooLarge, "import larger than 1 GiB")
			return
		}
		f, err := os.CreateTemp(dir, "import-*.zip")
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "cannot store the upload")
			return
		}
		defer os.Remove(f.Name())
		defer f.Close()
		_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, maxImportBytes))
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			_ = huma.WriteErr(api, ctx, http.StatusRequestEntityTooLarge, "import larger than 1 GiB")
			return
		}
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusBadRequest, "cannot read the upload")
			return
		}
		next(huma.WithValue(ctx, spooledKey{}, f))
	}
}

// requireAdmin returns the caller or a 401/403, the same rule as userapi's.
// A token is its owner here, so a member's token is refused too.
func requireAdmin(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if !u.Role.IsAdmin() {
		return user.User{}, huma.Error403Forbidden("admin role required")
	}
	return u, nil
}
