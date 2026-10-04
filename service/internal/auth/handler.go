package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// UserResponse is the public representation of the current user.
type UserResponse struct {
	ID          string      `json:"id" doc:"User id"`
	Username    string      `json:"username" doc:"Login name"`
	DisplayName string      `json:"displayName" doc:"Name shown wherever the UI names this person"`
	Role        string      `json:"role" enum:"superadmin,admin,user" doc:"Authorization role"`
	Color       string      `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Palette token identifying this person"`
	Locale      user.Locale `json:"locale" doc:"The account holder's interface language"`
	// CanSharePublicly decides, together with the household's publicShares
	// setting, whether the UI offers to create a public link.
	CanSharePublicly bool    `json:"canSharePublicly" doc:"Whether an admin lets this person create public links"`
	AvatarID         *string `json:"avatarId" nullable:"true" doc:"The account's picture, served at /avatars/{id}/{avatarId}.jpg; null when it has none"`
	Email            string  `json:"email" doc:"The account's email address; empty when none. Never used to sign in."`
	EmailVerified    bool    `json:"emailVerified" doc:"Whether the address is confirmed: an identity provider vouched for it, or the person opened a link mailed to it"`
	HasPassword      bool    `json:"hasPassword" doc:"False for an account that signs in only through a setup link or an identity provider"`
	// EmailConfirmationPending is read from the open confirmation links, not
	// the users row, so toResponse leaves it false and respond fills it.
	EmailConfirmationPending bool `json:"emailConfirmationPending" doc:"Whether a confirmation mail went to the current, unconfirmed address and its link is still open"`
}

type loginInput struct {
	Body struct {
		Username string `json:"username" minLength:"1" maxLength:"64"`
		Password string `json:"password" minLength:"1" maxLength:"1024"`
	}
}

type loginOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      UserResponse
}

type logoutInput struct {
	Cookie string `cookie:"rezepte_session"`
}

type logoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type meOutput struct {
	Body UserResponse
}

type changePasswordInput struct {
	Cookie string `cookie:"rezepte_session"`
	Body   struct {
		CurrentPassword string `json:"currentPassword,omitempty" maxLength:"1024" doc:"The password in use now; omitted when the account has none"`
		Password        string `json:"password" minLength:"8" maxLength:"128" doc:"The new password"`
	}
}

type changePasswordOutput struct{}

type updateProfileInput struct {
	Body struct {
		DisplayName *string      `json:"displayName,omitempty" maxLength:"64" doc:"Empty falls back to the login name"`
		Color       *string      `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate"`
		Locale      *user.Locale `json:"locale,omitempty" doc:"The account holder's interface language"`
		Email       *string      `json:"email,omitempty" maxLength:"254" doc:"Empty clears it; a changed address is unverified"`
	}
}

type updateProfileOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      UserResponse
}

type forgotInput struct {
	Body struct {
		Login string `json:"login" minLength:"1" maxLength:"254" doc:"A username, or an email address (contains @)"`
	}
}

// PasswordResetInfo is password-reset-config's response body.
type PasswordResetInfo struct {
	Available bool `json:"available" doc:"Whether a forgotten password can be reset by mail here, which needs mail to be configured"`
}

type passwordResetOutput struct{ Body PasswordResetInfo }

type confirmInput struct {
	Body struct {
		Token string `json:"token" minLength:"1" maxLength:"128"`
	}
}

// ConfirmedAddress is confirm-email's response body.
type ConfirmedAddress struct {
	Address string `json:"address" doc:"The address that is now confirmed"`
}

type confirmOutput struct{ Body ConfirmedAddress }

// forgotTimeout bounds the background work one forgotten-password request
// starts: lookups, the link, and every mail's 10 s SMTP deadline.
const forgotTimeout = 15 * time.Second

type colorUsageOutput struct {
	Body struct {
		Items []user.ColorCount `json:"items" doc:"Every palette color and how many accounts hold it, in palette order"`
	}
}

type setupTokenBody struct {
	Token string `json:"token" minLength:"1" maxLength:"128"`
}

type inspectSetupInput struct{ Body setupTokenBody }

// InvitedAccount is inspect-setup-link's response body: who a setup link
// belongs to, before it is used to set a password. A named type rather than
// an inline struct, so the OpenAPI document names this schema for what it is
// instead of a generated "InspectSetupOutputBody".
type InvitedAccount struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Purpose     string `json:"purpose" enum:"setup,reset" doc:"setup: set a password or connect a provider; reset: a forgotten-password link, which only sets a password"`
}

type inspectSetupOutput struct {
	Body InvitedAccount
}

// SetupRedemption is redeem-setup-link's request body.
type SetupRedemption struct {
	Token    string `json:"token" minLength:"1" maxLength:"128"`
	Password string `json:"password" minLength:"8" maxLength:"128"`
}

type redeemSetupInput struct {
	Body SetupRedemption
}

type redeemSetupOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      UserResponse
}

// Register installs the login, logout, me and change-password operations.
//
// The session security scheme itself is not declared here: it comes from
// SecuritySchemes, installed through httpserver.WithSecuritySchemes, which
// is the single place both schemes are described.
//
// The auth middleware itself is not installed here: it is passed to
// httpserver.New via httpserver.WithAPIMiddleware(auth.Middleware(...)),
// which applies it before this package (or any other) can register an
// operation. That ordering is structural, not a calling-convention
// requirement of this function.
func Register(api huma.API, svc *Service, secureCookies bool) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Log in with username and password",
		Tags:        []string{"auth"},
		Errors:      []int{401, 429, 503},
	}, func(ctx context.Context, in *loginInput) (*loginOutput, error) {
		sess, err := svc.Login(ctx, in.Body.Username, in.Body.Password)
		if errors.Is(err, user.ErrInvalidCredentials) {
			return nil, huma.Error401Unauthorized("invalid username or password")
		}
		var throttled *ThrottledError
		if errors.As(err, &throttled) {
			return nil, throttledError(throttled)
		}
		if mapped := BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		return &loginOutput{
			SetCookie: []http.Cookie{
				SessionCookie(sess.Token, sess.ExpiresAt, secureCookies),
				LocaleCookie(sess.User.Locale, secureCookies),
			},
			Body: respond(ctx, svc, sess.User),
		}, nil
	})
	DeclareRetryAfter(api, http.MethodPost, "/api/v1/auth/login", http.StatusTooManyRequests, http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/v1/auth/logout",
		Summary:       "End the current session",
		Tags:          []string{"auth"},
		Security:      SessionSecurity,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *logoutInput) (*logoutOutput, error) {
		if err := svc.Logout(ctx, in.Cookie); err != nil {
			return nil, err
		}
		return &logoutOutput{
			SetCookie: []http.Cookie{
				expiredSessionCookie(secureCookies),
				expiredLocaleCookie(secureCookies),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me",
		Summary:     "Return the current user",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*meOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		return &meOutput{Body: respond(ctx, svc, u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "change-own-password",
		Method:        http.MethodPatch,
		Path:          "/api/v1/auth/me",
		Summary:       "Change the current user's password",
		Description:   "Verifies the current password, stores the new one and ends every other session of the user; the session making the call stays valid. An account without a password sets one without currentPassword.",
		Tags:          []string{"auth"},
		Security:      SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 422, 503},
	}, func(ctx context.Context, in *changePasswordInput) (*changePasswordOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		err := svc.users.ChangePassword(ctx, u.ID, in.Body.CurrentPassword, in.Body.Password)
		if errors.Is(err, user.ErrWrongPassword) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.currentPassword",
				Message:  "current password is wrong",
			})
		}
		if mapped := BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteUserSessionsExcept(ctx, u.ID, in.Cookie); err != nil {
			return nil, err
		}
		return &changePasswordOutput{}, nil
	})
	DeclareRetryAfter(api, http.MethodPatch, "/api/v1/auth/me", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID: "update-own-profile",
		Method:      http.MethodPatch,
		Path:        "/api/v1/auth/me/profile",
		Summary:     "Change the current user's display name, color and interface language",
		Description: "Its own path rather than PATCH /api/v1/auth/me, which is the password change and demands the current password - a rename has nothing to do with it.",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401, 422},
	}, func(ctx context.Context, in *updateProfileInput) (*updateProfileOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if in.Body.DisplayName == nil && in.Body.Color == nil && in.Body.Locale == nil && in.Body.Email == nil {
			return nil, huma.Error422UnprocessableEntity("nothing to change")
		}
		update := user.ProfileUpdate{DisplayName: in.Body.DisplayName}
		if in.Body.Color != nil {
			c := user.Color(*in.Body.Color)
			update.Color = &c
		}
		update.Locale = in.Body.Locale
		update.Email = in.Body.Email
		updated, err := svc.users.SetProfile(ctx, u.ID, update)
		if mapped := ProfileError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		// After the write committed: a mail that fails changes nothing saved.
		svc.ConfirmChangedAddress(ctx, u, updated, "")
		// After the send, which opens or drops the link: pending says whether
		// the mail went out.
		return &updateProfileOutput{
			SetCookie: []http.Cookie{LocaleCookie(updated.Locale, secureCookies)},
			Body:      respond(ctx, svc, updated),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-color-usage",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me/colors",
		Summary:     "List the color palette and how many accounts hold each color",
		Description: "Counts, not names: the picker only needs to mark a color as taken. It sits under /auth/me because it describes what the caller may choose for themselves; /people names every account without counting colors, and everything under /users is admin-only.",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*colorUsageOutput, error) {
		if _, ok := UserFrom(ctx); !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		usage, err := svc.users.ColorUsage(ctx)
		if err != nil {
			return nil, err
		}
		out := &colorUsageOutput{}
		out.Body.Items = usage
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "inspect-setup-link",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/setup/inspect",
		Summary:     "Look up the account a setup link belongs to, without using it",
		Tags:        []string{"auth"},
		Errors:      []int{404},
	}, func(ctx context.Context, in *inspectSetupInput) (*inspectSetupOutput, error) {
		l, err := svc.PeekSetupLink(ctx, in.Body.Token)
		if errors.Is(err, ErrNoSetupLink) {
			return nil, huma.Error404NotFound("setup link not found or expired")
		}
		if err != nil {
			return nil, err
		}
		out := &inspectSetupOutput{}
		out.Body.Username = l.User.Username
		out.Body.DisplayName = l.User.DisplayName
		out.Body.Purpose = string(l.Purpose)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "redeem-setup-link",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/setup/password",
		Summary:     "Set a password through a setup link and sign in",
		Tags:        []string{"auth"},
		Errors:      []int{404, 422, 503},
	}, func(ctx context.Context, in *redeemSetupInput) (*redeemSetupOutput, error) {
		sess, err := svc.RedeemWithPassword(ctx, in.Body.Token, in.Body.Password)
		if errors.Is(err, ErrNoSetupLink) {
			return nil, huma.Error404NotFound("setup link not found or expired")
		}
		if mapped := BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		return &redeemSetupOutput{
			SetCookie: []http.Cookie{
				SessionCookie(sess.Token, sess.ExpiresAt, secureCookies),
				LocaleCookie(sess.User.Locale, secureCookies),
			},
			Body: respond(ctx, svc, sess.User),
		}, nil
	})
	DeclareRetryAfter(api, http.MethodPost, "/api/v1/auth/setup/password", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID: "password-reset-config",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/password",
		Summary:     "Tell whether a forgotten password can be reset by mail",
		Description: "Public, like GET /api/v1/auth/oidc: the login page asks before anyone is signed in.",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, _ *struct{}) (*passwordResetOutput, error) {
		return &passwordResetOutput{Body: PasswordResetInfo{Available: svc.mailOn(ctx)}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "forgot-password",
		Method:        http.MethodPost,
		Path:          "/api/v1/auth/password/forgot",
		Summary:       "Mail a password reset link",
		Description:   "Public. Always 204, whether or not an account matches, and the mail goes out in the background, so neither the answer nor its timing tells whether an account exists. A login with @ is an address and reaches every account whose confirmed address it is; otherwise it is a username with a confirmed address. An account without a password that signs in through the identity provider gets a hint instead. The owner is never reset by mail; at most one mail per account per 5 minutes.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{422},
	}, func(ctx context.Context, in *forgotInput) (*struct{}, error) {
		login := in.Body.Login
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), forgotTimeout)
			defer cancel()
			// No request is left to fail: a panic here would end the process.
			defer func() {
				if r := recover(); r != nil {
					slog.ErrorContext(ctx, "forgot password panicked", "panic", r)
				}
			}()
			svc.ForgotPassword(ctx, login)
		}()
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "confirm-email",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/email/confirm",
		Summary:     "Confirm an address through the link mailed to it",
		Description: "Public. Confirms only while the address is still the account's; never signs in.",
		Tags:        []string{"auth"},
		Errors:      []int{404},
	}, func(ctx context.Context, in *confirmInput) (*confirmOutput, error) {
		addr, err := svc.Confirm(ctx, in.Body.Token)
		if errors.Is(err, ErrNoConfirmation) {
			return nil, huma.Error404NotFound("this link no longer works")
		}
		if err != nil {
			return nil, err
		}
		return &confirmOutput{Body: ConfirmedAddress{Address: addr}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "resend-email-confirmation",
		Method:        http.MethodPost,
		Path:          "/api/v1/auth/me/email/confirmation",
		Summary:       "Mail a new confirmation link for the current user's address",
		Description:   "At most once a minute; the minute starts before the send, so a failed send also waits a minute. 409 when there is no address, it is confirmed, or mail is not configured.",
		Tags:          []string{"auth"},
		Security:      SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 409, 429, 502},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		err := svc.ResendConfirmation(ctx, u)
		var throttled *ThrottledError
		switch {
		case errors.Is(err, ErrNothingToConfirm):
			return nil, huma.Error409Conflict("no unconfirmed address")
		case errors.Is(err, mail.ErrDisabled):
			return nil, huma.Error409Conflict("mail is not configured")
		case errors.As(err, &throttled):
			return nil, tooManyRequests("a confirmation mail was sent a moment ago", throttled.RetryAfter)
		case errors.Is(err, mail.ErrSend):
			return nil, huma.Error502BadGateway("the confirmation mail could not be sent")
		case err != nil:
			return nil, err
		}
		return nil, nil
	})
	DeclareRetryAfter(api, http.MethodPost, "/api/v1/auth/me/email/confirmation", http.StatusTooManyRequests)
}

// DeclareRetryAfter adds the Retry-After header, in whole seconds, to the
// listed error responses of the operation registered at method and path.
// huma writes an operation's error responses from its Errors list and
// declares no headers on them, so this runs after huma.Register.
func DeclareRetryAfter(api huma.API, method, path string, statuses ...int) {
	item := api.OpenAPI().Paths[path]
	op := map[string]*huma.Operation{
		http.MethodPost:  item.Post,
		http.MethodPatch: item.Patch,
		http.MethodPut:   item.Put,
	}[method]
	for _, status := range statuses {
		resp := op.Responses[strconv.Itoa(status)]
		if resp.Headers == nil {
			resp.Headers = map[string]*huma.Header{}
		}
		resp.Headers["Retry-After"] = &huma.Header{
			Description: "Seconds to wait before trying again",
			Schema:      &huma.Schema{Type: huma.TypeInteger},
		}
	}
}

// BusyError maps a full argon2 queue (user.ErrBusy) to a 503 that asks the
// client to retry in a second, and returns nil for anything else. Every
// operation that hashes or checks a password can meet it.
func BusyError(err error) error {
	if !errors.Is(err, user.ErrBusy) {
		return nil
	}
	return huma.ErrorWithHeaders(
		huma.Error503ServiceUnavailable("too many password checks at once, try again"),
		http.Header{"Retry-After": {"1"}},
	)
}

// ProfileError maps the profile validation failures to the field they belong
// to, and returns nil for anything else. An unknown color or locale cannot
// reach it through the API - huma refuses a value outside the enum with its
// own 422 before the handler runs - but ErrInvalidColor and ErrInvalidLocale
// are mapped anyway, because the service may be called from somewhere with
// no enum in front of it.
func ProfileError(err error) error {
	switch {
	case errors.Is(err, user.ErrDisplayNameTooLong):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.displayName", Message: "display name is too long",
		})
	case errors.Is(err, user.ErrInvalidDisplayName):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.displayName", Message: "display name must not contain control characters",
		})
	case errors.Is(err, user.ErrInvalidColor):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.color", Message: "unknown color",
		})
	case errors.Is(err, user.ErrInvalidLocale):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.locale", Message: "unknown interface language",
		})
	case errors.Is(err, user.ErrInvalidEmail):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.email", Message: "not a valid email address",
		})
	}
	return nil
}

// throttledError is the 429 for a locked username.
func throttledError(e *ThrottledError) error {
	return tooManyRequests("too many failed login attempts, try again later", e.RetryAfter)
}

// tooManyRequests is a 429 whose Retry-After is wait rounded up to whole
// seconds, so a client that waits exactly that long is let through.
func tooManyRequests(msg string, wait time.Duration) error {
	seconds := int64((wait + time.Second - 1) / time.Second)
	return huma.ErrorWithHeaders(
		huma.Error429TooManyRequests(msg),
		http.Header{"Retry-After": {strconv.FormatInt(seconds, 10)}},
	)
}

// SessionCookie is the session cookie every sign-in sets: password login, a
// setup link, and an identity provider.
func SessionCookie(token string, expires time.Time, secure bool) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: Secure follows the secureCookies config flag (false only for local http dev); HttpOnly and SameSite are always set.
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredSessionCookie is a Set-Cookie value that tells the browser to
// delete the session cookie immediately.
func expiredSessionCookie(secure bool) http.Cookie {
	return expired(SessionCookie("", time.Time{}, secure))
}

// expired turns a live cookie into the Set-Cookie value that deletes it,
// keeping the attributes the browser matches it by.
func expired(c http.Cookie) http.Cookie { //nolint:gosec // G124: c keeps the Secure, HttpOnly and SameSite of the live cookie it came from.
	c.Value = ""
	c.Expires = time.Unix(0, 0)
	c.MaxAge = -1
	return c
}

// LocaleCookieName is the cookie Paraglide's cookie strategy reads. The name
// is Paraglide's own default, configured in frontend/vite.config.ts; the two
// have to agree, and this is the writing end.
const LocaleCookieName = "PARAGLIDE_LOCALE"

// LocaleCookie carries the account's interface language to the SPA, which
// resolves its locale before the first render and cannot wait for
// GET /auth/me. The users row stays the source of truth: only the service
// writes this cookie, and only from that column.
//
// Deliberately not HttpOnly - Paraglide reads it from JavaScript. It holds a
// display preference and nothing a session could be hijacked with.
func LocaleCookie(l user.Locale, secure bool) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: not HttpOnly by design (Paraglide reads it in the browser); Secure follows the secureCookies config flag and SameSite is always set.
		Name:     LocaleCookieName,
		Value:    string(l),
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredLocaleCookie clears the locale cookie on logout. The cookie names an
// account's language, and once nobody is signed in that preference no longer
// applies; the SPA's login screen falls back to Paraglide's preferredLanguage
// strategy (the browser's own Accept-Language) instead of staying stuck on
// whichever account last logged out - which matters on a shared machine.
func expiredLocaleCookie(secure bool) http.Cookie {
	return expired(LocaleCookie("", secure))
}

// respond is toResponse plus whether a confirmation link is open for the
// address. A failed lookup only leaves it false: the profile then offers to
// send one rather than claiming it was sent.
func respond(ctx context.Context, svc *Service, u user.User) UserResponse {
	r := toResponse(u)
	pending, err := svc.ConfirmationPending(ctx, u)
	if err != nil {
		slog.WarnContext(ctx, "confirmation lookup failed", "user", u.ID, "err", err)
	}
	r.EmailConfirmationPending = pending
	return r
}

func toResponse(u user.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        string(u.Role),
		Color:       string(u.Color),
		Locale:      u.Locale,
		// From the users row this request loaded, like every field here.
		CanSharePublicly: u.CanSharePublicly,
		AvatarID:         u.AvatarID,
		Email:            u.Email,
		EmailVerified:    u.EmailVerified,
		HasPassword:      u.HasPassword,
	}
}
