package mail

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"
)

var cfg = Config{Host: "h", Port: 25, Security: SecurityNone, From: "rezepte@example.org", FromName: "Rezepte"}

// picture is an inline part as it travels: its media type and its body
// still base64-encoded.
type picture struct{ contentType, raw string }

// parts reads env like a mail client: its body parts by media type and its
// inline pictures by Content-ID.
func parts(t *testing.T, env Envelope) (*mail.Message, map[string]string, map[string]picture) {
	t.Helper()
	msg, err := mail.ReadMessage(bytes.NewReader(env.Data))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	out, pics := map[string]string{}, map[string]picture{}
	readParts(t, msg.Body, params["boundary"], out, pics)
	return msg, out, pics
}

// readParts reads each part below boundary into out by its media type, or
// into pics by its Content-ID, walking into nested multiparts.
func readParts(t *testing.T, body io.Reader, boundary string, out map[string]string, pics map[string]picture) {
	t.Helper()
	r := multipart.NewReader(body, boundary)
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		ct, ps, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if strings.HasPrefix(ct, "multipart/") {
			// multipart/related wraps the alternative part in a mail
			// with inline pictures.
			readParts(t, p, ps["boundary"], out, pics)
			continue
		}
		b, _ := io.ReadAll(p) // multipart decodes quoted-printable for us
		if cid := strings.Trim(p.Header.Get("Content-Id"), "<>"); cid != "" {
			pics[cid] = picture{ct, string(b)}
			continue
		}
		out[ct] = string(b)
	}
}

func TestInviteMessage(t *testing.T) {
	env, err := BuildInvite(cfg, Invite{To: "lena@example.org", Name: "Lena", Inviter: "Samuel",
		Path: "/welcome#tok", Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "Samuel invited you to Rezepte" {
		t.Errorf("Subject = %q", got)
	}
	if msg.Header.Get("Date") == "" || msg.Header.Get("Message-Id") == "" {
		t.Error("Date or Message-ID missing")
	}
	if !strings.Contains(msg.Header.Get("From"), "rezepte@example.org") {
		t.Errorf("From = %q", msg.Header.Get("From"))
	}
	for _, ct := range []string{"text/plain", "text/html"} {
		if !strings.Contains(p[ct], "https://r.example.org/welcome#tok") {
			t.Errorf("%s lacks the link: %q", ct, p[ct])
		}
	}
	if !strings.Contains(p["text/html"], "Set up account") {
		t.Error("html lacks the button")
	}
}

func TestInviteMessageGerman(t *testing.T) {
	env, _ := BuildInvite(cfg, Invite{To: "a@b.c", Name: "Lena", Inviter: "Samuel", Path: "/welcome#t", Locale: "de"}, "https://r.example.org", time.Now())
	msg, p, _ := parts(t, env)
	got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if got != "Samuel hat dich zu Rezepte eingeladen" {
		t.Errorf("Subject = %q", got)
	}
	if !strings.Contains(p["text/plain"], "7 Tage gültig") {
		t.Errorf("plain = %q", p["text/plain"])
	}
}

func TestInviteMessageEncodesAndEscapes(t *testing.T) {
	env, _ := BuildInvite(cfg, Invite{To: "a@b.c", Name: "<b>Ö&</b>", Inviter: "Jürgen Groß", Path: "/welcome#t", Locale: "de"}, "https://r.example.org", time.Now())
	raw := string(env.Data)
	if strings.Contains(strings.SplitN(raw, "\r\n\r\n", 2)[0], "Jürgen") {
		t.Error("subject is not RFC 2047 encoded")
	}
	msg, p, _ := parts(t, env)
	got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if got != "Jürgen Groß hat dich zu Rezepte eingeladen" {
		t.Errorf("Subject = %q", got)
	}
	if strings.Contains(p["text/html"], "<b>Ö&") {
		t.Error("display name is not HTML-escaped")
	}
}

func TestUnknownLocaleFallsBackToEnglish(t *testing.T) {
	env, err := BuildInvite(cfg, Invite{To: "a@b.c", Name: "L", Inviter: "S", Path: "/w#t", Locale: "fr"}, "https://r.example.org", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, _, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "S invited you to Rezepte" {
		t.Errorf("Subject = %q", got)
	}
}

func TestTestMessage(t *testing.T) {
	env, err := BuildTest(cfg, "owner@example.org", "en", "https://r.example.org", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if env.To != "owner@example.org" {
		t.Errorf("To = %q", env.To)
	}
	_, p, _ := parts(t, env)
	if !strings.Contains(p["text/plain"], "mail works") {
		t.Errorf("plain = %q", p["text/plain"])
	}
}

func TestResetMessage(t *testing.T) {
	env, err := BuildReset(cfg, Reset{To: "mila@example.org", Username: "mila", Path: "/welcome#tok", Locale: "en"}, "https://r.example.org", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "A new password for Rezepte" {
		t.Errorf("Subject = %q", got)
	}
	for _, want := range []string{"mila", "https://r.example.org/welcome#tok", "valid for 1 hour", "Wasn't you?"} {
		if !strings.Contains(p["text/plain"], want) {
			t.Errorf("plain lacks %q: %q", want, p["text/plain"])
		}
	}
	if !strings.Contains(p["text/html"], "Set a new password") {
		t.Error("html lacks the button")
	}
}

func TestResetMessageGerman(t *testing.T) {
	env, _ := BuildReset(cfg, Reset{To: "a@b.c", Username: "mila", Path: "/welcome#t", Locale: "de"}, "https://r.example.org", time.Now())
	msg, p, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "Ein neues Passwort für Rezepte" {
		t.Errorf("Subject = %q", got)
	}
	if strings.Contains(strings.SplitN(string(env.Data), "\r\n\r\n", 2)[0], "für") {
		t.Error("subject is not RFC 2047 encoded")
	}
	if !strings.Contains(p["text/plain"], "1 Stunde gültig") {
		t.Errorf("plain = %q", p["text/plain"])
	}
}

func TestHintMessage(t *testing.T) {
	env, err := BuildHint(cfg, Hint{To: "ida@example.org", Provider: "Dex", Locale: "en"}, "https://r.example.org", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, p, _ := parts(t, env)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "Signing in to Rezepte" {
		t.Errorf("Subject = %q", got)
	}
	if !strings.Contains(p["text/plain"], "with Dex; you have no password") || !strings.Contains(p["text/plain"], "https://r.example.org/login") {
		t.Errorf("plain = %q", p["text/plain"])
	}
	if strings.Contains(p["text/plain"], "valid") {
		t.Errorf("a hint has no validity line: %q", p["text/plain"])
	}
}

func TestHintMessageWithoutProviderName(t *testing.T) {
	env, _ := BuildHint(cfg, Hint{To: "a@b.c", Locale: "en"}, "https://r.example.org", time.Now())
	_, p, _ := parts(t, env)
	if !strings.Contains(p["text/plain"], "with your sign-in provider;") {
		t.Errorf("plain = %q", p["text/plain"])
	}
}

func TestConfirmMessage(t *testing.T) {
	own, _ := BuildConfirm(cfg, Confirm{To: "a@b.c", Username: "mila", Path: "/confirm-email#t", Locale: "en"}, "https://r.example.org", time.Now())
	msg, p, _ := parts(t, own)
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != "Confirm your email address for Rezepte" {
		t.Errorf("Subject = %q", got)
	}
	if !strings.Contains(p["text/plain"], "https://r.example.org/confirm-email#t") || !strings.Contains(p["text/plain"], "your account mila so") || strings.Contains(p["text/plain"], "entered this email address") {
		t.Errorf("own: plain = %q", p["text/plain"])
	}
	byAdmin, _ := BuildConfirm(cfg, Confirm{To: "a@b.c", Username: "mila", Admin: "Jürgen <b>", Path: "/confirm-email#t", Locale: "de"}, "https://r.example.org", time.Now())
	_, p, _ = parts(t, byAdmin)
	if !strings.Contains(p["text/plain"], "Jürgen <b> hat diese E-Mail-Adresse für dein Konto mila eingetragen.") {
		t.Errorf("admin: plain = %q", p["text/plain"])
	}
	if strings.Contains(p["text/html"], "Jürgen <b>") {
		t.Error("admin name is not HTML-escaped")
	}
}
