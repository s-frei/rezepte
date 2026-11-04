package notify

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// ListInvited tells inviteeID, by mail and push, that inviterID invited them
// to the shopping list listID. It implements shopping.InviteNotifier.
func (s *Service) ListInvited(ctx context.Context, listID, inviterID, inviteeID string) {
	detach(ctx, "list invitation notification", func(ctx context.Context) { s.sendListInvite(ctx, listID, inviterID, inviteeID) })
}

// sendListInvite sends nothing when the list or either account went.
func (s *Service) sendListInvite(ctx context.Context, listID, inviterID, inviteeID string) {
	name, err := s.q.GetShoppingListName(ctx, listID)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "list invitation notification failed", "list", listID, "err", err)
		return
	}
	inviter, err1 := s.users.ByID(ctx, inviterID)
	invitee, err2 := s.users.ByID(ctx, inviteeID)
	if err := errors.Join(err1, err2); err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			slog.WarnContext(ctx, "list invitation notification failed", "list", listID, "err", err)
		}
		return
	}
	s.mailListInvite(ctx, name, inviter, invitee)
	s.pushListInvite(ctx, listID, name, inviter, invitee)
}

func (s *Service) mailListInvite(ctx context.Context, name string, inviter, invitee user.User) {
	if !mailable(invitee.Email, invitee.EmailVerified) {
		return
	}
	if !s.wanted(ctx, invitee.ID, KindListInvite, ChannelMail) {
		return
	}
	sctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	err := s.mailer.SendListInvite(sctx, mail.ListInvite{
		To: invitee.Email, ListName: name,
		InviterName: inviter.DisplayName, InviterColor: inviter.Color, InviterAvatar: s.picture(ctx, inviter),
		Unsubscribe: s.UnsubscribeLinks(invitee.ID, KindListInvite), Locale: invitee.Locale,
	})
	mail.LogUnsent(ctx, "list invitation mail failed", invitee.ID, err)
}

func (s *Service) pushListInvite(ctx context.Context, listID, name string, inviter, invitee user.User) {
	if !s.wanted(ctx, invitee.ID, KindListInvite, ChannelPush) {
		return
	}
	title, err1 := mail.Text(invitee.Locale, "mail_push_list_invite_title", nil)
	body, err2 := mail.Text(invitee.Locale, "mail_push_list_invite_body", map[string]string{"name": inviter.DisplayName, "list": name})
	if err := errors.Join(err1, err2); err != nil {
		slog.WarnContext(ctx, "list invitation push failed", "err", err)
		return
	}
	s.pusher.SendToUser(ctx, invitee.ID, push.Message{
		Kind: string(KindListInvite), Title: title, Body: body, URL: "/lists", Tag: "list-invite:" + listID,
	})
}
