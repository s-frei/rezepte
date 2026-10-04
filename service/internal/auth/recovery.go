package auth

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Mailer is what password resets and address confirmations need from
// package mail; *mail.Service is it.
type Mailer interface {
	Enabled(context.Context) bool
	SendReset(context.Context, mail.Reset) error
	SendHint(context.Context, mail.Hint) error
	SendConfirm(context.Context, mail.Confirm) error
}

// Provider is the identity provider an account without a password signs in
// with, for the hint a forgotten-password request sends it. Zero while OIDC
// is off.
type Provider struct{ Issuer, Name string }

// ErrNothingToConfirm means the account has no address, or it is confirmed.
var ErrNothingToConfirm = errors.New("no unconfirmed address")

// SetMail lets the service mail password resets and confirmations through
// m. Without it - every test that does not care, and the reset CLI - both
// are off.
func (s *Service) SetMail(m Mailer, p Provider) { s.mailer, s.provider = m, p }

func (s *Service) mailOn(ctx context.Context) bool { return s.mailer != nil && s.mailer.Enabled(ctx) }

// ForgotPassword mails whoever login names what gets them back in. It
// reports nothing: the caller answers the same whatever happens, so the
// response cannot tell whether an account exists. Failures are logged.
func (s *Service) ForgotPassword(ctx context.Context, login string) {
	if !s.mailOn(ctx) {
		return
	}
	accounts, err := s.resetTargets(ctx, strings.TrimSpace(login))
	if err != nil {
		// A fixed message: whatever the error wraps, the typed login (which
		// may be someone else's address) never reaches the log.
		slog.WarnContext(ctx, "forgot password: account lookup failed")
		return
	}
	for _, u := range accounts {
		if _, ok := s.resetCooldown.allow(u.ID, s.now()); !ok {
			continue
		}
		// A delivery failure is already logged by package mail, with the
		// recipient's domain and reply code only.
		if err := s.mailRecovery(ctx, u); err != nil && !errors.Is(err, mail.ErrSend) {
			slog.WarnContext(ctx, "forgot password", "user", u.ID, "err", err)
		}
	}
}

// resetTargets resolves login to the accounts a reset may reach: with an @
// it is first an address and reaches every account whose confirmed address
// it is; otherwise, or when no confirmed address matches (a username may hold
// an @ too), it is a username, whose address has to be confirmed. Never the
// owner, whose way back in is --reset-superadmin-password.
func (s *Service) resetTargets(ctx context.Context, login string) ([]user.User, error) {
	var found []user.User
	if strings.Contains(login, "@") {
		var err error
		if found, err = s.users.ListByVerifiedEmail(ctx, login); err != nil {
			return nil, err
		}
	}
	if len(found) == 0 {
		u, err := s.users.ByUsername(ctx, login)
		if errors.Is(err, user.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if u.Email != "" && u.EmailVerified {
			found = []user.User{u}
		}
	}
	return slices.DeleteFunc(found, func(u user.User) bool { return u.Role.IsSuperadmin() }), nil
}

// mailRecovery sends u a reset link when they have a password, a hint when
// the identity provider is their only way in, and nothing when they never
// set up or an admin's setup link is open: that link is the way in.
func (s *Service) mailRecovery(ctx context.Context, u user.User) error {
	if u.HasPassword {
		link, err := s.IssueResetLink(ctx, u.ID)
		if errors.Is(err, errSetupLinkOpen) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.mailer.SendReset(ctx, mail.Reset{To: u.Email, Username: u.Username, Path: SetupPath(link.Token), Locale: u.Locale})
	}
	if s.provider.Issuer == "" {
		return nil
	}
	if _, err := s.users.IdentityLinkedAt(ctx, u.ID, s.provider.Issuer); errors.Is(err, user.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	return s.mailer.SendHint(ctx, mail.Hint{To: u.Email, Provider: s.provider.Name, Locale: u.Locale})
}

// ConfirmChangedAddress mails a confirmation when a write turned before into
// an after with a new, unconfirmed address; by names the admin who entered
// it, empty when the account holder did. The holder's own change shares
// "Send again"'s minute, so repeated saves cannot mail a stranger without
// end: inside it nothing is sent. An admin's change is not held back. A
// failure never undoes the save; ConfirmationPending tells whether a mail
// went out.
func (s *Service) ConfirmChangedAddress(ctx context.Context, before, after user.User, by string) {
	if after.Email == "" || after.Email == before.Email || after.EmailVerified || !s.mailOn(ctx) {
		return
	}
	if by == "" {
		if _, ok := s.confirmCooldown.allow(after.ID, s.now()); !ok {
			return
		}
	}
	err := s.sendConfirmation(ctx, after, by)
	if err != nil && !errors.Is(err, mail.ErrDisabled) && !errors.Is(err, mail.ErrSend) {
		slog.WarnContext(ctx, "confirmation mail failed", "user", after.ID, "err", err)
	}
}

// ResendConfirmation is the profile's "Send again": at most one per account
// per minute (a *ThrottledError says how long to wait).
func (s *Service) ResendConfirmation(ctx context.Context, u user.User) error {
	if u.Email == "" || u.EmailVerified {
		return ErrNothingToConfirm
	}
	if !s.mailOn(ctx) {
		return mail.ErrDisabled
	}
	if wait, ok := s.confirmCooldown.allow(u.ID, s.now()); !ok {
		return &ThrottledError{RetryAfter: wait}
	}
	return s.sendConfirmation(ctx, u, "")
}

func (s *Service) sendConfirmation(ctx context.Context, u user.User, by string) error {
	if !s.mailOn(ctx) {
		return mail.ErrDisabled
	}
	token, err := s.IssueConfirmation(ctx, u.ID, u.Email)
	if err != nil {
		return err
	}
	err = s.mailer.SendConfirm(ctx, mail.Confirm{To: u.Email, Username: u.Username, Admin: by, Path: ConfirmPath(token), Locale: u.Locale})
	if err != nil {
		// A link nobody received is not an open confirmation: dropped, so
		// ConfirmationPending never claims a mail that did not go out. The
		// replaced link before it is dead either way. By token, so a newer
		// confirmation issued meanwhile stays.
		if derr := s.q.DeleteEmailConfirmation(ctx, hashToken(token)); derr != nil {
			slog.WarnContext(ctx, "drop unsent confirmation", "user", u.ID, "err", derr)
		}
		// A mail that never went out does not use up the minute, so the
		// send button offered next works at once.
		if errors.Is(err, mail.ErrSend) {
			s.confirmCooldown.forget(u.ID)
		}
	}
	return err
}
