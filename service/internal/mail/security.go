package mail

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/user"
)

// PasswordOrigin names the way a password was changed, for the notice.
type PasswordOrigin string

// The values of PasswordOrigin.
const (
	OriginProfile     PasswordOrigin = "profile"
	OriginAdmin       PasswordOrigin = "admin"
	OriginResetLink   PasswordOrigin = "reset_link"
	OriginSetupLink   PasswordOrigin = "setup_link"
	OriginCommandLine PasswordOrigin = "command_line"
)

// PasswordChanged tells an account's confirmed address that its password
// changed. Admin is the display name of who did it, set for OriginAdmin and
// OriginSetupLink. Owner marks the superadmin, whom /forgot-password never
// resets. First marks an account that had no password before (it signed in
// through an identity provider): the notice says a password was set. Only
// FirstOrigins have that copy.
type PasswordChanged struct {
	To, Username, Admin string
	Origin              PasswordOrigin
	At                  time.Time
	Locale              user.Locale
	Owner, First        bool
}

// FirstOrigins are the ways a first password can be set, and the ones with
// "set" copy: a reset link needs a password, and the owner always has one.
// First with any other origin falls back to the "changed" copy rather than
// fail the render.
var FirstOrigins = []PasswordOrigin{OriginProfile, OriginAdmin, OriginSetupLink}

// BuildPasswordChanged renders the password notice; its button opens
// /forgot-password for whoever did not do it, and the owner's notice points
// at the command line instead.
func BuildPasswordChanged(cfg Config, in PasswordChanged, publicURL string, now time.Time) (Envelope, error) {
	prefix := "mail_password_changed"
	if in.First && slices.Contains(FirstOrigins, in.Origin) {
		prefix = "mail_password_set"
	}
	return build(cfg, in.To, letter{
		Name:       "password_changed",
		Lang:       string(in.Locale),
		SubjectKey: prefix + "_subject",
		PublicURL:  publicURL,
		Data: struct {
			Prefix, Username, Admin, Origin, URL string
			At                                   time.Time
			Owner                                bool
		}{prefix, in.Username, in.Admin, string(in.Origin), publicURL + "/forgot-password", in.At, in.Owner},
	}, now)
}

// EmailChanged tells the address an account had confirmed before that it
// was changed (NewAddress) or removed (NewAddress empty). Admin is the
// display name of who did it, empty when the account holder did.
type EmailChanged struct {
	To, Username, Admin, NewAddress string
	At                              time.Time
	Locale                          user.Locale
}

// BuildEmailChanged renders the address notice; the new address shows
// masked, since the old mailbox may no longer be the holder's.
func BuildEmailChanged(cfg Config, in EmailChanged, publicURL string, now time.Time) (Envelope, error) {
	prefix, body, masked := "mail_email_changed_removed", "mail_email_changed_removed", ""
	if in.NewAddress != "" {
		prefix, body, masked = "mail_email_changed", "mail_email_changed_body", MaskAddress(in.NewAddress)
	}
	if in.Admin != "" {
		body += "_admin"
	}
	return build(cfg, in.To, letter{
		Name:       "email_changed",
		Lang:       string(in.Locale),
		SubjectKey: prefix + "_subject",
		PublicURL:  publicURL,
		Data: struct {
			Prefix, Body, Username, Admin, Address string
			At                                     time.Time
		}{prefix, body, in.Username, in.Admin, masked, in.At},
	}, now)
}

// IdentityOrigin names the way a sign-in identity was linked, for the
// notice.
type IdentityOrigin string

// The values of IdentityOrigin: the account holder linked it in their
// profile, or a setup link from an admin was redeemed with it.
const (
	IdentityProfile   IdentityOrigin = "profile"
	IdentitySetupLink IdentityOrigin = "setup_link"
)

// IdentityLinked tells an account's confirmed address that a sign-in
// identity was linked to it. Provider is the name of the provider in force
// (REZEPTE_OIDC_NAME or Settings → Sign-in), empty for the catalog's
// stand-in; Admin the display name of who issued the setup link, set for
// IdentitySetupLink. Owner marks the superadmin.
type IdentityLinked struct {
	To, Username, Admin, Provider string
	Origin                        IdentityOrigin
	At                            time.Time
	Locale                        user.Locale
	Owner                         bool
}

// BuildIdentityLinked renders the sign-in method notice.
func BuildIdentityLinked(cfg Config, in IdentityLinked, publicURL string, now time.Time) (Envelope, error) {
	provider, err := providerName(in.Locale, in.Provider)
	if err != nil {
		return Envelope{}, err
	}
	return build(cfg, in.To, letter{
		Name:       "identity_linked",
		Lang:       string(in.Locale),
		SubjectKey: "mail_identity_linked_subject",
		PublicURL:  publicURL,
		Data: struct {
			Username, Admin, Provider, Origin string
			At                                time.Time
			Owner                             bool
		}{in.Username, in.Admin, provider, string(in.Origin), in.At, in.Owner},
	}, now)
}

// MaskAddress keeps the first character of the local part and the domain:
// "mia@example.org" is "m…@example.org"; without an @ it is "…".
func MaskAddress(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at < 0 {
		return "…"
	}
	_, n := utf8.DecodeRuneInString(addr)
	return addr[:min(n, at)] + "…" + addr[at:]
}
