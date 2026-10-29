package notify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// signerLabel keeps unsubscribe tokens apart from every other token signed
// with sign; it never changes, or every mailed link stops working.
const signerLabel = "rezepte unsubscribe"

// tokenTTL is how long a mailed unsubscribe link works.
const tokenTTL = 90 * 24 * time.Hour

// errLinkGone is every unsubscribe answer for a link that does not work:
// forged, expired, or naming an account or kind that is gone.
var errLinkGone = errors.New("unsubscribe link no longer works")

// UnsubscribeLinks returns the links a mail of kind k to userID carries:
// the page for this kind, the page for its whole group, and the RFC 8058
// one-click address.
func (s *Service) UnsubscribeLinks(userID string, k Kind) mail.Unsubscribe {
	token, _ := s.signer.Sign(subject(userID, k), tokenTTL)
	q := url.Values{"u": {userID}, "k": {string(k)}, "s": {token}}.Encode()
	kindURL := s.publicURL + "/unsubscribe?" + q
	return mail.Unsubscribe{
		KindURL:     kindURL,
		GroupURL:    kindURL + "&scope=group",
		OneClickURL: s.publicURL + "/api/v1/unsubscribe/one-click?" + q,
	}
}

func subject(userID string, k Kind) string { return userID + ":" + string(k) }

// resolve checks an unsubscribe link and returns its account and kind, or
// errLinkGone. The signature is checked first, so a stranger's guess never
// reaches the database.
func (s *Service) resolve(ctx context.Context, userID, kind, token string) (user.User, Kind, error) {
	k := Kind(kind)
	if !s.signer.Verify(subject(userID, k), token) {
		return user.User{}, "", errLinkGone
	}
	u, err := s.users.ByID(ctx, userID)
	if errors.Is(err, user.ErrNotFound) {
		return user.User{}, "", errLinkGone
	}
	if err != nil {
		return user.User{}, "", err
	}
	if !slices.Contains(KindsFor(u.Role, ChannelMail), k) {
		return user.User{}, "", errLinkGone
	}
	return u, k, nil
}

// unsubscribeState is what the unsubscribe page shows. A link reaches only
// the mail channel.
type unsubscribeState struct {
	Username        string        `json:"username"`
	Email           string        `json:"email" doc:"The account's address, masked like the address-changed notice: the link's holder may be an old mailbox"`
	Kind            Kind          `json:"kind" enum:"comment,recipe,tasty,invite_accepted"`
	Group           Group         `json:"group" enum:"household,admin"`
	KindSubscribed  bool          `json:"kindSubscribed" doc:"Whether the account gets the link's kind by mail: its own switch and its group's are on"`
	GroupSubscribed bool          `json:"groupSubscribed" doc:"Whether the group's mail switch is on"`
	GroupKinds      map[Kind]bool `json:"groupKinds" doc:"Every kind of the group the account may have by mail, true when it gets that mail"`
}

func (s *Service) state(ctx context.Context, u user.User, k Kind) (unsubscribeState, error) {
	p, err := s.Preferences(ctx, u)
	if err != nil {
		return unsubscribeState{}, err
	}
	g := k.Group()
	groupOn := p.Groups[g][ChannelMail]
	// Masked: whoever holds an old link may be the address the account left.
	st := unsubscribeState{Username: u.Username, Email: mail.MaskAddress(u.Email), Kind: k, Group: g,
		KindSubscribed: groupOn && p.Kinds[k][ChannelMail], GroupSubscribed: groupOn, GroupKinds: map[Kind]bool{}}
	for _, gk := range KindsFor(u.Role, ChannelMail) {
		if gk.Group() == g {
			st.GroupKinds[gk] = groupOn && p.Kinds[gk][ChannelMail]
		}
	}
	return st, nil
}

// switchByLink switches the mail of a link's kind k, or with group its
// group's mail switch, to subscribed. Push and the other kinds' own
// choices are never touched, so an undo restores exactly what was before.
func (s *Service) switchByLink(ctx context.Context, u user.User, k Kind, group, subscribed bool) error {
	set := Prefs{Kinds: map[Kind]map[Channel]bool{k: {ChannelMail: subscribed}}}
	if group {
		set = Prefs{Groups: map[Group]map[Channel]bool{k.Group(): {ChannelMail: subscribed}}}
	}
	if _, err := s.SetPreferences(ctx, u, set); err != nil {
		return fmt.Errorf("unsubscribe link: %w", err)
	}
	return nil
}
