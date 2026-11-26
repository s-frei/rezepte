//go:build unix

package pdf

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	stdimage "image"
	"image/jpeg"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/s-frei/rezepte/service/internal/pdf/typstbin"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

type pdfHarness struct {
	h       http.Handler
	svc     *Service
	recipes *recipe.Service
	images  *image.Service
	sam     user.User // admin
	capture string    // the last data.json the fake typst saw
	dataDir string
}

// newPDFHarness wires create-pdf like main.go, with a fake typst that saves
// data.json and writes a stub PDF, so no test here needs the real one.
func newPDFHarness(t *testing.T, publicURL string) *pdfHarness {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.Create(ctx, user.CreateParams{Username: "kim", Password: "pw", Role: user.RoleUser}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{}, httpserver.Options{
		APIMiddleware: auth.Middleware(sessions, tokens, false),
	})
	auth.Register(srv.API(), sessions, auth.NewDevice(make([]byte, 32)), false)
	dataDir := t.TempDir()
	imageDir := filepath.Join(dataDir, "images")
	recipes := recipe.NewService(conn, imageDir)
	// recipe's operations in the same API: a pdf type named like one of
	// recipe's (Ingredient, Step) panics here with "duplicate name".
	recipe.Register(srv.API(), recipes)
	images := image.NewService(conn, imageDir)

	bin := filepath.Join(t.TempDir(), "typst")
	capture := filepath.Join(t.TempDir(), "data.json")
	script := "#!/bin/sh\ncp \"$3/data.json\" " + capture + "\necho \"$@\" > " + capture + ".args\nfor a; do out=$a; done\nprintf '%%PDF-fake' > \"$out\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(dataDir, 1)
	r.recipe, r.book, r.bin = tree("x"), tree("x"), bin
	svc := NewService(r, recipes, images, settings.NewService(conn), publicURL, "1.2.3", true, slog.New(slog.DiscardHandler))
	Register(srv.API(), svc)
	return &pdfHarness{h: srv.Handler(), svc: svc, recipes: recipes, images: images, sam: sam, capture: capture, dataDir: dataDir}
}

func (p *pdfHarness) recipe(t *testing.T, title string) recipe.Recipe {
	t.Helper()
	r, err := p.recipes.Create(context.Background(), p.sam.ID, recipe.Input{Title: title, Servings: 4})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (p *pdfHarness) post(t *testing.T, username string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return p.postTo(t, "/api/v1/pdf", username, body)
}

func (p *pdfHarness) postTo(t *testing.T, path, username string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"`+username+`","password":"pw"}`))
	login.Header.Set("Content-Type", "application/json")
	login.Host, login.Header["Origin"] = "localhost:8060", []string{"http://localhost:8060"}
	lrec := httptest.NewRecorder()
	p.h.ServeHTTP(lrec, login)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host, req.Header["Origin"] = "localhost:8060", []string{"http://localhost:8060"}
	for _, c := range lrec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	p.h.ServeHTTP(rec, req)
	return rec
}

func (p *pdfHarness) data(t *testing.T) Data {
	t.Helper()
	b, err := os.ReadFile(p.capture)
	if err != nil {
		t.Fatal(err)
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func (p *pdfHarness) assertNoWorkDirs(t *testing.T) {
	t.Helper()
	if m, _ := filepath.Glob(filepath.Join(p.dataDir, "pdf-*")); len(m) > 0 {
		t.Errorf("left behind: %v", m)
	}
}

func client(id, title string) ClientRecipe {
	return ClientRecipe{ID: id, Title: title, Tags: []string{}, Servings: Servings{Value: 4, Original: 4, Display: "4"},
		IngredientGroups: []Group{}, Steps: []PDFStep{}}
}

func pdfRequest(bundle Bundle, recipes ...ClientRecipe) PDFRequest {
	req := PDFRequest{
		Origin: "http://localhost:8060", Locale: "en", Labels: Labels{RecipeFallback: "Recipe"},
		Options: Options{Bundle: bundle, Paper: PaperA4, Photo: true, Footer: true},
		Recipes: recipes,
	}
	if bundle == BundleBook {
		req.Book = &Book{Title: "Recipes", Date: "2026-11-25", DateDisplay: "Nov 25, 2026", CountDisplay: "2 recipes"}
	}
	return req
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestContentDispositionEncodesUTF8(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Käsespätzle")
	rec := p.post(t, "kim", marshal(t, pdfRequest(BundleSingle, client(r.ID, "Käsespätzle"))))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, `filename*=utf-8''K%C3%A4sesp%C3%A4tzle.pdf`) || !strings.Contains(cd, `filename="K_sesp_tzle.pdf"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" || !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Errorf("Content-Type %q, body %q", ct, rec.Body.String())
	}
	p.assertNoWorkDirs(t)
	if got := contentDisposition("rezepte-2026-11-25.pdf"); got != `attachment; filename="rezepte-2026-11-25.pdf"` {
		t.Errorf("ASCII name: %q", got)
	}
}

func TestPreviewPNG(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	rec := p.postTo(t, "/api/v1/pdf/preview", "kim", marshal(t, pdfRequest(BundleSingle, client(r.ID, "Soup"))))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" ||
		rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("%d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	args, err := os.ReadFile(p.capture + ".args")
	if err != nil || !strings.Contains(string(args), "--pages 1 --ppi 96 ") || !strings.HasSuffix(strings.TrimSpace(string(args)), "out.png") {
		t.Errorf("typst args %q: %v", args, err)
	}
	p.assertNoWorkDirs(t)

	for _, b := range []Bundle{BundleBook, BundleZip} {
		if rec := p.postTo(t, "/api/v1/pdf/preview", "sam", marshal(t, pdfRequest(b, client(r.ID, "Soup")))); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d: %s", b, rec.Code, rec.Body.String())
		}
	}
}

func TestBulkNamesAndTypes(t *testing.T) {
	p := newPDFHarness(t, "")
	a, b := p.recipe(t, "Soup"), p.recipe(t, "Soup")
	day := time.Now().Format(time.DateOnly)

	rec := p.post(t, "sam", marshal(t, pdfRequest(BundleBook, client(a.ID, "Soup"), client(b.ID, "Soup"))))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Disposition") != `attachment; filename="rezepte-`+day+`.pdf"` {
		t.Fatalf("book: %d %q %s", rec.Code, rec.Header().Get("Content-Disposition"), rec.Body.String())
	}
	if d := p.data(t); d.Book == nil || len(d.Recipes) != 2 {
		t.Errorf("book data: %+v", d.Book)
	}

	rec = p.post(t, "sam", marshal(t, pdfRequest(BundleZip, client(a.ID, "Soup"), client(b.ID, "Soup"))))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" ||
		rec.Header().Get("Content-Disposition") != `attachment; filename="rezepte-pdf-`+day+`.zip"` {
		t.Fatalf("zip: %d %v", rec.Code, rec.Header())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 2 || zr.File[1].Name != "Soup (2).pdf" {
		t.Errorf("zip entries: %v %v", err, zr)
	}
	p.assertNoWorkDirs(t)
}

func TestUnknownIDIs404BeforeRender(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	rec := p.post(t, "sam", marshal(t, pdfRequest(BundleBook, client(r.ID, "Soup"), client("00000000-0000-0000-0000-000000000000", "Gone"))))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(p.capture); err == nil {
		t.Error("typst ran for a request with an unknown recipe")
	}
	p.assertNoWorkDirs(t)
}

func TestMemberBulkIs403(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	for _, b := range []Bundle{BundleBook, BundleZip} {
		if rec := p.post(t, "kim", marshal(t, pdfRequest(b, client(r.ID, "Soup")))); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d: %s", b, rec.Code, rec.Body.String())
		}
	}
}

func TestSingleNeedsExactlyOne(t *testing.T) {
	p := newPDFHarness(t, "")
	a, b := p.recipe(t, "A"), p.recipe(t, "B")
	if rec := p.post(t, "sam", marshal(t, pdfRequest(BundleSingle, client(a.ID, "A"), client(b.ID, "B")))); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("two: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := p.post(t, "sam", marshal(t, pdfRequest(BundleSingle))); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("none: status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBookNeedsBook(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "A")
	req := pdfRequest(BundleBook, client(r.ID, "A"))
	req.Book = nil
	if rec := p.post(t, "sam", marshal(t, req)); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOriginMustBeHTTP(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	for _, origin := range []string{"javascript:alert(1)", "ftp://example.com", "https://example.com/path", ""} {
		req := pdfRequest(BundleSingle, client(r.ID, "Soup"))
		req.Origin = origin
		if rec := p.post(t, "kim", marshal(t, req)); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%q: status %d: %s", origin, rec.Code, rec.Body.String())
		}
	}
}

func TestUnknownPropertyIs422(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	var body map[string]any
	if err := json.Unmarshal(marshal(t, pdfRequest(BundleSingle, client(r.ID, "Soup"))), &body); err != nil {
		t.Fatal(err)
	}
	body["recipes"].([]any)[0].(map[string]any)["author"] = map[string]any{"displayName": "Mallory"}
	if rec := p.post(t, "kim", marshal(t, body)); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLocaleValidated(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	for locale, want := range map[string]int{"fr": http.StatusUnprocessableEntity, "": http.StatusUnprocessableEntity, "de": http.StatusOK} {
		req := pdfRequest(BundleSingle, client(r.ID, "Soup"))
		req.Locale = locale
		if rec := p.post(t, "kim", marshal(t, req)); rec.Code != want {
			t.Errorf("%q: status %d, want %d: %s", locale, rec.Code, want, rec.Body.String())
		}
	}
}

func TestPublicURLWins(t *testing.T) {
	for _, tc := range []struct{ publicURL, want string }{
		{"https://rezepte.example", "https://rezepte.example"},
		{"", "http://localhost:8060"},
	} {
		p := newPDFHarness(t, tc.publicURL)
		r := p.recipe(t, "Soup")
		c := client(r.ID, "Soup")
		if rec := p.post(t, "kim", marshal(t, pdfRequest(BundleSingle, c))); rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		d := p.data(t)
		if d.Instance.URL != tc.want || d.Recipes[0].URL != tc.want+"/recipes/"+r.Slug {
			t.Errorf("publicURL %q: instance %q, recipe %q", tc.publicURL, d.Instance.URL, d.Recipes[0].URL)
		}
		if d.Instance.Product != "Rezepte" || d.Generator.Version != "1.2.3" || d.Recipes[0].Author.DisplayName != r.CreatedBy.DisplayName ||
			d.Recipes[0].Slug != r.Slug || d.Book != nil {
			t.Errorf("server-owned data: %+v %+v", d.Instance, d.Recipes[0])
		}
	}
}

func TestCoverPhotoSentEvenWhenHidden(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, stdimage.NewRGBA(stdimage.Rect(0, 0, 120, 80)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.images.Upload(context.Background(), r.ID, p.sam, &buf); err != nil {
		t.Fatal(err)
	}
	req := pdfRequest(BundleSingle, client(r.ID, "Soup"))
	req.Options.Photo = false
	if rec := p.post(t, "kim", marshal(t, req)); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	ph := p.data(t).Recipes[0].Photo
	if ph == nil || ph.Path != "photos/0.jpg" || ph.Width != 120 || ph.Height != 80 {
		t.Errorf("photo = %+v", ph)
	}
}

func TestUnavailableIs503(t *testing.T) {
	if typstbin.Embedded() {
		t.Skip("the build embeds typst")
	}
	t.Setenv("PATH", t.TempDir()) // no typst anywhere
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	body := marshal(t, pdfRequest(BundleSingle, client(r.ID, "Soup")))

	// Missing at the first render: 503, and off from then on.
	p.svc.r.bin = ""
	if rec := p.post(t, "kim", body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if p.svc.Info().Available {
		t.Error("still available after typst went missing")
	}

	// Missing at startup: off from the start.
	if Available() {
		t.Error("available without any typst")
	}
	p.assertNoWorkDirs(t)
}

func TestAvailabilityFlipsAfterExecFailure(t *testing.T) {
	p := newPDFHarness(t, "")
	if err := os.Chmod(p.svc.r.bin, 0o600); err != nil { // EACCES, as on a noexec temp dir
		t.Fatal(err)
	}
	r := p.recipe(t, "Soup")
	body := marshal(t, pdfRequest(BundleSingle, client(r.ID, "Soup")))
	if rec := p.post(t, "kim", body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if p.svc.Info().Available {
		t.Fatal("still available after exec failed")
	}
	// Fixed on disk, still off: the flip lasts until a restart.
	if err := os.Chmod(p.svc.r.bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if rec := p.post(t, "kim", body); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("second request: status %d", rec.Code)
	}
	p.assertNoWorkDirs(t)
}

// TestBrowserShapeAccepted posts what frontend/src/lib/recipe/pdf-document.ts
// builds, key for key, so the two sides cannot drift apart unnoticed.
func TestBrowserShapeAccepted(t *testing.T) {
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Käsespätzle")
	body := `{"origin":"http://localhost:8060","locale":"de",
	"labels":{"servings":"Portionen","prep":"Vorbereitung","cook":"Kochen","total":"Gesamt","ingredients":"Zutaten","steps":"Zubereitung","source":"Quelle","addedBy":"Hinzugefügt von","contents":"Inhalt","recipeFallback":"Rezept","continued":"(Forts.)","nextPage":"Weiter auf der nächsten Seite","openRecipe":"Rezept öffnen","scaledFrom":"umgerechnet von {n}","scaledFromShort":"von {n}"},
	"options":{"bundle":"single","paper":"a5","photo":true,"footer":false},
	"book":null,
	"recipes":[{"id":"` + r.ID + `","title":"Käsespätzle","description":"","tags":["Abendessen"],
		"servings":{"value":6,"original":4,"display":"6"},
		"times":{"prep":{"minutes":20,"display":"20 Min."},"cook":null,"total":{"minutes":20,"display":"20 Min."}},
		"source":{"name":"Oma","url":null,"display":"Oma"},
		"ingredientGroups":[{"name":null,"ingredients":[
			{"quantity":{"value":1.5,"display":"1½"},"unit":"EL","name":"Butter","note":null,"display":"1½ EL Butter"},
			{"quantity":null,"unit":null,"name":"Salz","note":"nach Geschmack","display":"Salz"}]}],
		"steps":[{"number":1,"text":"Butter 10 Minuten bräunen.","segments":[
			{"kind":"ingredient","text":"Butter","amount":"1½ EL","minutes":null},
			{"kind":"text","text":" ","amount":null,"minutes":null},
			{"kind":"time","text":"10 Minuten","amount":null,"minutes":10},
			{"kind":"text","text":" bräunen.","amount":null,"minutes":null}]}]}]}`
	if rec := p.post(t, "kim", []byte(body)); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if d := p.data(t); d.Options.Paper != PaperA5 || d.Recipes[0].Servings.Value != 6 || d.Recipes[0].QR != nil {
		t.Errorf("options %+v, qr %+v", d.Options, d.Recipes[0].QR)
	}
}

// TestOtherEACCESIs500 proves that a permission error that is not typst's
// (here the work dir inside Render) neither answers 503 nor switches PDFs off.
func TestOtherEACCESIs500(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	p := newPDFHarness(t, "")
	r := p.recipe(t, "Soup")
	if err := os.Chmod(p.dataDir, 0o500); err != nil { // MkdirTemp(dataDir, "pdf-*") gets EACCES
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p.dataDir, 0o700) })
	if rec := p.post(t, "kim", marshal(t, pdfRequest(BundleSingle, client(r.ID, "Soup")))); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !p.svc.Info().Available {
		t.Error("a work-dir permission error switched PDFs off")
	}
}
