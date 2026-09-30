package user

import (
	"errors"
	"net/mail"
	"strings"
)

// ErrInvalidEmail means the value is not a bare email address.
var ErrInvalidEmail = errors.New("not a valid email address")

// maxEmailLen is the longest address SMTP allows in a path (RFC 5321).
const maxEmailLen = 254

// ParseEmail trims s and accepts it only as a bare address: no display name,
// no angle brackets. "" (after trimming) is valid and means "no address".
func ParseEmail(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) > maxEmailLen {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Name != "" || addr.Address != s {
		return "", ErrInvalidEmail
	}
	return s, nil
}
