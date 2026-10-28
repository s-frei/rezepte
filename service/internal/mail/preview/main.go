// Command preview renders every mail.Sample to out/ beside this file: per
// template variant a folder with the HTML and text part in each language and
// a page that shows them at a desktop and a phone width, plus an index.html
// listing them all. `mise run //service:mail-preview` runs it.
package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/s-frei/rezepte/service/internal/image"
	rmail "github.com/s-frei/rezepte/service/internal/mail"
)

type header struct{ Name, Value string }

// lang is one language of a mail: what the client shows and the text part.
type lang struct {
	Locale, Subject, To, Text string
	Headers                   []header
}

// letter is one template variant, with a page of its own at <Slug>/.
// Prev and Next are its neighbors in the order of the samples.
type letter struct {
	Slug, Template, Variant, Audience, OptOut string
	Roles                                     []string
	Langs                                     []lang
	Prev, Next                                *letter
}

// group is a template with its variants, section a kind of mail.
type group struct {
	Template, Audience, OptOut string
	Roles                      []string
	Mails                      []*letter
}

// section is a kind of mail; Note says what sets them apart.
type section struct {
	Name, Note string
	Count      int
	Groups     []*group
}

var notes = map[string]string{
	"Account":          "Links a person asks for or an admin sends.",
	"Security notices": "Sent whenever the way into an account changes. Nobody can switch them off.",
	"Notifications":    "What happens in the household. Each person switches them off under Settings, Notifications.",
	"Owner tools":      "For the owner, to check that mail works.",
}

// roles names the roles the way the preview shows them.
var roles = map[string]string{"superadmin": "owner", "admin": "admin", "user": "member"}

// The pages inline their style and script, so each stands alone under file://.
var (
	//go:embed preview.html.tmpl
	page string
	//go:embed preview.css
	style string
	//go:embed preview.js
	script string
)

var pages = template.Must(template.New("").Funcs(template.FuncMap{
	"css": func() template.CSS { return template.CSS(style) }, //nolint:gosec // G203: our own embedded file
	"js":  func() template.JS { return template.JS(script) },  //nolint:gosec // G203: our own embedded file
}).Parse(page))

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mail-preview:", err)
		os.Exit(1)
	}
}

func run() error {
	_, self, _, _ := runtime.Caller(0)
	out := filepath.Join(filepath.Dir(self), "out")
	if err := os.RemoveAll(out); err != nil {
		return fmt.Errorf("clear %s: %w", out, err)
	}
	// The comment sample shows a demo person's picture.
	picture, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "demo", "avatars", "mila.jpg"))
	if err != nil {
		return fmt.Errorf("read sample picture: %w", err)
	}
	// The digest samples show the first photo of the English demo recipes,
	// in directory order, made into mail photos as a real digest does.
	dirs, err := filepath.Glob(filepath.Join(filepath.Dir(self), "..", "..", "demo", "photos", "en", "*", "1.jpg"))
	if err != nil {
		return fmt.Errorf("find sample photos: %w", err)
	}
	var photos [][]byte
	for _, path := range dirs[:min(8, len(dirs))] {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("read sample photo: %w", err)
		}
		photo, err := image.MailPhoto(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		photos = append(photos, photo)
	}
	cfg := rmail.Config{From: "rezepte@example.org", FromName: "Rezepte"}
	now := time.Now()
	var sections []*section
	var order []*letter
	letters := map[string]*letter{}
	var locales []string
	for _, s := range rmail.Samples("https://rezepte.example", picture, photos) {
		slug, locale := s.Template+"."+s.Variant, string(s.Locale)
		env, err := s.Build(cfg, now)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", slug, locale, err)
		}
		l, parts, err := parse(env)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", slug, locale, err)
		}
		l.Locale, l.Text = locale, parts["text/plain"]
		files, err := themed(locale, parts["text/html"])
		if err != nil {
			return fmt.Errorf("%s.%s: %w", slug, locale, err)
		}
		files[locale+".txt"] = l.Text
		for name, content := range files {
			if err := write(filepath.Join(out, slug, name), []byte(content)); err != nil {
				return err
			}
		}
		m := letters[slug]
		if m == nil {
			var rs []string
			for _, r := range s.Roles {
				rs = append(rs, roles[string(r)])
			}
			m = &letter{Slug: slug, Template: s.Template, Variant: s.Variant, Audience: s.Audience, OptOut: s.OptOut, Roles: rs}
			letters[slug] = m
			if n := len(order); n > 0 {
				m.Prev, order[n-1].Next = order[n-1], m
			}
			order = append(order, m)
			if len(sections) == 0 || sections[len(sections)-1].Name != s.Section {
				sections = append(sections, &section{Name: s.Section, Note: notes[s.Section]})
			}
			sec := sections[len(sections)-1]
			if len(sec.Groups) == 0 || sec.Groups[len(sec.Groups)-1].Template != s.Template {
				g := &group{Template: s.Template, Audience: s.Audience, OptOut: s.OptOut, Roles: rs}
				sec.Groups = append(sec.Groups, g)
			}
			sec.Groups[len(sec.Groups)-1].Mails = append(sec.Groups[len(sec.Groups)-1].Mails, m)
			sec.Count++
		}
		m.Langs = append(m.Langs, l)
		if !slices.Contains(locales, locale) {
			locales = append(locales, locale)
		}
	}
	for slug, m := range letters {
		data := map[string]any{"Sections": sections, "Locales": locales, "Current": slug, "Mail": m}
		if err := render(filepath.Join(out, slug, "index.html"), "mail", data); err != nil {
			return err
		}
	}
	path := filepath.Join(out, "index.html")
	if err := render(path, "index", map[string]any{"Sections": sections, "Locales": locales, "Count": len(order)}); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

// darkQuery is how the layout switches to dark. A frame does not take its
// color scheme from the page around it, so the theme toggle swaps in copies
// that force the query on or off.
const darkQuery = "@media (prefers-color-scheme: dark)"

func themed(locale, html string) (map[string]string, error) {
	if !strings.Contains(html, darkQuery) {
		return nil, fmt.Errorf("html part lacks %q", darkQuery)
	}
	return map[string]string{
		locale + ".html":       html,
		locale + ".light.html": strings.ReplaceAll(html, darkQuery, "@media not all"),
		locale + ".dark.html":  strings.ReplaceAll(html, darkQuery, "@media all"),
	}, nil
}

func render(path, name string, data any) error {
	var b bytes.Buffer
	if err := pages.ExecuteTemplate(&b, name, data); err != nil {
		return fmt.Errorf("render %s: %w", path, err)
	}
	return write(path, b.Bytes())
}

func write(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// parse reads an envelope back the way a mail client would: it walks
// nested multiparts and shows inline pictures in the HTML part as data URLs,
// which a page can load where a cid: URL means nothing.
func parse(env rmail.Envelope) (lang, map[string]string, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(env.Data))
	if err != nil {
		return lang{}, nil, fmt.Errorf("read message: %w", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		return lang{}, nil, fmt.Errorf("decode subject: %w", err)
	}
	l := lang{Subject: subject, To: msg.Header.Get("To")}
	for _, h := range []string{"List-Unsubscribe", "List-Unsubscribe-Post"} {
		if v := msg.Header.Get(h); v != "" {
			l.Headers = append(l.Headers, header{h, v})
		}
	}
	parts, pictures := map[string]string{}, map[string]string{}
	if err := walk(msg.Header.Get("Content-Type"), msg.Body, parts, pictures); err != nil {
		return lang{}, nil, err
	}
	for cid, url := range pictures {
		parts["text/html"] = strings.ReplaceAll(parts["text/html"], "cid:"+cid, url)
	}
	return l, parts, nil
}

func walk(contentType string, body io.Reader, parts, pictures map[string]string) error {
	ct, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("content type: %w", err)
	}
	r := multipart.NewReader(body, params["boundary"])
	for {
		p, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("next part of %s: %w", ct, err)
		}
		pct, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if strings.HasPrefix(pct, "multipart/") {
			if err := walk(p.Header.Get("Content-Type"), p, parts, pictures); err != nil {
				return err
			}
			continue
		}
		var b []byte
		if p.Header.Get("Content-Transfer-Encoding") == "base64" {
			b, err = io.ReadAll(base64.NewDecoder(base64.StdEncoding, p))
		} else {
			b, err = io.ReadAll(p) // multipart decodes quoted-printable
		}
		if err != nil {
			return fmt.Errorf("read part: %w", err)
		}
		if cid := strings.Trim(p.Header.Get("Content-Id"), "<>"); cid != "" {
			pictures[cid] = "data:" + pct + ";base64," + base64.StdEncoding.EncodeToString(b)
			continue
		}
		parts[pct] = string(b)
	}
}
