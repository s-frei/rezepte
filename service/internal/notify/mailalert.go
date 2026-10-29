package notify

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// mailAlertEvery is how often the owner hears again while mail keeps
// failing.
const mailAlertEvery = 24 * time.Hour

// MailFailed pushes the owner that mail is not getting out, for reason
// ("auth", "connect", "tls" or "sender"): at the first failure since mail
// last worked or was set up, and then once every mailAlertEvery. The state
// lives in instance_settings, so a restart does not re-arm it. It
// implements mail.DeliveryReporter.
func (s *Service) MailFailed(ctx context.Context, reason string) {
	s.alertMu.Lock()
	defer s.alertMu.Unlock()
	lastAt, err := s.q.GetMailAlertLastAt(ctx)
	if err != nil {
		slog.WarnContext(ctx, "mail alert: state lookup failed", "err", err)
		return
	}
	now := s.now()
	if last, err := db.ParseTime(lastAt); lastAt != "" && err == nil && now.Sub(last) < mailAlertEvery {
		return
	}
	if err := s.q.SetMailAlertLastAt(ctx, db.FormatTime(now)); err != nil {
		slog.WarnContext(ctx, "mail alert: state update failed", "err", err)
		return
	}
	if s.pusher != nil {
		detach(ctx, "mail alert", func(ctx context.Context) { s.pushMailFailing(ctx, reason) })
	}
}

// pushMailFailing pushes the owner reason's copy, unless they switched it
// off for push.
func (s *Service) pushMailFailing(ctx context.Context, reason string) {
	owner, err := s.q.GetSuperadmin(ctx)
	if err != nil {
		slog.WarnContext(ctx, "mail alert: owner lookup failed", "err", err)
		return
	}
	if !s.wanted(ctx, owner.ID, KindMailFailing, ChannelPush) {
		return
	}
	loc := user.Locale(owner.Locale)
	title, err1 := mail.Text(loc, "mail_push_mail_failing_title", nil)
	body, err2 := mail.Text(loc, "mail_push_mail_failing_"+reason, nil)
	if err := errors.Join(err1, err2); err != nil {
		slog.WarnContext(ctx, "mail alert push failed", "err", err)
		return
	}
	s.pusher.SendToUser(ctx, owner.ID, push.Message{Kind: string(KindMailFailing), Title: title, Body: body, URL: "/settings/mail", Tag: "mail-failing"})
}

// MailReset clears the alert state after a mail went out or mail was set
// up anew, so the next failure alerts at once. It implements
// mail.DeliveryReporter.
func (s *Service) MailReset(ctx context.Context) {
	s.alertMu.Lock()
	defer s.alertMu.Unlock()
	if err := s.q.SetMailAlertLastAt(ctx, ""); err != nil {
		slog.WarnContext(ctx, "mail alert: state reset failed", "err", err)
	}
}
