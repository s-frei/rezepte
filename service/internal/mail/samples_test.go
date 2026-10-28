package mail

import (
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/user"
)

var sampleCfg = Config{From: "rezepte@example.org", FromName: "Rezepte"}

// A template without a sample never shows up in mail-preview, so nobody
// looks at it before it goes out.
func TestEveryTemplateHasASample(t *testing.T) {
	names, err := fs.Glob(templateFiles, "templates/*.html.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, s := range Samples("https://r.example.org", nil, nil) {
		have[s.Template+"/"+string(s.Locale)] = true
	}
	for _, n := range names {
		name := strings.TrimSuffix(strings.TrimPrefix(n, "templates/"), ".html.tmpl")
		if name == "layout" || name == "partials" {
			continue
		}
		for _, locale := range []user.Locale{"en", "de"} {
			if !have[name+"/"+string(locale)] {
				t.Errorf("template %s has no %s sample", name, locale)
			}
		}
	}
}

func TestSamplesRender(t *testing.T) {
	for _, s := range Samples("https://r.example.org", nil, nil) {
		t.Run(s.Template+"."+s.Variant+"."+string(s.Locale), func(t *testing.T) {
			if s.Audience == "" || s.Section == "" || len(s.Roles) == 0 {
				t.Error("no audience: add the template to audiences")
			}
			env, err := s.Build(sampleCfg, time.Date(2026, 10, 26, 18, 30, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			_, p, _ := parts(t, env)
			if p["text/plain"] == "" {
				t.Error("no text/plain part")
			}
			for ct, part := range p {
				for _, bad := range []string{"ZgotmplZ", "<no value>", `href=""`} {
					if strings.Contains(part, bad) {
						t.Errorf("%s contains %q: %s", ct, bad, part)
					}
				}
			}
			// One rule on the sheet's cell breaks a long word anywhere in the
			// mail - a title, a name, a link - so none widens it past a phone.
			if !strings.Contains(p["text/html"], `<td class="inner" style="padding:44px 48px 36px;word-break:break-word;overflow-wrap:anywhere;`) {
				t.Error("the sheet's cell does not break long words")
			}
			if !strings.Contains(p["text/html"], `<html lang="`+string(s.Locale)+`">`) {
				t.Errorf("html lacks lang %s: %q", s.Locale, p["text/html"])
			}
		})
	}
}

func TestUnsubscribeHeaders(t *testing.T) {
	const oneClick = "https://x/api/v1/unsubscribe/one-click?u=1&k=comment&s=t"
	l := letter{Name: "test", Lang: "en", SubjectKey: "mail_test_subject", Data: struct{ Body string }{"b"}}
	env, err := build(sampleCfg, "a@b.c", l, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, _, _ := parts(t, env)
	if got := msg.Header.Get("List-Unsubscribe") + msg.Header.Get("List-Unsubscribe-Post"); got != "" {
		t.Errorf("a letter without unsubscribe carries %q", got)
	}
	l.Unsubscribe = Unsubscribe{OneClickURL: oneClick}
	env, err = build(sampleCfg, "a@b.c", l, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, _, _ = parts(t, env)
	if got := msg.Header.Get("List-Unsubscribe"); got != "<"+oneClick+">" {
		t.Errorf("List-Unsubscribe = %q", got)
	}
	if got := msg.Header.Get("List-Unsubscribe-Post"); got != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %q", got)
	}
}
