package mail

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeSMTP answers one connection with a scripted SMTP dialog. ext lists the
// EHLO extensions offered; authReply is what AUTH answers; it records the
// DATA it received.
type fakeSMTP struct {
	ln        net.Listener
	ext       []string
	authReply string
	tlsConf   *tls.Config
	hang      bool
	got       chan string
	auth      chan string // the AUTH command line, as received
	ehlo      chan string // the EHLO command line, as received
}

func startFake(t *testing.T, f *fakeSMTP) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.ln, f.got, f.auth, f.ehlo = ln, make(chan string, 1), make(chan string, 1), make(chan string, 1)
	t.Cleanup(func() { ln.Close() })
	go f.serve()
	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

func (f *fakeSMTP) serve() {
	c, err := f.ln.Accept()
	if err != nil {
		return
	}
	defer c.Close()
	if f.hang {
		time.Sleep(5 * time.Second)
		return
	}
	tp := textproto.NewConn(c)
	_ = tp.PrintfLine("220 fake ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.Fields(line + " x")[0])
		switch cmd {
		case "EHLO", "HELO":
			select {
			case f.ehlo <- line:
			default:
			}
			lines := append([]string{"fake"}, f.ext...)
			for i, l := range lines {
				sep := "-"
				if i == len(lines)-1 {
					sep = " "
				}
				_ = tp.PrintfLine("250%s%s", sep, l)
			}
		case "STARTTLS":
			_ = tp.PrintfLine("220 go ahead")
			tc := tls.Server(c, f.tlsConf)
			if err := tc.Handshake(); err != nil {
				return
			}
			c = tc
			tp = textproto.NewConn(tc)
		case "AUTH":
			f.auth <- line
			_ = tp.PrintfLine("%s", f.authReply)
		case "MAIL", "RCPT":
			_ = tp.PrintfLine("250 ok")
		case "DATA":
			_ = tp.PrintfLine("354 go")
			data, _ := tp.ReadDotBytes()
			f.got <- string(data)
			_ = tp.PrintfLine("250 queued")
		case "QUIT":
			_ = tp.PrintfLine("221 bye")
			return
		default:
			_ = tp.PrintfLine("502 unknown")
		}
	}
}

// testTLS returns a server config and swaps tlsConfig to trust it.
func testTLS(t *testing.T) *tls.Config {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	cert := srv.TLS.Certificates[0]
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	srv.Close()
	old := tlsConfig
	tlsConfig = func(host string) *tls.Config { return &tls.Config{RootCAs: pool, ServerName: "example.com"} }
	t.Cleanup(func() { tlsConfig = old })
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

var envelope = Envelope{To: "lena@example.org", Data: []byte("Subject: hi\r\n\r\nhello\r\n")}

func TestSendNoneWithoutAuth(t *testing.T) {
	f := &fakeSMTP{}
	host, port := startFake(t, f)
	cfg := Config{Host: host, Port: port, Security: SecurityNone, From: "r@example.org"}
	if err := Send(context.Background(), cfg, envelope); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := <-f.got; !strings.Contains(got, "hello") {
		t.Fatalf("DATA = %q", got)
	}
}

func TestSendSTARTTLSRequired(t *testing.T) {
	f := &fakeSMTP{} // offers no STARTTLS
	host, port := startFake(t, f)
	cfg := Config{Host: host, Port: port, Security: SecuritySTARTTLS, From: "r@example.org"}
	err := Send(context.Background(), cfg, envelope)
	if !errors.Is(err, ErrNoSTARTTLS) {
		t.Fatalf("err = %v, want ErrNoSTARTTLS", err)
	}
}

func TestSendSTARTTLSAndAuth(t *testing.T) {
	f := &fakeSMTP{ext: []string{"STARTTLS", "AUTH PLAIN"}, authReply: "235 ok"}
	f.tlsConf = testTLS(t)
	host, port := startFake(t, f)
	cfg := Config{Host: host, Port: port, Security: SecuritySTARTTLS, Username: "u", Password: "p", From: "r@example.org"}
	if err := Send(context.Background(), cfg, envelope); err != nil {
		t.Fatalf("Send: %v", err)
	}
	<-f.got
}

func TestSendSurfacesReply(t *testing.T) {
	f := &fakeSMTP{ext: []string{"STARTTLS", "AUTH PLAIN"}, authReply: "535 5.7.8 authentication failed"}
	f.tlsConf = testTLS(t)
	host, port := startFake(t, f)
	cfg := Config{Host: host, Port: port, Security: SecuritySTARTTLS, Username: "u", Password: "bad", From: "r@example.org"}
	err := Send(context.Background(), cfg, envelope)
	var reply *textproto.Error
	if !errors.As(err, &reply) || reply.Code != 535 {
		t.Fatalf("err = %v, want a 535 reply", err)
	}
	if got := <-f.ehlo; got != "EHLO example.org" {
		t.Errorf("greeting = %q, want EHLO with the sender's domain", got)
	}
	if !strings.Contains(err.Error(), "535") || !strings.Contains(err.Error(), "5.7.8 authentication failed") {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestSendDeadline(t *testing.T) {
	old := sendTimeout
	sendTimeout = 200 * time.Millisecond
	t.Cleanup(func() { sendTimeout = old })
	f := &fakeSMTP{hang: true}
	host, port := startFake(t, f)
	start := time.Now()
	err := Send(context.Background(), Config{Host: host, Port: port, Security: SecurityNone, From: "r@example.org"}, envelope)
	if err == nil {
		t.Fatal("Send succeeded against a server that never greets")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Send took %v, the deadline did not hold", time.Since(start))
	}
}

func TestSendConnectionRefused(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	err := Send(context.Background(), Config{Host: "127.0.0.1", Port: port, Security: SecurityNone, From: "r@example.org"}, envelope)
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:"+strconv.Itoa(port)) {
		t.Fatalf("err = %v, want it to name the address", err)
	}
}
