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

// Roles known to the service.
const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

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
	ErrLastAdmin             = errors.New("the last admin cannot be removed or demoted")
	ErrWrongPassword         = errors.New("current password is wrong")
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

// SetRole changes a user's role. Demoting the last admin fails with
// ErrLastAdmin so the instance can never end up without one.
func (s *Service) SetRole(ctx context.Context, id string, role Role) (User, error) {
	var out User
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getForUpdate(ctx, q, id)
		if err != nil {
			return err
		}
		if Role(row.Role) == RoleAdmin && role != RoleAdmin {
			if err := guardLastAdmin(ctx, q); err != nil {
				return err
			}
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

// Delete removes a user. Deleting yourself or the last admin is refused.
// Recipes the user created move to actorID (recipes.created_by is NOT
// NULL and has no ON DELETE clause); sessions go with the row through
// ON DELETE CASCADE.
func (s *Service) Delete(ctx context.Context, actorID, id string) error {
	if actorID == id {
		return ErrSelfDelete
	}
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getForUpdate(ctx, q, id)
		if err != nil {
			return err
		}
		if Role(row.Role) == RoleAdmin {
			if err := guardLastAdmin(ctx, q); err != nil {
				return err
			}
		}
		if err := q.ReassignRecipes(ctx, sqlc.ReassignRecipesParams{NewOwner: actorID, OldOwner: id}); err != nil {
			return fmt.Errorf("reassign recipes of %s: %w", id, err)
		}
		if _, err := q.DeleteUser(ctx, id); err != nil {
			return fmt.Errorf("delete user %s: %w", id, err)
		}
		return nil
	})
}

// SetPassword replaces a password without checking the old one (admin reset).
func (s *Service) SetPassword(ctx context.Context, id, password string) error {
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
	return s.SetPassword(ctx, id, next)
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

// guardLastAdmin returns ErrLastAdmin when at most one admin exists. It
// must run inside the transaction that removes or demotes an admin:
// db.Tx opens transactions with _txlock=immediate, so the count cannot
// be raced by a concurrent demotion or delete.
func guardLastAdmin(ctx context.Context, q *sqlc.Queries) error {
	n, err := q.CountAdmins(ctx)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if n <= 1 {
		return ErrLastAdmin
	}
	return nil
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

// EnsureInitialAdmin creates the admin account when no users exist yet.
// It is a no-op once any user exists.
func (s *Service) EnsureInitialAdmin(ctx context.Context, username, password string) error {
	n, err := s.q.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return nil
	}
	if password == "" {
		return ErrAdminPasswordRequired
	}
	if _, err := s.Create(ctx, username, password, RoleAdmin); err != nil {
		return fmt.Errorf("create initial admin: %w", err)
	}
	return nil
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
