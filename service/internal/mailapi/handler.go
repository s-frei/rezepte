// Package mailapi is the HTTP surface of the owner's mail settings. It is
// apart from package mail because config, which the auth stack imports,
// imports mail.
package mailapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"

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

type settingsOutput struct{ Body MailSettings }
type putInput struct{ Body MailConfig }
type testInput struct {
	Body struct {
		To     string      `json:"to" minLength:"1"`
		Config *MailConfig `json:"config,omitempty" doc:"Unsaved values to test instead of the stored configuration"`
	}
}

func owner(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if !u.Role.IsSuperadmin() {
		return user.User{}, huma.Error403Forbidden("superadmin role required")
	}
	return u, nil
}

// mapErr answers mail's errors with their status and leaves anything else
// to be the 500 it is. A delivery failure is 502 with the server's reply.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mail.ErrSend):
		detail := err.Error()
		// textproto.Error prints its text quoted; the owner reads it unquoted.
		if te := new(textproto.Error); errors.As(err, &te) {
			detail = strings.Replace(detail, te.Error(), fmt.Sprintf("%d %s", te.Code, te.Msg), 1)
		}
		return huma.NewError(http.StatusBadGateway, detail)
	case errors.Is(err, mail.ErrOwnerRequired):
		return huma.Error403Forbidden("superadmin role required")
	case errors.Is(err, mail.ErrEnvLocked):
		return huma.Error409Conflict("mail is set by the server environment")
	case errors.Is(err, mail.ErrDisabled):
		return huma.Error409Conflict("mail is not configured")
	case errors.Is(err, mail.ErrInvalidConfig):
		return huma.Error422UnprocessableEntity(err.Error())
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

// Register installs the owner's mail settings operations.
func Register(api huma.API, s *mail.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-mail-settings", Method: http.MethodGet, Path: "/api/v1/settings/mail",
		Summary: "Get the mail configuration", Tags: []string{"settings"},
		Security: auth.SessionSecurity, Errors: []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*settingsOutput, error) {
		if _, err := owner(ctx); err != nil {
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
		u, err := owner(ctx)
		if err != nil {
			return nil, err
		}
		c, keep := in.Body.config()
		if err := s.Save(ctx, u, c, keep); err != nil {
			return nil, mapErr(err)
		}
		b, err := body(ctx, s)
		return &settingsOutput{Body: b}, err
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-mail-settings", Method: http.MethodDelete, Path: "/api/v1/settings/mail",
		Summary: "Turn mail off", Tags: []string{"settings"},
		Security: auth.SessionSecurity, DefaultStatus: http.StatusNoContent, Errors: []int{401, 403, 409},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		u, err := owner(ctx)
		if err != nil {
			return nil, err
		}
		if err := s.Clear(ctx, u); err != nil {
			return nil, mapErr(err)
		}
		return &struct{}{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "send-test-mail", Method: http.MethodPost, Path: "/api/v1/settings/mail/test",
		Summary: "Send a test mail", Tags: []string{"settings"},
		Description: "With config, tests those unsaved values (a missing password uses the stored one, for the stored host and username only); without, the configuration in force. A refusal by the SMTP server answers 502 with the server's reply.",
		Security:    auth.SessionSecurity, DefaultStatus: http.StatusNoContent, Errors: []int{401, 403, 409, 422, 502},
	}, func(ctx context.Context, in *testInput) (*struct{}, error) {
		u, err := owner(ctx)
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
			return nil, mapErr(err)
		}
		return &struct{}{}, nil
	})
}
