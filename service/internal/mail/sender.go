package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// ErrNoSTARTTLS means the server was asked for STARTTLS and did not offer it.
// Rezepte never falls back to sending in plain text.
var ErrNoSTARTTLS = errors.New("the server does not offer STARTTLS")

// stepTimeout bounds each step of a session - the connection with its
// greeting, TLS, the sign-in, one message - rather than the session as a
// whole: a cold first connection (DNS, a TLS handshake, a greeting the
// server delays) may take several seconds and must not eat the time the
// message needs, while a server that goes silent still fails instead of
// hanging. A variable so tests can shorten it.
var stepTimeout = 30 * time.Second

// tlsConfig is the client TLS configuration for host; tests swap it.
var tlsConfig = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

// Envelope is one message for one recipient; Data is the full RFC 5322 text.
// Kind (the template's name) and Subject are what the mail history shows.
type Envelope struct {
	To      string
	Data    []byte
	Kind    string
	Subject string
}

// ErrSend marks every error Send returns, so a caller can tell a delivery
// failure from its own bug.
var ErrSend = errors.New("send failed")

// Send delivers env through cfg's server on a connection of its own. Every
// error wraps ErrSend; a server's refusal stays reachable as
// *textproto.Error, whose text is the reply the owner's test button shows.
func Send(ctx context.Context, cfg Config, env Envelope) error {
	s, err := open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSend, err)
	}
	defer s.close()
	if err := s.send(env); err != nil {
		return fmt.Errorf("%w: %w", ErrSend, err)
	}
	s.quit()
	return nil
}

// setupError is a send that failed on the instance's mail setup, before
// any recipient was named: Reason is "connect", "tls", "auth" or "sender".
// A refusal at RCPT TO or later is about one mail, not the setup.
type setupError struct {
	Reason string
	err    error
}

func (e *setupError) Error() string { return e.err.Error() }
func (e *setupError) Unwrap() error { return e.err }

func setup(reason string, err error) error { return &setupError{reason, err} }

// session is one signed-in SMTP connection. The queue sends every mail
// waiting behind the first over it, so a digest to ten people connects,
// encrypts and signs in once.
type session struct {
	conn net.Conn
	c    *smtp.Client
	from string
	stop func() bool
}

// open connects to cfg's server, secures the connection and signs in. The
// connection closes when ctx ends, so a canceled request or a shutdown
// stops a session that is still talking.
func open(ctx context.Context, cfg Config) (*session, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	d := net.Dialer{Timeout: stepTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, setup("connect", fmt.Errorf("connect to %s: %w", addr, err))
	}
	s := &session{conn: conn, from: cfg.From, stop: context.AfterFunc(ctx, func() { _ = conn.Close() })}
	if err := s.start(cfg, addr); err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

// step gives the next step of the session its own stepTimeout.
func (s *session) step() { _ = s.conn.SetDeadline(time.Now().Add(stepTimeout)) }

func (s *session) start(cfg Config, addr string) error {
	s.step()
	if cfg.Security == SecurityTLS {
		tc := tls.Client(s.conn, tlsConfig(cfg.Host))
		if err := tc.Handshake(); err != nil {
			return setup("tls", fmt.Errorf("TLS with %s: %w", addr, err))
		}
		s.conn = tc
	}
	c, err := smtp.NewClient(s.conn, cfg.Host)
	if err != nil {
		return setup("connect", fmt.Errorf("greeting from %s: %w", addr, err))
	}
	s.c = c
	// EHLO names the sender's domain: some servers refuse "localhost".
	if err := c.Hello(cfg.From[strings.LastIndex(cfg.From, "@")+1:]); err != nil {
		return setup("connect", fmt.Errorf("EHLO: %w", err))
	}
	if cfg.Security == SecuritySTARTTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return setup("tls", fmt.Errorf("%s: %w", addr, ErrNoSTARTTLS))
		}
		s.step()
		if err := c.StartTLS(tlsConfig(cfg.Host)); err != nil {
			return setup("tls", fmt.Errorf("STARTTLS: %w", err))
		}
	}
	if cfg.Username != "" {
		s.step()
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return setup("auth", fmt.Errorf("sign-in: %w", err))
		}
	}
	return nil
}

// send transmits one message. After a failure the session's state is
// unknown, so the caller closes it rather than send the next one.
func (s *session) send(env Envelope) error {
	s.step()
	if err := s.c.Mail(s.from); err != nil {
		// Only a refusal is about the sender; a 4xx or a broken connection
		// on a reused session ("too many messages") is not the setup's.
		if te, ok := errors.AsType[*textproto.Error](err); ok && te.Code/100 == 5 {
			return setup("sender", fmt.Errorf("sender: %w", err))
		}
		return fmt.Errorf("sender: %w", err)
	}
	if err := s.c.Rcpt(env.To); err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	w, err := s.c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(env.Data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		if _, ok := errors.AsType[*textproto.Error](err); ok {
			return fmt.Errorf("message: %w", err)
		}
		// The message is handed over; without the server's answer nobody
		// knows whether it was accepted.
		return fmt.Errorf("message: %w: %w", errUnconfirmed, err)
	}
	return nil
}

// errUnconfirmed marks a connection lost after the whole message was
// handed over, which the queue never retries.
var errUnconfirmed = errors.New("no answer after the message")

// quit ends the session politely; its messages are accepted already, so a
// QUIT hiccup must not turn them into a failure.
func (s *session) quit() {
	s.step()
	_ = s.c.Quit()
}

func (s *session) close() {
	s.stop()
	if s.c != nil {
		_ = s.c.Close()
		return
	}
	_ = s.conn.Close()
}
