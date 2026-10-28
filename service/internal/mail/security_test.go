package mail

import (
	"mime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/user"
)

func TestMaskAddress(t *testing.T) {
	for in, want := range map[string]string{
		"mia@example.org": "m…@example.org",
		"m@x.org":         "m…@x.org",
		"nobody":          "…",
	} {
		if got := MaskAddress(in); got != want {
			t.Errorf("MaskAddress(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPasswordChangedNamesOrigin(t *testing.T) {
	for origin, phrase := range map[PasswordOrigin]string{
		OriginProfile:     "in your profile",
		OriginAdmin:       "Damson Dill",
		OriginResetLink:   "“forgot password” link",
		OriginSetupLink:   "Damson Dill",
		OriginCommandLine: "command line",
	} {
		admin := ""
		if origin == OriginAdmin || origin == OriginSetupLink {
			admin = "Damson Dill"
		}
		env, err := BuildPasswordChanged(cfg, PasswordChanged{To: "mia@example.org", Username: "mia", Admin: admin,
			Origin: origin, At: time.Date(2026, 10, 26, 18, 42, 0, 0, time.UTC), Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		msg, p, _ := parts(t, env)
		for _, part := range []string{p["text/plain"], p["text/html"]} {
			if !strings.Contains(part, phrase) || !strings.Contains(part, "mia") {
				t.Errorf("%s: part lacks %q: %s", origin, phrase, part)
			}
		}
		if !strings.Contains(p["text/plain"], "https://r.example.org/forgot-password") {
			t.Errorf("%s: no forgot-password link: %s", origin, p["text/plain"])
		}
		if msg.Header.Get("List-Unsubscribe") != "" {
			t.Errorf("%s: a security notice carries List-Unsubscribe", origin)
		}
	}
}

func TestEmailChangedMasksNewAddress(t *testing.T) {
	build := func(newAddress string) map[string]string {
		env, err := BuildEmailChanged(cfg, EmailChanged{To: "old@example.org", Username: "mia", NewAddress: newAddress,
			At: time.Unix(0, 0), Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		if env.To != "old@example.org" {
			t.Fatalf("to %s", env.To)
		}
		_, p, _ := parts(t, env)
		return p
	}
	for _, part := range build("mia@example.org") {
		if !strings.Contains(part, "m…@example.org") || strings.Contains(part, "mia@example.org") {
			t.Errorf("changed: %s", part)
		}
	}
	for _, part := range build("") {
		if !strings.Contains(part, "Your email address was removed") || strings.Contains(part, "changed") {
			t.Errorf("removed: %s", part)
		}
	}
}

func TestFirstPasswordSaysSet(t *testing.T) {
	env, err := BuildPasswordChanged(cfg, PasswordChanged{To: "mia@example.org", Username: "mia", Origin: OriginProfile,
		First: true, At: time.Unix(0, 0), Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got := msg.Header.Get("Subject"); got != "A password was set for your Rezepte account" {
		t.Errorf("subject = %q", got)
	}
	for _, part := range p {
		if !strings.Contains(part, "A password was set for your account mia in your profile") || strings.Contains(part, "was changed") {
			t.Errorf("part: %s", part)
		}
	}
}

// A first password through an origin without "set" copy still mails, in
// the "changed" wording.
func TestFirstPasswordElsewhereSaysChanged(t *testing.T) {
	for _, origin := range []PasswordOrigin{OriginResetLink, OriginCommandLine} {
		env, err := BuildPasswordChanged(cfg, PasswordChanged{To: "mia@example.org", Username: "mia", Origin: origin,
			First: true, At: time.Unix(0, 0), Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
		if err != nil {
			t.Fatalf("%s: %v", origin, err)
		}
		if msg, _, _ := parts(t, env); msg.Header.Get("Subject") != "Your Rezepte password was changed" {
			t.Errorf("%s: subject = %q", origin, msg.Header.Get("Subject"))
		}
	}
}

func TestIdentityLinkedNamesProviderAndAdmin(t *testing.T) {
	env, err := BuildIdentityLinked(cfg, IdentityLinked{To: "mia@example.org", Username: "mia", Admin: "Damson Dill",
		Provider: "Pocket ID", Origin: IdentitySetupLink, At: time.Unix(0, 0), Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if msg.Header.Get("List-Unsubscribe") != "" {
		t.Error("a security notice carries List-Unsubscribe")
	}
	for _, part := range p {
		for _, want := range []string{"Signing in with Pocket ID", "setup link from Damson Dill (admin)", "let Damson Dill know", "be unsubscribed"} {
			if !strings.Contains(part, want) {
				t.Errorf("part lacks %q: %s", want, part)
			}
		}
	}
}

// Every key a template or builder puts together at run time - per origin,
// reason or variant - exists in both catalogs, so no enum value renders a
// mail with a missing line (text fails the render instead).
func TestEveryBuiltKeyExists(t *testing.T) {
	at := time.Unix(0, 0)
	for _, locale := range []user.Locale{"en", "de"} {
		var builds []func() (Envelope, error)
		for _, origin := range []PasswordOrigin{OriginProfile, OriginAdmin, OriginResetLink, OriginSetupLink, OriginCommandLine} {
			for _, first := range []bool{false, true} {
				builds = append(builds, func() (Envelope, error) {
					return BuildPasswordChanged(cfg, PasswordChanged{To: "a@b.c", Origin: origin, First: first, At: at, Locale: locale}, "", at)
				})
			}
		}
		for _, reason := range []CommentReason{ReasonCreated, ReasonCommented} {
			builds = append(builds, func() (Envelope, error) {
				return BuildComment(cfg, Comment{To: "a@b.c", Reason: reason, At: at, Locale: locale}, "", at)
			})
		}
		for _, origin := range []IdentityOrigin{IdentityProfile, IdentitySetupLink} {
			builds = append(builds, func() (Envelope, error) {
				return BuildIdentityLinked(cfg, IdentityLinked{To: "a@b.c", Origin: origin, At: at, Locale: locale}, "", at)
			})
		}
		for _, addr := range []string{"", "n@b.c"} {
			for _, admin := range []string{"", "Dill"} {
				builds = append(builds, func() (Envelope, error) {
					return BuildEmailChanged(cfg, EmailChanged{To: "a@b.c", NewAddress: addr, Admin: admin, At: at, Locale: locale}, "", at)
				})
			}
		}
		for _, servings := range []int{1, 2} {
			for _, minutes := range []int{0, 30} {
				builds = append(builds, func() (Envelope, error) {
					r := DigestRecipe{Servings: servings, Minutes: minutes}
					return BuildRecipeDigest(cfg, RecipeDigest{To: "a@b.c", Recipes: []DigestRecipe{r, r}, Locale: locale}, "", at)
				})
			}
		}
		builds = append(builds, func() (Envelope, error) {
			return BuildRecipeDigest(cfg, RecipeDigest{To: "a@b.c", Recipes: []DigestRecipe{{Servings: 1}}, Locale: locale}, "", at)
		})
		for _, existing := range []bool{false, true} {
			builds = append(builds, func() (Envelope, error) {
				return BuildInvite(cfg, Invite{To: "a@b.c", Existing: existing, Locale: locale}, "", at)
			})
		}
		for i, b := range builds {
			if _, err := b(); err != nil {
				t.Errorf("%s build %d: %v", locale, i, err)
			}
		}
	}
	for _, locale := range []string{"en", "de"} {
		for k := range catalogs["en"] {
			if _, ok := catalogs[locale][k]; !ok {
				t.Errorf("%s lacks %s", locale, k)
			}
		}
	}
}

func TestMissingKeyFailsTheRender(t *testing.T) {
	if _, err := text("de", "mail_no_such_key", nil); err == nil {
		t.Error("text: no error for a missing key")
	}
	_, err := BuildPasswordChanged(cfg, PasswordChanged{To: "a@b.c", Origin: "nope", Locale: "en"}, "", time.Unix(0, 0))
	if err == nil {
		t.Error("an origin without copy rendered")
	}
}

// A value is never scanned for placeholders again.
func TestPlaceholdersFillInOnePass(t *testing.T) {
	got, err := text("en", "mail_comment_subject", map[string]string{"author": "{title}", "title": "{author}"})
	if err != nil || got != "{title} commented on “{author}”" {
		t.Errorf("text = %q, %v", got, err)
	}
}

// A long non-ASCII title is folded into encoded words of at most 75
// characters, one per line, and decodes back to the subject.
func TestLongSubjectIsFolded(t *testing.T) {
	title := strings.Repeat("ä", 200)
	env, err := BuildComment(cfg, Comment{To: "a@b.c", RecipeTitle: title, AuthorName: "Mia", Reason: ReasonCreated,
		At: time.Unix(0, 0), Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(env.Data), "\r\n\r\n")
	for line := range strings.SplitSeq(head, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("header line of %d characters", len(line))
		}
		w := strings.TrimPrefix(strings.TrimPrefix(line, "Subject: "), " ")
		if !strings.HasPrefix(w, "=?") {
			continue
		}
		if len(w) > 75 {
			t.Errorf("encoded word of %d characters: %s", len(w), w)
		}
		// Each word holds whole characters: decoded alone it is valid UTF-8.
		if dec, err := new(mime.WordDecoder).Decode(w); err != nil || !utf8.ValidString(dec) {
			t.Errorf("encoded word %s decodes to %q, %v", w, dec, err)
		}
	}
	msg, _, _ := parts(t, env)
	got, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || got != "Mia commented on “"+title+"”" {
		t.Errorf("subject = %q, %v", got, err)
	}
	if n := strings.Count(head, "\r\n =?"); n < 10 {
		t.Errorf("subject folded %d times", n)
	}
}

// /forgot-password never resets the owner, so the owner's notice has no
// button and points at the command line.
func TestOwnerPasswordNoticeHasNoButton(t *testing.T) {
	env, err := BuildPasswordChanged(cfg, PasswordChanged{To: "o@example.org", Username: "owner", Origin: OriginCommandLine,
		At: time.Unix(0, 0), Locale: "en", Owner: true}, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, p, _ := parts(t, env)
	for ct, part := range p {
		if strings.Contains(part, "forgot-password") || !strings.Contains(part, "--reset-superadmin-password") {
			t.Errorf("%s: %s", ct, part)
		}
	}
	// The flag never breaks at its hyphens on a phone.
	if !strings.Contains(p["text/html"], `<span style="white-space:nowrap;">--reset-superadmin-password</span>`) {
		t.Errorf("html: flag may break: %s", p["text/html"])
	}
}

func TestTemplateFuncsRefuseBadArguments(t *testing.T) {
	f := funcs("en")
	tr := f["t"].(func(string, ...string) (string, error))
	if _, err := tr("mail_reset_body", "username"); err == nil {
		t.Error("t with an odd argument count: no error")
	}
	dict := f["dict"].(func(...any) (map[string]any, error))
	if _, err := dict("text"); err == nil {
		t.Error("dict with an odd argument count: no error")
	}
	if _, err := dict(1, "x"); err == nil {
		t.Error("dict with a non-string key: no error")
	}
	if m, err := dict("text", "x"); err != nil || m["text"] != "x" {
		t.Errorf("dict = %v, %v", m, err)
	}
}
