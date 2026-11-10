package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
)

func loginFrom(t *testing.T, e *testEnv, ip, email, password string) int {
	t.Helper()
	buf, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/v1/auth/login", strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	resp, _ := e.anon().do(req)
	return resp.StatusCode
}

func TestLoginThrottlePerAccountAndIP(t *testing.T) {
	e := newEnvCfg(t, func(c *Config) { c.TrustedProxy = true })
	const acct = "emp1@test.kz"
	for i := 0; i < 5; i++ {
		if got := loginFrom(t, e, "10.0.0.1", acct, "wrong-password"); got != 401 {
			t.Fatalf("failure %d: %d", i, got)
		}
	}
	if got := loginFrom(t, e, "10.0.0.1", acct, testPassword); got != 429 {
		t.Fatalf("pair (account, 10.0.0.1) must be throttled, got %d", got)
	}
	// an attacker at 10.0.0.1 cannot lock the real user out from another address
	if got := loginFrom(t, e, "10.0.0.2", acct, testPassword); got != 200 {
		t.Fatalf("correct password from another IP must work, got %d", got)
	}
	// the throttled IP still logs in to a different account (per-IP limit not reached)
	if got := loginFrom(t, e, "10.0.0.1", "emp2@test.kz", testPassword); got != 200 {
		t.Fatalf("other account from the throttled pair IP: %d", got)
	}
}

func TestLoginThrottleIPLimit(t *testing.T) {
	e := newEnvCfg(t, func(c *Config) { c.TrustedProxy = true })
	// 20 failures on distinct accounts from one IP exhaust the per-IP budget
	for i := 0; i < 20; i++ {
		loginFrom(t, e, "10.0.1.1", fmt.Sprintf("nobody%d@test.kz", i), "wrong-password")
	}
	if got := loginFrom(t, e, "10.0.1.1", "emp1@test.kz", testPassword); got != 429 {
		t.Fatalf("IP-blocked client must get 429 even with a correct password, got %d", got)
	}
	if got := loginFrom(t, e, "10.0.1.2", "emp1@test.kz", testPassword); got != 200 {
		t.Fatalf("other IP: %d", got)
	}
}

func TestForwardedHeadersIgnoredWithoutTrustedProxy(t *testing.T) {
	e := newEnv(t) // TRUSTED_PROXY off: a spoofed header must not give a fresh budget
	for i := 0; i < 5; i++ {
		loginFrom(t, e, fmt.Sprintf("172.16.0.%d", i+1), "emp1@test.kz", "wrong-password")
	}
	if got := loginFrom(t, e, "172.16.0.9", "emp1@test.kz", testPassword); got != 429 {
		t.Fatalf("spoofed X-Forwarded-For must be ignored, got %d", got)
	}
}

func TestLinkThemeRequiresScope(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	rA := e.create(f.emp1, f.catA, false) // dept A / mgrA
	rB := e.create(f.emp2, f.catB, false) // dept B / mgrB
	d, err := svc.LinkTheme(ctx, e.actor(f.mgrA), rA.ID, requests.ThemeInput{Title: ptrS("Секретная тема отдела А")})
	if err != nil || d.Theme == nil {
		t.Fatalf("create theme: %v", err)
	}
	themeA := d.Theme.ID
	// manager B has no request on that theme and did not create it: 404, same as a missing id
	_, err = svc.LinkTheme(ctx, e.actor(f.mgrB), rB.ID, requests.ThemeInput{ThemeID: &themeA})
	check(t, "foreign theme", err, apperr.KindNotFound)
	missing := int64(99999)
	_, err = svc.LinkTheme(ctx, e.actor(f.mgrB), rB.ID, requests.ThemeInput{ThemeID: &missing})
	check(t, "missing theme", err, apperr.KindNotFound)
	// admin may link any theme
	if _, err = svc.LinkTheme(ctx, e.actor(f.admin), rB.ID, requests.ThemeInput{ThemeID: &themeA}); err != nil {
		t.Fatalf("admin: %v", err)
	}
	// now manager B has a request on the theme in scope; a second B request may link it
	rB2 := e.create(f.emp2, f.catB, false)
	if _, err = svc.LinkTheme(ctx, e.actor(f.mgrB), rB2.ID, requests.ThemeInput{ThemeID: &themeA}); err != nil {
		t.Fatalf("theme with an in-scope request: %v", err)
	}
	// creator without any request on it
	own := e.exec(`INSERT INTO themes(title, created_by, created_at) VALUES ('Моя тема', ?, ?)`, f.mgrA, e.clk.Now().Unix())
	rA2 := e.create(f.emp1, f.catA, false)
	if _, err = svc.LinkTheme(ctx, e.actor(f.mgrA), rA2.ID, requests.ThemeInput{ThemeID: &own}); err != nil {
		t.Fatalf("creator: %v", err)
	}
	rB3 := e.create(f.emp2, f.catB, false)
	other := e.exec(`INSERT INTO themes(title, created_by, created_at) VALUES ('Чужая пустая тема', ?, ?)`, f.mgrA, e.clk.Now().Unix())
	_, err = svc.LinkTheme(ctx, e.actor(f.mgrB), rB3.ID, requests.ThemeInput{ThemeID: &other})
	check(t, "other manager, theme without requests", err, apperr.KindNotFound)
}

func TestSimilarCountAggregateForNonStaff(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	target := e.create(f.emp1, f.catA, false)
	e.create(f.emp2, f.catA, false)
	d, _ := svc.Get(ctx, e.actor(f.emp1), target.ID)
	if d.Similar.Count != 0 || len(d.Similar.Items) != 0 {
		t.Fatalf("one similar request must not be disclosed: %+v", d.Similar)
	}
	staff, _ := svc.Get(ctx, e.actor(f.mgrA), target.ID)
	if staff.Similar.Count != 1 {
		t.Fatalf("staff keeps the exact count: %+v", staff.Similar)
	}
	other := e.create(f.emp2, f.catA, false)
	d, _ = svc.Get(ctx, e.actor(f.emp1), target.ID)
	if d.Similar.Count != 2 || len(d.Similar.Items) != 0 {
		t.Fatalf("author sees an aggregate of 2 and no items: %+v", d.Similar)
	}
	// older than 90 days no longer counts for non-staff
	e.exec(`UPDATE requests SET created_at = ? WHERE id = ?`, e.clk.Now().Add(-100*24*time.Hour).Unix(), other.ID)
	d, _ = svc.Get(ctx, e.actor(f.emp1), target.ID)
	if d.Similar.Count != 0 {
		t.Fatalf("90-day window: %+v", d.Similar)
	}
	// another category does not count for non-staff
	e.exec(`UPDATE requests SET created_at = ?, category_id = ? WHERE id = ?`, e.clk.Now().Unix(), f.catB, other.ID)
	e.create(f.emp2, f.catB, false)
	d, _ = svc.Get(ctx, e.actor(f.emp1), target.ID)
	if d.Similar.Count != 0 {
		t.Fatalf("same category only: %+v", d.Similar)
	}
}

func TestStaffCannotReviewOwnRequest(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	for name, id := range map[string]int64{"admin": f.admin, "manager": f.mgrA} {
		r := e.create(id, f.catA, false)
		a := e.actor(id)
		_, err := svc.ChangeStatus(ctx, a, r.ID, requests.StatusInput{Status: "in_review"})
		if !apperr.Is(err, apperr.KindConflict) || !strings.Contains(err.Error(), "Нельзя рассматривать собственное обращение") {
			t.Errorf("%s status: %v", name, err)
		}
		_, err = svc.Assign(ctx, a, r.ID, requests.AssignInput{AssigneeSet: true, AssigneeID: &f.mgrA})
		if !apperr.Is(err, apperr.KindConflict) || !strings.Contains(err.Error(), "Нельзя рассматривать собственное обращение") {
			t.Errorf("%s assign: %v", name, err)
		}
	}
	// another staff member may still act on a staff-authored request
	r := e.create(f.mgrA, f.catA, false)
	if _, err := svc.ChangeStatus(ctx, e.actor(f.admin), r.ID, requests.StatusInput{Status: "in_review"}); err != nil {
		t.Fatalf("admin on manager's request: %v", err)
	}
}

func TestResultTextOnlyForTerminalStatus(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	r := e.create(f.emp1, f.catA, false)
	d, err := svc.ChangeStatus(ctx, e.actor(f.mgrA), r.ID, requests.StatusInput{Status: "in_review", ResultText: ptrS("не должно сохраниться")})
	if err != nil {
		t.Fatal(err)
	}
	if d.ResultText != nil {
		t.Fatalf("result_text stored for a non-terminal status: %q", *d.ResultText)
	}
	for _, it := range d.Timeline {
		if it.Text != nil && strings.Contains(*it.Text, "не должно") {
			t.Fatalf("result_text leaked into the timeline: %q", *it.Text)
		}
	}
	d, err = svc.ChangeStatus(ctx, e.actor(f.mgrA), r.ID, requests.StatusInput{Status: "declined", ResultText: ptrS("Сделано")})
	if err != nil || d.ResultText == nil || *d.ResultText != "Сделано" {
		t.Fatalf("terminal keeps result: %+v %v", d.ResultText, err)
	}
}

func TestRateLimitsPerUser(t *testing.T) {
	e := newEnvCfg(t, func(c *Config) { c.CreatePerHour, c.CommentPerHour = 2, 3 })
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	e.create(f.emp1, f.catA, false)
	r := e.create(f.emp1, f.catA, false)
	_, err := svc.Create(ctx, e.actor(f.emp1), requests.CreateInput{Type: requests.TypeProblem, CategoryID: f.catA,
		Title: "Ещё одно обращение", Body: "Подробное описание тестовой проблемы для проверки."}, nil)
	check(t, "create over limit", err, apperr.KindTooMany)
	// HTTP maps it to 429
	c := e.login("emp1@test.kz")
	resp, body := c.multipart(map[string]any{"type": "idea", "category_id": f.catA, "title": "Идея сверх лимита", "body": "Описание идеи достаточной длины для проверки."})
	expect(t, resp, body, 429)
	// other users are independent
	e.create(f.emp2, f.catB, false)

	for i := 0; i < 3; i++ {
		if _, err := svc.AddComment(ctx, e.actor(f.emp1), r.ID, requests.CommentInput{Kind: requests.KindComment, Body: "текст"}); err != nil {
			t.Fatalf("comment %d: %v", i, err)
		}
	}
	_, err = svc.AddComment(ctx, e.actor(f.emp1), r.ID, requests.CommentInput{Kind: requests.KindComment, Body: "текст"})
	check(t, "comment over limit", err, apperr.KindTooMany)
	resp, body = c.json("POST", fmt.Sprintf("/api/v1/requests/%d/comments", r.ID), map[string]string{"kind": "comment", "body": "ещё"})
	expect(t, resp, body, 429)

	e.clk.Advance(61 * time.Minute)
	if _, err := svc.AddComment(ctx, e.actor(f.emp1), r.ID, requests.CommentInput{Kind: requests.KindComment, Body: "после окна"}); err != nil {
		t.Fatalf("window must reopen: %v", err)
	}
}

func TestOverdueRefiresAfterDeadlineShift(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	r := e.create(f.emp1, f.catA, false) // first response 24h, resolution 120h
	e.clk.Advance(time.Hour)
	if _, err := svc.AddComment(ctx, e.actor(f.mgrA), r.ID, requests.CommentInput{Kind: requests.KindComment, Body: "принято"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(130 * time.Hour) // resolution (120h) missed
	if n, err := svc.CheckOverdue(ctx); err != nil || n != 1 {
		t.Fatalf("first overdue: n=%d err=%v", n, err)
	}
	if n, _ := svc.CheckOverdue(ctx); n != 0 {
		t.Fatalf("same deadline must stay idempotent, emitted %d", n)
	}
	// pause for 10h in needs_info: the deadline shifts, the answer lands right after the new deadline
	if _, err := svc.AddComment(ctx, e.actor(f.mgrA), r.ID, requests.CommentInput{Kind: requests.KindQuestion, Body: "уточните?"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(10 * time.Hour)
	if _, err := svc.AddComment(ctx, e.actor(f.emp1), r.ID, requests.CommentInput{Kind: requests.KindAnswer, Body: "ответ"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(2 * time.Hour)
	if n, err := svc.CheckOverdue(ctx); err != nil || n != 1 {
		t.Fatalf("re-overdue after shift must fire again: n=%d err=%v", n, err)
	}
	var cnt int
	if err := e.app.DB.QueryRow(`SELECT COUNT(DISTINCT meta) FROM request_events WHERE request_id = ? AND kind = 'overdue'`, r.ID).Scan(&cnt); err != nil || cnt != 2 {
		t.Fatalf("two overdue events with distinct due keys expected: %d %v", cnt, err)
	}
	var meta string
	_ = e.app.DB.QueryRow(`SELECT meta FROM request_events WHERE request_id = ? AND kind = 'overdue' ORDER BY id LIMIT 1`, r.ID).Scan(&meta)
	if !strings.HasPrefix(meta, "resolution@") {
		t.Fatalf("meta must be kind@due: %q", meta)
	}
}
