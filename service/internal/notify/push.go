package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// pushTextMax is how many runes of a comment a push shows.
const pushTextMax = 120

// shorten collapses s's whitespace runs to one space and cuts it to max
// runes plus "…", never inside a UTF-8 sequence and never between an emoji
// and the modifier, joiner or variation selector that belongs to it.
// ponytail: not full grapheme segmentation (a flag's two regional
// indicators can still part); a segmentation library if that ever shows.
func shorten(s string, max int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= max {
		return string(r)
	}
	i := max
	for i > 0 && (joins(r[i]) || r[i-1] == '‍') {
		i--
	}
	return strings.TrimRight(string(r[:i]), " ") + "…"
}

// joins reports whether r belongs to the rune before it.
func joins(r rune) bool {
	return r == '‍' || r >= 0x1f3fb && r <= 0x1f3ff || unicode.Is(unicode.Variation_Selector, r) || unicode.Is(unicode.Mn, r)
}

// pushComment pushes comment commentID to the followers of recipeID that
// want it by push, never its author. The pushes of one recipe share a tag,
// so a newer one replaces the one shown.
func (s *Service) pushComment(ctx context.Context, recipeID string, commentID int64) error {
	if s.pusher == nil {
		return nil
	}
	c, author, r, ok, err := s.commentFacts(ctx, recipeID, commentID)
	if !ok {
		return err
	}
	recipients, err := s.q.ListCommentRecipients(ctx, sqlc.ListCommentRecipientsParams{RecipeID: recipeID, AuthorID: author.ID, Channel: string(ChannelPush)})
	if err != nil {
		return fmt.Errorf("list comment push recipients: %w", err)
	}
	text := shorten(c.Body, pushTextMax)
	for _, to := range recipients {
		loc := user.Locale(to.Locale)
		// The title names what happened, as every other kind's does; the
		// recipe alone left a toast without its reason.
		title, err1 := mail.Text(loc, "mail_push_comment_title", map[string]string{"title": r.Title})
		titleMany, err2 := mail.Text(loc, "mail_push_comment_title_many", map[string]string{"title": r.Title})
		body, err3 := mail.Text(loc, "mail_push_comment_body", map[string]string{"author": author.DisplayName, "text": text})
		many, err4 := mail.Text(loc, "mail_push_comment_body_many", nil)
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			slog.WarnContext(ctx, "comment push failed", "user", to.ID, "err", err)
			continue
		}
		s.pusher.SendToUser(ctx, to.ID, push.Message{
			Kind: string(KindComment), Title: title, Body: body,
			URL: "/recipes/" + url.PathEscape(r.Slug) + "?tab=comments", Tag: "comment:" + r.ID,
			Collapse: &push.Collapse{Title: titleMany, BodyMany: many},
		})
	}
	return nil
}

// pushDigest pushes each recipient the count of recipes others created in
// due, and who; one left with none gets no push.
func (s *Service) pushDigest(ctx context.Context, recipients []sqlc.ListRecipeDigestRecipientsRow, due []sqlc.ListDueAnnouncementsRow, creators map[string]user.User) {
	for _, to := range recipients {
		loc := user.Locale(to.Locale)
		var theirs []sqlc.ListDueAnnouncementsRow
		var names []string
		for _, r := range due {
			if r.CreatedBy == to.ID {
				continue
			}
			theirs = append(theirs, r)
			name := creators[r.CreatedBy].DisplayName
			if name == "" {
				name, _ = mail.Text(loc, "mail_recipe_digest_former_member", nil)
			}
			if len(names) < 3 && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
		if len(theirs) == 0 {
			continue
		}
		args := map[string]string{"names": strings.Join(names, ", "), "title": theirs[0].Title, "count": strconv.Itoa(len(theirs))}
		key := "mail_push_recipe_body_one"
		if len(theirs) > 1 {
			key = "mail_push_recipe_body_many"
		}
		title, err1 := mail.Text(loc, "mail_push_recipe_title", nil)
		body, err2 := mail.Text(loc, key, args)
		if err := errors.Join(err1, err2); err != nil {
			slog.WarnContext(ctx, "recipe digest push failed", "user", to.ID, "err", err)
			continue
		}
		s.pusher.SendToUser(ctx, to.ID, push.Message{Kind: string(KindRecipe), Title: title, Body: body, URL: "/", Tag: "recipe-digest"})
	}
}

// pushInviteAccepted pushes inviter that newcomer set up their account,
// unless inviter switched it off for push.
func (s *Service) pushInviteAccepted(ctx context.Context, inviter, newcomer user.User) {
	if !s.wanted(ctx, inviter.ID, KindInviteAccepted, ChannelPush) {
		return
	}
	title, err1 := mail.Text(inviter.Locale, "mail_push_invite_accepted_title", nil)
	body, err2 := mail.Text(inviter.Locale, "mail_push_invite_accepted_body", map[string]string{"name": newcomer.DisplayName, "username": newcomer.Username})
	if err := errors.Join(err1, err2); err != nil {
		slog.WarnContext(ctx, "invitation push failed", "err", err)
		return
	}
	s.pusher.SendToUser(ctx, inviter.ID, push.Message{
		Kind: string(KindInviteAccepted), Title: title, Body: body, URL: "/settings/users", Tag: "invite:" + newcomer.ID,
	})
}
