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
	if !slices.Contains(KindsFor(u.Role), k) {
		return user.User{}, "", errLinkGone
	}
	return u, k, nil
}

// scopeKinds returns the kinds a link for k switches: k alone, or with
// group every kind of k's group that u may have.
func scopeKinds(u user.User, k Kind, group bool) []Kind {
	if !group {
		return []Kind{k}
	}
	allowed := KindsFor(u.Role)
	return slices.DeleteFunc(slices.Clone(k.Group().Kinds()), func(g Kind) bool { return !slices.Contains(allowed, g) })
}

// unsubscribeState is what the unsubscribe page shows.
type unsubscribeState struct {
	Username        string `json:"username"`
	Email           string `json:"email" doc:"The account's address, masked like the address-changed notice: the link's holder may be an old mailbox"`
	Kind            Kind   `json:"kind" enum:"comment,recipe,invite_accepted"`
	Group           Group  `json:"group" enum:"household,admin"`
	KindSubscribed  bool   `json:"kindSubscribed"`
	GroupSubscribed bool   `json:"groupSubscribed" doc:"Whether every kind of the group the account may have is on"`
	// GroupKinds lets the page undo a group unsubscribe for only the kinds
	// that were on before it.
	GroupKinds map[Kind]bool `json:"groupKinds" doc:"Every kind of the group the account may have, true when it gets that mail"`
}

func (s *Service) state(ctx context.Context, u user.User, k Kind) (unsubscribeState, error) {
	prefs, err := s.Preferences(ctx, u)
	if err != nil {
		return unsubscribeState{}, err
	}
	// Masked: whoever holds an old link may be the address the account left.
	st := unsubscribeState{Username: u.Username, Email: mail.MaskAddress(u.Email), Kind: k, Group: k.Group(),
		KindSubscribed: prefs[k], GroupSubscribed: true, GroupKinds: map[Kind]bool{}}
	for _, g := range scopeKinds(u, k, true) {
		st.GroupSubscribed = st.GroupSubscribed && prefs[g]
		st.GroupKinds[g] = prefs[g]
	}
	return st, nil
}

// switchByLink sets the kinds a link reaches to subscribed; with group and
// a non-nil only, just those of them (an undo restores what was on).
func (s *Service) switchByLink(ctx context.Context, u user.User, k Kind, group bool, only []Kind, subscribed bool) error {
	reach := scopeKinds(u, k, group)
	set := map[Kind]bool{}
	for _, g := range reach {
		if !group || only == nil || slices.Contains(only, g) {
			set[g] = subscribed
		}
	}
	for _, g := range only {
		if !slices.Contains(reach, g) {
			return ErrUnknownKind
		}
	}
	if _, err := s.SetPreferences(ctx, u, set); err != nil {
		return fmt.Errorf("unsubscribe link: %w", err)
	}
	return nil
}
