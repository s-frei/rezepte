package preview

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

// Signer makes and checks link preview tokens. A token names a recipe and an
// expiry and is signed with the key stored in instance_settings, so a token
// outlives a restart of the service.
type Signer struct {
	key []byte
	now func() time.Time
}

// NewSigner returns a Signer for key, which must be the 32 stored bytes.
func NewSigner(key []byte) *Signer {
	return &Signer{key: key, now: time.Now}
}

// Sign returns a token for recipeID that lasts ttl, and the moment it
// expires. The token is the expiry in Unix seconds, base 36, a dot and the
// MAC over both.
func (s *Signer) Sign(recipeID string, ttl time.Duration) (string, time.Time) {
	expires := s.now().Add(ttl).Truncate(time.Second)
	exp := strconv.FormatInt(expires.Unix(), 36)
	return exp + "." + base64.RawURLEncoding.EncodeToString(s.mac(recipeID, exp)), expires
}

// Verify reports whether token was made by Sign for recipeID and has not
// expired.
func (s *Signer) Verify(recipeID, token string) bool {
	exp, mac64, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	mac, err := base64.RawURLEncoding.DecodeString(mac64)
	if err != nil || !hmac.Equal(mac, s.mac(recipeID, exp)) {
		return false
	}
	unix, err := strconv.ParseInt(exp, 36, 64)
	return err == nil && s.now().Unix() < unix
}

func (s *Signer) mac(recipeID, exp string) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte("rezepte link preview\x00" + recipeID + "\x00" + exp))
	return h.Sum(nil)[:macSize]
}
