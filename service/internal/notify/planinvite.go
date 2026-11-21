package notify

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/url"

	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// PlanInvited tells inviteeID, by mail and push, that inviterID invited them
// to the meal plan planID. It implements mealplan.InviteNotifier.
func (s *Service) PlanInvited(ctx context.Context, planID, inviterID, inviteeID string) {
	detach(ctx, "plan invitation notification", func(ctx context.Context) { s.sendPlanInvite(ctx, planID, inviterID, inviteeID) })
}

// sendPlanInvite sends nothing when the plan or either account went.
func (s *Service) sendPlanInvite(ctx context.Context, planID, inviterID, inviteeID string) {
	n, err := s.q.GetMealPlanName(ctx, planID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && n == nil { // gone, or personal and never shared
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "plan invitation notification failed", "plan", planID, "err", err)
		return
	}
	name := *n
	inviter, err1 := s.users.ByID(ctx, inviterID)
	invitee, err2 := s.users.ByID(ctx, inviteeID)
	if err := errors.Join(err1, err2); err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			slog.WarnContext(ctx, "plan invitation notification failed", "plan", planID, "err", err)
		}
		return
	}
	s.mailPlanInvite(ctx, planID, name, inviter, invitee)
	s.pushPlanInvite(ctx, planID, name, inviter, invitee)
}

func (s *Service) mailPlanInvite(ctx context.Context, planID, name string, inviter, invitee user.User) {
	if !mailable(invitee.Email, invitee.EmailVerified) {
		return
	}
	if !s.wanted(ctx, invitee.ID, KindPlanInvite, ChannelMail) {
		return
	}
	sctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	err := s.mailer.SendPlanInvite(sctx, mail.PlanInvite{
		To: invitee.Email, PlanID: planID, PlanName: name,
		InviterName: inviter.DisplayName, InviterColor: inviter.Color, InviterAvatar: s.picture(ctx, inviter),
		Unsubscribe: s.UnsubscribeLinks(invitee.ID, KindPlanInvite), Locale: invitee.Locale,
	})
	mail.LogUnsent(ctx, "plan invitation mail failed", invitee.ID, err)
}

func (s *Service) pushPlanInvite(ctx context.Context, planID, name string, inviter, invitee user.User) {
	if !s.wanted(ctx, invitee.ID, KindPlanInvite, ChannelPush) {
		return
	}
	title, err1 := mail.Text(invitee.Locale, "mail_push_plan_invite_title", nil)
	body, err2 := mail.Text(invitee.Locale, "mail_push_plan_invite_body", map[string]string{"name": inviter.DisplayName, "plan": name})
	if err := errors.Join(err1, err2); err != nil {
		slog.WarnContext(ctx, "plan invitation push failed", "err", err)
		return
	}
	s.pusher.SendToUser(ctx, invitee.ID, push.Message{
		Kind: string(KindPlanInvite), Title: title, Body: body, URL: "/plans/" + url.PathEscape(planID), Tag: "plan-invite:" + planID, Icon: s.markURL(inviter),
	})
}
