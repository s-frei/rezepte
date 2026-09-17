// Package user manages accounts and password verification.
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	ErrInvalidCredentials    = errors.New("invalid username or password")
	ErrAdminPasswordRequired = errors.New("REZEPTE_ADMIN_PASSWORD is required on first start")
)

// Service reads and writes users.
type Service struct {
	q   *sqlc.Queries
	now func() time.Time
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB) *Service {
	return &Service{q: sqlc.New(conn), now: time.Now}
}

// Create stores a new user with a hashed password.
func (s *Service) Create(ctx context.Context, username, password string, role Role) (User, error) {
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

// Authenticate verifies the password and returns the user. Unknown users and
// wrong passwords both yield ErrInvalidCredentials.
func (s *Service) Authenticate(ctx context.Context, username, password string) (User, error) {
	row, err := s.q.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
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
