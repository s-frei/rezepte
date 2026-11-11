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

// ConfigError is a refused Config. Field names the setting it fails on as
// the API's request body spells it, so the owner is told which one to fix.
type ConfigError struct{ Field, Reason string }

func (e *ConfigError) Error() string { return ErrInvalidConfig.Error() + ": " + e.Reason }

// Unwrap makes every ConfigError an ErrInvalidConfig.
func (e *ConfigError) Unwrap() error { return ErrInvalidConfig }

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
	return "", &ConfigError{"security", fmt.Sprintf("security must be starttls, tls or none, got %q", s)}
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

// Validate reports the first problem with c as a *ConfigError.
func (c Config) Validate() error {
	if c.Host == "" {
		return &ConfigError{"host", "host is required"}
	}
	if c.Port < 1 || c.Port > 65535 {
		return &ConfigError{"port", fmt.Sprintf("port must be between 1 and 65535, got %d", c.Port)}
	}
	if _, err := ParseSecurity(string(c.Security)); err != nil {
		return err
	}
	if c.Security == SecurityNone && c.Username != "" && !isLoopback(c.Host) {
		return &ConfigError{"security", "credentials need starttls or tls"}
	}
	if c.From == "" {
		return &ConfigError{"from", "from address is required"}
	}
	if got, err := user.ParseEmail(c.From); err != nil || got == "" || got != c.From {
		return &ConfigError{"from", "from must be a bare address like rezepte@example.org"}
	}
	return nil
}

// isLoopback is the one case net/smtp sends credentials in plain text for.
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
