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

// ListInvited tells inviteeID, by mail and push, that inviterID invited them
// to the shopping list listID. It implements shopping.InviteNotifier.
func (s *Service) ListInvited(ctx context.Context, listID, inviterID, inviteeID string) {
	detach(ctx, "list invitation notification", func(ctx context.Context) { s.sendListInvite(ctx, listID, inviterID, inviteeID) })
}

// PlanInvited tells inviteeID, by mail and push, that inviterID invited them
// to the meal plan planID. It implements mealplan.InviteNotifier.
func (s *Service) PlanInvited(ctx context.Context, planID, inviterID, inviteeID string) {
	detach(ctx, "plan invitation notification", func(ctx context.Context) { s.sendPlanInvite(ctx, planID, inviterID, inviteeID) })
}

func (s *Service) sendListInvite(ctx context.Context, listID, inviterID, inviteeID string) {
	name, err := s.q.GetShoppingListName(ctx, listID)
	s.sendInvite(ctx, invite{kind: KindListInvite, thing: "list", id: listID, path: "/lists"}, name, err, inviterID, inviteeID)
}

func (s *Service) sendPlanInvite(ctx context.Context, planID, inviterID, inviteeID string) {
	n, err := s.q.GetMealPlanName(ctx, planID)
	if err == nil && n == nil { // personal and never shared
		return
	}
	in := invite{kind: KindPlanInvite, thing: "plan", id: planID, path: "/plans/" + url.PathEscape(planID)}
	s.sendInvite(ctx, in, deref(n), err, inviterID, inviteeID)
}

// invite is what tells a list invitation from a plan invitation: the
// notification kind, the copy's "list" or "plan" (mail_push_<thing>_invite_*,
// mail_<thing>_invite_*), the thing's id and where the push and mail lead.
type invite struct {
	kind            Kind
	thing, id, path string
}

// sendInvite sends nothing when the thing (err is the name lookup's) or
// either account went.
func (s *Service) sendInvite(ctx context.Context, in invite, name string, err error, inviterID, inviteeID string) {
	what := in.thing + " invitation notification failed"
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		slog.WarnContext(ctx, what, in.thing, in.id, "err", err)
		return
	}
	inviter, err1 := s.users.ByID(ctx, inviterID)
	invitee, err2 := s.users.ByID(ctx, inviteeID)
	if err := errors.Join(err1, err2); err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			slog.WarnContext(ctx, what, in.thing, in.id, "err", err)
		}
		return
	}
	if mailable(invitee.Email, invitee.EmailVerified) && s.wanted(ctx, invitee.ID, in.kind, ChannelMail) {
		sctx, cancel := context.WithTimeout(ctx, sendTimeout)
		err := s.mailer.SendShareInvite(sctx, mail.ShareInvite{
			Kind: in.thing, To: invitee.Email, Name: name, Path: in.path,
			InviterName: inviter.DisplayName, InviterColor: inviter.Color, InviterAvatar: s.picture(ctx, inviter),
			Unsubscribe: s.UnsubscribeLinks(invitee.ID, in.kind), Locale: invitee.Locale,
		})
		cancel()
		mail.LogUnsent(ctx, in.thing+" invitation mail failed", invitee.ID, err)
	}
	if !s.wanted(ctx, invitee.ID, in.kind, ChannelPush) {
		return
	}
	title, err1 := mail.Text(invitee.Locale, "mail_push_"+in.thing+"_invite_title", nil)
	body, err2 := mail.Text(invitee.Locale, "mail_push_"+in.thing+"_invite_body", map[string]string{"name": inviter.DisplayName, in.thing: name})
	if err := errors.Join(err1, err2); err != nil {
		slog.WarnContext(ctx, in.thing+" invitation push failed", "err", err)
		return
	}
	s.pusher.SendToUser(ctx, invitee.ID, push.Message{
		Kind: string(in.kind), Title: title, Body: body, URL: in.path, Tag: in.thing + "-invite:" + in.id, Icon: s.markURL(inviter),
	})
}
