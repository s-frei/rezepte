// Package mail sends Rezepte's mail through an SMTP server: it resolves the
// configuration (the environment beats the database, never mixed), talks
// SMTP with the standard library, and renders the messages.
package mail

import (
	"errors"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/user"
)

// ErrInvalidConfig wraps every reason a Config is refused.
var ErrInvalidConfig = errors.New("invalid mail configuration")

// Security is how the connection to the SMTP server is protected.
type Security string

const (
	// SecuritySTARTTLS dials in plain text and requires the upgrade; a
	// server that does not offer it is an error, never a plain-text send.
	SecuritySTARTTLS Security = "starttls"
	// SecurityTLS dials TLS from the first byte (usually port 465).
	SecurityTLS Security = "tls"
	// SecurityNone is plain text, for a relay in the operator's own network
	// or Mailpit. net/smtp still refuses PLAIN auth over it to anything but
	// localhost.
	SecurityNone Security = "none"
)

// Securities lists every Security, for enums and validation.
var Securities = []Security{SecuritySTARTTLS, SecurityTLS, SecurityNone}

// ParseSecurity accepts exactly the three names.
func ParseSecurity(s string) (Security, error) {
	for _, v := range Securities {
		if string(v) == s {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: security must be starttls, tls or none, got %q", ErrInvalidConfig, s)
}

// Config is everything needed to send. The zero value means "not configured".
type Config struct {
	Host     string
	Port     int
	Security Security
	Username string
	Password string
	From     string
	FromName string
}

// Configured reports whether c names a server at all.
func (c Config) Configured() bool { return c.Host != "" }

// Validate reports the first problem with c, wrapping ErrInvalidConfig.
func (c Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidConfig)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535, got %d", ErrInvalidConfig, c.Port)
	}
	if _, err := ParseSecurity(string(c.Security)); err != nil {
		return err
	}
	if c.Security == SecurityNone && c.Username != "" && !isLoopback(c.Host) {
		return fmt.Errorf("%w: credentials need starttls or tls", ErrInvalidConfig)
	}
	if c.From == "" {
		return fmt.Errorf("%w: from address is required", ErrInvalidConfig)
	}
	if got, err := user.ParseEmail(c.From); err != nil || got == "" || got != c.From {
		return fmt.Errorf("%w: from must be a bare address like rezepte@example.org", ErrInvalidConfig)
	}
	return nil
}

// isLoopback is the one case net/smtp sends credentials in plain text for.
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
