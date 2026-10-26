package oidc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

var (
	// ErrOwnerRequired means someone other than the superadmin tried to
	// configure sign-in.
	ErrOwnerRequired = errors.New("only the superadmin can configure sign-in")
	// ErrEnvLocked means REZEPTE_OIDC_* configure sign-in; the UI cannot change it.
	ErrEnvLocked = errors.New("sign-in is configured by the server environment")
	// ErrPublicURLMissing means REZEPTE_PUBLIC_URL is unset, so there is no
	// redirect URI to register at a provider.
	ErrPublicURLMissing = errors.New("REZEPTE_PUBLIC_URL is not set")
	// ErrInvalidConfig means a required field is missing.
	ErrInvalidConfig = errors.New("invalid sign-in configuration")
	// ErrDiscovery means no OpenID provider answered at the issuer; it wraps
	// the cause.
	ErrDiscovery = errors.New("no OpenID provider answers at this address")
	// ErrSecretRequired means a kept secret was asked for while the issuer or
	// the client ID changed: the stored one never goes to another provider.
	ErrSecretRequired = fmt.Errorf("%w: enter the secret for a different provider", ErrInvalidConfig)
)

// Source says where the effective configuration comes from.
type Source string

// The values of Source.
const (
	SourceEnv      Source = "env"
	SourceSettings Source = "settings"
	SourceNone     Source = "none"
)

// defaultName is the button's provider name when none is configured.
const defaultName = "single sign-on"

// Settings resolves the provider in force. It is the one place that decides
// "environment or database": everything else asks Effective.
type Settings struct {
	q   *sqlc.Queries
	env Config
	// discover runs OIDC discovery and returns the issuer the provider
	// confirmed; tests swap it.
	discover func(ctx context.Context, issuer string) (string, error)
}

// NewSettings returns Settings; env is the configuration REZEPTE_OIDC_* pin
// (zero issuer when unset), env.PublicURL is REZEPTE_PUBLIC_URL either way.
func NewSettings(conn *sql.DB, env Config) *Settings {
	return &Settings{q: sqlc.New(conn), env: env, discover: discover}
}

// discover asks the provider at issuer for its configuration. go-oidc refuses
// a document whose issuer differs from the one asked, so a typed trailing
// slash the provider does not use is retried without it.
func discover(ctx context.Context, issuer string) (string, error) {
	ctx = gooidc.ClientContext(ctx, &http.Client{Timeout: 10 * time.Second})
	p, err := gooidc.NewProvider(ctx, issuer)
	if err != nil && strings.HasSuffix(issuer, "/") {
		p, err = gooidc.NewProvider(ctx, strings.TrimSuffix(issuer, "/"))
	}
	if err != nil {
		return "", err
	}
	var doc struct {
		Issuer string `json:"issuer"`
	}
	if err := p.Claims(&doc); err != nil {
		return "", err
	}
	return doc.Issuer, nil
}

// Effective returns the configuration in force and where it comes from.
// PublicURL is always filled; with SourceNone nothing else is.
func (s *Settings) Effective(ctx context.Context) (Config, Source, error) {
	c, src, err := s.raw(ctx)
	if src != SourceNone && c.Name == "" {
		c.Name = defaultName
	}
	return c, src, err
}

// raw is Effective without the default name, which the settings form shows
// as it was stored.
func (s *Settings) raw(ctx context.Context) (Config, Source, error) {
	if s.env.Issuer != "" {
		return s.env, SourceEnv, nil
	}
	c, err := s.stored(ctx)
	if err != nil {
		return Config{PublicURL: s.env.PublicURL}, SourceNone, err
	}
	if c.Issuer == "" {
		return Config{PublicURL: s.env.PublicURL}, SourceNone, nil
	}
	c.PublicURL = s.env.PublicURL
	return c, SourceSettings, nil
}

func (s *Settings) stored(ctx context.Context) (Config, error) {
	row, err := s.q.GetOIDCSettings(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("get oidc settings: %w", err)
	}
	return Config{Issuer: row.OidcIssuer, ClientID: row.OidcClientID, ClientSecret: row.OidcClientSecret, Name: row.OidcName}, nil
}

// Enabled reports whether sign-in through the provider is offered right now.
func (s *Settings) Enabled(ctx context.Context) bool {
	c, src, err := s.Effective(ctx)
	return err == nil && src != SourceNone && c.PublicURL != ""
}

func (s *Settings) guardWrite(actor user.User) error {
	if !actor.Role.IsSuperadmin() {
		return ErrOwnerRequired
	}
	if s.env.Issuer != "" {
		return ErrEnvLocked
	}
	return nil
}

// Save stores c after the provider at c.Issuer answered discovery, under the
// issuer the provider confirmed. secret nil keeps the stored secret, but only
// for the same issuer and client ID; "" removes it (a public client); any
// other value replaces it.
func (s *Settings) Save(ctx context.Context, actor user.User, c Config, secret *string) error {
	if err := s.guardWrite(actor); err != nil {
		return err
	}
	if s.env.PublicURL == "" {
		return ErrPublicURLMissing
	}
	c.Issuer, c.ClientID, c.Name = strings.TrimSpace(c.Issuer), strings.TrimSpace(c.ClientID), strings.TrimSpace(c.Name)
	if c.Issuer == "" || c.ClientID == "" {
		return fmt.Errorf("%w: issuer and client ID are required", ErrInvalidConfig)
	}
	issuer, err := s.discover(ctx, c.Issuer)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDiscovery, err)
	}
	c.Issuer = issuer
	if secret != nil {
		c.ClientSecret = *secret
	} else {
		st, err := s.stored(ctx)
		if err != nil {
			return err
		}
		if st.ClientSecret != "" && (st.Issuer != c.Issuer || st.ClientID != c.ClientID) {
			return ErrSecretRequired
		}
		c.ClientSecret = st.ClientSecret
	}
	if err := s.q.SetOIDCSettings(ctx, sqlc.SetOIDCSettingsParams{
		OidcIssuer: c.Issuer, OidcClientID: c.ClientID, OidcClientSecret: c.ClientSecret, OidcName: c.Name,
	}); err != nil {
		return fmt.Errorf("set oidc settings: %w", err)
	}
	return nil
}

// Clear turns sign-in through a provider off. Linked identities stay.
func (s *Settings) Clear(ctx context.Context, actor user.User) error {
	if err := s.guardWrite(actor); err != nil {
		return err
	}
	if err := s.q.ClearOIDCSettings(ctx); err != nil {
		return fmt.Errorf("clear oidc settings: %w", err)
	}
	return nil
}

// Connected counts the accounts linked at the effective issuer, and how many
// of them have no password; zeros when sign-in is off.
func (s *Settings) Connected(ctx context.Context) (total, withoutPassword int, err error) {
	c, src, err := s.Effective(ctx)
	if err != nil || src == SourceNone {
		return 0, 0, err
	}
	row, err := s.q.CountIdentities(ctx, c.Issuer)
	if err != nil {
		return 0, 0, fmt.Errorf("count identities: %w", err)
	}
	return int(row.Total), int(row.WithoutPassword), nil
}
