package auth

import "testing"

// TestExpiredCookies pins the exact Set-Cookie values logout sends.
func TestExpiredCookies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secure bool
		got    func(bool) string
		want   string
	}{
		{"session", false, func(s bool) string { c := expiredSessionCookie(s); return c.String() },
			"rezepte_session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0; HttpOnly; SameSite=Lax"},
		{"session", true, func(s bool) string { c := expiredSessionCookie(s); return c.String() },
			"rezepte_session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Lax"},
		{"locale", false, func(s bool) string { c := expiredLocaleCookie(s); return c.String() },
			"PARAGLIDE_LOCALE=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0; SameSite=Lax"},
		{"locale", true, func(s bool) string { c := expiredLocaleCookie(s); return c.String() },
			"PARAGLIDE_LOCALE=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0; Secure; SameSite=Lax"},
	} {
		if got := tc.got(tc.secure); got != tc.want {
			t.Errorf("%s secure=%v:\n got %s\nwant %s", tc.name, tc.secure, got, tc.want)
		}
	}
}
