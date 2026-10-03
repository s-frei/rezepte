package mail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/textproto"
	"strings"
	"time"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

var (
	// ErrEnvLocked means REZEPTE_SMTP_* configure mail; the UI cannot change it.
	ErrEnvLocked = errors.New("mail is configured by the server environment")
	// ErrOwnerRequired means someone other than the superadmin tried to
	// configure mail.
	ErrOwnerRequired = errors.New("only the superadmin can configure mail")
	// ErrDisabled means there is no server, or no REZEPTE_PUBLIC_URL to link to.
	ErrDisabled = errors.New("mail is not configured")
	// ErrPasswordRequired means a kept password was asked for while the host
	// or the username changed: the stored one never goes to another server.
	ErrPasswordRequired = fmt.Errorf("%w: enter the password for a different server", ErrInvalidConfig)
)

// Source says where the effective configuration comes from.
type Source string

// The values of Source.
const (
	SourceEnv      Source = "env"
	SourceSettings Source = "settings"
	SourceNone     Source = "none"
)

// Service resolves the configuration and sends. It is the one place that
// decides "environment or database": everything else asks Effective.
type Service struct {
	q         *sqlc.Queries
	env       Config
	publicURL string
	logger    *slog.Logger
	now       func() time.Time
}

// NewService returns a Service; env is the configuration REZEPTE_SMTP_* pin
// (zero when unset), publicURL is REZEPTE_PUBLIC_URL.
func NewService(conn *sql.DB, env Config, publicURL string, logger *slog.Logger) *Service {
	return &Service{q: sqlc.New(conn), env: env, publicURL: publicURL, logger: logger, now: time.Now}
}

// PublicURL is the origin mailed links start with; empty disables mail.
func (s *Service) PublicURL() string { return s.publicURL }

// Effective returns the configuration in force and where it comes from.
// With SourceNone it is the stored row's defaults (port 587, starttls), the
// values a first edit starts from.
func (s *Service) Effective(ctx context.Context) (Config, Source, error) {
	if s.env.Configured() {
		return s.env, SourceEnv, nil
	}
	c, err := s.stored(ctx)
	if err != nil {
		return Config{}, SourceNone, err
	}
	if !c.Configured() {
		return c, SourceNone, nil
	}
	return c, SourceSettings, nil
}

func (s *Service) stored(ctx context.Context) (Config, error) {
	row, err := s.q.GetMailSettings(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("get mail settings: %w", err)
	}
	return Config{
		Host: row.SmtpHost, Port: int(row.SmtpPort), Security: Security(row.SmtpSecurity),
		Username: row.SmtpUsername, Password: row.SmtpPassword,
		From: row.SmtpFrom, FromName: row.SmtpFromName,
	}, nil
}

// Enabled reports whether a mail could be sent right now.
func (s *Service) Enabled(ctx context.Context) bool {
	_, src, err := s.Effective(ctx)
	return err == nil && src != SourceNone && s.publicURL != ""
}

func (s *Service) guardWrite(actor user.User) error {
	if !actor.Role.IsSuperadmin() {
		return ErrOwnerRequired
	}
	if s.env.Configured() {
		return ErrEnvLocked
	}
	return nil
}

// withStoredPassword fills c.Password from the database when keep is set,
// but only for the server and username it was stored for. An empty username
// means no sign-in, so nothing is kept.
func (s *Service) withStoredPassword(ctx context.Context, c Config, keep bool) (Config, error) {
	if !keep {
		return c, nil
	}
	st, err := s.stored(ctx)
	if err != nil {
		return Config{}, err
	}
	if c.Username == "" {
		// The new server needs no sign-in, so there is no password to keep.
		c.Password = ""
		return c, nil
	}
	if st.Password != "" && (st.Host != c.Host || st.Username != c.Username) {
		return Config{}, ErrPasswordRequired
	}
	c.Password = st.Password
	return c, nil
}

// normalize trims what a form leaves padded and names the sender when the
// owner left the name empty.
func normalize(c Config) Config {
	c.Host, c.From = strings.TrimSpace(c.Host), strings.TrimSpace(c.From)
	if strings.TrimSpace(c.FromName) == "" {
		c.FromName = "Rezepte"
	}
	return c
}

// Save stores c. keepPassword leaves the stored password as it is, which is
// what a form that never shows the password sends when it was not retyped.
func (s *Service) Save(ctx context.Context, actor user.User, c Config, keepPassword bool) error {
	if err := s.guardWrite(actor); err != nil {
		return err
	}
	c = normalize(c)
	if err := c.Validate(); err != nil {
		return err
	}
	c, err := s.withStoredPassword(ctx, c, keepPassword)
	if err != nil {
		return err
	}
	if err := s.q.SetMailSettings(ctx, sqlc.SetMailSettingsParams{
		SmtpHost: c.Host, SmtpPort: int64(c.Port), SmtpSecurity: string(c.Security),
		SmtpUsername: c.Username, SmtpPassword: c.Password, SmtpFrom: c.From, SmtpFromName: c.FromName,
	}); err != nil {
		return fmt.Errorf("set mail settings: %w", err)
	}
	return nil
}

// Clear turns mail off.
func (s *Service) Clear(ctx context.Context, actor user.User) error {
	if err := s.guardWrite(actor); err != nil {
		return err
	}
	if err := s.q.ClearMailSettings(ctx); err != nil {
		return fmt.Errorf("clear mail settings: %w", err)
	}
	return nil
}

// SendTest mails a test to to, through override when given (the edit
// dialog's unsaved values) or the effective configuration. An override is
// refused while the environment pins mail.
func (s *Service) SendTest(ctx context.Context, actor user.User, to string, override *Config, keepPassword bool) error {
	if !actor.Role.IsSuperadmin() {
		return ErrOwnerRequired
	}
	addr, err := user.ParseEmail(to)
	if err != nil || addr == "" {
		return user.ErrInvalidEmail
	}
	cfg, src, err := s.Effective(ctx)
	if err != nil {
		return err
	}
	if override != nil {
		if src == SourceEnv {
			return ErrEnvLocked
		}
		o := normalize(*override)
		if err := o.Validate(); err != nil {
			return err
		}
		if cfg, err = s.withStoredPassword(ctx, o, keepPassword); err != nil {
			return err
		}
	} else if src == SourceNone {
		return ErrDisabled
	}
	env, err := BuildTest(cfg, addr, actor.Locale, s.publicURL, s.now())
	if err != nil {
		return err
	}
	return Send(ctx, cfg, env)
}

// SendInvite mails a setup link; in.Path is appended to REZEPTE_PUBLIC_URL.
// ErrDisabled means mail is off and nothing was tried. A failure is returned
// and logged with the recipient's domain and, for a server refusal, its
// reply code only: the error text can quote the address, and the link is
// never logged.
func (s *Service) SendInvite(ctx context.Context, in Invite) error {
	cfg, src, err := s.Effective(ctx)
	if err != nil {
		return err
	}
	if src == SourceNone || s.publicURL == "" {
		return ErrDisabled
	}
	env, err := BuildInvite(cfg, in, s.publicURL, s.now())
	if err != nil {
		return err
	}
	if err := Send(ctx, cfg, env); err != nil {
		reason := any("send failed")
		var reply *textproto.Error
		if errors.As(err, &reply) {
			reason = reply.Code
		}
		s.logger.Warn("mail: setup link not sent", "domain", in.To[strings.LastIndex(in.To, "@")+1:], "reason", reason)
		return err
	}
	return nil
}
