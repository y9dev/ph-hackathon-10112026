package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/modules/insights"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
)

// TestCRManagerVisibilityHTTP (CR6) checks, per role over HTTP, read access and the observer
// restrictions on every request endpoint for a request routed to department A.
func TestCRManagerVisibilityHTTP(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	r := e.create(f.emp1, f.catA, false)
	orig := e.create(f.emp1, f.catA, false)
	p := fmt.Sprintf("/api/v1/requests/%d", r.ID)
	type role struct {
		name, email, level string
		read               int
	}
	roles := []role{
		{"author", "emp1@test.kz", requests.AccessAuthor, 200},
		{"other-employee", "emp2@test.kz", "", 404},
		{"owner", "mgra@test.kz", requests.AccessOwner, 200},
		{"observer", "mgrb@test.kz", requests.AccessObserver, 200},
		{"admin", "admin@test.kz", requests.AccessOwner, 200},
	}
	for _, ro := range roles {
		c := e.login(ro.email)
		resp, body := c.get(p)
		expect(t, resp, body, ro.read)
		if ro.read == 200 {
			if d := decode[requests.Detail](t, body); d.AccessLevel != ro.level {
				t.Errorf("%s: access_level %s want %s", ro.name, d.AccessLevel, ro.level)
			}
		}
	}
	resp, body := e.anon().get(p)
	expect(t, resp, body, 401)

	obs := e.login("mgrb@test.kz")
	_, body = obs.get(p)
	d := decode[requests.Detail](t, body)
	if strings.Join(d.AllowedCommentKinds, ",") != "comment,internal" || len(d.AllowedStatuses) != 0 || d.CanAssign {
		t.Fatalf("observer actions: %v %v %v", d.AllowedCommentKinds, d.AllowedStatuses, d.CanAssign)
	}
	// owner actions -> 403 for the observer
	forbidden := []struct {
		path string
		body map[string]any
	}{
		{p + "/status", map[string]any{"status": "in_review"}},
		{p + "/status", map[string]any{"status": "duplicate", "duplicate_of_id": orig.ID, "result_text": "дубль"}},
		{p + "/status", map[string]any{"status": "closed", "result_text": "закрыто"}},
		{p + "/assign", map[string]any{"assignee_id": f.mgrB}},
		{p + "/theme", map[string]any{"title": "Тема наблюдателя"}},
		{p + "/comments", map[string]any{"kind": "question", "body": "?"}},
		{p + "/comments", map[string]any{"kind": "report", "body": "отчёт"}},
		{p + "/comments", map[string]any{"kind": "answer", "body": "ответ"}},
	}
	for _, fb := range forbidden {
		resp, body := obs.json("POST", fb.path, fb.body)
		expect(t, resp, body, 403)
	}
	// observer comment: allowed, NOT a first response, notifies author and owner
	resp, body = obs.json("POST", p+"/comments", map[string]any{"kind": "comment", "body": "Коллеги, у нас похожая ситуация"})
	expect(t, resp, body, 201)
	if d := decode[requests.Detail](t, body); d.SLA.FirstRespondedAt != nil {
		t.Fatal("observer comment must not count as first response")
	}
	resp, body = obs.json("POST", p+"/comments", map[string]any{"kind": "internal", "body": "Заметка наблюдателя"})
	expect(t, resp, body, 201)
	ctx := context.Background()
	authorN, _ := e.app.Notifications.Service.List(ctx, e.actor(f.emp1), false, 50)
	ownerN, _ := e.app.Notifications.Service.List(ctx, e.actor(f.mgrA), false, 50)
	if !hasKind(authorN.Items, r.ID, "comment") || !hasKind(ownerN.Items, r.ID, "comment") {
		t.Fatalf("observer comment must notify author and owner: %+v / %+v", authorN.Items, ownerN.Items)
	}
	for _, n := range authorN.Items {
		if strings.Contains(n.Title, "внутренняя заметка") || strings.Contains(n.Body, "Заметка наблюдателя") {
			t.Fatal("author must not be notified about an internal note")
		}
	}
	internalToOwner := false
	for _, n := range ownerN.Items {
		internalToOwner = internalToOwner || (n.RequestID == r.ID && strings.Contains(n.Body, "Заметка наблюдателя"))
	}
	if !internalToOwner {
		t.Fatal("observer internal note must notify the owner")
	}
	// the author never sees the observer's internal note
	_, body = e.login("emp1@test.kz").get(p)
	if strings.Contains(string(body), "Заметка наблюдателя") {
		t.Fatal("internal note leaked to author")
	}
	// observer downloads attachments of readable requests? (no attachments here) -> lists
	resp, body = obs.get("/api/v1/requests?scope=all")
	expect(t, resp, body, 200)
	all := decode[requests.ListResult](t, body)
	if !containsID(all.Items, r.ID) {
		t.Fatal("scope=all must list foreign requests for managers")
	}
	for _, it := range all.Items {
		if it.ID == r.ID && it.AccessLevel != requests.AccessObserver {
			t.Fatalf("list access level: %s", it.AccessLevel)
		}
	}
	resp, body = obs.get("/api/v1/requests?scope=all&mine_scope=true")
	expect(t, resp, body, 200)
	if containsID(decode[requests.ListResult](t, body).Items, r.ID) {
		t.Fatal("mine_scope=true must keep owned requests only")
	}
	resp, body = obs.get("/api/v1/requests?scope=inbox")
	expect(t, resp, body, 200)
	if containsID(decode[requests.ListResult](t, body).Items, r.ID) {
		t.Fatal("inbox stays 'requests I own'")
	}
	resp, body = e.login("emp2@test.kz").get("/api/v1/requests?scope=all")
	expect(t, resp, body, 403)
	resp, body = obs.get("/api/v1/requests?scope=all&mine_scope=maybe")
	expect(t, resp, body, 400)
	// the owner still has full powers; the observer's comment did not steal the first response
	resp, body = e.login("mgra@test.kz").json("POST", p+"/status", map[string]any{"status": "in_review"})
	expect(t, resp, body, 200)
	if d := decode[requests.Detail](t, body); d.SLA.FirstRespondedAt == nil {
		t.Fatal("owner status change is the first response")
	}
}

// TestCRSimilarAndAnalyticsCompanyWide (CR6/CR5): similar items and analytics cover the company.
func TestCRSimilarAndAnalyticsCompanyWide(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.fx
	svc := e.app.Requests.Service
	mk := func(author, cat int64, title string) requests.Detail {
		d, err := svc.Create(ctx, e.actor(author), requests.CreateInput{Type: requests.TypeProblem, CategoryID: cat, Title: title,
			Body: "Двойной ввод данных клиента в Битрикс24 и 1С занимает время и даёт ошибки в реквизитах."}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	a := mk(f.emp1, f.catA, "Двойной ввод данных в Битрикс24 и 1С")
	b := mk(f.emp2, f.catB, "Двойной ввод реквизитов в 1С и Битрикс24")
	got, err := svc.Get(ctx, e.actor(f.mgrA), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range got.Similar.Items {
		found = found || it.ID == b.ID
	}
	if !found {
		t.Fatalf("similar items must include other departments' requests: %+v", got.Similar)
	}
	// analytics company-wide for a manager, department filter honoured
	all, err := e.app.Insights.Service.Summary(ctx, e.actor(f.mgrA), insights.SummaryFilter{})
	if err != nil || all.KPIs.Total != 2 {
		t.Fatalf("manager analytics must be company-wide: %v %d", err, all.KPIs.Total)
	}
	onlyB, err := e.app.Insights.Service.Summary(ctx, e.actor(f.mgrA), insights.SummaryFilter{DepartmentID: &f.deptB})
	if err != nil || onlyB.KPIs.Total != 1 {
		t.Fatalf("department filter: %v %d", err, onlyB.KPIs.Total)
	}
	if _, err := e.app.Insights.Service.Summary(ctx, e.actor(f.emp1), insights.SummaryFilter{}); !apperr.Is(err, apperr.KindForbidden) {
		t.Fatalf("employee analytics: %v", err)
	}
}

// TestCRPriorityAndTypeFilters (CR1): priority/request_type filters, sort=priority, validation, feedback type.
func TestCRPriorityAndTypeFilters(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	c := e.login("emp1@test.kz")
	mk := func(typ, prio string) {
		payload := map[string]any{"type": typ, "category_id": f.catA, "title": "Обращение " + typ + " " + prio, "body": "Описание обращения достаточной длины для проверки."}
		if prio != "" {
			payload["priority"] = prio
		}
		resp, body := c.multipart(payload)
		expect(t, resp, body, 201)
		want := prio
		if want == "" {
			want = "medium"
		}
		if d := decode[requests.Detail](t, body); d.Priority != want || d.Type != typ {
			t.Fatalf("created %s/%s: %s/%s", typ, prio, d.Type, d.Priority)
		}
	}
	mk("problem", "low")
	mk("idea", "")
	mk("feedback", "high")
	mk("problem", "high")
	resp, body := c.multipart(map[string]any{"type": "problem", "category_id": f.catA, "title": "Плохой приоритет", "body": "Описание обращения достаточной длины.", "priority": "urgent"})
	expect(t, resp, body, 400)
	if !strings.Contains(string(body), `"priority"`) {
		t.Fatalf("field error: %s", body)
	}
	mgr := e.login("mgra@test.kz")
	list := func(q string) requests.ListResult {
		resp, body := mgr.get("/api/v1/requests?scope=inbox&" + q)
		expect(t, resp, body, 200)
		return decode[requests.ListResult](t, body)
	}
	if l := list("priority=high"); l.Total != 2 {
		t.Fatalf("priority=high: %d", l.Total)
	}
	if l := list("request_type=feedback"); l.Total != 1 || l.Items[0].Type != "feedback" {
		t.Fatalf("request_type=feedback: %+v", l.Items)
	}
	l := list("sort=priority")
	order := []string{}
	for _, it := range l.Items {
		order = append(order, it.Priority)
	}
	if strings.Join(order, ",") != "high,high,medium,low" {
		t.Fatalf("sort=priority: %v", order)
	}
	for _, bad := range []string{"priority=urgent", "request_type=bug", "sort=random"} {
		resp, body := mgr.get("/api/v1/requests?scope=inbox&" + bad)
		expect(t, resp, body, 400)
	}
	// feedback routes and previews like any type; meta advertises features
	resp, body = c.get(fmt.Sprintf("/api/v1/routing/preview?category_id=%d&type=feedback", f.catA))
	expect(t, resp, body, 200)
	resp, body = c.get("/api/v1/meta")
	expect(t, resp, body, 200)
	if !strings.Contains(string(body), `"features":{"telegram":false,"priority":true}`) {
		t.Fatalf("meta features: %s", body)
	}
}

// TestCRAnalyticsKPIs (CR5): on-time share, sample, median resolution; p90 is gone.
func TestCRAnalyticsKPIs(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.fx
	svc := e.app.Requests.Service
	mgr := e.actor(f.mgrA) // catA: 24 h first response
	onTime := e.create(f.emp1, f.catA, false)
	late := e.create(f.emp1, f.catA, false)
	e.create(f.emp1, f.catA, false) // never answered
	e.clk.Advance(2 * time.Hour)
	if _, err := svc.ChangeStatus(ctx, mgr, onTime.ID, requests.StatusInput{Status: "in_review"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(28 * time.Hour) // 30 h after creation: late
	if _, err := svc.ChangeStatus(ctx, mgr, late.ID, requests.StatusInput{Status: "in_review"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(10 * time.Hour) // onTime closed 40 h after creation
	if _, err := svc.ChangeStatus(ctx, mgr, onTime.ID, requests.StatusInput{Status: "closed", ResultText: ptrS("Готово")}); err != nil {
		t.Fatal(err)
	}
	sum, err := e.app.Insights.Service.Summary(ctx, mgr, insights.SummaryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	k := sum.KPIs
	if k.FirstResponseSample != 2 || k.FirstResponseOnTimeShare == nil || *k.FirstResponseOnTimeShare != 0.5 {
		t.Fatalf("on-time: sample=%d share=%v", k.FirstResponseSample, k.FirstResponseOnTimeShare)
	}
	if k.MedianFirstResponseHours == nil || *k.MedianFirstResponseHours != 16 {
		t.Fatalf("median first response (2h, 30h): %v", k.MedianFirstResponseHours)
	}
	if k.MedianResolutionHours == nil || *k.MedianResolutionHours != 40 {
		t.Fatalf("median resolution: %v", k.MedianResolutionHours)
	}
	resp, body := e.login("mgra@test.kz").get("/api/v1/analytics/summary")
	expect(t, resp, body, 200)
	s := string(body)
	if strings.Contains(s, "p90") || !strings.Contains(s, `"first_response_on_time_share":0.5`) || !strings.Contains(s, `"median_resolution_hours":40`) {
		t.Fatalf("summary payload: %s", s)
	}
}
