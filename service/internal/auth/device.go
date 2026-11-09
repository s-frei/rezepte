package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/s-frei/rezepte/service/internal/sign"
)

// AccountsCookieName is the cookie holding every account signed in on this
// device, the active one first. The active one is also the session cookie;
// its entry here is what keeps the person on the device once the browser
// drops that cookie.
const AccountsCookieName = "rezepte_accounts"

// accountsMaxAge is how long the accounts cookie, and each entry's
// signature, lasts after its last write. Every write re-signs every entry.
const accountsMaxAge = 400 * 24 * time.Hour

// maxDeviceAccounts caps the entries besides the active one, about 140
// bytes each, so the cookie stays well inside a browser's 4 KB.
const maxDeviceAccounts = 10

// accountsPath keeps the cookie on the auth operations, off every other
// request; the OIDC flow lives below it too.
const accountsPath = "/api/v1/auth"

// Device reads and writes the accounts cookie.
type Device struct{ signer *sign.Signer }

// NewDevice returns a Device signing with key (32 bytes, the instance key).
func NewDevice(key []byte) *Device {
	return &Device{signer: sign.New(key, "rezepte device account")}
}

// DeviceEntry is one other account on the device: who, and the raw token of
// its session there, which may have run out or been ended since.
type DeviceEntry struct{ UserID, Token string }

// Parse reads a cookie value, entries userID.token.sig joined by "~", and
// drops every entry that is malformed or not signed by this server.
func (d *Device) Parse(cookie string) []DeviceEntry {
	var out []DeviceEntry
	for raw := range strings.SplitSeq(cookie, "~") {
		parts := strings.SplitN(raw, ".", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
			continue
		}
		if d.signer.Verify(parts[0]+"."+parts[1], parts[2]) {
			out = append(out, DeviceEntry{UserID: parts[0], Token: parts[1]})
		}
	}
	return out
}

// Cookie is the Set-Cookie value for entries; an empty list deletes the
// cookie.
func (d *Device) Cookie(entries []DeviceEntry, secure bool) http.Cookie {
	c := http.Cookie{ //nolint:gosec // G124: Secure follows the secureCookies config flag (false only for local http dev); HttpOnly and SameSite are always set.
		Name:     AccountsCookieName,
		Path:     accountsPath,
		MaxAge:   int(accountsMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if len(entries) == 0 {
		return expired(c)
	}
	values := make([]string, len(entries))
	for i, e := range entries {
		subject := e.UserID + "." + e.Token
		sig, _ := d.signer.Sign(subject, accountsMaxAge)
		values[i] = subject + "." + sig
	}
	c.Value = strings.Join(values, "~")
	return c
}

// Handover returns entries without any entry of activeUserID and, when prev
// is non-nil and someone else, with prev in front: the person whose session
// was active until now. Past maxDeviceAccounts the oldest entries drop.
func Handover(entries []DeviceEntry, prev *DeviceEntry, activeUserID string) []DeviceEntry {
	var out []DeviceEntry
	if prev != nil && prev.UserID != activeUserID {
		out = append(out, *prev)
	}
	for _, e := range entries {
		if e.UserID != activeUserID && (prev == nil || e.UserID != prev.UserID) {
			out = append(out, e)
		}
	}
	return out[:min(len(out), maxDeviceAccounts)]
}
