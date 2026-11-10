package identity

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by the store when a row does not exist.
var ErrNotFound = errors.New("identity: not found")

// ErrDuplicateEmail is returned when an email is already taken.
var ErrDuplicateEmail = errors.New("identity: duplicate email")

// ErrBadDepartment is returned when department_id does not exist.
var ErrBadDepartment = errors.New("identity: bad department")

// Store persists users and sessions.
type Store interface {
	UserByEmail(ctx context.Context, email string) (User, error)
	UserByID(ctx context.Context, id int64) (User, error)
	LedDepartments(ctx context.Context, userID int64) ([]int64, error)
	ListUsers(ctx context.Context) ([]User, error)
	UsersByEmails(ctx context.Context, emails []string) ([]User, error)
	ActiveAdminIDs(ctx context.Context) ([]int64, error)
	ActiveStaff(ctx context.Context) ([]User, error)
	InsertUser(ctx context.Context, u User) (int64, error)
	UpdateUser(ctx context.Context, u User) error

	CreateSession(ctx context.Context, tokenHash string, userID int64, expires, now time.Time) error
	SessionUserID(ctx context.Context, tokenHash string, now time.Time) (int64, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteUserSessions(ctx context.Context, userID int64) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
}
