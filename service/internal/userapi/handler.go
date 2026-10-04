// Package userapi exposes user management (admin only) and the people list
// (every signed-in account) over HTTP. It is a separate package because it
// needs auth.UserFrom and auth.Protected, and package auth already
// imports package user.
package userapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// UserAccount is a user as the API shows it (no secrets).
type UserAccount struct {
	ID          string      `json:"id" doc:"User id"`
	Username    string      `json:"username" doc:"Login name"`
	DisplayName string      `json:"displayName" doc:"Name shown wherever the UI names this person"`
	Role        string      `json:"role" enum:"superadmin,admin,user" doc:"Authorization role"`
	Color       string      `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Palette token identifying this person"`
	Locale      user.Locale `json:"locale" doc:"The account holder's interface language"`
	// CanSharePublicly is an admin's per-person switch; see share.
	CanSharePublicly   bool       `json:"canSharePublicly" doc:"Whether this person may create public links; their existing links pause while it is off"`
	CreatedAt          time.Time  `json:"createdAt" doc:"When the account was created"`
	AvatarID           *string    `json:"avatarId" nullable:"true" doc:"The account's picture, served at /avatars/{id}/{avatarId}.jpg; null when it has none"`
	HasPassword        bool       `json:"hasPassword" doc:"False until the person sets one through a setup link or their profile"`
	SetupLinkExpiresAt *time.Time `json:"setupLinkExpiresAt,omitempty" doc:"When the open setup link expires; absent when none is open"`
	Email              string     `json:"email" doc:"Profile address; where setup links are mailed. Empty means none"`
	EmailVerified      bool       `json:"emailVerified" doc:"True once a provider vouched for it or the person redeemed a link mailed to it"`
	HasIdentity        bool       `json:"hasIdentity" doc:"Whether the person has connected an account at the configured identity provider; false while none is configured; set only by list-users"`
}

// setupLinkBody is a freshly issued setup link, shown once.
type setupLinkBody struct {
	Path      string    `json:"path" doc:"Append to the instance's origin"`
	ExpiresAt time.Time `json:"expiresAt"`
	MailedTo  string    `json:"mailedTo,omitempty" doc:"The address the link was mailed to, or that mailing it failed for; absent when no mail was tried"`
	MailError string    `json:"mailError,omitempty" enum:"send_failed" doc:"Set when mailing to mailedTo failed; the link is still valid and shown"`
}

// UserAccountList is the response body of list-users.
type UserAccountList struct {
	Items []UserAccount `json:"items"`
}

type listOutput struct {
	Body UserAccountList
}

// PersonEntry is an account as every signed-in account may see it: who takes
// part and in which role, nothing about the account itself - no language, no
// creation date. The JSON is the person a recipe's createdBy carries, plus
// the role.
type PersonEntry struct {
	ID          string  `json:"id" doc:"User id"`
	Username    string  `json:"username" doc:"Login name"`
	DisplayName string  `json:"displayName" doc:"Name shown wherever the UI names this person"`
	Color       string  `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Palette token identifying this person"`
	Role        string  `json:"role" enum:"superadmin,admin,user" doc:"Authorization role"`
	AvatarID    *string `json:"avatarId" nullable:"true" doc:"The account's picture, served at /avatars/{id}/{avatarId}.jpg; null when it has none"`
}

// PersonList is the response body of list-people.
type PersonList struct {
	Items []PersonEntry `json:"items"`
}

type peopleOutput struct {
	Body PersonList
}

type createInput struct {
	Body struct {
		Username    string       `json:"username" minLength:"1" maxLength:"64"`
		Password    string       `json:"password,omitempty" maxLength:"128" doc:"Omitted creates the account without a password and returns a setupLink the person uses to set their own"`
		Role        string       `json:"role" enum:"admin,user"`
		DisplayName *string      `json:"displayName,omitempty" maxLength:"64" doc:"Empty falls back to the login name"`
		Color       *string      `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Omitted picks the least-used color"`
		Locale      *user.Locale `json:"locale,omitempty" doc:"Interface language; defaults to REZEPTE_LOCALE"`
		Email       *string      `json:"email,omitempty" maxLength:"254" doc:"Where the setup link is mailed, when mail is configured; stored unverified"`
	}
}

type userOutput struct {
	Body UserAccount
}

// CreatedUserAccount is create-user's response body: the new account, plus
// the setup link issued for it when it was created with no password. A named
// type rather than an inline struct, so the OpenAPI document names this
// schema for what it is instead of a generated "CreateOutputBody".
type CreatedUserAccount struct {
	UserAccount
	SetupLink *setupLinkBody `json:"setupLink,omitempty"`
}

type createOutput struct {
	Body CreatedUserAccount
}

type setupLinkInput struct {
	ID string `path:"id"`
}

type issueSetupLinkInput struct {
	ID   string `path:"id"`
	Body *struct {
		Mail  *bool   `json:"mail,omitempty" doc:"false issues the link without mailing it"`
		Email *string `json:"email,omitempty" maxLength:"254" doc:"Sets the person's address first - the rank rule of a password reset; a changed address is unconfirmed - and the link is mailed there, whose redemption confirms it. With mail false a changed address gets a confirmation mail instead"`
	}
}

type setupLinkOutput struct {
	Body setupLinkBody
}

type revokeSetupLinkOutput struct{}

type updateInput struct {
	ID   string `path:"id"`
	Body struct {
		Password    *string `json:"password,omitempty" minLength:"8" maxLength:"128" doc:"New password; ends all of the user's sessions"`
		Role        *string `json:"role,omitempty" enum:"admin,user"`
		DisplayName *string `json:"displayName,omitempty" maxLength:"64" doc:"Owner only"`
		Color       *string `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Owner only"`
		// CanSharePublicly is refused for the owner's own row (the owner
		// governs public sharing for the whole instance) and for the
		// caller's own row (one admin cannot re-grant a right another admin
		// just withdrew from them).
		Email            *string `json:"email,omitempty" maxLength:"254" doc:"Same rank rule as a password reset; clears the confirmed mark when it changes and mails a confirmation; session only"`
		CanSharePublicly *bool   `json:"canSharePublicly,omitempty" doc:"Allow or withdraw creating public links; withdrawing pauses the person's existing links. Same rank rule as a password reset: only the owner reaches an admin, and nobody the owner."`
	}
}

type deleteInput struct {
	ID string `path:"id"`
}

type deleteOutput struct{}

// requireAdmin returns the calling user or a 401/403 huma error. Every
// operation in this package calls it first; there is no permission table,
// the rank of the session's user against the rank of the target is the whole
// authorization model. IsAdmin rather than a comparison against RoleAdmin,
// because the superadmin is an admin.
func requireAdmin(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if !u.Role.IsAdmin() {
		return user.User{}, huma.Error403Forbidden("admin role required")
	}
	return u, nil
}

// rankError maps the two authorization errors the user service returns.
// ErrSuperadminProtected states a fact about the instance that holds for every
// caller, which is the register ErrSelfDelete already uses; ErrSuperadminRequired
// is about who is asking. Returns nil for anything else.
func rankError(err error) error {
	switch {
	case errors.Is(err, user.ErrSuperadminProtected):
		return huma.Error409Conflict("the superadmin cannot be modified")
	case errors.Is(err, user.ErrSuperadminRequired):
		return huma.Error403Forbidden("superadmin role required")
	}
	return nil
}

func toResponse(u user.User) UserAccount {
	return UserAccount{
		ID:               u.ID,
		Username:         u.Username,
		DisplayName:      u.DisplayName,
		Role:             string(u.Role),
		Color:            string(u.Color),
		Locale:           u.Locale,
		CanSharePublicly: u.CanSharePublicly,
		CreatedAt:        u.CreatedAt,
		AvatarID:         u.AvatarID,
		HasPassword:      u.HasPassword,
		Email:            u.Email,
		EmailVerified:    u.EmailVerified,
	}
}

// Mailer is what user management needs from package mail.
type Mailer interface {
	SendInvite(context.Context, mail.Invite) error
}

// mailLink mails link to target when target has an address and mail is on,
// and records where it went: mailedTo names the address whenever a send was
// tried, mailError marks a failed one. A failure is never an error here:
// the account and the link exist either way. A nil mailer is mail off, for
// tests that do not care.
func mailLink(ctx context.Context, mailer Mailer, sessions *auth.Service, actor, target user.User, link auth.SetupLink, body *setupLinkBody) {
	if mailer == nil || target.Email == "" {
		return
	}
	err := mailer.SendInvite(ctx, mail.Invite{
		To: target.Email, Name: target.DisplayName, Inviter: actor.DisplayName,
		Path: auth.SetupPath(link.Token), Locale: target.Locale,
	})
	if errors.Is(err, mail.ErrDisabled) {
		return
	}
	if err != nil && !errors.Is(err, mail.ErrSend) {
		// Not a delivery failure (settings lookup, rendering): nothing was sent.
		slog.WarnContext(ctx, "mail invite failed", "user", target.ID, "err", err)
		return
	}
	body.MailedTo = target.Email
	if err != nil {
		body.MailError = "send_failed"
		return
	}
	// The mail went out, so it is reported even when the mark fails; the
	// only cost is that redeeming the link does not verify the address.
	// No handler gets a logger injected; slog's default is main's logger.
	if err := sessions.MarkSetupLinkSent(ctx, link.Token, target.Email); err != nil {
		slog.WarnContext(ctx, "mark setup link sent failed", "user", target.ID, "err", err)
	}
}

// confirmAddress mails a confirmation when actor's write turned before into
// an after with a new address, naming actor unless they changed their own.
func confirmAddress(ctx context.Context, sessions *auth.Service, actor, before, after user.User) {
	by := actor.DisplayName
	if actor.ID == after.ID {
		by = ""
	}
	sessions.ConfirmChangedAddress(ctx, before, after, by)
}

// Register installs list, create, update and delete for users, which require
// an admin, and list-people, which any signed-in account may read. avatars
// removes a deleted account's pictures. issuer is the configured OIDC issuer,
// empty when OIDC is off: list-users reports hasIdentity only for an identity
// there, the one sign-in and disconnect use.
func Register(api huma.API, users *user.Service, sessions *auth.Service, avatars *avatar.Service, issuer string, mailer Mailer) {
	huma.Register(api, huma.Operation{
		OperationID: "list-people",
		Method:      http.MethodGet,
		Path:        "/api/v1/people",
		Summary:     "List people",
		Description: "Every account's login name, display name, color and role, readable by any signed-in account so everyone can see who takes part. Managing accounts, and their language and creation date, is /api/v1/users, which requires an admin.",
		Tags:        []string{"users"},
		Security:    auth.Protected(auth.ScopeUsersRead),
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*peopleOutput, error) {
		if _, ok := auth.UserFrom(ctx); !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		list, err := users.List(ctx)
		if err != nil {
			return nil, err
		}
		items := make([]PersonEntry, 0, len(list))
		for _, u := range list {
			items = append(items, PersonEntry{
				ID:          u.ID,
				Username:    u.Username,
				DisplayName: u.DisplayName,
				Color:       string(u.Color),
				Role:        string(u.Role),
				AvatarID:    u.AvatarID,
			})
		}
		return &peopleOutput{Body: PersonList{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/api/v1/users",
		Summary:     "List users",
		Tags:        []string{"users"},
		Security:    auth.Protected(auth.ScopeUsersRead),
		Errors:      []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		list, err := users.List(ctx)
		if err != nil {
			return nil, err
		}
		open, err := sessions.OpenSetupLinks(ctx)
		if err != nil {
			return nil, err
		}
		linked := map[string]bool{}
		if issuer != "" {
			if linked, err = users.LinkedUserIDs(ctx, issuer); err != nil {
				return nil, err
			}
		}
		items := make([]UserAccount, 0, len(list))
		for _, u := range list {
			item := toResponse(u)
			item.HasIdentity = linked[u.ID]
			if expires, ok := open[u.ID]; ok {
				item.SetupLinkExpiresAt = &expires
			}
			items = append(items, item)
		}
		return &listOutput{Body: UserAccountList{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/api/v1/users",
		Summary:       "Create a user",
		Tags:          []string{"users"},
		Security:      auth.Protected(auth.ScopeUsersWrite),
		DefaultStatus: http.StatusCreated,
		Errors:        []int{401, 403, 409, 422, 503},
	}, func(ctx context.Context, in *createInput) (*createOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		// Create carries no authorization check of its own - that is what
		// lets the bootstrap path write the one superadmin row - so the rank
		// rule for a brand new account is checked here, where there is a
		// requested role but no target row to read.
		if err := user.CanAssignRole(actor.Role, user.Role(in.Body.Role)); err != nil {
			if mapped := rankError(err); mapped != nil {
				return nil, mapped
			}
			return nil, err
		}
		if in.Body.Password != "" && len(in.Body.Password) < 8 {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.password", Message: "expected length >= 8",
			})
		}
		// A setup link is handed over through the admin UI, which an API
		// token has no dialog to show; nothing would ever redeem it, so a
		// token creating a user has to choose a password like the old form
		// did.
		if in.Body.Password == "" && auth.ViaToken(ctx) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.password",
				Message:  "a password is required when creating an account with an API token",
			})
		}
		params := user.CreateParams{
			Username: in.Body.Username,
			Password: in.Body.Password,
			Role:     user.Role(in.Body.Role),
		}
		if in.Body.DisplayName != nil {
			params.DisplayName = *in.Body.DisplayName
		}
		if in.Body.Color != nil {
			params.Color = user.Color(*in.Body.Color)
		}
		if in.Body.Locale != nil {
			params.Locale = *in.Body.Locale
		}
		if in.Body.Email != nil {
			params.Email = *in.Body.Email
		}
		u, err := users.Create(ctx, params)
		if errors.Is(err, user.ErrUsernameTaken) {
			return nil, huma.Error409Conflict("username already taken")
		}
		if errors.Is(err, user.ErrInvalidUsername) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.username",
				Message:  "username must not be empty",
			})
		}
		if mapped := auth.ProfileError(err); mapped != nil {
			return nil, mapped
		}
		if mapped := auth.BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		out := &createOutput{}
		out.Body.UserAccount = toResponse(u)
		// With a password nothing mails a link, so the address is confirmed
		// by its own mail; a mailed setup link confirms it when redeemed.
		if in.Body.Password != "" {
			sessions.ConfirmChangedAddress(ctx, user.User{}, u, actor.DisplayName)
		}
		if in.Body.Password == "" {
			link, err := sessions.IssueSetupLink(ctx, actor, u.ID)
			if err != nil {
				return nil, err
			}
			body := setupLinkBody{Path: auth.SetupPath(link.Token), ExpiresAt: link.ExpiresAt}
			mailLink(ctx, mailer, sessions, actor, u, link, &body)
			out.Body.SetupLink = &body
			out.Body.SetupLinkExpiresAt = &link.ExpiresAt
		}
		return out, nil
	})
	auth.DeclareRetryAfter(api, http.MethodPost, "/api/v1/users", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID:   "issue-setup-link",
		Method:        http.MethodPost,
		Path:          "/api/v1/users/{id}/setup-link",
		Summary:       "Issue a one-time setup link for a user, replacing any open one",
		Tags:          []string{"users"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusCreated,
		Errors:        []int{401, 403, 404, 409, 422},
	}, func(ctx context.Context, in *issueSetupLinkInput) (*setupLinkOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		var before, after user.User
		if in.Body != nil && in.Body.Email != nil {
			before, err = users.ByID(ctx, in.ID)
			if errors.Is(err, user.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			if err != nil {
				return nil, err
			}
			after, err = users.SetEmail(ctx, actor, in.ID, *in.Body.Email)
			if mapped := rankError(err); mapped != nil {
				return nil, mapped
			}
			if mapped := auth.ProfileError(err); mapped != nil {
				return nil, mapped
			}
			if errors.Is(err, user.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			if err != nil {
				return nil, err
			}
		}
		link, err := sessions.IssueSetupLink(ctx, actor, in.ID)
		if mapped := rankError(err); mapped != nil {
			return nil, mapped
		}
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		body := setupLinkBody{Path: auth.SetupPath(link.Token), ExpiresAt: link.ExpiresAt}
		if in.Body == nil || in.Body.Mail == nil || *in.Body.Mail {
			mailLink(ctx, mailer, sessions, actor, link.Target, link, &body)
		} else {
			// No link went to a new address, so nothing else would confirm it.
			confirmAddress(ctx, sessions, actor, before, after)
		}
		return &setupLinkOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revoke-setup-link",
		Method:        http.MethodDelete,
		Path:          "/api/v1/users/{id}/setup-link",
		Summary:       "Revoke a user's open setup link",
		Tags:          []string{"users"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403, 404, 409},
	}, func(ctx context.Context, in *setupLinkInput) (*revokeSetupLinkOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		err = sessions.RevokeSetupLink(ctx, actor, in.ID)
		if mapped := rankError(err); mapped != nil {
			return nil, mapped
		}
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		return &revokeSetupLinkOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPatch,
		Path:        "/api/v1/users/{id}",
		Summary:     "Change a user's role, profile or public sharing, and/or reset the password",
		Tags:        []string{"users"},
		Security:    auth.Protected(auth.ScopeUsersWrite),
		Errors:      []int{401, 403, 404, 409, 422, 503},
	}, func(ctx context.Context, in *updateInput) (*userOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		hasProfile := in.Body.DisplayName != nil || in.Body.Color != nil
		if in.Body.Password == nil && in.Body.Role == nil && !hasProfile && in.Body.CanSharePublicly == nil && in.Body.Email == nil {
			return nil, huma.Error422UnprocessableEntity("nothing to change")
		}
		// An address is where part 2 sends password resets, so like the reset
		// it is session-only. Checked first, before anything is written.
		if in.Body.Email != nil && auth.ViaToken(ctx) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.email", Message: "an address can only be set from a signed-in session",
			})
		}
		// Renaming somebody is not administration, so it is the owner's alone -
		// a different rule from guardTarget, which still decides the role
		// change and the password reset below.
		if hasProfile {
			if err := user.CanEditProfile(actor.Role); err != nil {
				if mapped := rankError(err); mapped != nil {
					return nil, mapped
				}
				if mapped := auth.BusyError(err); mapped != nil {
					return nil, mapped
				}
				return nil, err
			}
		}
		// Every value is checked before the first write, so a refused field
		// leaves nothing half-written.
		update := user.ProfileUpdate{DisplayName: in.Body.DisplayName, Email: in.Body.Email}
		if in.Body.Color != nil {
			c := user.Color(*in.Body.Color)
			update.Color = &c
		}
		if mapped := auth.ProfileError(update.Validate()); mapped != nil {
			return nil, mapped
		}
		var u user.User
		if in.Body.Role != nil {
			u, err = users.SetRole(ctx, actor, in.ID, user.Role(*in.Body.Role))
		} else {
			u, err = users.ByID(ctx, in.ID)
		}
		if mapped := rankError(err); mapped != nil {
			return nil, mapped
		}
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		// It follows guardTarget like the role change above, the address and
		// the password reset below, so none of them can land while another is
		// refused.
		if in.Body.CanSharePublicly != nil {
			u, err = users.SetCanSharePublicly(ctx, actor, in.ID, *in.Body.CanSharePublicly)
			if mapped := rankError(err); mapped != nil {
				return nil, mapped
			}
			if errors.Is(err, user.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			if err != nil {
				return nil, err
			}
		}
		before := u
		if in.Body.Email != nil {
			u, err = users.SetEmail(ctx, actor, in.ID, *in.Body.Email)
			if mapped := rankError(err); mapped != nil {
				return nil, mapped
			}
			if mapped := auth.ProfileError(err); mapped != nil {
				return nil, mapped
			}
			if errors.Is(err, user.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			if err != nil {
				return nil, err
			}
		}
		if hasProfile {
			u, err = users.SetProfile(ctx, in.ID, user.ProfileUpdate{DisplayName: update.DisplayName, Color: update.Color})
			if mapped := auth.ProfileError(err); mapped != nil {
				return nil, mapped
			}
			if errors.Is(err, user.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			if err != nil {
				return nil, err
			}
		}
		if in.Body.Password != nil {
			if err := users.SetPassword(ctx, actor, in.ID, *in.Body.Password); err != nil {
				if mapped := rankError(err); mapped != nil {
					return nil, mapped
				}
				return nil, err
			}
			// A reset means the old credential is compromised or forgotten:
			// every device logged in with it goes.
			if err := sessions.DeleteUserSessionsExcept(ctx, in.ID, ""); err != nil {
				return nil, err
			}
		}
		// Only once the whole change went through: a refused PATCH mails nothing.
		confirmAddress(ctx, sessions, actor, before, u)
		return &userOutput{Body: toResponse(u)}, nil
	})
	auth.DeclareRetryAfter(api, http.MethodPatch, "/api/v1/users/{id}", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-user",
		Method:        http.MethodDelete,
		Path:          "/api/v1/users/{id}",
		Summary:       "Delete a user",
		Tags:          []string{"users"},
		Security:      auth.Protected(auth.ScopeUsersWrite),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403, 404, 409},
	}, func(ctx context.Context, in *deleteInput) (*deleteOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		err = users.Delete(ctx, actor, in.ID)
		if mapped := rankError(err); mapped != nil {
			return nil, mapped
		}
		switch {
		case errors.Is(err, user.ErrSelfDelete):
			return nil, huma.Error409Conflict("cannot delete yourself")
		case errors.Is(err, user.ErrNotFound):
			return nil, huma.Error404NotFound("user not found")
		case err != nil:
			return nil, err
		}
		// The row is gone, so nothing names the files any more; a leftover
		// would only cost disk, never leak, which is why a failure here does
		// not fail the delete.
		_ = avatars.RemoveAll(in.ID)
		return &deleteOutput{}, nil
	})
}
