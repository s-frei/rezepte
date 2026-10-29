// Package notify decides who wants which switchable notification on which
// channel: the notification kinds, their groups, every account's
// preferences and the signed unsubscribe links a mail carries. Security notices are not kinds; nothing
// here can switch them off.
package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/sign"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Kind is one switchable notification. Its value is stored in
// notification_optouts and travels in unsubscribe links, so it never
// changes.
type Kind string

// The kinds: a comment on a recipe the account follows, the daily digest
// of recipes others added, a tasty mark on the account's recipe, a person
// setting up an account the admin invited, and the owner's warning that
// mail is not getting out.
const (
	KindComment        Kind = "comment"
	KindRecipe         Kind = "recipe"
	KindTasty          Kind = "tasty"
	KindInviteAccepted Kind = "invite_accepted"
	KindMailFailing    Kind = "mail_failing"
)

// Channel is a way a notification reaches the account. Its value is
// stored, so it never changes.
type Channel string

// The channels: a mail to the confirmed address, a push to every device.
const (
	ChannelMail Channel = "mail"
	ChannelPush Channel = "push"
)

// channels lists every channel, in the order they are shown.
var channels = []Channel{ChannelMail, ChannelPush}

// Group is a set of kinds with a master switch per channel, which an
// unsubscribe link can switch off for mail.
type Group string

// The groups: what happens in the household, and what only admins get.
const (
	GroupHousehold Group = "household"
	GroupAdmin     Group = "admin"
)

// ErrUnknownKind is returned for a kind that does not exist or that the
// account may not have.
var ErrUnknownKind = errors.New("unknown notification kind")

// groups lists every kind under its group, in the order they are shown.
var groups = []struct {
	group Group
	kinds []Kind
}{
	{GroupHousehold, []Kind{KindComment, KindRecipe, KindTasty}},
	{GroupAdmin, []Kind{KindInviteAccepted, KindMailFailing}},
}

// Group returns the group k belongs to, empty for an unknown kind.
func (k Kind) Group() Group {
	for _, g := range groups {
		if slices.Contains(g.kinds, k) {
			return g.group
		}
	}
	return ""
}

// Kinds returns the kinds of g.
func (g Group) Kinds() []Kind {
	for _, gr := range groups {
		if gr.group == g {
			return gr.kinds
		}
	}
	return nil
}

// KindsFor returns the kinds an account of role may have on ch: the admin
// group only for admins and the owner, mail_failing only for the owner and
// only by push (a mail cannot warn that mail fails). None for an unknown
// channel.
func KindsFor(role user.Role, ch Channel) []Kind {
	if !slices.Contains(channels, ch) {
		return nil
	}
	kinds := slices.Clone(GroupHousehold.Kinds())
	if role.IsAdmin() {
		for _, k := range GroupAdmin.Kinds() {
			if k != KindMailFailing || role.IsSuperadmin() && ch == ChannelPush {
				kinds = append(kinds, k)
			}
		}
	}
	return kinds
}

// hasGroup reports whether an account of role has a kind of g on ch, and
// with it g's switch there.
func hasGroup(role user.Role, g Group, ch Channel) bool {
	return slices.ContainsFunc(KindsFor(role, ch), func(k Kind) bool { return k.Group() == g })
}

// Prefs is an account's preferences: every kind's own choice per channel
// (true unless switched off, whatever its group says) and every group's
// master switch per channel. A kind goes out on a channel only when both
// are on. A pair the account may not have is absent.
type Prefs struct {
	Kinds  map[Kind]map[Channel]bool
	Groups map[Group]map[Channel]bool
}

func put[K comparable](m map[K]map[Channel]bool, k K, ch Channel, on bool) {
	if m[k] == nil {
		m[k] = map[Channel]bool{}
	}
	m[k][ch] = on
}

// Mailer is the mail side notify sends through; *mail.Service is it.
type Mailer interface {
	Enabled(context.Context) bool
	SendComment(context.Context, mail.Comment) error
	SendInviteAccepted(context.Context, mail.InviteAccepted) error
	SendRecipeDigest(context.Context, mail.RecipeDigest) error
	SendTasty(context.Context, mail.Tasty) error
}

// Pusher is the push side notify sends through; *push.Service is it.
// SendToUser logs its own failures and gives each device its own timeout.
type Pusher interface {
	SendToUser(ctx context.Context, userID string, m push.Message)
}

// Service reads and writes notification preferences.
type Service struct {
	conn      *sql.DB
	q         *sqlc.Queries
	signer    *sign.Signer
	users     *user.Service
	avatars   *avatar.Service
	images    *image.Service
	mailer    Mailer
	pusher    Pusher
	publicURL string
	now       func() time.Time
	alertMu   sync.Mutex // one mail alert decision at a time
}

// NewService returns a Service backed by conn that signs unsubscribe links
// with key (32 bytes, settings.Service.LinkPreviewKey) and puts publicURL in
// front of them. images serves the digest's cover photos; p pushes beside
// the mails.
func NewService(conn *sql.DB, key []byte, users *user.Service, avatars *avatar.Service, images *image.Service, m Mailer, p Pusher, publicURL string) *Service {
	return &Service{
		conn:      conn,
		q:         sqlc.New(conn),
		signer:    sign.New(key, signerLabel),
		users:     users,
		avatars:   avatars,
		images:    images,
		mailer:    m,
		pusher:    p,
		publicURL: publicURL,
		now:       time.Now,
	}
}

// Preferences returns every kind and group u may have, per channel, true
// unless u switched it off.
func (s *Service) Preferences(ctx context.Context, u user.User) (Prefs, error) {
	kindsOff, err := s.q.ListOptOuts(ctx, u.ID)
	if err != nil {
		return Prefs{}, fmt.Errorf("list opt-outs of %s: %w", u.ID, err)
	}
	groupsOff, err := s.q.ListGroupOptOuts(ctx, u.ID)
	if err != nil {
		return Prefs{}, fmt.Errorf("list group opt-outs of %s: %w", u.ID, err)
	}
	p := Prefs{Kinds: map[Kind]map[Channel]bool{}, Groups: map[Group]map[Channel]bool{}}
	for _, ch := range channels {
		for _, k := range KindsFor(u.Role, ch) {
			put(p.Kinds, k, ch, !slices.Contains(kindsOff, sqlc.ListOptOutsRow{Kind: string(k), Channel: string(ch)}))
			put(p.Groups, k.Group(), ch, !slices.Contains(groupsOff, sqlc.ListGroupOptOutsRow{Grp: string(k.Group()), Channel: string(ch)}))
		}
	}
	return p, nil
}

// SetPreferences switches the pairs in set and returns every preference of
// u. A kind, group or channel u may not have is ErrUnknownKind, and then
// nothing changes. A group's switch leaves its kinds' own choices alone.
func (s *Service) SetPreferences(ctx context.Context, u user.User, set Prefs) (Prefs, error) {
	for k, chs := range set.Kinds {
		for ch := range chs {
			if !slices.Contains(KindsFor(u.Role, ch), k) {
				return Prefs{}, fmt.Errorf("%w: %q by %q", ErrUnknownKind, k, ch)
			}
		}
	}
	for g, chs := range set.Groups {
		for ch := range chs {
			if !hasGroup(u.Role, g, ch) {
				return Prefs{}, fmt.Errorf("%w: group %q by %q", ErrUnknownKind, g, ch)
			}
		}
	}
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		for k, chs := range set.Kinds {
			for ch, on := range chs {
				var err error
				if on {
					err = q.RemoveOptOut(ctx, sqlc.RemoveOptOutParams{UserID: u.ID, Kind: string(k), Channel: string(ch)})
				} else {
					err = q.AddOptOut(ctx, sqlc.AddOptOutParams{UserID: u.ID, Kind: string(k), Channel: string(ch)})
				}
				if err != nil {
					return fmt.Errorf("set %s/%s of %s: %w", k, ch, u.ID, err)
				}
			}
		}
		for g, chs := range set.Groups {
			for ch, on := range chs {
				var err error
				if on {
					err = q.RemoveGroupOptOut(ctx, sqlc.RemoveGroupOptOutParams{UserID: u.ID, Grp: string(g), Channel: string(ch)})
				} else {
					err = q.AddGroupOptOut(ctx, sqlc.AddGroupOptOutParams{UserID: u.ID, Grp: string(g), Channel: string(ch)})
				}
				if err != nil {
					return fmt.Errorf("set group %s/%s of %s: %w", g, ch, u.ID, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return Prefs{}, err
	}
	return s.Preferences(ctx, u)
}

// wanted is Wants for a send that is skipped when the answer is unknown:
// a failed lookup is logged and sends nothing.
func (s *Service) wanted(ctx context.Context, userID string, k Kind, ch Channel) bool {
	on, err := s.Wants(ctx, userID, k, ch)
	if err != nil {
		slog.WarnContext(ctx, "notification preference lookup failed", "kind", k, "channel", ch, "err", err)
	}
	return on
}

// Wants reports whether userID gets k on ch: neither k nor its group is
// switched off there. Whether the account may have k is the caller's.
func (s *Service) Wants(ctx context.Context, userID string, k Kind, ch Channel) (bool, error) {
	off, err := s.q.HasOptOut(ctx, sqlc.HasOptOutParams{UserID: userID, Kind: string(k), Channel: string(ch), Grp: string(k.Group())})
	if err != nil {
		return false, fmt.Errorf("opt-out %s/%s of %s: %w", k, ch, userID, err)
	}
	return !off, nil
}
