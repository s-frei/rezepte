package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// ErrNoSTARTTLS means the server was asked for STARTTLS and did not offer it.
// Rezepte never falls back to sending in plain text.
var ErrNoSTARTTLS = errors.New("the server does not offer STARTTLS")

// sendTimeout bounds one whole send - dial, dialog, data - so a server that
// accepts the connection and then says nothing cannot hold a request open.
var sendTimeout = 10 * time.Second

// tlsConfig is the client TLS configuration for host; tests swap it.
var tlsConfig = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

// Envelope is one message for one recipient; Data is the full RFC 5322 text.
type Envelope struct {
	To   string
	Data []byte
}

// ErrSend marks every error Send returns, so a caller can tell a delivery
// failure from its own bug.
var ErrSend = errors.New("send failed")

// Send delivers env through cfg's server. Every error wraps ErrSend; a
// server's refusal stays reachable as *textproto.Error, whose text is the
// reply the owner's test button shows.
func Send(ctx context.Context, cfg Config, env Envelope) error {
	if err := send(ctx, cfg, env); err != nil {
		return fmt.Errorf("%w: %w", ErrSend, err)
	}
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

// setupFailure returns why err is a failure of the mail setup, empty when
// it is not one.
func setupFailure(err error) string {
	var se *setupError
	if errors.As(err, &se) {
		return se.Reason
	}
	return ""
}

func send(ctx context.Context, cfg Config, env Envelope) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return setup("connect", fmt.Errorf("connect to %s: %w", addr, err))
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	if cfg.Security == SecurityTLS {
		tc := tls.Client(conn, tlsConfig(cfg.Host))
		if err := tc.HandshakeContext(ctx); err != nil {
			return setup("tls", fmt.Errorf("TLS with %s: %w", addr, err))
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return setup("connect", fmt.Errorf("greeting from %s: %w", addr, err))
	}
	defer c.Close()
	// EHLO names the sender's domain: some servers refuse "localhost".
	if err := c.Hello(cfg.From[strings.LastIndex(cfg.From, "@")+1:]); err != nil {
		return setup("connect", fmt.Errorf("EHLO: %w", err))
	}
	if cfg.Security == SecuritySTARTTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return setup("tls", fmt.Errorf("%s: %w", addr, ErrNoSTARTTLS))
		}
		if err := c.StartTLS(tlsConfig(cfg.Host)); err != nil {
			return setup("tls", fmt.Errorf("STARTTLS: %w", err))
		}
	}
	if cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return setup("auth", fmt.Errorf("sign-in: %w", err))
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return setup("sender", fmt.Errorf("sender: %w", err))
	}
	if err := c.Rcpt(env.To); err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(env.Data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("message: %w", err)
	}
	_ = c.Quit() // the message is accepted; a QUIT hiccup must not turn it into a failure
	return nil
}
