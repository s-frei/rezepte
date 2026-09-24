package recipe

import (
	"errors"

	"github.com/s-frei/rezepte/service/internal/user"
)

// Errors returned when the caller may not change a recipe. Both are answered
// with 403.
var (
	ErrEditForbidden   = errors.New("recipe edit locked")
	ErrDeleteForbidden = errors.New("recipe delete not allowed")
)

// Policy says who may edit a recipe besides its author and admins.
// PolicyDefault follows the household setting; the other two override it.
type Policy string

// The three policies. The database stores PolicyDefault as NULL.
const (
	PolicyDefault Policy = "default"
	PolicyOpen    Policy = "open"
	PolicyLocked  Policy = "locked"
)

// policyFromDB maps the stored column to a Policy; NULL is PolicyDefault.
func policyFromDB(p *string) Policy {
	if p == nil {
		return PolicyDefault
	}
	return Policy(*p)
}

// toDB maps p to the stored column; PolicyDefault and "" become NULL.
func (p Policy) toDB() *string {
	if p == PolicyDefault || p == "" {
		return nil
	}
	s := string(p)
	return &s
}

// Guarded is the part of a recipe the permission rule reads.
type Guarded struct {
	CreatedBy string
	Policy    Policy
}

// Locked reports whether g is closed to everyone but its author and admins,
// given the household default.
func Locked(g Guarded, lockedByDefault bool) bool {
	switch g.Policy {
	case PolicyOpen:
		return false
	case PolicyLocked:
		return true
	default:
		return lockedByDefault
	}
}

// privileged reports whether actor always may change g: its author, or an
// admin (the owner included).
func privileged(actor user.User, g Guarded) bool {
	return actor.ID == g.CreatedBy || actor.Role.IsAdmin()
}

// CanEdit reports whether actor may edit g and its images.
func CanEdit(actor user.User, g Guarded, lockedByDefault bool) bool {
	return privileged(actor, g) || !Locked(g, lockedByDefault)
}

// CanDelete reports whether actor may delete g. An open policy never grants
// it: only the author and admins delete.
func CanDelete(actor user.User, g Guarded) bool {
	return privileged(actor, g)
}

// CanChangePolicy reports whether actor may change g's policy.
func CanChangePolicy(actor user.User, g Guarded) bool {
	return privileged(actor, g)
}
