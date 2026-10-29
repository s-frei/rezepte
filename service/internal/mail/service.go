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
	reporter  DeliveryReporter
}

// DeliveryReporter hears how real sends go, so the owner can be warned
// when mail stops getting out. MailFailed gets a setup failure only (see
// setupError) with its reason; MailWorked follows every accepted send;
// MailReconfigured follows a Save or Clear. The test send never reports:
// the owner reads its error on screen.
type DeliveryReporter interface {
	MailFailed(ctx context.Context, reason string)
	MailWorked(ctx context.Context)
	MailReconfigured(ctx context.Context)
}

// SetReporter makes s report its sends to r; nil, the default, reports
// nothing. Call it before serving.
func (s *Service) SetReporter(r DeliveryReporter) { s.reporter = r }

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
	if s.reporter != nil {
		s.reporter.MailReconfigured(ctx)
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
	if s.reporter != nil {
		s.reporter.MailReconfigured(ctx)
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
func (s *Service) SendInvite(ctx context.Context, in Invite) error {
	return s.deliver(ctx, in.To, "setup link", func(cfg Config) (Envelope, error) {
		return BuildInvite(cfg, in, s.publicURL, s.now())
	})
}

// SendReset mails a password reset link.
func (s *Service) SendReset(ctx context.Context, in Reset) error {
	return s.deliver(ctx, in.To, "password reset", func(cfg Config) (Envelope, error) {
		return BuildReset(cfg, in, s.publicURL, s.now())
	})
}

// SendHint mails the sign-in hint for an account without a password.
func (s *Service) SendHint(ctx context.Context, in Hint) error {
	return s.deliver(ctx, in.To, "sign-in hint", func(cfg Config) (Envelope, error) {
		return BuildHint(cfg, in, s.publicURL, s.now())
	})
}

// SendConfirm mails an address confirmation link.
func (s *Service) SendConfirm(ctx context.Context, in Confirm) error {
	return s.deliver(ctx, in.To, "address confirmation", func(cfg Config) (Envelope, error) {
		return BuildConfirm(cfg, in, s.publicURL, s.now())
	})
}

// SendPasswordChanged mails the notice that a password changed.
func (s *Service) SendPasswordChanged(ctx context.Context, in PasswordChanged) error {
	return s.deliver(ctx, in.To, "password notice", func(cfg Config) (Envelope, error) {
		return BuildPasswordChanged(cfg, in, s.publicURL, s.now())
	})
}

// SendEmailChanged mails the notice that an address changed to the old one.
func (s *Service) SendEmailChanged(ctx context.Context, in EmailChanged) error {
	return s.deliver(ctx, in.To, "address notice", func(cfg Config) (Envelope, error) {
		return BuildEmailChanged(cfg, in, s.publicURL, s.now())
	})
}

// SendIdentityLinked mails the notice that a sign-in identity was linked.
func (s *Service) SendIdentityLinked(ctx context.Context, in IdentityLinked) error {
	return s.deliver(ctx, in.To, "sign-in method notice", func(cfg Config) (Envelope, error) {
		return BuildIdentityLinked(cfg, in, s.publicURL, s.now())
	})
}

// deliver renders a mail with build and sends it to to. ErrDisabled means
// mail is off and nothing was tried. A failure is returned and logged with
// the recipient's domain and, for a server refusal, its reply code only: the
// error text can quote the address, and a link is never logged. The
// reporter hears of a setup failure and of every accepted send.
func (s *Service) deliver(ctx context.Context, to, what string, build func(Config) (Envelope, error)) error {
	cfg, src, err := s.Effective(ctx)
	if err != nil {
		return err
	}
	if src == SourceNone || s.publicURL == "" {
		return ErrDisabled
	}
	env, err := build(cfg)
	if err != nil {
		return err
	}
	if err := Send(ctx, cfg, env); err != nil {
		reason := any("send failed")
		var reply *textproto.Error
		if errors.As(err, &reply) {
			reason = reply.Code
		}
		s.logger.Warn("mail: "+what+" not sent", "domain", to[strings.LastIndex(to, "@")+1:], "reason", reason)
		if why := setupFailure(err); why != "" && s.reporter != nil {
			s.reporter.MailFailed(ctx, why)
		}
		return err
	}
	if s.reporter != nil {
		s.reporter.MailWorked(ctx)
	}
	return nil
}

// LogUnsent logs a send's err unless it is nil, ErrDisabled or a refused
// send, which deliver already logged; with userID, never the address.
func LogUnsent(ctx context.Context, msg, userID string, err error) {
	if err != nil && !errors.Is(err, ErrDisabled) && !errors.Is(err, ErrSend) {
		slog.WarnContext(ctx, msg, "user", userID, "err", err)
	}
}

// SendComment mails a follower of a recipe about a new comment.
func (s *Service) SendComment(ctx context.Context, in Comment) error {
	return s.deliver(ctx, in.To, "comment notification", func(cfg Config) (Envelope, error) {
		return BuildComment(cfg, in, s.publicURL, s.now())
	})
}

// SendRecipeDigest mails one person the new recipes digest.
func (s *Service) SendRecipeDigest(ctx context.Context, in RecipeDigest) error {
	return s.deliver(ctx, in.To, "recipe digest", func(cfg Config) (Envelope, error) {
		return BuildRecipeDigest(cfg, in, s.publicURL, s.now())
	})
}

// SendTasty mails a recipe's author that somebody finds it tasty.
func (s *Service) SendTasty(ctx context.Context, in Tasty) error {
	return s.deliver(ctx, in.To, "tasty notification", func(cfg Config) (Envelope, error) {
		return BuildTasty(cfg, in, s.publicURL, s.now())
	})
}

// SendInviteAccepted mails an admin that their invitation was accepted.
func (s *Service) SendInviteAccepted(ctx context.Context, in InviteAccepted) error {
	return s.deliver(ctx, in.To, "invitation notification", func(cfg Config) (Envelope, error) {
		return BuildInviteAccepted(cfg, in, s.publicURL, s.now())
	})
}
