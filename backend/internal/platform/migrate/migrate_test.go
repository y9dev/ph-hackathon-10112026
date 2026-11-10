package migrate_test

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"modusvoice/internal/platform/db"
	"modusvoice/internal/platform/migrate"
	"modusvoice/migrations"
)

// TestLifecycleMigrationFromAccepted applies 0001, stores data in the legacy `accepted`
// status with dependent rows, then applies 0002 and checks data survives intact.
func TestLifecycleMigrationFromAccepted(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	first := subset(t, "0001_init.sql")
	upTo0002 := subset(t, "0001_init.sql", "0002_lifecycle.sql") // later migrations are tested on their own
	if applied, err := migrate.Up(ctx, d, first); err != nil || len(applied) != 1 {
		t.Fatalf("0001: %v %v", applied, err)
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO departments(id, name) VALUES (1, 'D')`)
	mustExec(`INSERT INTO users(id, email, name, password_hash, role, created_at) VALUES (1, 'a@x.kz', 'A', 'h', 'employee', 1)`)
	mustExec(`INSERT INTO categories(id, name) VALUES (1, 'C')`)
	insReq := func(id int64, num, status string) {
		mustExec(`INSERT INTO requests(id, number, type, category_id, title, body, author_id, status, routing_source, first_response_due, resolution_due, created_at, updated_at)
			VALUES (?, ?, 'problem', 1, 't', 'b', 1, ?, 'rule', 10, 20, 1, 1)`, id, num, status)
	}
	insReq(1, "MV-0001", "accepted")
	insReq(2, "MV-0002", "in_review")
	mustExec(`INSERT INTO requests(id, number, type, category_id, title, body, author_id, status, routing_source, first_response_due, resolution_due, closed_at, result_text, created_at, updated_at) VALUES (3, 'MV-0003', 'problem', 1, 't', 'b', 1, 'declined', 'rule', 10, 20, 5, 'r', 1, 1)`)
	mustExec(`INSERT INTO comments(id, request_id, author_id, kind, body, created_at) VALUES (1, 1, 1, 'comment', 'hello', 1)`)
	mustExec(`INSERT INTO attachments(request_id, comment_id, filename, stored_name, mime, size, uploaded_by, created_at) VALUES (1, 1, 'f', 's', 'text/plain', 1, 1, 1)`)
	mustExec(`INSERT INTO request_events(request_id, kind, from_status, to_status, created_at) VALUES (1, 'status_changed', 'in_review', 'accepted', 1)`)
	mustExec(`INSERT INTO request_events(request_id, kind, from_status, to_status, created_at) VALUES (1, 'status_changed', 'accepted', 'implemented', 2)`)

	applied, err := migrate.Up(ctx, d, upTo0002)
	if err != nil || len(applied) != 1 || applied[0] != "0002_lifecycle" {
		t.Fatalf("0002: %v %v", applied, err)
	}
	count := func(q string) int {
		t.Helper()
		var n int
		if err := d.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	if n := count(`SELECT COUNT(*) FROM requests WHERE id = 1 AND status = 'in_progress'`); n != 1 {
		t.Fatalf("accepted request must become in_progress, got %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM requests WHERE id = 2 AND status = 'in_review'`); n != 1 {
		t.Fatal("other statuses must be preserved")
	}
	if count(`SELECT COUNT(*) FROM request_events WHERE to_status = 'accepted' OR from_status = 'accepted'`) != 0 ||
		count(`SELECT COUNT(*) FROM request_events WHERE to_status = 'in_progress'`) != 1 ||
		count(`SELECT COUNT(*) FROM request_events WHERE from_status = 'in_progress'`) != 1 {
		t.Fatal("history events must be rewritten from accepted to in_progress")
	}
	// the rebuild must not cascade-delete dependants
	if count(`SELECT COUNT(*) FROM comments`) != 1 || count(`SELECT COUNT(*) FROM attachments`) != 1 || count(`SELECT COUNT(*) FROM request_events`) != 2 {
		t.Fatal("dependent rows lost during table rebuild")
	}
	// new constraints: decision/in_progress and report accepted, accepted rejected
	if _, err := d.ExecContext(ctx, `UPDATE requests SET status = 'decision' WHERE id = 2`); err != nil {
		t.Fatalf("decision status: %v", err)
	}
	if _, err := d.ExecContext(ctx, `INSERT INTO comments(request_id, author_id, kind, body, created_at) VALUES (1, 1, 'report', 'r', 1)`); err != nil {
		t.Fatalf("report kind: %v", err)
	}
	if _, err := d.ExecContext(ctx, `UPDATE requests SET status = 'accepted' WHERE id = 2`); err == nil {
		t.Fatal("legacy accepted status must be rejected by the CHECK")
	}
	if _, err := d.ExecContext(ctx, `UPDATE requests SET status = 'closed', closed_at = 9 WHERE id = 2`); err == nil {
		t.Fatal("closed without result_text must violate the CHECK")
	}
	if _, err := d.ExecContext(ctx, `UPDATE requests SET status = 'closed', closed_at = 9, result_text = 'done' WHERE id = 2`); err != nil {
		t.Fatalf("closed with result: %v", err)
	}
	// foreign keys are enforced again on pooled connections
	if _, err := d.ExecContext(ctx, `INSERT INTO comments(request_id, author_id, kind, body, created_at) VALUES (999, 1, 'comment', 'x', 1)`); err == nil {
		t.Fatal("foreign keys must be re-enabled after migrating")
	}
	if count(`SELECT COUNT(*) FROM pragma_foreign_key_check`) != 0 {
		t.Fatal("foreign key violations after migration")
	}
	// idempotent
	if applied, err := migrate.Up(ctx, d, upTo0002); err != nil || len(applied) != 0 {
		t.Fatalf("rerun: %v %v", applied, err)
	}
}

// subset returns a migration FS holding only the named files of migrations.FS.
func subset(t *testing.T, names ...string) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	for _, name := range names {
		b, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = &fstest.MapFile{Data: b}
	}
	return out
}
