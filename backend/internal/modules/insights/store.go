package insights

import (
	"context"
	"time"
)

// Fact is one request reduced to what analytics needs.
type Fact struct {
	ID               int64
	Status           string
	Category         string
	Department       *string
	CreatedAt        time.Time
	FirstResponseDue time.Time
	ResolutionDue    time.Time
	FirstRespondedAt *time.Time
	ClosedAt         *time.Time
	HasResult        bool
	AutoRouted       bool
}

// ThemeRow is one (theme, request) pair.
type ThemeRow struct {
	ThemeID  int64
	Title    string
	Status   string
	Category string
}

// Scope is a trusted SQL predicate over requests aliased r (from authz.ScopeSQL) plus args.
type Scope struct {
	SQL  string
	Args []any
}

// Store is a READ-ONLY query interface over request data (R6).
type Store interface {
	Doc(ctx context.Context, id int64) (Doc, error)
	Docs(ctx context.Context, scope Scope, since time.Time, limit int) ([]Doc, error)
	Candidates(ctx context.Context, categoryID *int64, since time.Time, limit int) ([]Doc, error)
	Facts(ctx context.Context, scope Scope, from, to *time.Time, departmentID *int64) ([]Fact, error)
	FirstCreated(ctx context.Context, scope Scope) (*time.Time, error)
	ThemeRows(ctx context.Context, scope Scope) ([]ThemeRow, error)
}
