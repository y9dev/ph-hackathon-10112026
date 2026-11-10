package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/modules/insights"
	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/eventbus"
	"modusvoice/internal/seed"
)

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func notifiedAbout(items []notifications.View, reqID int64) bool {
	for _, n := range items {
		if n.RequestID == reqID {
			return true
		}
	}
	return false
}

func hasKind(items []notifications.View, reqID int64, kind string) bool {
	for _, n := range items {
		if n.RequestID == reqID && n.Kind == kind {
			return true
		}
	}
	return false
}

type errEnvelope struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	} `json:"error"`
}

// TestJourneys runs J1 submit -> J2 track -> J3 review -> J4 clarify over HTTP.
func TestJourneys(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	emp := e.login("emp1@test.kz")
	mgr := e.login("mgra@test.kz")

	// J1 submit
	resp, body := emp.get("/api/v1/meta")
	expect(t, resp, body, 200)
	meta := decode[requests.Meta](t, body)
	if len(meta.Categories) != 3 || meta.Limits.MaxFiles != 5 || len(meta.AssignableUsers) != 0 {
		t.Fatalf("meta for employee: %+v", meta)
	}
	resp, body = emp.get(fmt.Sprintf("/api/v1/routing/preview?category_id=%d&type=problem", f.catA))
	expect(t, resp, body, 200)
	prev := decode[requests.Preview](t, body)
	if !prev.Routed || prev.Department == nil || prev.Department.ID != f.deptA || prev.FirstResponseHours != 24 || prev.AssigneeName == nil {
		t.Fatalf("preview: %s", body)
	}
	resp, body = emp.multipart(map[string]any{"type": "problem", "category_id": f.catA, "title": "Медленно открывается 1С",
		"body": "После обновления 1С открывается около пяти минут, работать невозможно.", "proposed_solution": "Проверить сервер"},
		file{"screen.png", pngBytes})
	expect(t, resp, body, 201)
	created := decode[requests.Detail](t, body)
	if created.Number != "MV-0001" || created.Status != "new" || created.Assignee == nil || created.Assignee.ID != f.mgrA || len(created.Attachments) != 1 {
		t.Fatalf("created: %s", body)
	}
	if !strings.Contains(string(body), "Медленно открывается 1С") {
		t.Fatal("UTF-8 text must round-trip")
	}
	id := created.ID
	path := fmt.Sprintf("/api/v1/requests/%d", id)

	// J2 track
	resp, body = emp.get("/api/v1/requests?scope=mine")
	expect(t, resp, body, 200)
	if l := decode[requests.ListResult](t, body); l.Total != 1 || l.Items[0].ID != id || !l.Items[0].Author.IsMe {
		t.Fatalf("mine: %s", body)
	}
	if kinds := timelineKinds(t, emp, path); kinds != "created,routed" {
		t.Fatalf("timeline: %s", kinds)
	}

	// J3 review: inbox shows it under needs_action, notification arrived
	resp, body = mgr.get("/api/v1/requests?scope=inbox&tab=needs_action")
	expect(t, resp, body, 200)
	inbox := decode[requests.ListResult](t, body)
	if inbox.Total != 1 || inbox.Counts.NeedsAction != 1 || inbox.Counts.All != 1 {
		t.Fatalf("inbox: %s", body)
	}
	resp, body = mgr.get("/api/v1/notifications?unread=true")
	expect(t, resp, body, 200)
	if n := decode[notifications.List](t, body); n.UnreadCount != 1 || n.Items[0].Kind != "created" || n.Items[0].RequestNumber != "MV-0001" {
		t.Fatalf("manager notifications: %s", body)
	}
	resp, body = mgr.json("POST", path+"/status", map[string]any{"status": "in_review"})
	expect(t, resp, body, 200)
	d := decode[requests.Detail](t, body)
	if d.SLA.FirstRespondedAt == nil || !contains(d.AllowedStatuses, "accepted") || !d.CanAssign {
		t.Fatalf("after in_review: %s", body)
	}
	resp, body = mgr.json("POST", path+"/status", map[string]any{"status": "implemented"})
	expect(t, resp, body, 409) // in_review -> implemented not allowed

	// J4 clarify
	resp, body = mgr.json("POST", path+"/comments", map[string]any{"kind": "question", "body": "На каком компьютере?"})
	expect(t, resp, body, 201)
	if d := decode[requests.Detail](t, body); d.Status != "needs_info" || d.SLA.State != "paused" {
		t.Fatalf("question: %s", body)
	}
	resp, body = emp.get(path)
	expect(t, resp, body, 200)
	if d := decode[requests.Detail](t, body); !d.AwaitingAuthor || !contains(d.AllowedCommentKinds, "answer") {
		t.Fatalf("author view in needs_info: %s", body)
	}
	resp, body = emp.json("POST", path+"/comments", map[string]any{"kind": "answer", "body": "Ноутбук в отделе А"})
	expect(t, resp, body, 201)
	if d := decode[requests.Detail](t, body); d.Status != "in_review" {
		t.Fatalf("answer: %s", body)
	}
	resp, body = mgr.json("POST", path+"/comments", map[string]any{"kind": "internal", "body": "Внутренняя заметка"})
	expect(t, resp, body, 201)

	// close with result (AC-03)
	mgr.json("POST", path+"/status", map[string]any{"status": "accepted"})
	resp, body = mgr.json("POST", path+"/status", map[string]any{"status": "implemented", "result_text": "  "})
	expect(t, resp, body, 400)
	if ee := decode[errEnvelope](t, body); ee.Error.Code != "validation_failed" || ee.Error.Fields["result_text"] == "" {
		t.Fatalf("result_text required: %s", body)
	}
	resp, body = mgr.json("POST", path+"/status", map[string]any{"status": "implemented", "result_text": "Сервер 1С обновлён, время запуска 20 секунд."})
	expect(t, resp, body, 200)

	resp, body = emp.get(path)
	expect(t, resp, body, 200)
	final := decode[requests.Detail](t, body)
	if final.Status != "implemented" || final.ResultText == nil || final.ClosedAt == nil || final.SLA.State != "done" {
		t.Fatalf("author sees result: %s", body)
	}
	if strings.Contains(string(body), "Внутренняя заметка") {
		t.Fatal("internal notes must never reach the author")
	}
	if len(final.AllowedStatuses) != 0 || contains(final.AllowedCommentKinds, "comment") {
		t.Fatalf("closed request offers actions: %v %v", final.AllowedStatuses, final.AllowedCommentKinds)
	}
	resp, body = emp.get("/api/v1/notifications")
	n := decode[notifications.List](t, body)
	kinds := map[string]int{}
	for _, it := range n.Items {
		kinds[it.Kind]++
	}
	if kinds["status_changed"] < 3 || kinds["question"] != 1 {
		t.Fatalf("author notifications (AC-08): %v", kinds)
	}
	resp, body = emp.json("POST", "/api/v1/notifications/read", map[string]any{"all": true})
	expect(t, resp, body, 204)
	resp, body = emp.get("/api/v1/notifications?unread=true")
	if decode[notifications.List](t, body).UnreadCount != 0 {
		t.Fatal("mark all read")
	}

	// attachment download: author and manager yes, others 404
	attPath := fmt.Sprintf("/api/v1/attachments/%d", created.Attachments[0].ID)
	resp, body = emp.get(attPath)
	expect(t, resp, body, 200)
	if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;") || resp.Header.Get("Content-Type") != "image/png" || !bytes.Equal(body, pngBytes) {
		t.Fatalf("download headers: %v", resp.Header)
	}
	resp, body = e.login("emp2@test.kz").get(attPath)
	expect(t, resp, body, 404)
	resp, body = e.login("mgrb@test.kz").get(path)
	expect(t, resp, body, 404)
}

func timelineKinds(t *testing.T, c *client, path string) string {
	t.Helper()
	_, body := c.get(path)
	d := decode[requests.Detail](t, body)
	var ks []string
	for _, it := range d.Timeline {
		ks = append(ks, it.Kind)
	}
	return strings.Join(ks, ",")
}

func contains[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestDuplicateRequiresOriginalInScope(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("mgra@test.kz")
	a := e.create(e.fx.emp1, e.fx.catA, false)
	b := e.create(e.fx.emp1, e.fx.catA, false)
	other := e.create(e.fx.emp2, e.fx.catB, false) // department B, out of mgrA scope
	p := fmt.Sprintf("/api/v1/requests/%d/status", b.ID)
	resp, body := mgr.json("POST", p, map[string]any{"status": "duplicate", "result_text": "Дубликат"})
	expect(t, resp, body, 400)
	resp, body = mgr.json("POST", p, map[string]any{"status": "duplicate", "result_text": "Дубликат", "duplicate_of_id": other.ID})
	expect(t, resp, body, 400)
	resp, body = mgr.json("POST", p, map[string]any{"status": "duplicate", "result_text": "Дубликат", "duplicate_of_id": a.ID})
	expect(t, resp, body, 200)
	if d := decode[requests.Detail](t, body); d.DuplicateOf == nil || d.DuplicateOf.Number != a.Number {
		t.Fatalf("duplicate_of: %s", body)
	}
}

func TestUnauthenticatedAndErrors(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for _, p := range []string{"/api/v1/auth/me", "/api/v1/requests?scope=mine", "/api/v1/meta", "/api/v1/notifications", "/api/v1/analytics/summary", "/api/v1/admin/users", "/api/v1/attachments/1"} {
		resp, body := c.get(p)
		expect(t, resp, body, 401)
		if ee := decode[errEnvelope](t, body); ee.Error.Code != "unauthorized" || ee.Error.Message == "" {
			t.Fatalf("%s envelope: %s", p, body)
		}
	}
	emp := e.login("emp1@test.kz")
	for p, status := range map[string]int{
		"/api/v1/requests?scope=inbox":          403,
		"/api/v1/requests?scope=bogus":          400,
		"/api/v1/requests?scope=mine&sort=x":    400,
		"/api/v1/requests/999999":               404,
		"/api/v1/requests/abc":                  404,
		"/api/v1/requests/export.csv":           403,
		"/api/v1/analytics/suggested-groups":    403,
		"/api/v1/admin/routing-rules":           403,
		"/api/v1/nope":                          404,
		"/api/v1/routing/preview?category_id=1": 400,
	} {
		resp, body := emp.get(p)
		expect(t, resp, body, status)
	}
	resp, body := emp.json("POST", "/api/v1/admin/categories", map[string]any{"name": "X"})
	expect(t, resp, body, 403)
	// CSRF: cross-origin unsafe request rejected
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/v1/auth/logout", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, body = emp.do(req)
	expect(t, resp, body, 403)
	// JSON body required
	req, _ = http.NewRequest("POST", e.srv.URL+"/api/v1/notifications/read", strings.NewReader(`{"all":true}`))
	req.Header.Set("Content-Type", "text/plain")
	resp, body = emp.do(req)
	expect(t, resp, body, 400)
	// security headers
	resp, _ = emp.get("/api/v1/auth/me")
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("security headers: %v", resp.Header)
	}
	// SPA fallback and health
	resp, body = e.anon().get("/requests/42")
	if resp.StatusCode != 200 || !strings.Contains(string(body), "<html") {
		t.Fatalf("spa fallback: %d", resp.StatusCode)
	}
	resp, body = e.anon().get("/healthz")
	expect(t, resp, body, 200)
}

func TestLoginThrottleAndSession(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	resp, body := c.json("POST", "/api/v1/auth/login", map[string]string{"email": "emp1@test.kz", "password": testPassword})
	expect(t, resp, body, 200)
	var cookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "mv_session" {
			cookie = ck
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 7*24*3600 {
		t.Fatalf("session cookie: %+v", cookie)
	}
	resp, body = c.json("POST", "/api/v1/auth/logout", nil)
	expect(t, resp, body, 204)
	resp, body = c.get("/api/v1/auth/me")
	expect(t, resp, body, 401)

	for i := 0; i < 5; i++ {
		resp, body = c.json("POST", "/api/v1/auth/login", map[string]string{"email": "emp1@test.kz", "password": "wrong-password"})
		expect(t, resp, body, 401)
	}
	resp, body = c.json("POST", "/api/v1/auth/login", map[string]string{"email": "emp1@test.kz", "password": testPassword})
	expect(t, resp, body, 429)
	if ee := decode[errEnvelope](t, body); ee.Error.Code != "too_many_requests" {
		t.Fatalf("429 envelope: %s", body)
	}
	resp, body = c.json("POST", "/api/v1/auth/login", map[string]string{"email": "emp2@test.kz", "password": testPassword})
	expect(t, resp, body, 200) // other account unaffected (per-IP limit not reached)
	resp, body = e.anon().get("/api/v1/auth/demo-users")
	expect(t, resp, body, 200)
}

func TestAttachmentRejections(t *testing.T) {
	e := newEnv(t)
	c := e.login("emp1@test.kz")
	payload := map[string]any{"type": "idea", "category_id": e.fx.catA, "title": "Идея с вложением", "body": "Описание идеи достаточной длины для проверки."}
	big := make([]byte, 6<<20)
	copy(big, "%PDF-1.4")
	cases := map[string][]file{
		"too many files":    {{"1.txt", []byte("a")}, {"2.txt", []byte("a")}, {"3.txt", []byte("a")}, {"4.txt", []byte("a")}, {"5.txt", []byte("a")}, {"6.txt", []byte("a")}},
		"forbidden type":    {{"tool.exe", []byte("MZ\x90\x00")}},
		"spoofed png":       {{"photo.png", []byte("MZ\x90\x00 not a png")}},
		"file over 10 MB":   {{"big.pdf", append(append([]byte{}, big...), big...)}},
		"total over 10 MB":  {{"a.pdf", big}, {"b.pdf", big}},
		"binary as txt":     {{"notes.txt", []byte("abc\x00def")}},
		"html as allowed":   {{"page.html", []byte("<script>alert(1)</script>")}},
		"double extension":  {{"photo.png.exe", pngBytes}},
		"svg not allowlist": {{"img.svg", []byte("<svg/>")}},
	}
	for name, files := range cases {
		resp, body := c.multipart(payload, files...)
		if resp.StatusCode != 400 {
			t.Errorf("%s: status %d: %s", name, resp.StatusCode, body)
			continue
		}
		if ee := decode[errEnvelope](t, body); ee.Error.Fields["files"] == "" {
			t.Errorf("%s: no files field error: %s", name, body)
		}
	}
	resp, body := c.get("/api/v1/requests?scope=mine")
	if decode[requests.ListResult](t, body).Total != 0 {
		t.Fatalf("rejected uploads must not create requests: %s", body)
	}
	_ = resp
	resp, body = c.multipart(payload, file{"doc.pdf", []byte("%PDF-1.4 ok")}, file{"Отчёт.txt", []byte("Текст отчёта")})
	expect(t, resp, body, 201)
}

// TestAnonymity serializes every endpoint that can mention an anonymous request, as another
// employee, the managing manager and an admin, and asserts the author's identity never appears.
func TestAnonymity(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.fx
	var busEvents []eventbus.Event
	e.app.Bus.Subscribe("*", func(_ context.Context, ev eventbus.Event) error { busEvents = append(busEvents, ev); return nil })

	author := e.login("emp1@test.kz")
	resp, body := author.multipart(map[string]any{"type": "problem", "category_id": f.catA, "title": "Переработки без компенсации",
		"body": "Бригады регулярно работают до ночи без компенсации переработок.", "is_anonymous": true}, file{"scan.pdf", []byte("%PDF-1.4")})
	expect(t, resp, body, 201)
	anon := decode[requests.Detail](t, body)
	if !anon.Author.IsMe || !anon.Author.Anonymous {
		t.Fatalf("author must see own anonymous request: %+v", anon.Author)
	}
	// a similar non-anonymous request so similar/group endpoints have content
	e.create(f.emp2, f.catA, false)
	mgr := e.login("mgra@test.kz")
	admin := e.login("admin@test.kz")
	p := fmt.Sprintf("/api/v1/requests/%d", anon.ID)
	mgr.json("POST", p+"/comments", map[string]any{"kind": "question", "body": "Уточните отдел"})
	author.json("POST", p+"/comments", map[string]any{"kind": "answer", "body": "Монтажный участок"})
	author.json("POST", p+"/comments", map[string]any{"kind": "comment", "body": "Дополнение от автора"})
	mgr.json("POST", p+"/theme", map[string]any{"title": "Переработки"})
	mgr.json("POST", p+"/status", map[string]any{"status": "accepted"})
	e.clk.Advance(150 * time.Hour) // (< 7-day session TTL) make it overdue so it appears in analytics overdue list
	if _, err := e.app.Requests.Service.CheckOverdue(ctx); err != nil {
		t.Fatal(err)
	}

	secrets := []string{"Сотрудник Первый", "emp1@test.kz"}
	endpoints := map[string][]string{
		"other-employee": {"/api/v1/requests?scope=mine", "/api/v1/notifications", p, "/api/v1/meta"},
		"manager": {p, "/api/v1/requests?scope=inbox", "/api/v1/requests?scope=all&q=Переработки", "/api/v1/notifications",
			"/api/v1/analytics/summary", "/api/v1/analytics/suggested-groups", "/api/v1/themes", "/api/v1/requests/export.csv",
			fmt.Sprintf("/api/v1/attachments/%d", anon.Attachments[0].ID)},
		"admin": {p, "/api/v1/requests?scope=all", "/api/v1/requests?scope=inbox&assignee=none", "/api/v1/notifications",
			"/api/v1/analytics/summary", "/api/v1/analytics/suggested-groups", "/api/v1/themes", "/api/v1/requests/export.csv"},
	}
	clients := map[string]*client{"other-employee": e.login("emp2@test.kz"), "manager": mgr, "admin": admin}
	sawAnonLabel := false
	for who, paths := range endpoints {
		for _, path := range paths {
			resp, body := clients[who].get(path)
			if resp.StatusCode >= 500 || resp.StatusCode == 401 {
				t.Fatalf("%s %s: %d", who, path, resp.StatusCode)
			}
			s := string(body)
			for _, secret := range secrets {
				if strings.Contains(s, secret) {
					t.Errorf("%s %s leaks %q", who, path, secret)
				}
			}
			assertNoAuthorID(t, who+" "+path, body, f.emp1)
			if strings.Contains(s, requests.AnonymousLabel) {
				sawAnonLabel = true
			}
		}
	}
	if !sawAnonLabel {
		t.Fatal("anonymous label never rendered: test is not exercising the mapper")
	}
	// the author still sees their own name and can track it
	_, body = author.get(p)
	if !strings.Contains(string(body), "Сотрудник Первый") {
		t.Fatal("author must see own identity")
	}
	// control: a non-anonymous request does show the author's name to the manager
	open := e.create(f.emp1, f.catA, false)
	_, body = mgr.get(fmt.Sprintf("/api/v1/requests/%d", open.ID))
	if !strings.Contains(string(body), "Сотрудник Первый") {
		t.Fatal("control failed: non-anonymous author should be visible")
	}
	// event payloads never carry the anonymous author's id
	for _, ev := range busEvents {
		if ev.RequestID == anon.ID {
			if v, okk := ev.Data["actor_id"]; okk && v == f.emp1 {
				t.Errorf("event %s leaks author id", ev.Topic)
			}
		}
	}
	// service-level: summary for admin contains the overdue anonymous request masked
	sum, err := e.app.Insights.Service.Summary(ctx, e.actor(f.admin), insights.SummaryFilter{})
	if err != nil || len(sum.Overdue) == 0 {
		t.Fatalf("overdue list: %v %d", err, len(sum.Overdue))
	}
}

// assertNoAuthorID walks JSON and fails if any id-like author/user/actor field equals uid.
func assertNoAuthorID(t *testing.T, where string, body []byte, uid int64) {
	t.Helper()
	var v any
	if json.Unmarshal(body, &v) != nil {
		return // CSV / binary
	}
	var walk func(x any)
	walk = func(x any) {
		switch n := x.(type) {
		case map[string]any:
			for k, val := range n {
				switch k {
				case "author_id", "user_id", "actor_id", "uploaded_by", "created_by":
					if num, isNum := val.(float64); isNum && int64(num) == uid {
						t.Errorf("%s: field %s exposes author id", where, k)
					}
				case "author", "actor":
					if m, isMap := val.(map[string]any); isMap {
						if _, has := m["id"]; has {
							t.Errorf("%s: %s object carries an id", where, k)
						}
						if _, has := m["email"]; has {
							t.Errorf("%s: %s object carries an email", where, k)
						}
					}
				}
				walk(val)
			}
		case []any:
			for _, it := range n {
				walk(it)
			}
		}
	}
	walk(v)
}

func TestCSVExport(t *testing.T) {
	e := newEnv(t)
	e.create(e.fx.emp1, e.fx.catA, true)
	e.create(e.fx.emp1, e.fx.catA, false)
	resp, body := e.login("mgra@test.kz").get("/api/v1/requests/export.csv")
	expect(t, resp, body, 200)
	if !bytes.HasPrefix(body, []byte("\xEF\xBB\xBF")) || !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("csv must start with BOM and be an attachment")
	}
	s := string(body)
	if !strings.Contains(s, "Номер;Тип;Категория") || !strings.Contains(s, requests.AnonymousLabel) || strings.Count(s, "Сотрудник Первый") != 1 {
		t.Fatalf("csv content: %s", s)
	}
}

func TestAdminUserManagement(t *testing.T) {
	e := newEnv(t)
	admin := e.login("admin@test.kz")
	resp, body := admin.json("POST", "/api/v1/admin/users", map[string]any{"name": "Новый Сотрудник", "email": "New@Test.kz", "role": "employee", "department_id": e.fx.deptA, "password": "Secret123"})
	expect(t, resp, body, 201)
	resp, body = admin.json("POST", "/api/v1/admin/users", map[string]any{"name": "Дубль", "email": "new@test.kz", "role": "employee", "password": "Secret123"})
	expect(t, resp, body, 409)
	resp, body = admin.json("POST", "/api/v1/admin/users", map[string]any{"name": "X", "email": "bad", "role": "boss", "password": "1"})
	expect(t, resp, body, 400)
	if ee := decode[errEnvelope](t, body); len(ee.Error.Fields) < 3 {
		t.Fatalf("field errors: %s", body)
	}
	c := e.anon()
	resp, body = c.json("POST", "/api/v1/auth/login", map[string]string{"email": "new@test.kz", "password": "Secret123"})
	expect(t, resp, body, 200)
	id := decode[struct {
		ID int64 `json:"id"`
	}](t, body).ID
	resp, body = admin.json("PATCH", fmt.Sprintf("/api/v1/admin/users/%d", id), map[string]any{"active": false})
	expect(t, resp, body, 200)
	resp, body = c.get("/api/v1/auth/me")
	expect(t, resp, body, 401) // deactivation revokes sessions
	resp, body = admin.json("PATCH", fmt.Sprintf("/api/v1/admin/users/%d", e.fx.admin), map[string]any{"role": "employee"})
	expect(t, resp, body, 400) // cannot demote self
	resp, body = admin.json("POST", "/api/v1/admin/categories", map[string]any{"name": "Новая категория", "icon": "star"})
	expect(t, resp, body, 201)
	resp, body = admin.json("POST", "/api/v1/admin/categories", map[string]any{"name": "Новая категория"})
	expect(t, resp, body, 409)
	resp, body = admin.json("PATCH", fmt.Sprintf("/api/v1/admin/routing-rules/%d", e.fx.ruleA), map[string]any{"assignee_user_id": nil, "first_response_hours": 12})
	expect(t, resp, body, 200)
	if !strings.Contains(string(body), `"assignee_user_id":null`) || !strings.Contains(string(body), `"first_response_hours":12`) {
		t.Fatalf("patch rule: %s", body)
	}
	resp, body = admin.json("DELETE", fmt.Sprintf("/api/v1/admin/routing-rules/%d", e.fx.ruleA), nil)
	expect(t, resp, body, 204)
	resp, body = admin.get("/api/v1/admin/departments")
	expect(t, resp, body, 200)
}

// TestSeedDemo checks the demo seed loads, is idempotent, and yields the 4 deliberate clusters.
func TestSeedDemo(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// seed needs an empty DB: wipe the fixture
	for _, q := range []string{`DELETE FROM routing_rules`, `DELETE FROM categories`, `UPDATE departments SET head_user_id = NULL`, `DELETE FROM users`, `DELETE FROM departments`} {
		e.exec(q)
	}
	if err := seed.Run(ctx, e.app.DB, e.clk.Now(), "$2a$04$invalidhashforseedtestxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(ctx, e.app.DB, e.clk.Now(), "x"); err != seed.ErrAlreadySeeded {
		t.Fatalf("second seed: %v", err)
	}
	var total, anonCount, users int
	_ = e.app.DB.QueryRow(`SELECT COUNT(*), SUM(is_anonymous) FROM requests`).Scan(&total, &anonCount)
	_ = e.app.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	if total < 65 || anonCount < 6 || users < 20 {
		t.Fatalf("seed volume: requests=%d anon=%d users=%d", total, anonCount, users)
	}
	var adminID int64
	_ = e.app.DB.QueryRow(`SELECT id FROM users WHERE email = 'admin@modus.demo'`).Scan(&adminID)
	admin := e.actor(adminID)
	groups, err := e.app.Insights.Service.SuggestedGroups(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) < 4 {
		t.Fatalf("want >= 4 suggested groups, got %d", len(groups))
	}
	sum, err := e.app.Insights.Service.Summary(ctx, admin, insights.SummaryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range sum.ByStatus {
		if st.Count == 0 {
			t.Errorf("status %s not represented in seed", st.Status)
		}
	}
	if sum.KPIs.OutcomeShare != 1 || sum.KPIs.OverdueCount == 0 || sum.KPIs.MedianFirstResponseHours == nil {
		t.Fatalf("seed KPIs: %+v", sum.KPIs)
	}
	var unrouted int
	_ = e.app.DB.QueryRow(`SELECT COUNT(*) FROM requests WHERE routing_source = 'unrouted'`).Scan(&unrouted)
	if unrouted == 0 {
		t.Fatal("seed must show the unrouted flow")
	}
}
