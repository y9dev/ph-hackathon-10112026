package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"modusvoice/internal/modules/insights"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/seed"
)

// seededEnv returns a test app whose fixture is replaced by the demo seed (CR1).
func seededEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	for _, q := range []string{`DELETE FROM routing_rules`, `DELETE FROM categories`, `UPDATE departments SET head_user_id = NULL`, `DELETE FROM users`, `DELETE FROM departments`} {
		e.exec(q)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(seed.DemoPassword), bcrypt.MinCost)
	if err := seed.Run(context.Background(), e.app.DB, e.clk.Now(), string(hash)); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *testEnv) loginAs(email, password string) *client {
	e.t.Helper()
	c := e.anon()
	resp, body := c.json("POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password})
	if resp.StatusCode != 200 {
		e.t.Fatalf("login %s: %d %s", email, resp.StatusCode, body)
	}
	return c
}

// TestCRSeedDataset: the seed holds exactly the 20 dataset requests with verbatim texts, roles as
// users, 18 topic categories, one category without a rule, 3 anonymous requests without author link,
// realistic test dates and is idempotent.
func TestCRSeedDataset(t *testing.T) {
	e := seededEnv(t)
	ctx := context.Background()
	if err := seed.Run(ctx, e.app.DB, e.clk.Now(), "x"); err != seed.ErrAlreadySeeded {
		t.Fatalf("second seed: %v", err)
	}
	dataset, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "DATASET.md"))
	if err != nil {
		t.Fatalf("docs/DATASET.md: %v", err)
	}
	ds := string(dataset)
	// every request body is a verbatim dataset text; every user is a dataset role (or the admin)
	rows, err := e.app.DB.Query(`SELECT number, body, type, priority, status FROM requests ORDER BY number`)
	if err != nil {
		t.Fatal(err)
	}
	type rq struct{ number, body, typ, prio, status string }
	var got []rq
	for rows.Next() {
		var r rq
		_ = rows.Scan(&r.number, &r.body, &r.typ, &r.prio, &r.status)
		got = append(got, r)
	}
	_ = rows.Close()
	if len(got) != 20 {
		t.Fatalf("want exactly 20 requests, got %d", len(got))
	}
	for _, r := range got {
		if !strings.Contains(ds, "Текст обращения. "+r.body) {
			t.Errorf("%s body is not a verbatim dataset text: %q", r.number, r.body)
		}
	}
	// dataset status / priority / type mapping (MV-00NN = A-0NN)
	want := map[string][3]string{
		"MV-0001": {"problem", "medium", "new"}, "MV-0004": {"idea", "low", "implemented"}, "MV-0005": {"problem", "medium", "needs_info"},
		"MV-0003": {"problem", "high", "in_progress"}, "MV-0017": {"idea", "low", "declined"}, "MV-0018": {"feedback", "low", "closed"},
		"MV-0019": {"problem", "medium", "in_review"}, "MV-0013": {"problem", "medium", "needs_info"},
	}
	for _, r := range got {
		if w, ok := want[r.number]; ok && (r.typ != w[0] || r.prio != w[1] || r.status != w[2]) {
			t.Errorf("%s: %s/%s/%s want %v", r.number, r.typ, r.prio, r.status, w)
		}
	}
	for _, name := range queryStrings(t, e, `SELECT title FROM users`) {
		if name != "Администратор системы" && !strings.Contains(ds, name) {
			t.Errorf("user title %q is not a dataset role", name)
		}
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM users`); n != 23 {
		t.Errorf("users: %d", n)
	}
	cats := queryStrings(t, e, `SELECT name FROM categories`)
	if len(cats) != 18 {
		t.Errorf("categories: %d", len(cats))
	}
	for _, c := range cats {
		if !strings.Contains(ds, "· "+c+"\n") && !strings.Contains(ds, "· "+c+"\r\n") {
			t.Errorf("category %q is not a dataset topic", c)
		}
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM departments`); n != 8 {
		t.Errorf("departments: %d", n)
	}
	// one category intentionally without a rule; A-019 went through the admin queue
	if norule := queryStrings(t, e, `SELECT c.name FROM categories c WHERE NOT EXISTS (SELECT 1 FROM routing_rules r WHERE r.category_id = c.id)`); len(norule) != 1 || norule[0] != "Хранение документов" {
		t.Errorf("categories without a rule: %v", norule)
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM requests r WHERE r.number = 'MV-0019' AND r.routing_source = 'manual'
		AND EXISTS (SELECT 1 FROM request_events e WHERE e.request_id = r.id AND e.kind = 'routed' AND e.meta = 'unrouted')
		AND EXISTS (SELECT 1 FROM request_events e WHERE e.request_id = r.id AND e.kind = 'assigned')`); n != 1 {
		t.Error("A-019 must show the unrouted -> manual assignment flow")
	}
	// CR2: anonymous seed rows have no author link and no author notifications
	if n := countRows(t, e, `SELECT COUNT(*) FROM requests WHERE is_anonymous = 1 AND author_id IS NULL AND anon_token_hash IS NOT NULL AND number IN ('MV-0005','MV-0017','MV-0018')`); n != 3 {
		t.Errorf("anonymous seed rows: %d", n)
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM request_events e JOIN requests r ON r.id = e.request_id WHERE r.is_anonymous = 1 AND e.by_author = 1 AND e.actor_id IS NOT NULL`); n != 0 {
		t.Error("anonymous author events must have no actor")
	}
	// "Пример дальнейшего действия" -> internal notes; clarifications -> questions; results on all closed
	if n := countRows(t, e, `SELECT COUNT(*) FROM comments WHERE kind = 'internal'`); n != 15 {
		t.Errorf("internal notes: %d", n)
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM comments WHERE kind = 'question'`); n != 2 {
		t.Errorf("questions: %d", n)
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM requests WHERE status IN ('implemented','declined','closed') AND result_text IS NOT NULL AND closed_at IS NOT NULL`); n != 4 {
		t.Errorf("closed with result: %d", n)
	}
	// test dates: ~60 days, deterministic, a few overdue, on-time share between 0 and 1
	var minAge float64
	_ = e.app.DB.QueryRow(`SELECT (? - MIN(created_at)) / 86400.0 FROM requests`, e.clk.Now().Unix()).Scan(&minAge)
	if minAge < 55 || minAge > 62 {
		t.Errorf("oldest request is %.1f days old", minAge)
	}
	var adminID int64
	_ = e.app.DB.QueryRow(`SELECT id FROM users WHERE email = 'admin@modus.demo'`).Scan(&adminID)
	sum, err := e.app.Insights.Service.Summary(ctx, e.actor(adminID), insights.SummaryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	k := sum.KPIs
	if k.Total != 20 || k.OverdueCount < 3 || k.OutcomeShare != 1 || k.FirstResponseOnTimeShare == nil ||
		*k.FirstResponseOnTimeShare <= 0 || *k.FirstResponseOnTimeShare >= 1 || k.MedianResolutionHours == nil || k.FirstResponseSample != 15 {
		t.Fatalf("seed KPIs: %+v", k)
	}
	// demo logins and demo-users list
	resp, body := e.anon().get("/api/v1/auth/demo-users")
	expect(t, resp, body, 200)
	for _, email := range seed.DemoEmails {
		if !strings.Contains(string(body), email) {
			t.Errorf("demo-users lacks %s", email)
		}
	}
	emp := e.loginAs("purchasing.employee@modus.demo", seed.DemoPassword)
	resp, body = emp.get("/api/v1/requests?scope=mine")
	expect(t, resp, body, 200)
	if l := decode[requests.ListResult](t, body); l.Total != 4 {
		t.Errorf("Сотрудник закупок has 4 dataset requests (A-001,002,004,020), got %d", l.Total)
	}
	obs := e.loginAs("design.head@modus.demo", seed.DemoPassword)
	resp, body = obs.get("/api/v1/requests?scope=all&page_size=100")
	expect(t, resp, body, 200)
	l := decode[requests.ListResult](t, body)
	observer := 0
	for _, it := range l.Items {
		if it.AccessLevel == requests.AccessObserver {
			observer++
		}
	}
	if l.Total != 20 || observer != 19 {
		t.Errorf("observer demo: total %d observer rows %d", l.Total, observer)
	}
	// demo tokens open the anonymous requests
	for num, tok := range map[string]string{"MV-0005": seed.DemoTokenA005, "MV-0017": seed.DemoTokenA017, "MV-0018": seed.DemoTokenA018} {
		resp, body := e.anon().json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": tok})
		expect(t, resp, body, 200)
		if d := decode[requests.Detail](t, body); d.Number != num || !d.Author.IsMe {
			t.Errorf("token for %s opened %s", num, d.Number)
		}
	}
}
