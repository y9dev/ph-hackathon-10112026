package app

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// newEnvWithLogs is newEnvCfg with every log line (app logger AND slog.Default) captured.
func newEnvWithLogs(t *testing.T, tweak func(*Config)) (*testEnv, *syncBuffer) {
	t.Helper()
	buf := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	prev := slog.Default()
	slog.SetDefault(log)
	t.Cleanup(func() { slog.SetDefault(prev) })
	clk := &testClock{t: time.Date(2026, 11, 10, 6, 0, 0, 0, time.UTC)}
	cfg := Config{DataDir: t.TempDir(), BcryptCost: bcrypt.MinCost, DisableTicker: true, DisplayTZ: "UTC", Now: clk.Now, DemoMode: true, CreatePerHour: -1, CommentPerHour: -1}
	if tweak != nil {
		tweak(&cfg)
	}
	a, err := New(cfg, log)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	e := &testEnv{t: t, app: a, clk: clk}
	e.fixture()
	e.srv = httptest.NewServer(a.Handler)
	t.Cleanup(func() { e.srv.Close(); _ = a.Close() })
	return e, buf
}

// identityTables legitimately hold the user (who they are / that they are logged in); they never
// reference requests. Everything else must not link to an anonymous author.
var identityTables = map[string]bool{"users": true, "sessions": true, "schema_migrations": true}

// assertNoUserLinkInDB scans EVERY table: columns with a foreign key to users(id) must never hold
// uid, and no text column may contain one of the author's identifying strings.
func assertNoUserLinkInDB(t *testing.T, e *testEnv, uid int64, secrets ...string) {
	t.Helper()
	tables := queryStrings(t, e, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if len(tables) < 10 {
		t.Fatalf("table scan found only %v", tables)
	}
	scanned := 0
	for _, tbl := range tables {
		if identityTables[tbl] {
			continue
		}
		// columns referencing users
		rows, err := e.app.DB.Query(fmt.Sprintf(`SELECT "from" FROM pragma_foreign_key_list('%s') WHERE "table" = 'users'`, tbl))
		if err != nil {
			t.Fatal(err)
		}
		var userCols []string
		for rows.Next() {
			var c string
			_ = rows.Scan(&c)
			userCols = append(userCols, c)
		}
		_ = rows.Close()
		for _, c := range userCols {
			var n int
			if err := e.app.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE "%s" = ?`, tbl, c), uid).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n > 0 {
				t.Errorf("table %s column %s holds the anonymous author's id in %d row(s)", tbl, c, n)
			}
			scanned++
		}
		// every column as text
		cols := queryStrings(t, e, fmt.Sprintf(`SELECT name FROM pragma_table_info('%s')`, tbl))
		for _, c := range cols {
			for _, s := range secrets {
				var n int
				if err := e.app.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE instr(CAST("%s" AS TEXT), ?) > 0`, tbl, c), s).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n > 0 {
					t.Errorf("table %s column %s contains %q", tbl, c, s)
				}
			}
		}
	}
	if scanned < 8 {
		t.Fatalf("expected to scan the user-referencing columns of requests/comments/events/attachments/notifications/themes, scanned %d", scanned)
	}
}

func queryStrings(t *testing.T, e *testEnv, q string, args ...any) []string {
	t.Helper()
	rows, err := e.app.DB.Query(q, args...)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func countRows(t *testing.T, e *testEnv, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.app.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func mustNotContain(t *testing.T, where, s string, secrets ...string) {
	t.Helper()
	for _, x := range secrets {
		if x != "" && strings.Contains(s, x) {
			t.Errorf("%s contains %q", where, x)
		}
	}
}
