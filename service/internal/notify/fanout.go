package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// commentTimeout bounds the background fan-out of one comment; each mail in
// it has sendTimeout of its own, so a slow server costs the follower it is
// slow for, not every follower after them.
const commentTimeout = 10 * time.Minute

// sendTimeout bounds one follower's mail: the settings lookup, rendering
// and the SMTP dialog (whose own deadline is 10 seconds). A variable so a
// test can shorten it.
var sendTimeout = 15 * time.Second

// CommentAdded mails the followers of recipeID about comment commentID in
// the background, so the request that wrote it never waits on SMTP. It
// implements recipe.CommentNotifier.
func (s *Service) CommentAdded(ctx context.Context, recipeID string, commentID int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commentTimeout)
		defer cancel()
		// No request is left to fail: a panic here would end the process.
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "comment mail panicked", "panic", r)
			}
		}()
		if err := s.mailComment(ctx, recipeID, commentID); err != nil {
			slog.WarnContext(ctx, "comment mail failed", "recipe", recipeID, "comment", commentID, "err", err)
		}
	}()
}

// mailComment sends one mail per follower. It returns only lookup errors: a
// failed send is logged (by mail.Service.deliver, or here for anything
// else) without the address, and the next follower still gets theirs.
func (s *Service) mailComment(ctx context.Context, recipeID string, commentID int64) error {
	if !s.mailer.Enabled(ctx) {
		return nil
	}
	c, err := s.q.GetComment(ctx, commentID)
	if err != nil {
		return fmt.Errorf("get comment %d: %w", commentID, err)
	}
	if c.AuthorID == nil {
		return nil // the author's account went in the meantime
	}
	author, err := s.users.ByID(ctx, *c.AuthorID)
	if err != nil {
		return fmt.Errorf("get comment author: %w", err)
	}
	r, err := s.q.GetRecipe(ctx, recipeID)
	if err != nil {
		return fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	at, err := db.ParseTime(c.CreatedAt)
	if err != nil {
		return err
	}
	picture := s.picture(ctx, author)
	photos := s.commentPhotos(ctx, r.ID, commentID)
	recipients, err := s.q.ListCommentMailRecipients(ctx, sqlc.ListCommentMailRecipientsParams{RecipeID: recipeID, AuthorID: author.ID})
	if err != nil {
		return fmt.Errorf("list comment mail recipients: %w", err)
	}
	for _, to := range recipients {
		reason := mail.ReasonCommented
		if to.IsCreator {
			reason = mail.ReasonCreated
		}
		sctx, cancel := context.WithTimeout(ctx, sendTimeout)
		err := s.mailer.SendComment(sctx, mail.Comment{
			To: to.Email, RecipeTitle: r.Title, RecipeURLPath: "/recipes/" + url.PathEscape(r.Slug) + "?tab=comments",
			AuthorName: author.DisplayName, AuthorColor: author.Color, AuthorAvatar: picture, Body: c.Body, Photos: photos, At: at, Reason: reason,
			Unsubscribe: s.UnsubscribeLinks(to.ID, KindComment), Locale: user.Locale(to.Locale),
		})
		cancel()
		if errors.Is(err, mail.ErrDisabled) {
			return nil
		}
		mail.LogUnsent(ctx, "comment mail failed", to.ID, err)
	}
	return nil
}

// commentPhotos reads the photos of comment commentID for a mail, in order;
// one that cannot be read is left out, and the mail goes without it.
func (s *Service) commentPhotos(ctx context.Context, recipeID string, commentID int64) [][]byte {
	if s.images == nil {
		return nil
	}
	rows, err := s.q.ListCommentImagesForComment(ctx, &commentID)
	if err != nil {
		slog.WarnContext(ctx, "comment mail: photos unreadable", "comment", commentID, "err", err)
		return nil
	}
	var photos [][]byte
	for _, row := range rows {
		f, err := s.images.OpenCommentImage(ctx, recipeID, row.ID, "thumb")
		if err != nil {
			slog.WarnContext(ctx, "comment mail: photo unreadable", "comment", commentID, "err", err)
			continue
		}
		b, err := image.MailPhoto(f)
		_ = f.Close()
		if err != nil {
			slog.WarnContext(ctx, "comment mail: photo unreadable", "comment", commentID, "err", err)
			continue
		}
		photos = append(photos, b)
	}
	return photos
}

// picture reads u's picture for a mail; nil, and the mail shows the
// initial, when u has none or it cannot be read.
func (s *Service) picture(ctx context.Context, u user.User) []byte {
	if u.AvatarID == nil || s.avatars == nil {
		return nil
	}
	f, err := s.avatars.Open(u.ID, *u.AvatarID)
	if err != nil {
		slog.WarnContext(ctx, "mail: picture unreadable", "user", u.ID, "err", err)
		return nil
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(f)
	if err != nil {
		slog.WarnContext(ctx, "mail: picture unreadable", "user", u.ID, "err", err)
		return nil
	}
	return b
}

// InviteAccepted mails inviterID that newcomer set up the account their
// setup link was for. It implements auth.Notifier and runs after the
// redeem committed; failures are logged.
func (s *Service) InviteAccepted(ctx context.Context, inviterID string, newcomer user.User) {
	if inviterID == newcomer.ID {
		return
	}
	inviter, err := s.users.ByID(ctx, inviterID)
	if errors.Is(err, user.ErrNotFound) {
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "invitation mail: inviter lookup failed", "err", err)
		return
	}
	if !inviter.Role.IsAdmin() || inviter.Email == "" || !inviter.EmailVerified {
		return
	}
	on, err := s.Wants(ctx, inviter.ID, KindInviteAccepted, ChannelMail)
	if err != nil {
		slog.WarnContext(ctx, "invitation mail: preference lookup failed", "err", err)
		return
	}
	if !on {
		return
	}
	err = s.mailer.SendInviteAccepted(ctx, mail.InviteAccepted{
		To: inviter.Email, Username: newcomer.Username, DisplayName: newcomer.DisplayName,
		Unsubscribe: s.UnsubscribeLinks(inviter.ID, KindInviteAccepted), Locale: inviter.Locale,
	})
	mail.LogUnsent(ctx, "invitation mail failed", inviter.ID, err)
}
