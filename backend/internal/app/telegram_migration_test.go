package app

import (
	"strings"
	"testing"

	"modusvoice/migrations"
)

func hasColumn(t *testing.T, e *testEnv, table, col string) bool {
	t.Helper()
	var n int
	if err := e.app.DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// 0004 runs clean on an empty database (newEnv migrates from scratch) and its .down.sql reverts it.
func TestMigration0004TelegramUpAndDown(t *testing.T) {
	e := newEnv(t)
	if !hasColumn(t, e, "notifications", "telegram_sent_at") || !hasColumn(t, e, "notifications", "telegram_attempts") || !hasColumn(t, e, "telegram_links", "chat_id") {
		t.Fatal("0004 schema missing after migrate")
	}
	// constraints: one chat per user and one user per chat
	e.exec(`INSERT INTO telegram_links(user_id, chat_id, linked_at) VALUES (?, 1, 1)`, e.fx.emp1)
	if _, err := e.app.DB.Exec(`INSERT INTO telegram_links(user_id, chat_id, linked_at) VALUES (?, 2, 1)`, e.fx.emp1); err == nil {
		t.Fatal("a user may be linked only once")
	}
	if _, err := e.app.DB.Exec(`INSERT INTO telegram_links(user_id, chat_id, linked_at) VALUES (?, 1, 1)`, e.fx.emp2); err == nil {
		t.Fatal("a chat may be linked only once")
	}

	down, err := migrations.FS.ReadFile("0004_telegram.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(down), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := e.app.DB.Exec(stmt); err != nil {
			t.Fatalf("down statement %q: %v", stmt, err)
		}
	}
	if hasColumn(t, e, "notifications", "telegram_sent_at") || hasColumn(t, e, "telegram_links", "chat_id") {
		t.Fatal("down migration left schema behind")
	}
}
