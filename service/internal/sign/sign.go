// Package sign makes and checks short, stateless tokens: an expiry and an
// HMAC over a label, a subject and that expiry. The label keeps tokens of
// different features apart even under one key.
package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// macSize is how many bytes of the HMAC-SHA256 a token carries: 128 bits
// is far beyond guessing within a token's lifetime.
const macSize = 16

// Signer signs subjects for one purpose, named by its label.
type Signer struct {
	key   []byte
	label string
	now   func() time.Time
}

// New returns a Signer for key (32 bytes) and label.
func New(key []byte, label string) *Signer {
	return &Signer{key: key, label: label, now: time.Now}
}

// Sign returns a token for subject that lasts ttl, and the moment it
// expires. The token is the expiry in Unix seconds, base 36, a dot and the
// MAC.
func (s *Signer) Sign(subject string, ttl time.Duration) (string, time.Time) {
	expires := s.now().Add(ttl).Truncate(time.Second)
	exp := strconv.FormatInt(expires.Unix(), 36)
	return exp + "." + base64.RawURLEncoding.EncodeToString(s.mac(subject, exp)), expires
}

// Verify reports whether token was made by Sign for subject under this
// label and key and has not expired.
func (s *Signer) Verify(subject, token string) bool {
	exp, mac64, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	mac, err := base64.RawURLEncoding.DecodeString(mac64)
	if err != nil || !hmac.Equal(mac, s.mac(subject, exp)) {
		return false
	}
	unix, err := strconv.ParseInt(exp, 36, 64)
	return err == nil && s.now().Unix() < unix
}

func (s *Signer) mac(subject, exp string) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(s.label + "\x00" + subject + "\x00" + exp))
	return h.Sum(nil)[:macSize]
}
