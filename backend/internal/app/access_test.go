package app

import (
	"context"
	"testing"
	"time"

	"modusvoice/internal/modules/insights"
	"modusvoice/internal/modules/org"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
)

// outcome of a matrix cell: ok, or an error kind.
const ok apperr.Kind = 0

func check(t *testing.T, cell string, err error, want apperr.Kind) {
	t.Helper()
	if want == ok {
		if err != nil {
			t.Errorf("%s: want ok, got %v", cell, err)
		}
		return
	}
	if !apperr.Is(err, want) {
		t.Errorf("%s: want kind %d, got %v", cell, want, err)
	}
}

// TestAccessMatrix covers every cell of ARCHITECTURE §5 for a request routed to department A:
// author (employee), other employee, manager in scope, manager out of scope, admin.
func TestAccessMatrix(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	type who struct {
		name string
		id   int64
	}
	actors := []who{{"author", f.emp1}, {"other-employee", f.emp2}, {"manager-in-scope", f.mgrA}, {"manager-out-of-scope", f.mgrB}, {"admin", f.admin}}

	type cellWant map[string]apperr.Kind
	cases := []struct {
		action string
		run    func(a authz.Actor, reqID int64) error
		want   cellWant
	}{
		{"read", func(a authz.Actor, id int64) error { _, err := svc.Get(ctx, a, id); return err },
			cellWant{"author": ok, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
		{"comment", func(a authz.Actor, id int64) error {
			_, err := svc.AddComment(ctx, a, id, requests.CommentInput{Kind: requests.KindComment, Body: "текст"})
			return err
		}, cellWant{"author": ok, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
		{"internal note", func(a authz.Actor, id int64) error {
			_, err := svc.AddComment(ctx, a, id, requests.CommentInput{Kind: requests.KindInternal, Body: "внутр"})
			return err
		}, cellWant{"author": apperr.KindForbidden, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
		{"question", func(a authz.Actor, id int64) error {
			_, err := svc.AddComment(ctx, a, id, requests.CommentInput{Kind: requests.KindQuestion, Body: "уточните"})
			return err
		}, cellWant{"author": apperr.KindForbidden, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
		{"change status", func(a authz.Actor, id int64) error {
			_, err := svc.ChangeStatus(ctx, a, id, requests.StatusInput{Status: "in_review"})
			return err
		}, cellWant{"author": apperr.KindForbidden, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
		{"assign", func(a authz.Actor, id int64) error {
			_, err := svc.Assign(ctx, a, id, requests.AssignInput{AssigneeSet: true, AssigneeID: &f.admin})
			return err
		}, cellWant{"author": apperr.KindForbidden, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
		{"link theme", func(a authz.Actor, id int64) error {
			_, err := svc.LinkTheme(ctx, a, id, requests.ThemeInput{Title: ptrS("Тема проверки")})
			return err
		}, cellWant{"author": apperr.KindForbidden, "other-employee": apperr.KindNotFound, "manager-in-scope": ok, "manager-out-of-scope": apperr.KindNotFound, "admin": ok}},
	}
	for _, c := range cases {
		for _, w := range actors {
			r := e.create(f.emp1, f.catA, false) // fresh request per cell
			check(t, c.action+"/"+w.name, c.run(e.actor(w.id), r.ID), c.want[w.name])
		}
	}

	// answer: author only, and only in needs_info
	r := e.create(f.emp1, f.catA, false)
	_, err := svc.AddComment(ctx, e.actor(f.emp1), r.ID, requests.CommentInput{Kind: requests.KindAnswer, Body: "ответ"})
	check(t, "answer/author-not-needs-info", err, apperr.KindConflict)
	if _, err := svc.AddComment(ctx, e.actor(f.mgrA), r.ID, requests.CommentInput{Kind: requests.KindQuestion, Body: "?"}); err != nil {
		t.Fatal(err)
	}
	for _, w := range actors {
		_, err := svc.AddComment(ctx, e.actor(w.id), r.ID, requests.CommentInput{Kind: requests.KindAnswer, Body: "ответ"})
		want := map[string]apperr.Kind{"author": ok, "other-employee": apperr.KindNotFound, "manager-in-scope": apperr.KindForbidden,
			"manager-out-of-scope": apperr.KindNotFound, "admin": apperr.KindForbidden}[w.name]
		check(t, "answer/"+w.name, err, want)
		if err == nil {
			break // status went back to in_review
		}
	}

	// list scopes
	target := e.create(f.emp1, f.catA, false)
	for _, w := range actors {
		a := e.actor(w.id)
		res, err := svc.List(ctx, a, requests.ListQuery{Scope: "inbox", PageSize: 100})
		switch w.name {
		case "author", "other-employee":
			check(t, "inbox/"+w.name, err, apperr.KindForbidden)
		default:
			check(t, "inbox/"+w.name, err, ok)
			if found := containsID(res.Items, target.ID); found != (w.name != "manager-out-of-scope") {
				t.Errorf("inbox/%s: contains=%v", w.name, found)
			}
		}
		mine, err := svc.List(ctx, a, requests.ListQuery{Scope: "mine"})
		check(t, "mine/"+w.name, err, ok)
		if found := containsID(mine.Items, target.ID); found != (w.name == "author") {
			t.Errorf("mine/%s: contains=%v", w.name, found)
		}
		_, err = svc.List(ctx, a, requests.ListQuery{Scope: "all"})
		if a.IsStaff() {
			check(t, "all/"+w.name, err, ok)
		} else {
			check(t, "all/"+w.name, err, apperr.KindForbidden)
		}
	}

	// analytics: employees forbidden, manager scoped, admin all
	for _, w := range actors {
		sum, err := e.app.Insights.Service.Summary(ctx, e.actor(w.id), insights.SummaryFilter{})
		_, gerr := e.app.Insights.Service.SuggestedGroups(ctx, e.actor(w.id))
		_, terr := e.app.Insights.Service.Themes(ctx, e.actor(w.id))
		if !e.actor(w.id).IsStaff() {
			check(t, "analytics/"+w.name, err, apperr.KindForbidden)
			check(t, "groups/"+w.name, gerr, apperr.KindForbidden)
			check(t, "themes/"+w.name, terr, apperr.KindForbidden)
			continue
		}
		check(t, "analytics/"+w.name, err, ok)
		switch w.name {
		case "manager-out-of-scope":
			if sum.KPIs.Total != 0 {
				t.Errorf("analytics: out-of-scope manager sees %d requests", sum.KPIs.Total)
			}
		default:
			if sum.KPIs.Total == 0 {
				t.Errorf("analytics/%s: expected requests in scope", w.name)
			}
		}
	}

	// admin configuration: only admin
	for _, w := range actors {
		a := e.actor(w.id)
		_, cerr := e.app.Org.Service.AdminCategories(ctx, a)
		_, rerr := e.app.Org.Service.AdminRules(ctx, a)
		_, uerr := e.app.Identity.Service.ListUsers(ctx, a)
		_, derr := e.app.Org.Service.AdminDepartments(ctx, a)
		want := apperr.KindForbidden
		if a.IsAdmin() {
			want = ok
		}
		for name, err := range map[string]error{"categories": cerr, "rules": rerr, "users": uerr, "departments": derr} {
			check(t, "admin-"+name+"/"+w.name, err, want)
		}
	}
}

func TestManagerScopeIncludesAssignedRequests(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	r := e.create(e.fx.emp1, e.fx.catA, false)
	if _, err := e.app.Requests.Service.Get(ctx, e.actor(e.fx.mgrNone), r.ID); !apperr.Is(err, apperr.KindNotFound) {
		t.Fatalf("not yet assigned: %v", err)
	}
	if _, err := e.app.Requests.Service.Assign(ctx, e.actor(e.fx.mgrA), r.ID, requests.AssignInput{AssigneeSet: true, AssigneeID: &e.fx.mgrNone}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Requests.Service.ChangeStatus(ctx, e.actor(e.fx.mgrNone), r.ID, requests.StatusInput{Status: "in_review"}); err != nil {
		t.Fatalf("assigned manager must manage: %v", err)
	}
}

func TestRoutingAndAdminRuleChange(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.fx
	// rule with assignee
	r := e.create(f.emp1, f.catA, false)
	if r.RoutingSource != "rule" || r.Assignee == nil || r.Assignee.ID != f.mgrA {
		t.Fatalf("catA routing: %+v %+v", r.RoutingSource, r.Assignee)
	}
	// rule without assignee -> department head
	r = e.create(f.emp1, f.catB, false)
	if r.Assignee == nil || r.Assignee.ID != f.mgrB || r.Department == nil || r.Department.ID != f.deptB {
		t.Fatalf("catB head fallback: %+v", r.Assignee)
	}
	// head inactive -> unrouted (admin queue)
	e.exec(`UPDATE users SET active = 0 WHERE id = ?`, f.mgrB)
	r = e.create(f.emp1, f.catB, false)
	if r.RoutingSource != "unrouted" || r.Assignee != nil {
		t.Fatalf("inactive head must give unrouted: %s %+v", r.RoutingSource, r.Assignee)
	}
	e.exec(`UPDATE users SET active = 1 WHERE id = ?`, f.mgrB)
	// no rule -> unrouted, admins notified, deadline still set (AC-06)
	r = e.create(f.emp1, f.catOther, false)
	if r.RoutingSource != "unrouted" || r.Department != nil || r.SLA.FirstResponseDue.IsZero() {
		t.Fatalf("catOther: %+v", r)
	}
	n, _ := e.app.Notifications.Service.List(ctx, e.actor(f.admin), true, 50)
	if !notifiedAbout(n.Items, r.ID) {
		t.Fatal("admin must be notified about unrouted request")
	}
	if _, err := e.app.Requests.Service.Get(ctx, e.actor(f.mgrA), r.ID); err == nil {
		t.Fatal("managers must not see the unrouted request")
	}
	// AC-10: admin adds a rule, the next request follows it
	admin := e.actor(f.admin)
	if _, err := e.app.Org.Service.CreateRule(ctx, admin, org.RuleInput{CategoryID: &f.catOther, DepartmentID: &f.deptA}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Org.Service.CreateRule(ctx, admin, org.RuleInput{CategoryID: &f.catOther, DepartmentID: &f.deptB}); !apperr.Is(err, apperr.KindConflict) {
		t.Fatalf("duplicate active rule must be 409, got %v", err)
	}
	r = e.create(f.emp1, f.catOther, false)
	if r.RoutingSource != "rule" || r.Department == nil || r.Department.ID != f.deptA || r.Assignee == nil || r.Assignee.ID != f.mgrA {
		t.Fatalf("new rule not applied: %+v %+v", r.Department, r.Assignee)
	}
	// SLA from rule (AC-02): deadline = created + 48h default for new rule
	if got := r.SLA.FirstResponseDue.Sub(r.CreatedAt).Hours(); got != 48 {
		t.Fatalf("first response window %vh", got)
	}
}

func TestSLAPauseAndOverdueTicker(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	r := e.create(f.emp1, f.catA, false) // 24h / 120h
	e.clk.Advance(2 * time.Hour)
	if _, err := svc.AddComment(ctx, e.actor(f.mgrA), r.ID, requests.CommentInput{Kind: requests.KindQuestion, Body: "Уточните?"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(10 * time.Hour)
	d, err := svc.AddComment(ctx, e.actor(f.emp1), r.ID, requests.CommentInput{Kind: requests.KindAnswer, Body: "Ответ"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != requests.StatusInReview {
		t.Fatalf("answer must return to in_review, got %s", d.Status)
	}
	if got := d.SLA.ResolutionDue.Sub(r.SLA.ResolutionDue).Hours(); got != 10 {
		t.Fatalf("resolution due shifted by %vh, want 10", got)
	}
	if d.SLA.FirstRespondedAt == nil || d.SLA.FirstRespondedAt.Sub(r.CreatedAt).Hours() != 2 {
		t.Fatalf("first response = question time: %v", d.SLA.FirstRespondedAt)
	}

	// overdue: a request without response past 24h notifies owner once, admins after +24h
	o := e.create(f.emp2, f.catA, false)
	e.clk.Advance(25 * time.Hour)
	if n, err := svc.CheckOverdue(ctx); err != nil || n != 1 {
		t.Fatalf("first check: n=%d err=%v", n, err)
	}
	if n, _ := svc.CheckOverdue(ctx); n != 0 {
		t.Fatalf("ticker must be idempotent, emitted %d", n)
	}
	mgr, _ := e.app.Notifications.Service.List(ctx, e.actor(f.mgrA), true, 100)
	if !hasKind(mgr.Items, o.ID, "overdue") {
		t.Fatal("owner not notified about overdue")
	}
	adm, _ := e.app.Notifications.Service.List(ctx, e.actor(f.admin), true, 100)
	if hasKind(adm.Items, o.ID, "overdue_escalated") {
		t.Fatal("admins must not be notified before +24h")
	}
	e.clk.Advance(24 * time.Hour)
	if n, _ := svc.CheckOverdue(ctx); n < 1 {
		t.Fatal("escalation expected")
	}
	adm, _ = e.app.Notifications.Service.List(ctx, e.actor(f.admin), true, 100)
	if !hasKind(adm.Items, o.ID, "overdue_escalated") {
		t.Fatal("admins not notified after +24h")
	}
	// AC-07: overdue requests come first in the inbox
	list, err := svc.List(ctx, e.actor(f.mgrA), requests.ListQuery{Scope: "inbox"})
	if err != nil || len(list.Items) == 0 || list.Items[0].SLA.State != "overdue" {
		t.Fatalf("overdue must be listed first: %+v", list.Items)
	}
	if list.Counts.Overdue == 0 {
		t.Fatal("overdue counter")
	}
}

func containsID(items []requests.Summary, id int64) bool {
	for _, i := range items {
		if i.ID == id {
			return true
		}
	}
	return false
}

func ptrS(s string) *string { return &s }
