// Package notify decides who wants which switchable mail: the notification
// kinds, their groups, every account's preferences and the signed
// unsubscribe links a mail carries. Security notices are not kinds; nothing
// here can switch them off.
package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/sign"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Kind is one switchable mail. Its value is stored in notification_optouts
// and travels in unsubscribe links, so it never changes.
type Kind string

// The kinds: a comment on a recipe the account follows, the daily digest
// of recipes others added, and a person setting up an account the admin
// invited.
const (
	KindComment        Kind = "comment"
	KindRecipe         Kind = "recipe"
	KindInviteAccepted Kind = "invite_accepted"
)

// Group is a set of kinds an unsubscribe link can switch off at once.
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
	{GroupHousehold, []Kind{KindComment, KindRecipe}},
	{GroupAdmin, []Kind{KindInviteAccepted}},
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

// KindsFor returns the kinds an account of role may have: the admin group
// only for admins and the owner.
func KindsFor(role user.Role) []Kind {
	kinds := slices.Clone(GroupHousehold.Kinds())
	if role.IsAdmin() {
		kinds = append(kinds, GroupAdmin.Kinds()...)
	}
	return kinds
}

// Mailer is the mail side notify sends through; *mail.Service is it.
type Mailer interface {
	Enabled(context.Context) bool
	SendComment(context.Context, mail.Comment) error
	SendInviteAccepted(context.Context, mail.InviteAccepted) error
	SendRecipeDigest(context.Context, mail.RecipeDigest) error
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
	publicURL string
}

// NewService returns a Service backed by conn that signs unsubscribe links
// with key (32 bytes, settings.Service.LinkPreviewKey) and puts publicURL in
// front of them. images serves the digest's cover photos.
func NewService(conn *sql.DB, key []byte, users *user.Service, avatars *avatar.Service, images *image.Service, m Mailer, publicURL string) *Service {
	return &Service{
		conn:      conn,
		q:         sqlc.New(conn),
		signer:    sign.New(key, signerLabel),
		users:     users,
		avatars:   avatars,
		images:    images,
		mailer:    m,
		publicURL: publicURL,
	}
}

// Preferences returns every kind u may have, true unless u opted out.
func (s *Service) Preferences(ctx context.Context, u user.User) (map[Kind]bool, error) {
	off, err := s.q.ListOptOuts(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("list opt-outs of %s: %w", u.ID, err)
	}
	prefs := map[Kind]bool{}
	for _, k := range KindsFor(u.Role) {
		prefs[k] = !slices.Contains(off, string(k))
	}
	return prefs, nil
}

// SetPreferences switches the kinds in set and returns every preference of
// u. A kind u may not have is ErrUnknownKind, and then nothing changes.
func (s *Service) SetPreferences(ctx context.Context, u user.User, set map[Kind]bool) (map[Kind]bool, error) {
	allowed := KindsFor(u.Role)
	for k := range set {
		if !slices.Contains(allowed, k) {
			return nil, fmt.Errorf("%w: %q", ErrUnknownKind, k)
		}
	}
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		for k, on := range set {
			var err error
			if on {
				err = q.RemoveOptOut(ctx, sqlc.RemoveOptOutParams{UserID: u.ID, Kind: string(k)})
			} else {
				err = q.AddOptOut(ctx, sqlc.AddOptOutParams{UserID: u.ID, Kind: string(k)})
			}
			if err != nil {
				return fmt.Errorf("set %s of %s: %w", k, u.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Preferences(ctx, u)
}

// Wants reports whether userID has not opted out of k.
func (s *Service) Wants(ctx context.Context, userID string, k Kind) (bool, error) {
	off, err := s.q.HasOptOut(ctx, sqlc.HasOptOutParams{UserID: userID, Kind: string(k)})
	if err != nil {
		return false, fmt.Errorf("opt-out %s of %s: %w", k, userID, err)
	}
	return !off, nil
}
