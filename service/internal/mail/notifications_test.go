package mail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"strings"
	"testing"
	"time"
)

var sampleLinks = Unsubscribe{
	KindURL:     "https://r.example.org/unsubscribe?u=1&k=comment&s=t",
	GroupURL:    "https://r.example.org/unsubscribe?u=1&k=comment&s=t&scope=group",
	OneClickURL: "https://r.example.org/api/v1/unsubscribe/one-click?u=1&k=comment&s=t",
}

func comment(body string) Comment {
	return Comment{To: "olga@example.org", RecipeTitle: "Dal <3", RecipeURLPath: "/recipes/dal?tab=comments",
		AuthorName: "Mia", AuthorColor: "sage", Body: body, At: time.Unix(0, 0), Reason: ReasonCreated,
		Unsubscribe: sampleLinks, Locale: "en"}
}

func TestCommentMailEscapesBody(t *testing.T) {
	long := strings.Repeat("x", 200)
	env, err := BuildComment(cfg, comment("<b>x</b>\nline2 "+long), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, p, _ := parts(t, env)
	// Quoted-printable carries line breaks as CRLF.
	html, txt := strings.ReplaceAll(p["text/html"], "\r\n", "\n"), strings.ReplaceAll(p["text/plain"], "\r\n", "\n")
	if strings.Contains(html, "<b>x</b>") || !strings.Contains(html, "&lt;b&gt;x&lt;/b&gt;") {
		t.Errorf("body not escaped: %s", html)
	}
	if strings.Contains(html, "Dal <3") {
		t.Errorf("title not escaped: %s", html)
	}
	if !strings.Contains(html, "white-space:pre-line") || !strings.Contains(html, "&lt;/b&gt;\nline2") {
		t.Errorf("line break not kept: %s", html)
	}
	if !strings.Contains(html, long) {
		t.Fatalf("long word missing: %s", html)
	}
	if !strings.Contains(txt, "<b>x</b>\nline2") {
		t.Errorf("text part: %s", txt)
	}
	if !strings.Contains(txt, "https://r.example.org/recipes/dal?tab=comments") {
		t.Errorf("text part lacks the recipe link: %s", txt)
	}
}

func TestCommentMailHasUnsubscribe(t *testing.T) {
	env, err := BuildComment(cfg, comment("hi"), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	for _, part := range []string{p["text/plain"], p["text/html"]} {
		for _, want := range []string{"because this recipe is yours", "/unsubscribe?u=1", "scope=group"} {
			if !strings.Contains(part, want) {
				t.Errorf("part lacks %q: %s", want, part)
			}
		}
	}
	if got := msg.Header.Get("List-Unsubscribe"); got != "<"+sampleLinks.OneClickURL+">" {
		t.Errorf("List-Unsubscribe = %q", got)
	}
	if msg.Header.Get("List-Unsubscribe-Post") == "" {
		t.Error("no List-Unsubscribe-Post")
	}
}

// Headers are written only from OneClickURL; a line break there would
// inject one.
func TestOneClickURLRefusesLineBreaks(t *testing.T) {
	for _, u := range []string{"https://x/\r\nBcc: a@b.c", "https://x/\nX: y"} {
		l := letter{Name: "test", Lang: "en", SubjectKey: "mail_test_subject", Data: struct{ Body string }{"b"}, Unsubscribe: Unsubscribe{OneClickURL: u}}
		if _, err := build(cfg, "a@b.c", l, time.Now()); err == nil {
			t.Errorf("built with %q", u)
		}
	}
}

func TestCommentMailCarriesTheAuthorsPicture(t *testing.T) {
	in := comment("hi")
	in.AuthorAvatar = []byte(strings.Repeat("\xff\xd8jpeg", 40))
	env, err := BuildComment(cfg, in, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, pics := parts(t, env)
	ct, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if ct != "multipart/related" || params["type"] != "multipart/alternative" {
		t.Fatalf("Content-Type = %q", msg.Header.Get("Content-Type"))
	}
	html := p["text/html"]
	// The Content-ID shares the Message-ID's random part (RFC 2392: unique).
	id := strings.TrimSuffix(strings.TrimPrefix(msg.Header.Get("Message-Id"), "<"), "@example.org>")
	cid := "avatar." + id + "@example.org"
	if len(id) != 24 || !strings.Contains(html, `src="cid:`+cid+`"`) || strings.Contains(html, "ZgotmplZ") {
		t.Errorf("html does not show the picture %s: %s", cid, html)
	}
	pic, ok := pics[cid]
	if !ok || pic.contentType != "image/jpeg" {
		t.Errorf("pictures = %v", pics)
	}
	// The alternative part comes first: it is the related part's root.
	if bytes.Index(env.Data, []byte("Content-Id: <"+cid)) < bytes.Index(env.Data, []byte("text/html")) {
		t.Error("the picture comes before the html")
	}
	again, err := BuildComment(cfg, in, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(again.Data, []byte(cid)) {
		t.Errorf("two mails share the Content-ID %s", cid)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(pic.raw), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("base64 line of %d characters", len(line))
		}
	}
	got, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(pic.raw, "\r\n", ""))
	if err != nil || !bytes.Equal(got, in.AuthorAvatar) {
		t.Errorf("picture = %q, %v", got, err)
	}

	// Without a picture the mail stays multipart/alternative with the initial.
	env, err = BuildComment(cfg, comment("hi"), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	m, p, _ := parts(t, env)
	if !strings.HasPrefix(m.Header.Get("Content-Type"), "multipart/alternative") || strings.Contains(p["text/html"], "cid:") {
		t.Errorf("mail without a picture: %s", m.Header.Get("Content-Type"))
	}
}

func digestRecipe(n int) DigestRecipe {
	return DigestRecipe{Title: fmt.Sprintf("Dish %d", n), URLPath: fmt.Sprintf("/recipes/dish-%d", n),
		CreatorName: "Mia", CreatorColor: "sage", Photo: []byte("\xff\xd8photo"), Servings: 4, Minutes: 45}
}

func digest(recipes ...DigestRecipe) RecipeDigest {
	return RecipeDigest{To: "olga@example.org", Recipes: recipes, Unsubscribe: sampleLinks, Locale: "en"}
}

func TestRecipeDigestSingle(t *testing.T) {
	r := digestRecipe(1)
	r.Title, r.URLPath, r.Servings = "Dal", "/recipes/dal", 1
	r.CreatedAt = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	env, err := BuildRecipeDigest(cfg, digest(r), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "New recipe: Dal" {
		t.Errorf("Subject = %q", got)
	}
	html := p["text/html"]
	if n := strings.Count(html, ">View recipe</a>"); n != 1 {
		t.Errorf("%d buttons, want 1: %s", n, html)
	}
	for _, want := range []string{`href="https://r.example.org/recipes/dal"`, "1 serving", "45 min", "1 new recipe", "added on October 27, 2026"} {
		if !strings.Contains(html, want) {
			t.Errorf("html lacks %q: %s", want, html)
		}
	}
	if !strings.Contains(p["text/plain"], "https://r.example.org/recipes/dal") {
		t.Errorf("text part lacks the link: %s", p["text/plain"])
	}
}

func TestRecipeDigestCardsAndList(t *testing.T) {
	var recipes []DigestRecipe
	for n := 1; n <= 40; n++ {
		r := digestRecipe(n)
		// Two creators with a picture each, alternating.
		r.CreatorAvatar = []byte(fmt.Sprintf("\xff\xd8avatar%d", n%2))
		recipes = append(recipes, r)
	}
	env, err := BuildRecipeDigest(cfg, digest(recipes...), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "40 new recipes" {
		t.Errorf("Subject = %q", got)
	}
	html := p["text/html"]
	if n := strings.Count(html, `src="cid:photo.`); n != 6 {
		t.Errorf("%d photos, want 6", n)
	}
	if n := strings.Count(html, ">View recipe</a>"); n != 6 {
		t.Errorf("%d buttons, want 6", n)
	}
	for n := 7; n <= 36; n++ {
		if !strings.Contains(html, fmt.Sprintf(`href="https://r.example.org/recipes/dish-%d"`, n)) {
			t.Errorf("list lacks dish %d", n)
		}
	}
	if strings.Contains(html, "/recipes/dish-37") || !strings.Contains(html, "and 4 more") ||
		!strings.Contains(html, `href="https://r.example.org/recipes"`) {
		t.Errorf("tail beyond 36: %s", html)
	}
	// One inline part per photo and per distinct creator picture.
	if n := bytes.Count(env.Data, []byte("Content-Id: <photo.")); n != 6 {
		t.Errorf("%d photo parts, want 6", n)
	}
	if n := bytes.Count(env.Data, []byte("Content-Id: <avatar.")); n != 2 {
		t.Errorf("%d avatar parts, want 2", n)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(msg.Header.Get("Message-Id"), "<"), "@example.org>")
	if !strings.Contains(html, `src="cid:photo.0.`+id+`@example.org"`) || !strings.Contains(html, `src="cid:avatar.0.`+id+`@example.org"`) {
		t.Errorf("Content-IDs do not share the Message-ID %s", id)
	}
	if !strings.Contains(p["text/plain"], "and 4 more") || !strings.Contains(p["text/plain"], "/recipes/dish-36") {
		t.Errorf("text part: %s", p["text/plain"])
	}
}

func TestRecipeDigestWithoutPhoto(t *testing.T) {
	r := digestRecipe(1)
	r.Photo, r.Minutes = nil, 0
	env, err := BuildRecipeDigest(cfg, digest(r), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if !strings.HasPrefix(msg.Header.Get("Content-Type"), "multipart/alternative") || strings.Contains(p["text/html"], "cid:") {
		t.Errorf("mail without pictures: %s", msg.Header.Get("Content-Type"))
	}
	if strings.Contains(p["text/html"], " min") || !strings.Contains(p["text/html"], ">View recipe</a>") {
		t.Errorf("card without photo or time: %s", p["text/html"])
	}
}

func TestRecipeDigestUnsubscribe(t *testing.T) {
	env, err := BuildRecipeDigest(cfg, digest(digestRecipe(1)), "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got := msg.Header.Get("List-Unsubscribe"); got != "<"+sampleLinks.OneClickURL+">" {
		t.Errorf("List-Unsubscribe = %q", got)
	}
	if msg.Header.Get("List-Unsubscribe-Post") == "" {
		t.Error("no List-Unsubscribe-Post")
	}
	for _, part := range []string{p["text/plain"], p["text/html"]} {
		for _, want := range []string{"No more mails about new recipes", "/unsubscribe?u=1", "scope=group"} {
			if !strings.Contains(part, want) {
				t.Errorf("part lacks %q: %s", want, part)
			}
		}
	}
}

// The digest goes only to someone with recipes to read about.
func TestRecipeDigestRefusesAnEmptyList(t *testing.T) {
	if _, err := BuildRecipeDigest(cfg, digest(), "https://r.example.org", time.Unix(0, 0)); err == nil {
		t.Fatal("built a digest without recipes")
	}
}

// A creator whose account is gone has no name: the card names a former
// member, and the circle shows its initial, not U+FFFD.
func TestRecipeDigestFormerMember(t *testing.T) {
	r := digestRecipe(1)
	r.CreatorName = ""
	in := digest(r)
	in.Locale = "de"
	env, err := BuildRecipeDigest(cfg, in, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, p, _ := parts(t, env)
	html := p["text/html"]
	if !strings.Contains(html, "Ehemaliges Mitglied") || strings.Contains(html, "�") {
		t.Errorf("former member: %s", html)
	}
}

func TestCircleWithoutName(t *testing.T) {
	for _, name := range []string{"", "\xff"} {
		if _, _, initial := circle("sage", name); initial != "?" {
			t.Errorf("circle(%q) initial = %q", name, initial)
		}
	}
	if _, _, initial := circle("sage", "élise"); initial != "É" {
		t.Errorf("initial = %q", initial)
	}
}
