package mail

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
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
	catalogs   = loadCatalogs()
	layoutHTML = htmltemplate.Must(htmltemplate.ParseFS(templateFiles, "templates/layout.html.tmpl"))
	inviteText = texttemplate.Must(texttemplate.ParseFS(templateFiles, "templates/invite.txt.tmpl"))
	testText   = texttemplate.Must(texttemplate.ParseFS(templateFiles, "templates/test.txt.tmpl"))
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

// text returns key in locale with {placeholders} filled, falling back to the
// base locale for a locale or key the catalog lacks.
func text(locale user.Locale, key string, args map[string]string) string {
	s, ok := catalogs[string(locale)][key]
	if !ok {
		s = catalogs[string(user.BaseLocale)][key]
	}
	for k, v := range args {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// page is what layout.html.tmpl and the text templates read.
type page struct {
	Lang, Subject, Greeting, Body, Button, Validity, Fallback, Footer, URL, PublicURL string
}

// Invite is a setup link mailed to the person it was issued for.
type Invite struct {
	To, Name, Inviter, Path string
	Locale                  user.Locale
}

// BuildInvite renders the setup mail.
func BuildInvite(cfg Config, in Invite, publicURL string, now time.Time) (Envelope, error) {
	who := map[string]string{"inviter": in.Inviter, "name": in.Name}
	p := page{
		Lang:      string(in.Locale),
		Subject:   text(in.Locale, "mail_invite_subject", who),
		Greeting:  text(in.Locale, "mail_invite_greeting", who),
		Body:      text(in.Locale, "mail_invite_body", who),
		Button:    text(in.Locale, "mail_invite_button", nil),
		Validity:  text(in.Locale, "mail_invite_validity", nil),
		Fallback:  text(in.Locale, "mail_fallback_link", nil),
		Footer:    text(in.Locale, "mail_invite_ignore", who),
		URL:       publicURL + in.Path,
		PublicURL: publicURL,
	}
	return build(cfg, in.To, p, inviteText, now)
}

// BuildTest renders the owner's test mail.
func BuildTest(cfg Config, to string, locale user.Locale, publicURL string, now time.Time) (Envelope, error) {
	p := page{
		Lang:      string(locale),
		Subject:   text(locale, "mail_test_subject", nil),
		Body:      text(locale, "mail_test_body", nil),
		PublicURL: publicURL,
	}
	return build(cfg, to, p, testText, now)
}

func build(cfg Config, to string, p page, plain *texttemplate.Template, now time.Time) (Envelope, error) {
	var txt, html bytes.Buffer
	if err := plain.Execute(&txt, p); err != nil {
		return Envelope{}, fmt.Errorf("render text mail: %w", err)
	}
	if err := layoutHTML.Execute(&html, p); err != nil {
		return Envelope{}, fmt.Errorf("render html mail: %w", err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ ct, content string }{
		{"text/plain; charset=utf-8", txt.String()},
		{"text/html; charset=utf-8", html.String()},
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
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := cfg.From[strings.LastIndex(cfg.From, "@")+1:]
	var msg bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&msg, "%s: %s\r\n", k, v) }
	header("From", (&mail.Address{Name: cfg.FromName, Address: cfg.From}).String())
	header("To", (&mail.Address{Address: to}).String())
	header("Subject", mime.QEncoding.Encode("utf-8", p.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())
	return Envelope{To: to, Data: msg.Bytes()}, nil
}
