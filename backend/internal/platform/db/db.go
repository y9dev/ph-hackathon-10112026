// Package db opens the SQLite database and offers small helpers shared by stores.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// Querier is satisfied by *sql.DB and *sql.Tx so store code can run inside or outside a transaction.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func init() {
	// mv_lower lowercases Unicode text (SQLite's lower() is ASCII-only), used for Cyrillic search.
	_ = sqlite.RegisterDeterministicScalarFunction("mv_lower", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		switch v := args[0].(type) {
		case string:
			return strings.ToLower(v), nil
		case []byte:
			return strings.ToLower(string(v)), nil
		case nil:
			return "", nil
		default:
			return strings.ToLower(fmt.Sprint(v)), nil
		}
	})
}

// Open opens (creating if needed) the SQLite database at path with WAL, foreign keys and
// IMMEDIATE transactions so concurrent writers wait on busy_timeout instead of failing.
func Open(path string) (*sql.DB, error) {
	dsn := filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	d.SetMaxOpenConns(8)
	d.SetMaxIdleConns(8)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return d, nil
}

// InTx runs fn in a transaction, rolling back on error or panic and committing otherwise.
func InTx(ctx context.Context, d *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// IsUniqueViolation reports whether err is a UNIQUE constraint failure.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// IsForeignKeyViolation reports whether err is a FOREIGN KEY constraint failure.
func IsForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

// ErrNoRows is re-exported for stores.
var ErrNoRows = sql.ErrNoRows

// IsNoRows reports whether err is sql.ErrNoRows.
func IsNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// Unix converts t to stored unix seconds.
func Unix(t time.Time) int64 { return t.UTC().Unix() }

// Time converts stored unix seconds to UTC time.
func Time(v int64) time.Time { return time.Unix(v, 0).UTC() }

// NullTime converts a nullable column to *time.Time.
func NullTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := Time(v.Int64)
	return &t
}

// NullInt converts a nullable integer column to *int64.
func NullInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

// NullStr converts a nullable text column to *string.
func NullStr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

// IntOrNil converts *int64 to a driver value (nil for NULL).
func IntOrNil(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// TimeOrNil converts *time.Time to a driver value.
func TimeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return Unix(*t)
}

// Placeholders returns "?,?,?" for n parameters (n >= 1).
func Placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
