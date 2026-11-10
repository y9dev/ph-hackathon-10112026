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
		if err := applyOne(ctx, d, version, string(body)); err != nil {
			return applied, err
		}
		applied = append(applied, version)
	}
	return applied, nil
}

// applyOne runs one migration in a transaction on a dedicated connection with foreign keys
// switched off (required to rebuild tables referenced by FKs without cascading deletes; the
// pragma is a no-op inside a transaction). Integrity is verified before commit.
func applyOne(ctx context.Context, d *sql.DB, version, body string) error {
	conn, err := d.Conn(ctx)
	if err != nil {
		return fmt.Errorf("conn for migration %s: %w", version, err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable fk: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA foreign_keys = ON`) }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, body); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	var violations int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("fk check %s: %w", version, err)
	}
	if violations > 0 {
		_ = tx.Rollback()
		return fmt.Errorf("migration %s leaves %d foreign key violations", version, violations)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Unix()); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
