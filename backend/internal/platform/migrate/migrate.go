// Package migrate is a tiny ordered SQL migration runner.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// Up applies every *.sql file (except *.down.sql) from fsys that is not yet recorded
// in schema_migrations, each in its own transaction, in lexical order.
func Up(ctx context.Context, d *sql.DB, fsys fs.FS) ([]string, error) {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".sql") || strings.HasSuffix(n, ".down.sql") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)

	var applied []string
	for _, n := range names {
		version := strings.TrimSuffix(n, ".sql")
		var exists int
		if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&exists); err != nil {
			return applied, fmt.Errorf("check migration %s: %w", version, err)
		}
		if exists > 0 {
			continue
		}
		body, err := fs.ReadFile(fsys, n)
		if err != nil {
			return applied, fmt.Errorf("read migration %s: %w", n, err)
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return applied, fmt.Errorf("begin migration %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Unix()); err != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("record migration %s: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return applied, fmt.Errorf("commit migration %s: %w", version, err)
		}
		applied = append(applied, version)
	}
	return applied, nil
}
