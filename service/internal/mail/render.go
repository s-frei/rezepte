package mail

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/s-frei/rezepte/service/internal/user"
)

//go:embed messages/*.json
var messageFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

var (
	catalogs                 = loadCatalogs()
	htmlLetters, textLetters = loadTemplates()
)

func loadCatalogs() map[string]map[string]string {
	out := map[string]map[string]string{}
	entries, err := messageFiles.ReadDir("messages")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		raw, err := messageFiles.ReadFile("messages/" + e.Name())
		if err != nil {
			panic(err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			panic(fmt.Errorf("mail catalog %s: %w", e.Name(), err))
		}
		out[strings.TrimSuffix(e.Name(), ".json")] = m
	}
	return out
}

// loadTemplates parses every templates/<name>.html.tmpl and its .txt.tmpl
// into a clone of the layouts, keyed by name. The funcs parsed in are stubs;
// render binds them to the letter's locale.
func loadTemplates() (map[string]*htmltemplate.Template, map[string]*texttemplate.Template) {
	stubs := funcs("")
	htmlBase := htmltemplate.Must(htmltemplate.New("layout.html.tmpl").Funcs(stubs).
		ParseFS(templateFiles, "templates/layout.html.tmpl", "templates/partials.html.tmpl"))
	textBase := texttemplate.Must(texttemplate.New("layout.txt.tmpl").Funcs(stubs).
		ParseFS(templateFiles, "templates/layout.txt.tmpl"))
	files, err := fs.Glob(templateFiles, "templates/*.html.tmpl")
	if err != nil {
		panic(err)
	}
	htmls := map[string]*htmltemplate.Template{}
	texts := map[string]*texttemplate.Template{}
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(f, "templates/"), ".html.tmpl")
		if name == "layout" || name == "partials" {
			continue
		}
		htmls[name] = htmltemplate.Must(htmltemplate.Must(htmlBase.Clone()).ParseFS(templateFiles, f))
		texts[name] = texttemplate.Must(texttemplate.Must(textBase.Clone()).ParseFS(templateFiles, "templates/"+name+".txt.tmpl"))
	}
	return htmls, texts
}

// funcs are the template funcs, bound to locale:
//
//	{{t "mail_invite_body" "inviter" .Data.Inviter}}  copy with {inviter} filled
//	{{datetime .Data.At}}                             a time in mail_datetime_format
//	{{dict "text" "Go" "url" .Data.URL}}              arguments for a partial
//	{{command (t "mail_..._owner")}}                  copy with the reset flag kept on one line
func funcs(locale user.Locale) map[string]any {
	return map[string]any{
		"command": command,
		"t": func(key string, kv ...string) (string, error) {
			if len(kv)%2 != 0 {
				return "", fmt.Errorf("t %s: odd number of arguments", key)
			}
			args := map[string]string{}
			for i := 0; i < len(kv); i += 2 {
				args[kv[i]] = kv[i+1]
			}
			return Text(locale, key, args)
		},
		"datetime": func(t time.Time) (string, error) {
			layout, err := Text(locale, "mail_datetime_format", nil)
			// time.Local is the server's TZ, UTC unless the deployment sets it.
			return t.In(time.Local).Format(layout), err
		},
		"dict": func(kv ...any) (map[string]any, error) {
			if len(kv)%2 != 0 {
				return nil, errors.New("dict: odd number of arguments")
			}
			m := map[string]any{}
			for i := 0; i < len(kv); i += 2 {
				k, ok := kv[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key %v is not a string", kv[i])
				}
				m[k] = kv[i+1]
			}
			return m, nil
		},
	}
}

// resetCommand is the owner's way back in, named in two notices.
const resetCommand = "rezepte --reset-superadmin-password"

// command escapes s for HTML and sets resetCommand in it as code whose flag
// never breaks at its hyphens: on a phone it would no longer copy as one
// word. Only the HTML part uses it.
func command(s string) htmltemplate.HTML {
	code := `<code style="font-family:Menlo,Consolas,monospace;font-size:13px;">rezepte <span style="white-space:nowrap;">--reset-superadmin-password</span></code>`
	return htmltemplate.HTML(strings.Replace(htmltemplate.HTMLEscapeString(s), resetCommand, code, 1)) //nolint:gosec // G203: s is escaped; code is a constant.
}

// FilterCatalog keeps the mail_* keys of a frontend catalog: the copy the
// templates read. UI copy about mail is named settings_mail_* to stay out.
func FilterCatalog(raw []byte) (map[string]string, error) {
	var all map[string]any
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	out := map[string]string{}
	for k, v := range all {
		if s, ok := v.(string); ok && strings.HasPrefix(k, "mail_") {
			out[k] = s
		}
	}
	return out, nil
}

// Text returns key in locale with {placeholders} filled, falling back to the
// base locale for a locale or key the catalog lacks. A key the base catalog
// lacks too is an error, never an empty line in a mail. The placeholders are
// filled in one pass, so a value holding "{title}" stays as typed.
func Text(locale user.Locale, key string, args map[string]string) (string, error) {
	s, ok := catalogs[string(locale)][key]
	if !ok {
		if s, ok = catalogs[string(user.BaseLocale)][key]; !ok {
			return "", fmt.Errorf("mail copy %q missing", key)
		}
	}
	pairs := make([]string, 0, 2*len(args))
	for k, v := range args {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(s), nil
}

// Unsubscribe holds a switchable mail's opt-out links; the zero value is
// none, which is what a security notice carries.
type Unsubscribe struct {
	KindURL, GroupURL, OneClickURL string
}

// letter is one mail to render: Name picks templates/<Name>.html.tmpl and
// <Name>.txt.tmpl, which read Data; the layouts read the rest. render fills
// Subject from SubjectKey and SubjectArgs.
type letter struct {
	Name        string
	Lang        string
	SubjectKey  string
	SubjectArgs map[string]string
	Subject     string
	PublicURL   string
	// ID is the random part of the Message-ID; build draws one when empty.
	// A builder that names inline pictures draws it first (newID), so
	// their Content-IDs share it.
	ID          string
	Data        any
	Unsubscribe Unsubscribe
	// Inline are pictures the HTML part shows through cid:<CID>.
	Inline []inline
}

// inline is a JPEG carried in the mail itself: a picture behind a login
// cannot be linked, and clients block remote pictures anyway.
type inline struct {
	CID  string
	JPEG []byte
}

func render(l letter) (subject, txt, html string, err error) {
	h, ok := htmlLetters[l.Name]
	if !ok {
		return "", "", "", fmt.Errorf("no mail template %q", l.Name)
	}
	if l.Subject, err = Text(user.Locale(l.Lang), l.SubjectKey, l.SubjectArgs); err != nil {
		return "", "", "", err
	}
	f := funcs(user.Locale(l.Lang))
	// Clone per render: the cached templates are never executed, so each
	// clone can take its own locale's funcs.
	hc, err := h.Clone()
	if err != nil {
		return "", "", "", fmt.Errorf("clone html mail: %w", err)
	}
	tc, err := textLetters[l.Name].Clone()
	if err != nil {
		return "", "", "", fmt.Errorf("clone text mail: %w", err)
	}
	var tb, hb bytes.Buffer
	if err := tc.Funcs(f).ExecuteTemplate(&tb, "layout.txt.tmpl", l); err != nil {
		return "", "", "", fmt.Errorf("render text mail: %w", err)
	}
	if err := hc.Funcs(f).ExecuteTemplate(&hb, "layout.html.tmpl", l); err != nil {
		return "", "", "", fmt.Errorf("render html mail: %w", err)
	}
	return l.Subject, tb.String(), hb.String(), nil
}

func build(cfg Config, to string, l letter, now time.Time) (Envelope, error) {
	if strings.ContainsAny(l.Unsubscribe.OneClickURL, "\r\n") {
		// It is the one value written into a header.
		return Envelope{}, errors.New("one-click unsubscribe URL holds a line break")
	}
	subject, txt, html, err := render(l)
	if err != nil {
		return Envelope{}, err
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	contentType := "multipart/alternative; boundary=" + mw.Boundary()
	for _, part := range []struct{ ct, content string }{
		{"text/plain; charset=utf-8", txt},
		{"text/html; charset=utf-8", html},
	} {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ct},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return Envelope{}, fmt.Errorf("mail part: %w", err)
		}
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(part.content)); err != nil {
			return Envelope{}, fmt.Errorf("mail part: %w", err)
		}
		if err := qp.Close(); err != nil {
			return Envelope{}, fmt.Errorf("mail part: %w", err)
		}
	}
	if err := mw.Close(); err != nil {
		return Envelope{}, fmt.Errorf("mail body: %w", err)
	}
	if len(l.Inline) > 0 {
		contentType, body, err = related(contentType, body, l.Inline)
		if err != nil {
			return Envelope{}, err
		}
	}
	id := l.ID
	if id == "" {
		id = newID()
	}
	var msg bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&msg, "%s: %s\r\n", k, v) }
	header("From", (&mail.Address{Name: cfg.FromName, Address: cfg.From}).String())
	header("To", (&mail.Address{Address: to}).String())
	header("Subject", foldSubject(subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+id+"@"+domainOf(cfg.From)+">")
	if u := l.Unsubscribe.OneClickURL; u != "" {
		// RFC 8058: a mail client may POST here to unsubscribe in one click.
		header("List-Unsubscribe", "<"+u+">")
		header("List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
	}
	header("MIME-Version", "1.0")
	header("Content-Type", contentType)
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())
	return Envelope{To: to, Data: msg.Bytes()}, nil
}

// newID is the random part of a Message-ID, and of the Content-IDs of the
// pictures in that mail: RFC 2392 wants both unique.
func newID() string {
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	return hex.EncodeToString(id)
}

func domainOf(addr string) string { return addr[strings.LastIndex(addr, "@")+1:] }

// foldSubject Q-encodes subject and puts each encoded word, at most 75
// characters, on a line of its own (RFC 2047, 5322): a long recipe title
// would otherwise make a header line too long for strict servers. An ASCII
// subject is not encoded; at its longest (a 200-character title) it stays
// under the 998-character limit.
func foldSubject(subject string) string {
	enc := mime.QEncoding.Encode("utf-8", subject)
	if enc == subject {
		return subject
	}
	return strings.ReplaceAll(enc, "?= =?", "?=\r\n =?")
}

// related wraps the alternative body and its pictures in multipart/related
// (RFC 2387), so the HTML part can show them.
func related(altType string, alt bytes.Buffer, pictures []inline) (string, bytes.Buffer, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	w, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {altType}})
	if err != nil {
		return "", body, fmt.Errorf("mail part: %w", err)
	}
	if _, err := w.Write(alt.Bytes()); err != nil {
		return "", body, fmt.Errorf("mail part: %w", err)
	}
	for _, p := range pictures {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {"image/jpeg"},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Id":                {"<" + p.CID + ">"},
			"Content-Disposition":       {"inline"},
		})
		if err != nil {
			return "", body, fmt.Errorf("mail picture: %w", err)
		}
		// RFC 2045 caps a base64 line at 76 characters.
		enc := base64.StdEncoding.EncodeToString(p.JPEG)
		for len(enc) > 0 {
			n := min(76, len(enc))
			if _, err := io.WriteString(w, enc[:n]+"\r\n"); err != nil {
				return "", body, fmt.Errorf("mail picture: %w", err)
			}
			enc = enc[n:]
		}
	}
	if err := mw.Close(); err != nil {
		return "", body, fmt.Errorf("mail body: %w", err)
	}
	return `multipart/related; type="multipart/alternative"; boundary=` + mw.Boundary(), body, nil
}
