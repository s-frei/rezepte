package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// digestAge is how old a recipe must be to go out: one saved in steps gets
// its corrections and photo in before the household hears of it.
const digestAge = time.Hour

// digestTick is how often StartRecipeDigest checks; a variable so tests
// need not wait a minute.
var digestTick = time.Minute

// StartRecipeDigest checks every digestTick, until ctx ends, whether the
// day's digest is due, in its own goroutine. The returned channel closes
// once it has stopped, after a run under way has sent its mails.
func (s *Service) StartRecipeDigest(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(digestTick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tickRecipeDigest(ctx)
			}
		}
	}()
	return done
}

func (s *Service) tickRecipeDigest(ctx context.Context) {
	// A panic would end the process, and with it the next day's run.
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "recipe digest panicked", "panic", r)
		}
	}()
	if err := s.RunRecipeDigest(ctx, time.Now()); err != nil {
		slog.WarnContext(ctx, "recipe digest failed", "err", err)
	}
}

// RunRecipeDigest sends the day's new recipes digest, by mail and push,
// when now, in the server's time zone, is at or past the owner's hour and no
// run happened on that date yet. Each recipient gets the announced recipes
// at least digestAge old that others created; one with none gets nothing.
// With mail off and no pusher nothing is claimed, so the run happens once
// mail is on.
func (s *Service) RunRecipeDigest(ctx context.Context, now time.Time) error {
	mailOn := s.mailer.Enabled(ctx)
	if !mailOn && s.pusher == nil {
		return nil
	}
	local := now.In(time.Local)
	day := local.Format(time.DateOnly)
	st, err := s.q.GetRecipeDigestState(ctx)
	if err != nil {
		return fmt.Errorf("recipe digest state: %w", err)
	}
	if local.Hour() < int(st.RecipeDigestHour) || st.RecipeDigestLastDay == day {
		return nil
	}
	// Read before the claim, so a failure here leaves the day unclaimed.
	var recipients, pushTo []sqlc.ListRecipeDigestRecipientsRow
	if mailOn {
		if recipients, err = s.q.ListRecipeDigestRecipients(ctx, string(ChannelMail)); err != nil {
			return fmt.Errorf("list recipe digest recipients: %w", err)
		}
		recipients = slices.DeleteFunc(recipients, func(r sqlc.ListRecipeDigestRecipientsRow) bool { return !mailable(r.Email, r.EmailVerified) })
	}
	if s.pusher != nil {
		if pushTo, err = s.q.ListRecipeDigestRecipients(ctx, string(ChannelPush)); err != nil {
			return fmt.Errorf("list recipe digest push recipients: %w", err)
		}
	}
	due, err := s.claimRecipeDigest(ctx, day, now.Add(-digestAge))
	if err != nil || len(due) == 0 {
		return err
	}
	// The recipes are off the queue now: a shutdown must not cut the run
	// short (main waits for it). Each send keeps its own sendTimeout.
	ctx = context.WithoutCancel(ctx)
	recipes := make([]mail.DigestRecipe, len(due))
	creators := map[string]user.User{}
	for i, r := range due {
		c, ok := creators[r.CreatedBy]
		if !ok {
			c, err = s.users.ByID(ctx, r.CreatedBy)
			if err != nil {
				if !errors.Is(err, user.ErrNotFound) {
					slog.WarnContext(ctx, "recipe digest: creator unreadable", "user", r.CreatedBy, "err", err)
				}
				c = user.User{ID: r.CreatedBy} // no name: the mail names a former member
			}
			creators[r.CreatedBy] = c
		}
		created, err := db.ParseTime(r.CreatedAt)
		if err != nil {
			slog.WarnContext(ctx, "recipe digest: creation time unreadable", "recipe", r.ID, "err", err)
		}
		recipes[i] = mail.DigestRecipe{
			Title: r.Title, URLPath: "/recipes/" + url.PathEscape(r.Slug),
			CreatorName: c.DisplayName, CreatorColor: c.Color, CreatorAvatar: s.picture(ctx, c),
			Photo: s.coverPhoto(ctx, r), Servings: int(r.Servings), Minutes: int(deref(r.PrepMinutes) + deref(r.CookMinutes)),
			CreatedAt: created,
		}
	}
	for i, to := range recipients {
		var theirs []mail.DigestRecipe
		for i, r := range due {
			if r.CreatedBy != to.ID {
				theirs = append(theirs, recipes[i])
			}
		}
		if len(theirs) == 0 {
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, sendTimeout)
		err := s.mailer.SendRecipeDigest(sctx, mail.RecipeDigest{
			To: to.Email, Recipes: theirs, Unsubscribe: s.UnsubscribeLinks(to.ID, KindRecipe), Locale: user.Locale(to.Locale),
		})
		cancel()
		if errors.Is(err, mail.ErrDisabled) {
			slog.WarnContext(ctx, "recipe digest: mail turned off mid-run", "unsent", len(recipients)-i)
			break
		}
		mail.LogUnsent(ctx, "recipe digest mail failed", to.ID, err)
	}
	if s.pusher != nil {
		s.pushDigest(ctx, pushTo, due, creators)
	}
	return nil
}

// claimRecipeDigest marks day as run and takes the recipes due by cutoff
// off the queue, in one transaction; nil when another run claimed day.
// ponytail: the claim commits before the mails go out, so a crash, mail
// switched off mid-run or a run outlasting main's shutdown wait loses that
// day's remaining mails (at most once, never twice); a per-recipient outbox
// if that ever matters.
func (s *Service) claimRecipeDigest(ctx context.Context, day string, cutoff time.Time) ([]sqlc.ListDueAnnouncementsRow, error) {
	var due []sqlc.ListDueAnnouncementsRow
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		n, err := q.ClaimRecipeDigestDay(ctx, day)
		if err != nil {
			return fmt.Errorf("claim recipe digest day: %w", err)
		}
		if n != 1 {
			return nil
		}
		if due, err = q.ListDueAnnouncements(ctx, db.FormatTime(cutoff)); err != nil {
			return fmt.Errorf("list due announcements: %w", err)
		}
		if len(due) == 0 {
			return nil
		}
		ids := make([]string, len(due))
		for i, r := range due {
			ids[i] = r.ID
		}
		if err := q.ClearAnnouncements(ctx, ids); err != nil {
			return fmt.Errorf("clear announcements: %w", err)
		}
		return nil
	})
	return due, err
}

// coverPhoto is r's cover for the digest, nil (a card without photo) when r
// has none or it cannot be read.
func (s *Service) coverPhoto(ctx context.Context, r sqlc.ListDueAnnouncementsRow) []byte {
	if r.CoverImageID == nil || s.images == nil {
		return nil
	}
	f, err := s.images.Open(r.ID, *r.CoverImageID, "thumb")
	if err != nil {
		slog.WarnContext(ctx, "recipe digest: photo unreadable", "recipe", r.ID, "err", err)
		return nil
	}
	defer func() { _ = f.Close() }()
	b, err := image.MailPhoto(f)
	if err != nil {
		slog.WarnContext(ctx, "recipe digest: photo unreadable", "recipe", r.ID, "err", err)
		return nil
	}
	return b
}

func deref(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}
