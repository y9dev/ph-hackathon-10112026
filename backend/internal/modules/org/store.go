package org

import (
	"context"
	"errors"
)

var (
	ErrNotFound  = errors.New("org: not found")
	ErrDuplicate = errors.New("org: duplicate")
	ErrBadRef    = errors.New("org: bad reference")
)

// Store persists departments, categories and routing rules.
type Store interface {
	Departments(ctx context.Context) ([]Department, error)
	Department(ctx context.Context, id int64) (Department, error)
	Categories(ctx context.Context, activeOnly bool) ([]Category, error)
	Category(ctx context.Context, id int64) (Category, error)
	InsertCategory(ctx context.Context, c Category) (int64, error)
	UpdateCategory(ctx context.Context, c Category) error

	Rules(ctx context.Context) ([]Rule, error)
	Rule(ctx context.Context, id int64) (Rule, error)
	// ActiveRule finds the active rule for (category, type); typ nil means the "any type" rule.
	ActiveRule(ctx context.Context, categoryID int64, typ *string) (*Rule, error)
	InsertRule(ctx context.Context, r Rule) (int64, error)
	UpdateRule(ctx context.Context, r Rule) error
	DeleteRule(ctx context.Context, id int64) error
}
