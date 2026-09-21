// Package user manages accounts and password verification.
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// Role is the authorization level of a user.
type Role string

// Roles known to the service, in ascending rank: a superadmin is an admin,
// an admin is not a superadmin.
const (
	RoleSuperadmin Role = "superadmin"
	RoleAdmin      Role = "admin"
	RoleUser       Role = "user"
)

// IsAdmin reports whether r carries admin rights. The superadmin does, so
// this is the only way to ask the question - a bare comparison against
// RoleAdmin silently excludes the instance owner and is always a bug.
func (r Role) IsAdmin() bool { return r == RoleAdmin || r == RoleSuperadmin }

// IsSuperadmin reports whether r is the instance owner.
func (r Role) IsSuperadmin() bool { return r == RoleSuperadmin }

// User is an account without secrets.
type User struct {
	ID        string
	Username  string
	Role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Errors returned by the service.
var (
	ErrNotFound              = errors.New("user not found")
	ErrUsernameTaken         = errors.New("username already taken")
	ErrInvalidUsername       = errors.New("username must not be empty")
	ErrInvalidCredentials    = errors.New("invalid username or password")
	ErrAdminPasswordRequired = errors.New("REZEPTE_ADMIN_PASSWORD is required on first start")
	ErrSelfDelete            = errors.New("cannot delete your own account")
	ErrWrongPassword         = errors.New("current password is wrong")
	ErrSuperadminProtected   = errors.New("the superadmin cannot be deleted, demoted or reset, and the role cannot be handed out")
	ErrSuperadminRequired    = errors.New("only the superadmin can manage admins")
	ErrNoSuperadmin          = errors.New("no superadmin in a non-empty users table; recreate the database")
)

// Service reads and writes users.
type Service struct {
	conn *sql.DB
	q    *sqlc.Queries
	now  func() time.Time
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB) *Service {
	return &Service{conn: conn, q: sqlc.New(conn), now: time.Now}
}

// Create stores a new user with a hashed password. username is trimmed of
// surrounding whitespace first; a username that is empty after trimming is
// rejected with ErrInvalidUsername.
func (s *Service) Create(ctx context.Context, username, password string, role Role) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, ErrInvalidUsername
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	now := db.FormatTime(s.now())
	row, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           uuid.Must(uuid.NewV7()).String(),
		Username:     username,
		PasswordHash: hash,
		Role:         string(role),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return fromRow(row)
}

// ByID loads a user by id.
func (s *Service) ByID(ctx context.Context, id string) (User, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user %s: %w", id, err)
	}
	return fromRow(row)
}

// List returns every user ordered by username.
func (s *Service) List(ctx context.Context) ([]User, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := make([]User, 0, len(rows))
	for _, row := range rows {
		u, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// SetRole changes a user's role. Both rank rules apply: who the target is,
// and which role is being handed out.
func (s *Service) SetRole(ctx context.Context, actor User, id string, role Role) (User, error) {
	var out User
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getForUpdate(ctx, q, id)
		if err != nil {
			return err
		}
		if err := guardTarget(actor.Role, Role(row.Role)); err != nil {
			return err
		}
		if err := guardAssignRole(actor.Role, role); err != nil {
			return err
		}
		updated, err := q.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{
			Role:      string(role),
			UpdatedAt: db.FormatTime(s.now()),
			ID:        id,
		})
		if err != nil {
			return fmt.Errorf("update role of %s: %w", id, err)
		}
		out, err = fromRow(updated)
		return err
	})
	return out, err
}

// Delete removes a user. Deleting yourself is refused, and so is any target
// the actor outranks too little to touch. Recipes the user created or last
// edited move to actor.ID (created_by and updated_by are both NOT NULL and
// have no ON DELETE clause); sessions go with the row through ON DELETE
// CASCADE.
func (s *Service) Delete(ctx context.Context, actor User, id string) error {
	if actor.ID == id {
		return ErrSelfDelete
	}
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getForUpdate(ctx, q, id)
		if err != nil {
			return err
		}
		if err := guardTarget(actor.Role, Role(row.Role)); err != nil {
			return err
		}
		if err := q.ReassignRecipes(ctx, sqlc.ReassignRecipesParams{NewOwner: actor.ID, OldOwner: id}); err != nil {
			return fmt.Errorf("reassign recipes of %s: %w", id, err)
		}
		if _, err := q.DeleteUser(ctx, id); err != nil {
			return fmt.Errorf("delete user %s: %w", id, err)
		}
		return nil
	})
}

// SetPassword replaces a password without checking the old one (admin reset).
// It reads the target first because the rank rules apply to it: the superadmin
// is never a valid target, not even for themselves - their password is theirs
// alone, and self-service goes through ChangePassword. The read, the guard and
// the write share one transaction, so a concurrent promotion cannot slip the
// target out from under a check that already passed.
func (s *Service) SetPassword(ctx context.Context, actor User, id, password string) error {
	// Hashing is the expensive half and needs no row, so it happens before
	// the transaction: db.Tx takes a write lock at BEGIN, and holding that
	// for an argon2id run would block every other writer for ~100ms.
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getForUpdate(ctx, q, id)
		if err != nil {
			return err
		}
		if err := guardTarget(actor.Role, Role(row.Role)); err != nil {
			return err
		}
		n, err := q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
			PasswordHash: hash,
			UpdatedAt:    db.FormatTime(s.now()),
			ID:           id,
		})
		if err != nil {
			return fmt.Errorf("update password of %s: %w", id, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// setPassword writes a new hash with no authorization check. ChangePassword
// and the operator reset are its other callers; both have already established
// that the write is allowed.
func (s *Service) setPassword(ctx context.Context, id, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	n, err := s.q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
		PasswordHash: hash,
		UpdatedAt:    db.FormatTime(s.now()),
		ID:           id,
	})
	if err != nil {
		return fmt.Errorf("update password of %s: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ChangePassword verifies current before storing next. A wrong current
// password yields ErrWrongPassword.
func (s *Service) ChangePassword(ctx context.Context, id, current, next string) error {
	row, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get user %s: %w", id, err)
	}
	ok, err := VerifyPassword(row.PasswordHash, current)
	if err != nil {
		return fmt.Errorf("verify password of %s: %w", id, err)
	}
	if !ok {
		return ErrWrongPassword
	}
	return s.setPassword(ctx, id, next)
}

// getForUpdate loads a user, mapping a missing row to ErrNotFound. It must
// run inside the transaction that updates or removes the row: db.Tx opens
// transactions with _txlock=immediate (BEGIN IMMEDIATE), so the isolation
// that keeps the row from changing before the caller writes it back comes
// from that lock, not from this read itself.
func getForUpdate(ctx context.Context, q *sqlc.Queries, id string) (sqlc.User, error) {
	row, err := q.GetUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sqlc.User{}, ErrNotFound
	}
	if err != nil {
		return sqlc.User{}, fmt.Errorf("get user %s: %w", id, err)
	}
	return row, nil
}

// dummyHash is a valid argon2id hash with no corresponding password, computed
// once on first use. Authenticate runs VerifyPassword against it for unknown
// usernames so that path costs the same one argon2 evaluation as a real user,
// which keeps an unknown-username response indistinguishable from a
// wrong-password response by timing.
var dummyHash = sync.OnceValue(func() string {
	hash, err := HashPassword("dummy")
	if err != nil {
		// Only fails if the OS RNG is broken, which is unrecoverable anyway.
		panic(fmt.Sprintf("hash dummy password: %v", err))
	}
	return hash
})

// Authenticate verifies the password and returns the user. username is
// trimmed the same way Create trims it, so a user created via the UI can
// log in with surrounding spaces typed by mistake. Unknown users and wrong
// passwords both yield ErrInvalidCredentials.
func (s *Service) Authenticate(ctx context.Context, username, password string) (User, error) {
	row, err := s.q.GetUserByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = VerifyPassword(dummyHash(), password)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("get user %q: %w", username, err)
	}
	ok, err := VerifyPassword(row.PasswordHash, password)
	if err != nil {
		return User{}, fmt.Errorf("verify password for %q: %w", username, err)
	}
	if !ok {
		return User{}, ErrInvalidCredentials
	}
	return fromRow(row)
}

// EnsureSuperadmin creates the instance owner when there is none and the table
// is empty. It asks about the owner rather than about users in general,
// which splits the start into three honest cases: a fresh instance gets the
// bootstrap account, an instance that already has an owner is left alone, and
// a populated instance with no owner refuses to come up rather than run
// without one.
func (s *Service) EnsureSuperadmin(ctx context.Context, username, password string) error {
	_, err := s.q.GetSuperadmin(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get superadmin: %w", err)
	}
	n, err := s.q.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return ErrNoSuperadmin
	}
	if password == "" {
		return ErrAdminPasswordRequired
	}
	// Create carries no authorization check - the rank rule for creation
	// lives at the API boundary - which is what lets this write the one
	// superadmin row.
	if _, err := s.Create(ctx, username, password, RoleSuperadmin); err != nil {
		return fmt.Errorf("create superadmin: %w", err)
	}
	return nil
}

// ResetSuperadminPassword sets the owner's password with no acting user. It is
// the operator's way back in after a forgotten password: nobody inside the
// application can reset that account, and the hash is argon2id, so no amount
// of sqlite3 produces a replacement by hand. Reaching this requires shell
// access to the host, which is the operator by definition.
func (s *Service) ResetSuperadminPassword(ctx context.Context, password string) (User, error) {
	row, err := s.q.GetSuperadmin(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoSuperadmin
	}
	if err != nil {
		return User{}, fmt.Errorf("get superadmin: %w", err)
	}
	if password == "" {
		return User{}, ErrAdminPasswordRequired
	}
	if err := s.setPassword(ctx, row.ID, password); err != nil {
		return User{}, err
	}
	return fromRow(row)
}

func fromRow(row sqlc.User) (User, error) {
	created, err := db.ParseTime(row.CreatedAt)
	if err != nil {
		return User{}, err
	}
	updated, err := db.ParseTime(row.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	return User{
		ID:        row.ID,
		Username:  row.Username,
		Role:      Role(row.Role),
		CreatedAt: created,
		UpdatedAt: updated,
	}, nil
}

func isUniqueViolation(err error) bool {
	var serr *sqlite.Error
	return errors.As(err, &serr) && serr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
