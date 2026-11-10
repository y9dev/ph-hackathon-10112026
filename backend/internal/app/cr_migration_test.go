package app

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

// TestCRMigration0003FromPre0003 builds a 0002-schema database with a named and an anonymous
// request (author comments, events, attachments, notifications), applies the remaining migrations
// and checks the CR2 link removal and the new constraints.
func TestCRMigration0003FromPre0003(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	pre := fstest.MapFS{}
	for _, name := range []string{"0001_init.sql", "0002_lifecycle.sql"} {
		b, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		pre[name] = &fstest.MapFile{Data: b}
	}
	if _, err := migrate.Up(ctx, d, pre); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO departments(id, name) VALUES (1, 'D')`)
	exec(`INSERT INTO users(id, email, name, password_hash, role, created_at) VALUES (1, 'a@x.kz', 'Автор', 'h', 'employee', 1), (2, 'm@x.kz', 'Рук', 'h', 'manager', 1)`)
	exec(`INSERT INTO categories(id, name) VALUES (1, 'C')`)
	exec(`INSERT INTO routing_rules(category_id, request_type, department_id, first_response_hours, resolution_hours) VALUES (1, 'idea', 1, 24, 120)`)
	for _, r := range []struct {
		id   int
		anon int
	}{{1, 0}, {2, 1}} {
		exec(`INSERT INTO requests(id, number, type, category_id, title, body, author_id, is_anonymous, status, routing_source, first_response_due, resolution_due, created_at, updated_at)
			VALUES (?, ?, 'problem', 1, 't', 'b', 1, ?, 'in_review', 'rule', 10, 20, 1, 1)`, r.id, "MV-000"+string(rune('0'+r.id)), r.anon)
		exec(`INSERT INTO comments(request_id, author_id, kind, body, created_at) VALUES (?, 1, 'comment', 'author', 1), (?, 2, 'question', 'staff', 2)`, r.id, r.id)
		exec(`INSERT INTO request_events(request_id, actor_id, kind, created_at) VALUES (?, 1, 'created', 1), (?, 2, 'status_changed', 2), (?, NULL, 'routed', 1)`, r.id, r.id, r.id)
		exec(`INSERT INTO attachments(request_id, filename, stored_name, mime, size, uploaded_by, created_at) VALUES (?, 'f', ?, 'text/plain', 1, 1, 1)`, r.id, "s"+string(rune('0'+r.id)))
		exec(`INSERT INTO notifications(user_id, kind, request_id, title, body, created_at) VALUES (1, 'status_changed', ?, 't', 'b', 1), (2, 'created', ?, 't', 'b', 1)`, r.id, r.id)
	}
	applied, err := migrate.Up(ctx, d, migrations.FS)
	if err != nil || len(applied) == 0 || applied[0] != "0003_dataset_anonymity" {
		t.Fatalf("0003: %v %v", applied, err)
	}
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := d.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	checks := []struct {
		name string
		q    string
		want int
	}{
		{"named author kept", `SELECT COUNT(*) FROM requests WHERE id = 1 AND author_id = 1 AND priority = 'medium'`, 1},
		{"anonymous link nulled", `SELECT COUNT(*) FROM requests WHERE id = 2 AND author_id IS NULL AND is_anonymous = 1`, 1},
		{"named author comment flagged", `SELECT COUNT(*) FROM comments WHERE request_id = 1 AND author_id = 1 AND by_author = 1`, 1},
		{"anonymous author comment unlinked", `SELECT COUNT(*) FROM comments WHERE request_id = 2 AND author_id IS NULL AND by_author = 1`, 1},
		{"staff comments untouched", `SELECT COUNT(*) FROM comments WHERE author_id = 2 AND by_author = 0`, 2},
		{"anonymous author event unlinked", `SELECT COUNT(*) FROM request_events WHERE request_id = 2 AND kind = 'created' AND actor_id IS NULL AND by_author = 1`, 1},
		{"named author event flagged", `SELECT COUNT(*) FROM request_events WHERE request_id = 1 AND kind = 'created' AND actor_id = 1 AND by_author = 1`, 1},
		{"system events stay system", `SELECT COUNT(*) FROM request_events WHERE kind = 'routed' AND by_author = 0 AND actor_id IS NULL`, 2},
		{"anonymous attachment unlinked", `SELECT COUNT(*) FROM attachments WHERE request_id = 2 AND uploaded_by IS NULL`, 1},
		{"named attachment kept", `SELECT COUNT(*) FROM attachments WHERE request_id = 1 AND uploaded_by = 1`, 1},
		{"author notifications about anonymous request deleted", `SELECT COUNT(*) FROM notifications WHERE request_id = 2 AND user_id = 1`, 0},
		{"other notifications kept", `SELECT COUNT(*) FROM notifications`, 3},
		{"rules survive", `SELECT COUNT(*) FROM routing_rules WHERE request_type = 'idea'`, 1},
		{"no author id left on the anonymous request", `SELECT (SELECT COUNT(*) FROM comments WHERE request_id = 2 AND author_id = 1) + (SELECT COUNT(*) FROM request_events WHERE request_id = 2 AND actor_id = 1) + (SELECT COUNT(*) FROM attachments WHERE request_id = 2 AND uploaded_by = 1)`, 0},
		{"fk clean", `SELECT COUNT(*) FROM pragma_foreign_key_check`, 0},
	}
	for _, c := range checks {
		if got := count(c.q); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
	// new constraints
	bad := []struct{ name, q string }{
		{"anonymous with author", `UPDATE requests SET author_id = 1 WHERE id = 2`},
		{"named without author", `UPDATE requests SET author_id = NULL WHERE id = 1`},
		{"token on named", `UPDATE requests SET anon_token_hash = '` + string(make64('a')) + `' WHERE id = 1`},
		{"bad priority", `UPDATE requests SET priority = 'urgent' WHERE id = 1`},
		{"staff comment without author", `INSERT INTO comments(request_id, author_id, by_author, kind, body, created_at) VALUES (1, NULL, 0, 'comment', 'x', 1)`},
	}
	for _, b := range bad {
		if _, err := d.ExecContext(ctx, b.q); err == nil {
			t.Errorf("%s must violate a constraint", b.name)
		}
	}
	good := []string{
		`UPDATE requests SET type = 'feedback', priority = 'high' WHERE id = 1`,
		`UPDATE requests SET anon_token_hash = '` + string(make64('b')) + `' WHERE id = 2`,
		`INSERT INTO routing_rules(category_id, request_type, department_id, first_response_hours, resolution_hours) VALUES (1, 'feedback', 1, 24, 120)`,
	}
	for _, q := range good {
		exec(q)
	}
	if _, err := d.ExecContext(ctx, `UPDATE requests SET anon_token_hash = '`+string(make64('b'))+`' WHERE id = 2`); err != nil {
		t.Fatalf("same row update: %v", err)
	}
	// idempotent
	if again, err := migrate.Up(ctx, d, migrations.FS); err != nil || len(again) != 0 {
		t.Fatalf("rerun: %v %v", again, err)
	}
}

func make64(c byte) []byte {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return b
}
