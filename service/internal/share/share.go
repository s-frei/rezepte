// Package share manages public, revocable recipe links: creating, listing,
// revoking, resolving and sweeping them, and the rule that decides whether a
// link currently serves the recipe it points at.
package share

import (
	"errors"
	"time"
)

// Errors returned by Service.
var (
	// ErrExists means the actor already has a public link for this recipe;
	// the caller gets that existing Share alongside the error.
	ErrExists = errors.New("a public link already exists for this recipe")
	// ErrSharingOff means the instance owner has switched public sharing off.
	ErrSharingOff = errors.New("public sharing is off")
	// ErrNotAllowed means an admin has withdrawn public sharing from this
	// user.
	ErrNotAllowed = errors.New("public sharing not allowed for this user")
	// ErrLifetime means the requested lifetime is not one of
	// settings.ShareLifetimes, or exceeds the instance maximum.
	ErrLifetime = errors.New("public share lifetime not allowed")
	// ErrRecipeNotFound means the recipe id does not exist.
	ErrRecipeNotFound = errors.New("recipe not found")
	// ErrNotFound means no share matches - unknown id, or one that belongs
	// to somebody else and the actor is not an admin.
	ErrNotFound = errors.New("share not found")
	// ErrAdminRequired means the operation is admin-only.
	ErrAdminRequired = errors.New("admin required")
)

// Status is where a public link currently stands.
type Status string

// Statuses a share can be in.
const (
	// StatusActive serves the recipe (subject to its own expiry, which this
	// status does not by itself guarantee - see evaluate).
	StatusActive Status = "active"
	// StatusPaused is dormant: public sharing is off instance-wide, or the
	// creator's own permission to share was withdrawn. It resumes once
	// whichever caused it is switched back on, unless expired meanwhile.
	StatusPaused Status = "paused"
	// StatusLimited is dormant because a lowered instance maximum, counted
	// from the link's creation, has passed. Reversible: raising the maximum
	// again revives it.
	StatusLimited Status = "limited"
)

// RecipeRef is the recipe a share points at.
type RecipeRef struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Creator names who created a share (not recipe.Person: the OpenAPI document
// needs every schema name once). It is only attached when listing every
// share in the household as an admin, who filters the list by ID - display
// names need not be unique.
type Creator struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
}

// Share is a public, revocable link to a recipe.
type Share struct {
	ID        string    `json:"id"`
	Recipe    RecipeRef `json:"recipe"`
	Path      string    `json:"path" doc:"The public page's path, /s/{token}"`
	CreatedAt time.Time `json:"createdAt"`
	// ExpiresAt is the effective expiry - the earlier of the share's own
	// expiry and the instance maximum counted from CreatedAt. nil means
	// permanent.
	ExpiresAt *time.Time `json:"expiresAt" nullable:"true" doc:"Effective expiry: the earlier of the link's own and the instance maximum counted from createdAt; null is permanent"`
	Status    Status     `json:"status" enum:"active,paused,limited" doc:"active serves the recipe; paused and limited serve nothing until the owner or an admin allows it again"`
	// CreatedBy is only set when List is called by an admin with all=true;
	// every other response omits it.
	CreatedBy *Creator `json:"createdBy,omitempty" doc:"Who created the link; only in an admin's list of everyone's links"`
}

// evaluate decides a share's Status, its effective expiry (the earlier of
// its own expiresAt and createdAt+maxDays), and whether it currently serves
// the recipe.
//
// instanceOn and creatorAllowed dormancy (paused) takes priority over
// everything else - a link that is switched off is not "limited" even if
// its maximum has also passed. A limit from maxDays that has passed makes
// the link "limited", not serving. Otherwise the link is "active", and it
// serves unless its own expiresAt has passed - that state (own expiry past,
// otherwise unrestricted) is not a distinct status because it is terminal:
// unlike paused or limited it does not reverse, and Sweep removes the row
// before it would matter to a caller for long.
func evaluate(createdAt time.Time, expiresAt *time.Time, instanceOn, creatorAllowed bool, maxDays *int, now time.Time) (Status, *time.Time, bool) {
	effective := expiresAt
	limited := false
	if maxDays != nil {
		limit := createdAt.AddDate(0, 0, *maxDays)
		if effective == nil || limit.Before(*effective) {
			effective = &limit
		}
		limited = !limit.After(now)
	}
	switch {
	case !instanceOn || !creatorAllowed:
		return StatusPaused, effective, false
	case limited:
		return StatusLimited, effective, false
	default:
		serving := expiresAt == nil || expiresAt.After(now)
		return StatusActive, effective, serving
	}
}
