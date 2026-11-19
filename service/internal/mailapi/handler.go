// Package mailapi is the HTTP surface of the owner's mail settings. It is
// apart from package mail because config, which the auth stack imports,
// imports mail.
package mailapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// MailSettings is the mail configuration as the owner sees it; the password
// never leaves the service, only whether one is set.
type MailSettings struct {
	Source           mail.Source `json:"source" enum:"env,settings,none" doc:"env: REZEPTE_SMTP_* pin it and the UI is read-only; settings: configured here; none: no mail"`
	Host             string      `json:"host"`
	Port             int         `json:"port"`
	Security         string      `json:"security" enum:"starttls,tls,none"`
	Username         string      `json:"username"`
	PasswordSet      bool        `json:"passwordSet"`
	From             string      `json:"from"`
	FromName         string      `json:"fromName"`
	PublicURLMissing bool        `json:"publicUrlMissing" doc:"REZEPTE_PUBLIC_URL is unset, so no mail can carry a link"`
}

// MailConfig is a configuration as the edit dialog sends it.
type MailConfig struct {
	Host     string  `json:"host" minLength:"1"`
	Port     int     `json:"port" minimum:"1" maximum:"65535"`
	Security string  `json:"security" enum:"starttls,tls,none"`
	Username string  `json:"username,omitempty"`
	Password *string `json:"password,omitempty" doc:"Omitted keeps the stored password"`
	From     string  `json:"from" minLength:"1"`
	FromName string  `json:"fromName,omitempty"`
}

func (b MailConfig) config() (mail.Config, bool) {
	c := mail.Config{Host: b.Host, Port: b.Port, Security: mail.Security(b.Security), Username: b.Username, From: b.From, FromName: b.FromName}
	if b.Password != nil {
		c.Password = *b.Password
	}
	return c, b.Password == nil
}

// SentMail is one mail in the owner's outbox: what went out, to whom and
// how it went, never what it said.
type SentMail struct {
	ID            int64        `json:"id"`
	Kind          string       `json:"kind" doc:"The template: invite, reset, hint, confirm, password_changed, email_changed, identity_linked, comment, recipe_digest, list_invite, tasty, invite_accepted or test"`
	To            string       `json:"to"`
	Subject       string       `json:"subject"`
	Status        string       `json:"status" enum:"queued,sent,failed" doc:"queued: waiting for the mail server or for a retry"`
	Attempts      int          `json:"attempts"`
	Error         string       `json:"error,omitempty" doc:"Why the last failed attempt failed, as the mail server said it; also on a mail a retry got through"`
	CreatedAt     time.Time    `json:"createdAt"`
	FinishedAt    *time.Time   `json:"finishedAt,omitempty"`
	NextAttemptAt *time.Time   `json:"nextAttemptAt,omitempty" doc:"When the next attempt is due, while a retry waits"`
	People        []MailPerson `json:"people" doc:"The accounts whose address the recipient is now; empty for an address no account holds, several when accounts share it"`
}

// MailPerson is an account a mail's address belongs to, enough to show its
// mark beside the address.
type MailPerson struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	Color       string  `json:"color"`
	AvatarID    *string `json:"avatarId,omitempty"`
}

type historyInput struct {
	Before int64  `query:"before" minimum:"0" doc:"The id of the last mail shown; the page holds the ones before it. 0 or absent: the newest."`
	Status string `query:"status" enum:"queued,sent,failed" doc:"Only the mails in this state; absent: all"`
}

type historyOutput struct {
	Body struct {
		Items  []SentMail `json:"items"`
		More   bool       `json:"more" doc:"Older mails follow; ask again with before set to the last id"`
		Total  int        `json:"total" doc:"Every mail in the outbox, across all pages"`
		Failed int        `json:"failed"`
		Queued int        `json:"queued"`
	}
}

type settingsOutput struct{ Body MailSettings }
type putInput struct{ Body MailConfig }
type testInput struct {
	Body struct {
		To     string      `json:"to" minLength:"1"`
		Config *MailConfig `json:"config,omitempty" doc:"Unsaved values to test instead of the stored configuration"`
	}
}

// mapErr answers mail's errors with their status and leaves anything else
// to be the 500 it is. A delivery failure is 502 with the server's reply; a
// refused setting is 422 at its field, prefix plus its name in MailConfig.
func mapErr(err error, prefix string) error {
	var ce *mail.ConfigError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mail.ErrSend):
		return huma.NewError(http.StatusBadGateway, mail.Detail(err))
	case errors.Is(err, mail.ErrOwnerRequired):
		return huma.Error403Forbidden("superadmin role required")
	case errors.Is(err, mail.ErrEnvLocked):
		return huma.Error409Conflict("mail is set by the server environment")
	case errors.Is(err, mail.ErrDisabled):
		return huma.Error409Conflict("mail is not configured")
	case errors.As(err, &ce):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: prefix + ce.Field, Message: ce.Reason})
	case errors.Is(err, user.ErrInvalidEmail):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.to", Message: "not a valid email address"})
	}
	return err
}

func body(ctx context.Context, s *mail.Service) (MailSettings, error) {
	c, src, err := s.Effective(ctx)
	if err != nil {
		return MailSettings{}, err
	}
	return MailSettings{Source: src, Host: c.Host, Port: c.Port, Security: string(c.Security),
		Username: c.Username, PasswordSet: c.Password != "", From: c.From, FromName: c.FromName,
		PublicURLMissing: s.PublicURL() == ""}, nil
}

// Register installs the owner's mail settings operations. users names the
// people an outbox mail went to.
func Register(api huma.API, s *mail.Service, users *user.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-mail-settings", Method: http.MethodGet, Path: "/api/v1/settings/mail",
		Summary: "Get the mail configuration", Tags: []string{"settings"},
		Security: auth.SessionSecurity, Errors: []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*settingsOutput, error) {
		if _, err := auth.RequireRole(ctx, user.RoleSuperadmin); err != nil {
			return nil, err
		}
		b, err := body(ctx, s)
		return &settingsOutput{Body: b}, err
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-mail-settings", Method: http.MethodPut, Path: "/api/v1/settings/mail",
		Summary: "Configure mail", Tags: []string{"settings"},
		Security: auth.SessionSecurity, Errors: []int{401, 403, 409, 422},
	}, func(ctx context.Context, in *putInput) (*settingsOutput, error) {
		u, err := auth.RequireRole(ctx, user.RoleSuperadmin)
		if err != nil {
			return nil, err
		}
		c, keep := in.Body.config()
		if err := s.Save(ctx, u, c, keep); err != nil {
			return nil, mapErr(err, "body.")
		}
		b, err := body(ctx, s)
		return &settingsOutput{Body: b}, err
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-mail-settings", Method: http.MethodDelete, Path: "/api/v1/settings/mail",
		Summary: "Turn mail off", Tags: []string{"settings"},
		Security: auth.SessionSecurity, DefaultStatus: http.StatusNoContent, Errors: []int{401, 403, 409},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		u, err := auth.RequireRole(ctx, user.RoleSuperadmin)
		if err != nil {
			return nil, err
		}
		if err := s.Clear(ctx, u); err != nil {
			return nil, mapErr(err, "")
		}
		return &struct{}{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "send-test-mail", Method: http.MethodPost, Path: "/api/v1/settings/mail/test",
		Summary: "Send a test mail", Tags: []string{"settings"},
		Description: "With config, tests those unsaved values (a missing password uses the stored one, for the stored host and username only); without, the configuration in force. A refusal by the SMTP server answers 502 with the server's reply.",
		Security:    auth.SessionSecurity, DefaultStatus: http.StatusNoContent, Errors: []int{401, 403, 409, 422, 502},
	}, func(ctx context.Context, in *testInput) (*struct{}, error) {
		u, err := auth.RequireRole(ctx, user.RoleSuperadmin)
		if err != nil {
			return nil, err
		}
		var override *mail.Config
		keep := false
		if in.Body.Config != nil {
			c, k := in.Body.Config.config()
			override, keep = &c, k
		}
		if err := s.SendTest(ctx, u, in.Body.To, override, keep); err != nil {
			return nil, mapErr(err, "body.config.")
		}
		return &struct{}{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-sent-mails", Method: http.MethodGet, Path: "/api/v1/settings/mail/history",
		Summary: "List the outbox", Tags: []string{"settings"},
		Description: "The mails Rezepte handed to its mail server in the last 30 days, newest first, 50 per page: kind, recipient, subject and outcome, never the content.",
		Security:    auth.SessionSecurity, Errors: []int{401, 403},
	}, func(ctx context.Context, in *historyInput) (*historyOutput, error) {
		if _, err := auth.RequireRole(ctx, user.RoleSuperadmin); err != nil {
			return nil, err
		}
		box, err := s.History(ctx, in.Before, in.Status)
		if err != nil {
			return nil, err
		}
		// ponytail: matches the address as accounts hold it now, not as they
		// did when the mail went out; a user_id on mail_log if that matters.
		all, err := users.List(ctx)
		if err != nil {
			return nil, err
		}
		byAddress := map[string][]MailPerson{}
		for _, u := range all {
			if u.Email != "" {
				key := strings.ToLower(u.Email)
				byAddress[key] = append(byAddress[key], MailPerson{
					ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Color: string(u.Color), AvatarID: u.AvatarID,
				})
			}
		}
		out := &historyOutput{}
		out.Body.Items = make([]SentMail, len(box.Items))
		for i, m := range box.Items {
			people := byAddress[strings.ToLower(m.To)]
			if people == nil {
				people = []MailPerson{}
			}
			out.Body.Items[i] = SentMail{ID: m.ID, Kind: m.Kind, To: m.To, Subject: m.Subject, Status: m.Status,
				Attempts: m.Attempts, Error: m.Error, CreatedAt: m.CreatedAt, FinishedAt: m.FinishedAt,
				NextAttemptAt: m.NextAttemptAt, People: people}
		}
		out.Body.More, out.Body.Total, out.Body.Failed, out.Body.Queued = box.More, box.Total, box.Failed, box.Queued
		return out, nil
	})
}
