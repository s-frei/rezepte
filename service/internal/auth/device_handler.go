package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/user"
)

// DeviceAccount is one other account signed in on this device, as
// list-device-accounts reports it.
type DeviceAccount struct {
	ID          string  `json:"id"`
	Username    string  `json:"username" doc:"Login name, which the login page fills in for an expired account"`
	DisplayName string  `json:"displayName"`
	Color       string  `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate"`
	AvatarID    *string `json:"avatarId" nullable:"true" doc:"The account's picture, served at /avatars/{id}/{avatarId}.jpg; null when it has none"`
	Expired     bool    `json:"expired" doc:"The session on this device ran out or was ended; switching needs the password again"`
	// LastSeenAt is read from the session row, which is gone once the
	// session was ended: then it is the zero time, under expired.
	LastSeenAt time.Time `json:"lastSeenAt" doc:"When this account last used its session on this device, to the day; the zero time when that session no longer exists"`
}

type listDeviceAccountsOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      []DeviceAccount
}

type switchAccountInput struct {
	Session  string `cookie:"rezepte_session"`
	Accounts string `cookie:"rezepte_accounts"`
	Body     struct {
		UserID string `json:"userId" minLength:"1" maxLength:"64"`
	}
}

type removeDeviceAccountInput struct {
	Session  string `cookie:"rezepte_session"`
	Accounts string `cookie:"rezepte_accounts"`
	UserID   string `path:"userId" maxLength:"64"`
}

// RegisterDevice installs the operations on the other accounts of a device:
// list them, switch to one, remove one.
//
// They are public, not SessionSecurity: the switcher has to work with nobody
// active, after a sign-out or once the active session ran out. Their
// credential is the accounts cookie itself, whose entries only this server
// can sign and each of which carries a real session token; switching is
// using that token. checkOrigin covers the POST and DELETE.
func RegisterDevice(api huma.API, svc *Service, device *Device, secureCookies bool) {
	huma.Register(api, huma.Operation{
		OperationID: "list-device-accounts",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/accounts",
		Summary:     "List the other accounts signed in on this device",
		Description: "Public; reads the device's accounts cookie. Every account except the active session's, in the order they were last active here. Expiry is read without renewing a session. An account that no longer exists, or an entry this server did not sign, is dropped and the cookie rewritten.",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, in *logoutInput) (*listDeviceAccountsOutput, error) {
		active, err := svc.ActiveUserID(ctx, in.Session)
		if err != nil {
			return nil, err
		}
		entries := device.Parse(in.Accounts)
		out := &listDeviceAccountsOutput{Body: []DeviceAccount{}}
		var kept []DeviceEntry
		for _, e := range entries {
			// The active person's own entry stays in the cookie, not the list.
			if e.UserID == active {
				kept = append(kept, e)
				continue
			}
			u, err := svc.users.ByID(ctx, e.UserID)
			if errors.Is(err, user.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			p, err := svc.Peek(ctx, e.Token)
			if err != nil {
				return nil, err
			}
			// A token of someone else under this user's name is forged.
			if p.UserID != "" && p.UserID != e.UserID {
				continue
			}
			kept = append(kept, e)
			out.Body = append(out.Body, DeviceAccount{
				ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Color: string(u.Color), AvatarID: u.AvatarID,
				Expired: !p.Valid, LastSeenAt: p.LastSeen,
			})
		}
		// Every raw entry is one "~"-separated part; fewer kept means
		// something was dropped.
		if in.Accounts != "" && len(kept) != strings.Count(in.Accounts, "~")+1 {
			out.SetCookie = []http.Cookie{device.Cookie(kept, secureCookies)}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "switch-account",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/switch",
		Summary:     "Make another account on this device the active one",
		Description: "Public; reads the device's accounts cookie. The chosen account's session becomes the session cookie and goes first in the accounts cookie, which keeps the previous one. Answers like login. 404 when the account is not on this device, 409 when its session here ran out or was ended: it needs its password again.",
		Tags:        []string{"auth"},
		Errors:      []int{404, 409},
	}, func(ctx context.Context, in *switchAccountInput) (*loginOutput, error) {
		entries := device.Parse(in.Accounts)
		i := slices.IndexFunc(entries, func(e DeviceEntry) bool { return e.UserID == in.Body.UserID })
		if i < 0 {
			return nil, huma.Error404NotFound("account not on this device")
		}
		// A deleted account took its sessions along; it is gone, not expired.
		if _, err := svc.users.ByID(ctx, in.Body.UserID); errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("account no longer exists")
		} else if err != nil {
			return nil, err
		}
		v, err := svc.Authenticate(ctx, entries[i].Token)
		if errors.Is(err, ErrNoSession) {
			return nil, huma.Error409Conflict("the session on this device has ended, sign in again")
		}
		if err != nil {
			return nil, err
		}
		// The token must be the entry's own; anyone else's is not on this device.
		if v.User.ID != entries[i].UserID {
			return nil, huma.Error404NotFound("account not on this device")
		}
		accounts, err := device.SignedIn(ctx, svc, in.Session, entries, entries[i], secureCookies)
		if err != nil {
			return nil, err
		}
		return &loginOutput{
			SetCookie: []http.Cookie{
				SessionCookie(entries[i].Token, v.ExpiresAt, secureCookies),
				LocaleCookie(v.User.Locale, secureCookies),
				accounts,
			},
			Body: respond(ctx, svc, v.User),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "remove-device-account",
		Method:        http.MethodDelete,
		Path:          "/api/v1/auth/accounts/{userId}",
		Summary:       "Remove an account from this device",
		Description:   "Public; reads the device's accounts cookie. Ends that account's session on this device and drops it from the cookie; the account itself stays. 404 when it is not on this device, 409 for the active account, which signs out instead.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{404, 409},
	}, func(ctx context.Context, in *removeDeviceAccountInput) (*logoutOutput, error) {
		entries := device.Parse(in.Accounts)
		i := slices.IndexFunc(entries, func(e DeviceEntry) bool { return e.UserID == in.UserID })
		if i < 0 {
			return nil, huma.Error404NotFound("account not on this device")
		}
		active, err := svc.ActiveUserID(ctx, in.Session)
		if err != nil {
			return nil, err
		}
		if in.UserID == active {
			return nil, huma.Error409Conflict("the active account signs out instead")
		}
		rest := Handover(entries, nil, in.UserID)
		if err := endDropped(ctx, svc, entries, rest); err != nil {
			return nil, err
		}
		return &logoutOutput{SetCookie: []http.Cookie{device.Cookie(rest, secureCookies)}}, nil
	})
}

// ActiveUserID is the user of a still valid session token, read without
// using it; empty for none.
func (s *Service) ActiveUserID(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	p, err := s.Peek(ctx, token)
	if err != nil || !p.Valid {
		return "", err
	}
	return p.UserID, nil
}

// SignedIn is the accounts cookie for a device on which active just became
// the active account: active goes first, with its new token. session is the
// session cookie the request came with; the person it belongs to, while it
// is valid and someone else, follows with that token. Everyone else stays,
// also a previous person whose session cookie the browser already dropped:
// their entry was written when they became active. Every sign-in hands over
// this way: password, setup link, identity provider and a switch.
func (d *Device) SignedIn(ctx context.Context, svc *Service, session string, entries []DeviceEntry, active DeviceEntry, secure bool) (http.Cookie, error) {
	prevID, err := svc.ActiveUserID(ctx, session)
	if err != nil {
		return http.Cookie{}, err
	}
	var prev *DeviceEntry
	if prevID != "" {
		prev = &DeviceEntry{UserID: prevID, Token: session}
	}
	next := append([]DeviceEntry{active}, Handover(entries, prev, active.UserID)...)
	if err := endDropped(ctx, svc, append(entries, DeviceEntry{Token: session}), next); err != nil {
		return http.Cookie{}, err
	}
	return d.Cookie(next, secure), nil
}

// endDropped ends the session of every token in before that after no
// longer holds: the cookie was its only way back to this device, so nobody
// could use or end it anymore. The active person's older token when they
// sign in again goes this way, and so does whatever the cap cuts. A token
// after still holds is never ended.
func endDropped(ctx context.Context, svc *Service, before, after []DeviceEntry) error {
	for _, b := range before {
		if b.Token == "" || slices.ContainsFunc(after, func(a DeviceEntry) bool { return a.Token == b.Token }) {
			continue
		}
		if err := svc.Logout(ctx, b.Token); err != nil {
			return err
		}
	}
	return nil
}
