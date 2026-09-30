package share_test

import (
	"bytes"
	"context"
	"encoding/json"
	stdimage "image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/share"
	"github.com/s-frei/rezepte/service/internal/user"
)

// stack is the whole HTTP side of public shares wired the way main.go does
// it, over a household of an owner (olga), an admin (adam) and a member
// (mia), with public sharing switched on and one recipe of mia's.
type stack struct {
	h        http.Handler
	users    *user.Service
	tokens   *auth.TokenService
	settings *settings.Service
	recipes  *recipe.Service
	images   *image.Service
	shares   *share.Service
	owner    user.User
	admin    user.User
	member   user.User
	recipe   recipe.Recipe
	clock    time.Time
}

// shellPage is a stand-in for the built index.html: enough of app.html's
// link preview tags for the shell to have something to fill.
const shellPage = `<html><head><title>Rezepte</title>` +
	`<meta property="og:url" content="/"><meta property="og:title" content="Rezepte">` +
	`<meta property="og:description" content="Recipes"><meta property="og:image" content="/og.png">` +
	`</head><body>app</body></html>`

func newStack(t *testing.T) *stack {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	s := &stack{users: users, clock: time.Now().UTC()}
	for name, role := range map[string]user.Role{"olga": user.RoleSuperadmin, "adam": user.RoleAdmin, "mia": user.RoleUser} {
		u, err := users.Create(ctx, user.CreateParams{Username: name, Password: "pw", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		switch role {
		case user.RoleSuperadmin:
			s.owner = u
		case user.RoleAdmin:
			s.admin = u
		default:
			s.member = u
		}
	}
	s.settings = settings.NewService(conn)
	if _, err := s.settings.SetPublicShares(ctx, s.owner, true); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")
	s.recipes = recipe.NewService(conn, imageDir)
	s.images = image.NewService(conn, imageDir)
	s.shares = share.NewService(conn, s.settings, users)
	s.shares.SetClock(func() time.Time { return s.clock })
	s.recipe = s.newRecipe(t, "Asiatischer Gurkensalat")

	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	s.tokens = auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{"index.html": {Data: []byte(shellPage)}},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, s.tokens, false)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()),
		httpserver.WithLinkPreview(s.shares.ForRequest),
		httpserver.WithShellHeaders(share.ShellHeaders))
	auth.Register(srv.API(), sessions, false)
	share.Register(srv.API(), s.shares)
	share.RegisterPublic(srv.API(), s.shares, s.recipes, s.images)
	s.h = srv.Handler()
	return s
}

func (s *stack) newRecipe(t *testing.T, title string) recipe.Recipe {
	t.Helper()
	r, err := s.recipes.Create(context.Background(), s.member.ID, recipe.Input{
		Title: title, Description: "Knackig,  frisch\nund scharf.", Servings: 2,
		PrepMinutes: new(10), SourceURL: new("https://example.com/gurke"), Tags: []string{"Salat"},
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Gurke"}}}},
		Steps:            []recipe.Step{{Text: "Gurke schneiden.", References: []recipe.IngredientRef{{Word: "Gurke", IngredientName: "Gurke"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// upload adds a photo to recipeID as its author.
func (s *stack) upload(t *testing.T, recipeID string) recipe.Image {
	t.Helper()
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, 1200, 800))
	for y := range 800 {
		for x := range 1200 {
			img.Set(x, y, color.NRGBA{60, 140, 70, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	im, err := s.images.Upload(context.Background(), recipeID, s.member, &buf)
	if err != nil {
		t.Fatal(err)
	}
	return im
}

func (s *stack) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, req)
	return rec
}

func (s *stack) login(t *testing.T, u user.User) *http.Cookie {
	t.Helper()
	rec := s.do(http.MethodPost, "/api/v1/auth/login", `{"username":"`+u.Username+`","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", u.Username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

// create opens a public link to recipeID for u through the API and returns
// it.
func (s *stack) create(t *testing.T, cookie *http.Cookie, recipeID, body string) share.Share {
	t.Helper()
	rec := s.do(http.MethodPost, "/api/v1/recipes/"+recipeID+"/public-share", body, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	return decode[share.Share](t, rec)
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

func tokenOf(sh share.Share) string { return strings.TrimPrefix(sh.Path, "/s/") }

// TestShareOperationsAreSessionOnly: none of the management operations
// answers without a session, and an API token cannot reach any of them - a
// program handing out public links to the household's recipes is exactly
// what these operations must not allow. The middleware refuses a bearer
// token on a session-only operation with 403 before looking at it, the same
// answer every other session-only operation gives.
func TestShareOperationsAreSessionOnly(t *testing.T) {
	s := newStack(t)
	raw, _, err := s.tokens.Create(context.Background(), s.admin.ID, "t",
		[]string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite, auth.ScopeRecipesDelete, auth.ScopeUsersRead, auth.ScopeUsersWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ops := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/recipes/" + s.recipe.ID + "/public-share", `{"days":null}`},
		{http.MethodGet, "/api/v1/shares/by-recipe/" + s.recipe.ID, ""},
		{http.MethodGet, "/api/v1/shares", ""},
		{http.MethodGet, "/api/v1/shares?all=true", ""},
		{http.MethodDelete, "/api/v1/shares/x", ""},
		{http.MethodDelete, "/api/v1/shares", ""},
	}
	for _, op := range ops {
		if rec := s.do(op.method, op.path, op.body, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session: %d, want 401", op.method, op.path, rec.Code)
		}
		req := httptest.NewRequest(op.method, op.path, strings.NewReader(op.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+raw)
		rec := httptest.NewRecorder()
		s.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with an API token: %d, want 403: %s", op.method, op.path, rec.Code, rec.Body.String())
		}
	}
}

func TestCreatePublicShare(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	mia := s.login(t, s.member)
	path := "/api/v1/recipes/" + s.recipe.ID + "/public-share"

	created := s.create(t, mia, s.recipe.ID, `{"days":7}`)
	if !strings.HasPrefix(created.Path, "/s/") || len(tokenOf(created)) != 22 || created.Status != share.StatusActive ||
		created.ExpiresAt == nil || created.Recipe.ID != s.recipe.ID || created.Recipe.Slug != s.recipe.Slug {
		t.Errorf("created = %+v", created)
	}

	rec := s.do(http.MethodPost, path, `{"days":30}`, mia)
	if rec.Code != http.StatusConflict {
		t.Fatalf("create again: %d, want 409: %s", rec.Code, rec.Body.String())
	}
	conflict := decode[struct {
		Share share.Share `json:"share"`
	}](t, rec)
	if conflict.Share.ID != created.ID || conflict.Share.Path != created.Path {
		t.Errorf("409 carries %+v, want the existing share %+v", conflict.Share, created)
	}

	rec = s.do(http.MethodGet, "/api/v1/shares/by-recipe/"+s.recipe.ID, "", mia)
	mine := decode[struct {
		Share *share.Share `json:"share"`
	}](t, rec)
	if rec.Code != http.StatusOK || mine.Share == nil || mine.Share.ID != created.ID {
		t.Errorf("get my share: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "createdBy") {
		t.Errorf("a member's own share names its creator: %s", rec.Body.String())
	}
	// No link is a normal 200, not a 404: the recipe page calls this on
	// every view, and "nobody has shared this yet" is not an error.
	other := s.newRecipe(t, "Tomatensalat")
	rec = s.do(http.MethodGet, "/api/v1/shares/by-recipe/"+other.ID, "", mia)
	unshared := decode[struct {
		Share *share.Share `json:"share"`
	}](t, rec)
	if rec.Code != http.StatusOK || unshared.Share != nil {
		t.Errorf("get my share of a recipe I have not shared: %d %s, want 200 share:null", rec.Code, rec.Body.String())
	}
	rec = s.do(http.MethodGet, "/api/v1/shares/by-recipe/01a0dcd1-18a9-77b8-95a5-467da6963beb", "", mia)
	unknown := decode[struct {
		Share *share.Share `json:"share"`
	}](t, rec)
	if rec.Code != http.StatusOK || unknown.Share != nil {
		t.Errorf("get my share of an unknown recipe: %d %s, want 200 share:null", rec.Code, rec.Body.String())
	}

	// The creator's right, withdrawn by an admin, is read per request from
	// the database: the same session sees its share paused at once.
	if _, err := s.users.SetCanSharePublicly(ctx, s.admin, s.member.ID, false); err != nil {
		t.Fatal(err)
	}
	paused := decode[struct {
		Share *share.Share `json:"share"`
	}](t, s.do(http.MethodGet, "/api/v1/shares/by-recipe/"+s.recipe.ID, "", mia))
	if paused.Share == nil || paused.Share.Status != share.StatusPaused {
		t.Errorf("status after the right was withdrawn = %+v, want paused", paused.Share)
	}
	rec = s.do(http.MethodPost, "/api/v1/recipes/"+other.ID+"/public-share", `{"days":null}`, mia)
	if refusal := decode[share.RefusedError](t, rec); rec.Code != http.StatusForbidden || refusal.Reason != share.ReasonNotAllowed {
		t.Errorf("create while withdrawn: %d %s, want 403 reason %s", rec.Code, rec.Body.String(), share.ReasonNotAllowed)
	}
	if _, err := s.users.SetCanSharePublicly(ctx, s.admin, s.member.ID, true); err != nil {
		t.Fatal(err)
	}

	if _, err := s.settings.SetPublicShares(ctx, s.owner, false); err != nil {
		t.Fatal(err)
	}
	rec = s.do(http.MethodPost, "/api/v1/recipes/"+other.ID+"/public-share", `{"days":null}`, mia)
	if refusal := decode[share.RefusedError](t, rec); rec.Code != http.StatusForbidden || refusal.Reason != share.ReasonSharingOff {
		t.Errorf("create while sharing is off: %d %s, want 403 reason %s", rec.Code, rec.Body.String(), share.ReasonSharingOff)
	}
	if _, err := s.settings.SetPublicShares(ctx, s.owner, true); err != nil {
		t.Fatal(err)
	}

	if _, err := s.settings.SetShareLifetimes(ctx, s.owner, nil, new(30)); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"days":null}`, `{"days":365}`, `{"days":2}`} {
		if rec := s.do(http.MethodPost, "/api/v1/recipes/"+other.ID+"/public-share", body, mia); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("create with %s above a 30 day maximum: %d, want 422: %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := s.do(http.MethodPost, "/api/v1/recipes/"+other.ID+"/public-share", `{}`, mia); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("create without days: %d, want 422", rec.Code)
	}
	if rec := s.do(http.MethodPost, "/api/v1/recipes/01a0dcd1-18a9-77b8-95a5-467da6963beb/public-share", `{"days":7}`, mia); rec.Code != http.StatusNotFound {
		t.Errorf("create for an unknown recipe: %d, want 404: %s", rec.Code, rec.Body.String())
	}
	s.create(t, mia, other.ID, `{"days":30}`)
}

// TestGetMyPublicShareSchemaIsNullable checks the OpenAPI document says
// get-my-public-share's body.share may be null, matching the runtime answer
// for "no live link" - a plain 200, never a 404. huma emits OpenAPI 3.1,
// where nullability is a `"type": [..., "null"]` entry rather than a
// `nullable` keyword.
func TestGetMyPublicShareSchemaIsNullable(t *testing.T) {
	s := newStack(t)
	rec := s.do(http.MethodGet, "/api/v1/openapi.json", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("fetch openapi.json: %d %s", rec.Code, rec.Body.String())
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Type json.RawMessage `json:"type"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	share := doc.Components.Schemas["MyShareBody"].Properties["share"]
	if !bytes.Contains(share.Type, []byte(`"null"`)) {
		t.Errorf("get-my-public-share body.share type = %s, want it to include null", share.Type)
	}
}

func TestListShares(t *testing.T) {
	s := newStack(t)
	mia, adam := s.login(t, s.member), s.login(t, s.admin)
	mine := s.create(t, mia, s.recipe.ID, `{"days":null}`)
	s.create(t, adam, s.recipe.ID, `{"days":null}`)

	rec := s.do(http.MethodGet, "/api/v1/shares", "", mia)
	list := decode[struct{ Items []share.Share }](t, rec)
	if rec.Code != http.StatusOK || len(list.Items) != 1 || list.Items[0].ID != mine.ID {
		t.Errorf("member's own list: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "createdBy") {
		t.Errorf("own list names creators: %s", rec.Body.String())
	}

	if rec := s.do(http.MethodGet, "/api/v1/shares?all=true", "", mia); rec.Code != http.StatusForbidden {
		t.Errorf("member lists everyone's: %d, want 403", rec.Code)
	}

	rec = s.do(http.MethodGet, "/api/v1/shares?all=true", "", adam)
	all := decode[struct{ Items []share.Share }](t, rec)
	if rec.Code != http.StatusOK || len(all.Items) != 2 {
		t.Fatalf("admin lists everyone's: %d %s", rec.Code, rec.Body.String())
	}
	for _, sh := range all.Items {
		if sh.CreatedBy == nil || sh.CreatedBy.DisplayName == "" || sh.CreatedBy.Color == "" {
			t.Fatalf("admin list entry without its creator: %+v", sh)
		}
		want := s.admin.ID
		if sh.ID == mine.ID {
			want = s.member.ID
		}
		if sh.CreatedBy.ID != want {
			t.Errorf("creator of %s = %q, want %q", sh.ID, sh.CreatedBy.ID, want)
		}
	}
}

func TestRevokeShares(t *testing.T) {
	s := newStack(t)
	mia, adam := s.login(t, s.member), s.login(t, s.admin)
	mine := s.create(t, mia, s.recipe.ID, `{"days":null}`)
	admins := s.create(t, adam, s.recipe.ID, `{"days":null}`)

	if rec := s.do(http.MethodDelete, "/api/v1/shares/"+admins.ID, "", mia); rec.Code != http.StatusNotFound {
		t.Errorf("member revokes another's: %d, want 404", rec.Code)
	}
	if rec := s.do(http.MethodDelete, "/api/v1/shares/"+mine.ID, "", mia); rec.Code != http.StatusNoContent {
		t.Errorf("member revokes own: %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if rec := s.do(http.MethodDelete, "/api/v1/shares/"+mine.ID, "", mia); rec.Code != http.StatusNotFound {
		t.Errorf("revoke twice: %d, want 404", rec.Code)
	}
	again := s.create(t, mia, s.recipe.ID, `{"days":null}`)
	if rec := s.do(http.MethodDelete, "/api/v1/shares/"+again.ID, "", adam); rec.Code != http.StatusNoContent {
		t.Errorf("admin revokes a member's: %d, want 204", rec.Code)
	}

	s.create(t, mia, s.recipe.ID, `{"days":null}`)
	if rec := s.do(http.MethodDelete, "/api/v1/shares", "", mia); rec.Code != http.StatusForbidden {
		t.Errorf("member revokes all: %d, want 403", rec.Code)
	}
	if rec := s.do(http.MethodDelete, "/api/v1/shares", "", adam); rec.Code != http.StatusNoContent {
		t.Errorf("admin revokes all: %d, want 204", rec.Code)
	}
	if list := decode[struct{ Items []share.Share }](t, s.do(http.MethodGet, "/api/v1/shares?all=true", "", adam)); len(list.Items) != 0 {
		t.Errorf("after revoke all: %+v", list.Items)
	}
}
