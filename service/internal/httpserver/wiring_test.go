package httpserver_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

// TestEveryFeatureRegisters builds the whole API the way main.go does and
// renders the OpenAPI document. huma panics when two feature packages give
// different Go types the same schema name (e.g. two types called List), and
// it does so while generating the schema - so without this test such a
// clash only surfaces when the binary serves /api/v1/openapi.json, which no
// per-package handler test reaches because each registers a single feature.
//
// It lives in package httpserver_test (not httpserver) because the feature
// packages import httpserver in their own tests; an external test package
// keeps that from becoming an import cycle.
func TestEveryFeatureRegisters(t *testing.T) {
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	sessions := auth.NewService(conn, users)

	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{"index.html": {Data: []byte("app")}},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, cfg.SecureCookies)))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	imageDir := filepath.Join(t.TempDir(), "images")
	recipe.Register(srv.API(), recipe.NewService(conn, recipe.WithImageDir(imageDir)))
	images := image.NewService(conn, imageDir)
	image.Register(srv.API(), images)
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}",
		auth.RequireSession(sessions, cfg.SecureCookies)(image.FileHandler(images)))
	userapi.Register(srv.API(), users, sessions)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi.json: status %d, body %s", rec.Code, rec.Body.String())
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not JSON: %v", err)
	}
	ids := make(map[string]bool)
	for _, methods := range doc.Paths {
		for _, op := range methods {
			ids[op.OperationID] = true
		}
	}
	// One operation per feature package: proof that all four registrations
	// made it into the same document.
	for _, id := range []string{"login", "list-recipes", "upload-image", "list-users"} {
		if !ids[id] {
			t.Errorf("operation %q missing from the OpenAPI document", id)
		}
	}
}
