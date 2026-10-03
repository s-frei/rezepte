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

func parts(t *testing.T, env Envelope) (*mail.Message, map[string]string) {
	t.Helper()
	msg, err := mail.ReadMessage(bytes.NewReader(env.Data))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	r := multipart.NewReader(msg.Body, params["boundary"])
	out := map[string]string{}
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		ct, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		b, _ := io.ReadAll(p) // multipart decodes quoted-printable for us
		out[ct] = string(b)
	}
	return msg, out
}

func TestInviteMessage(t *testing.T) {
	env, err := BuildInvite(cfg, Invite{To: "lena@example.org", Name: "Lena", Inviter: "Samuel",
		Path: "/welcome#tok", Locale: "en"}, "https://r.example.org", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg, p := parts(t, env)
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
	msg, p := parts(t, env)
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
	msg, p := parts(t, env)
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
	msg, _ := parts(t, env)
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
	_, p := parts(t, env)
	if !strings.Contains(p["text/plain"], "mail works") {
		t.Errorf("plain = %q", p["text/plain"])
	}
}
