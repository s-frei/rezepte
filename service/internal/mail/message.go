package mail

import (
	"time"

	"github.com/s-frei/rezepte/service/internal/user"
)

// Invite is a setup link mailed to the person it was issued for. Existing
// marks an account that could sign in already (a lost password or
// provider): its copy does not say an account was created.
type Invite struct {
	To, Name, Inviter, Path string
	Locale                  user.Locale
	Existing                bool
}

// BuildInvite renders the setup mail.
func BuildInvite(cfg Config, in Invite, publicURL string, now time.Time) (Envelope, error) {
	prefix := "mail_invite"
	if in.Existing {
		prefix = "mail_invite_existing"
	}
	return build(cfg, in.To, letter{
		Name:        "invite",
		Lang:        string(in.Locale),
		SubjectKey:  prefix + "_subject",
		SubjectArgs: map[string]string{"inviter": in.Inviter},
		PublicURL:   publicURL,
		Data:        struct{ Prefix, Name, Inviter, URL string }{prefix, in.Name, in.Inviter, publicURL + in.Path},
	}, now)
}

// Reset is a password reset link mailed to an account's confirmed address.
type Reset struct {
	To, Username, Path string
	Locale             user.Locale
}

// BuildReset renders the forgotten-password mail.
func BuildReset(cfg Config, in Reset, publicURL string, now time.Time) (Envelope, error) {
	return build(cfg, in.To, letter{
		Name:       "reset",
		Lang:       string(in.Locale),
		SubjectKey: "mail_reset_subject",
		PublicURL:  publicURL,
		Data:       struct{ Username, URL string }{in.Username, publicURL + in.Path},
	}, now)
}

// Hint answers a forgotten-password request from an account that signs in
// only through an identity provider: it has no password to reset.
type Hint struct {
	To, Provider string
	Locale       user.Locale
}

// BuildHint renders the sign-in hint; its button opens the login page.
func BuildHint(cfg Config, in Hint, publicURL string, now time.Time) (Envelope, error) {
	provider, err := providerName(in.Locale, in.Provider)
	if err != nil {
		return Envelope{}, err
	}
	return build(cfg, in.To, letter{
		Name:       "hint",
		Lang:       string(in.Locale),
		SubjectKey: "mail_hint_subject",
		PublicURL:  publicURL,
		Data:       struct{ Provider, URL string }{provider, publicURL + "/login"},
	}, now)
}

// providerName is name, or the catalog's stand-in while the provider in
// force, from REZEPTE_OIDC_NAME or Settings → Sign-in, has none.
func providerName(locale user.Locale, name string) (string, error) {
	if name != "" {
		return name, nil
	}
	return text(locale, "mail_hint_provider_fallback", nil)
}

// Confirm asks whoever reads To to confirm it for the account Username, so
// two accounts sharing an address can tell their mails apart. Admin names
// who entered the address; empty when the account holder did.
type Confirm struct {
	To, Username, Admin, Path string
	Locale                    user.Locale
}

// BuildConfirm renders the address confirmation mail.
func BuildConfirm(cfg Config, in Confirm, publicURL string, now time.Time) (Envelope, error) {
	return build(cfg, in.To, letter{
		Name:       "confirm",
		Lang:       string(in.Locale),
		SubjectKey: "mail_confirm_subject",
		PublicURL:  publicURL,
		Data:       struct{ Username, Admin, URL string }{in.Username, in.Admin, publicURL + in.Path},
	}, now)
}

// BuildTest renders the owner's test mail.
func BuildTest(cfg Config, to string, locale user.Locale, publicURL string, now time.Time) (Envelope, error) {
	return build(cfg, to, letter{
		Name:       "test",
		Lang:       string(locale),
		SubjectKey: "mail_test_subject",
		PublicURL:  publicURL,
	}, now)
}
