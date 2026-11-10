package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
)

// moveTo walks a fresh request to the wanted status through legal transitions as the routed manager.
func (e *testEnv) moveTo(id int64, path ...requests.Status) {
	e.t.Helper()
	mgr := e.actor(e.fx.mgrA)
	for _, st := range path {
		in := requests.StatusInput{Status: string(st)}
		if st.Terminal() {
			res := "Результат рассмотрения зафиксирован."
			in.ResultText = &res
		}
		if _, err := e.app.Requests.Service.ChangeStatus(context.Background(), mgr, id, in); err != nil {
			e.t.Fatalf("move %d -> %s: %v", id, st, err)
		}
	}
}

func TestLifecycleTransitionsService(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	mgr := e.actor(e.fx.mgrA)
	type step struct {
		to   requests.Status
		want apperr.Kind
	}
	cases := []struct {
		name  string
		setup []requests.Status
		steps []step
	}{
		{"new straight to decision", nil, []step{{requests.StatusDecision, ok}}},
		{"decision cannot skip to implemented", []requests.Status{requests.StatusDecision}, []step{{requests.StatusImplemented, apperr.KindConflict}}},
		{"decision returns to in_review", []requests.Status{requests.StatusDecision}, []step{{requests.StatusInReview, ok}, {requests.StatusDecision, ok}}},
		{"in_progress back to decision", []requests.Status{requests.StatusDecision, requests.StatusInProgress}, []step{{requests.StatusDecision, ok}, {requests.StatusInProgress, ok}}},
		{"in_progress cannot go to in_review", []requests.Status{requests.StatusDecision, requests.StatusInProgress}, []step{{requests.StatusInReview, apperr.KindConflict}}},
		{"new cannot jump to in_progress", nil, []step{{requests.StatusInProgress, apperr.KindConflict}}},
		{"in_review cannot jump to in_progress", []requests.Status{requests.StatusInReview}, []step{{requests.StatusInProgress, apperr.KindConflict}}},
		{"decision can be declined", []requests.Status{requests.StatusDecision}, []step{{requests.StatusDeclined, ok}}},
		{"closed from open status", []requests.Status{requests.StatusDecision}, []step{{requests.StatusClosed, ok}, {requests.StatusInReview, apperr.KindConflict}, {requests.StatusDecision, apperr.KindConflict}}},
		{"closed from new", nil, []step{{requests.StatusClosed, ok}}},
		{"closed from in_progress", []requests.Status{requests.StatusDecision, requests.StatusInProgress}, []step{{requests.StatusClosed, ok}}},
		{"closed from in_review", []requests.Status{requests.StatusInReview}, []step{{requests.StatusClosed, ok}}},
		{"in_progress implemented", []requests.Status{requests.StatusDecision, requests.StatusInProgress}, []step{{requests.StatusImplemented, ok}, {requests.StatusInProgress, apperr.KindConflict}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := e.create(e.fx.emp1, e.fx.catA, false)
			e.moveTo(d.ID, tc.setup...)
			for _, s := range tc.steps {
				in := requests.StatusInput{Status: string(s.to)}
				if s.to.Terminal() {
					res := "Итог"
					in.ResultText = &res
				}
				_, err := svc.ChangeStatus(ctx, mgr, d.ID, in)
				check(t, fmt.Sprintf("-> %s", s.to), err, s.want)
			}
		})
	}
	t.Run("terminal needs result_text", func(t *testing.T) {
		d := e.create(e.fx.emp1, e.fx.catA, false)
		e.moveTo(d.ID, requests.StatusDecision, requests.StatusInProgress)
		_, err := svc.ChangeStatus(ctx, mgr, d.ID, requests.StatusInput{Status: "implemented"})
		check(t, "implemented without result", err, apperr.KindValidation)
		_, err = svc.ChangeStatus(ctx, mgr, d.ID, requests.StatusInput{Status: "declined"})
		check(t, "declined without result", err, apperr.KindValidation)
		_, err = svc.ChangeStatus(ctx, mgr, d.ID, requests.StatusInput{Status: "closed"})
		check(t, "closed without result", err, apperr.KindValidation)
	})
	t.Run("needs_info only returns via author answer", func(t *testing.T) {
		d := e.create(e.fx.emp1, e.fx.catA, false)
		if _, err := svc.AddComment(ctx, mgr, d.ID, requests.CommentInput{Kind: requests.KindQuestion, Body: "Уточните"}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.ChangeStatus(ctx, mgr, d.ID, requests.StatusInput{Status: "decision"})
		check(t, "needs_info -> decision", err, apperr.KindConflict)
		got, err := svc.AddComment(ctx, e.actor(e.fx.emp1), d.ID, requests.CommentInput{Kind: requests.KindAnswer, Body: "Ответ"})
		if err != nil || got.Status != requests.StatusInReview {
			t.Fatalf("answer: %v %s", err, got.Status)
		}
	})
}

func TestReportPermissionsAndStatuses(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.app.Requests.Service
	f := e.fx
	actors := []struct {
		name string
		id   int64
		want apperr.Kind // in an allowed status
	}{
		{"author", f.emp1, apperr.KindForbidden},
		{"other-employee", f.emp2, apperr.KindNotFound},
		{"manager-out-of-scope", f.mgrB, apperr.KindNotFound},
		{"manager-in-scope", f.mgrA, ok},
		{"admin", f.admin, ok},
	}
	statuses := []struct {
		name    string
		path    []requests.Status
		allowed bool
	}{
		{"new", nil, false},
		{"in_review", []requests.Status{requests.StatusInReview}, true},
		{"decision", []requests.Status{requests.StatusDecision}, true},
		{"in_progress", []requests.Status{requests.StatusDecision, requests.StatusInProgress}, true},
		{"implemented", []requests.Status{requests.StatusDecision, requests.StatusInProgress, requests.StatusImplemented}, false},
		{"declined", []requests.Status{requests.StatusDeclined}, false},
		{"closed", []requests.Status{requests.StatusClosed}, false},
	}
	for _, st := range statuses {
		for _, a := range actors {
			t.Run(st.name+"/"+a.name, func(t *testing.T) {
				d := e.create(f.emp1, f.catA, false)
				e.moveTo(d.ID, st.path...)
				want := a.want
				if want == ok && !st.allowed {
					want = apperr.KindConflict
				}
				if a.want == apperr.KindForbidden && st.allowed == false {
					want = apperr.KindForbidden // permission is checked before status
				}
				_, err := svc.AddComment(ctx, e.actor(a.id), d.ID, requests.CommentInput{Kind: requests.KindReport, Body: "Подготовлен черновик чек-листа"})
				check(t, "report", err, want)
			})
		}
		t.Run(st.name+"/allowed_comment_kinds", func(t *testing.T) {
			d := e.create(f.emp1, f.catA, false)
			e.moveTo(d.ID, st.path...)
			for _, a := range actors {
				if a.id == f.emp2 || a.id == f.mgrB {
					continue
				}
				got, err := svc.Get(ctx, e.actor(a.id), d.ID)
				if err != nil {
					t.Fatal(err)
				}
				has := false
				for _, k := range got.AllowedCommentKinds {
					has = has || k == requests.KindReport
				}
				if wantHas := st.allowed && a.want == ok; has != wantHas {
					t.Errorf("%s in %s: report in allowed_comment_kinds=%v want %v (%v)", a.name, st.name, has, wantHas, got.AllowedCommentKinds)
				}
			}
		})
	}
	// manager out of scope stays 404 even over HTTP and the author gets 403 with the standard envelope
	d := e.create(f.emp1, f.catA, false)
	e.moveTo(d.ID, requests.StatusInReview)
	path := fmt.Sprintf("/api/v1/requests/%d/comments", d.ID)
	body := map[string]any{"kind": "report", "body": "Отчёт"}
	resp, b := e.anon().json("POST", path, body)
	expect(t, resp, b, 401)
	resp, b = e.login("emp1@test.kz").json("POST", path, body)
	expect(t, resp, b, 403)
	resp, b = e.login("mgrb@test.kz").json("POST", path, body)
	expect(t, resp, b, 404)
	resp, b = e.login("mgra@test.kz").json("POST", path, body)
	expect(t, resp, b, 201)
	// conflict carries a Russian message and no internals
	e.moveTo(d.ID, requests.StatusDeclined)
	resp, b = e.login("mgra@test.kz").json("POST", path, body)
	expect(t, resp, b, 409)
	if !strings.Contains(string(b), "Отчёт о ходе") {
		t.Fatalf("409 message: %s", b)
	}
}

func TestReportSideEffectsAnonymousAuthor(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.fx
	d := e.create(f.emp1, f.catA, true)
	// move without a staff comment first: the report itself must be the first response
	svc := e.app.Requests.Service
	mgr := e.actor(f.mgrA)
	if _, err := svc.ChangeStatus(ctx, e.actor(f.admin), d.ID, requests.StatusInput{Status: "in_review"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.DB.Exec(`UPDATE requests SET first_responded_at = NULL WHERE id = ?`, d.ID); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(time.Hour)
	got, err := svc.AddComment(ctx, mgr, d.ID, requests.CommentInput{Kind: requests.KindReport, Body: "Собрали требования, начинаем настройку."})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != requests.StatusInReview {
		t.Fatalf("report must not change status: %s", got.Status)
	}
	if got.SLA.FirstRespondedAt == nil {
		t.Fatal("report counts as first response")
	}
	last := got.Timeline[len(got.Timeline)-1]
	if last.Kind != "report" || last.Text == nil || !strings.Contains(*last.Text, "Собрали") {
		t.Fatalf("timeline: %+v", last)
	}

	author := e.login("emp1@test.kz")
	resp, body := author.get("/api/v1/notifications")
	expect(t, resp, body, 200)
	list := decode[notifications.List](t, body)
	found := false
	for _, n := range list.Items {
		if n.Kind == "report" {
			found = true
			if n.RequestNumber != d.Number || !strings.Contains(n.Body, "Собрали") {
				t.Fatalf("report notification: %+v", n)
			}
		}
	}
	if !found {
		t.Fatalf("author (anonymous) must be notified about a report: %s", body)
	}
	// the author sees the report in their own card
	resp, body = author.get(fmt.Sprintf("/api/v1/requests/%d", d.ID))
	expect(t, resp, body, 200)
	if !strings.Contains(string(body), `"kind":"report"`) {
		t.Fatalf("author card lacks the report: %s", body)
	}
	// nobody else learns the author's identity from the report
	for _, who := range []string{"mgra@test.kz", "admin@test.kz", "emp2@test.kz"} {
		c := e.login(who)
		for _, p := range []string{"/api/v1/notifications", fmt.Sprintf("/api/v1/requests/%d", d.ID)} {
			_, b := c.get(p)
			for _, secret := range []string{"Сотрудник Первый", "emp1@test.kz"} {
				if strings.Contains(string(b), secret) {
					t.Errorf("%s %s leaks %q", who, p, secret)
				}
			}
			assertNoAuthorID(t, who+" "+p, b, f.emp1)
		}
	}
	// the manager's own inbox gets no notification about their own report
	_, body = e.login("mgra@test.kz").get("/api/v1/notifications")
	for _, n := range decode[notifications.List](t, body).Items {
		if n.Kind == "report" {
			t.Fatalf("report notification must go to the author only: %+v", n)
		}
	}
}

func TestInboxTabsAndCounts(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	ctx := context.Background()
	svc := e.app.Requests.Service
	mk := func(path ...requests.Status) int64 {
		d := e.create(f.emp1, f.catA, false)
		e.moveTo(d.ID, path...)
		return d.ID
	}
	idNew := mk()
	idReview := mk(requests.StatusInReview)
	idDecision := mk(requests.StatusDecision)
	idProgress := mk(requests.StatusDecision, requests.StatusInProgress)
	idWaiting := mk()
	if _, err := svc.AddComment(ctx, e.actor(f.mgrA), idWaiting, requests.CommentInput{Kind: requests.KindQuestion, Body: "Уточните"}); err != nil {
		t.Fatal(err)
	}
	idDone := mk(requests.StatusDecision, requests.StatusInProgress, requests.StatusImplemented)
	idClosed := mk(requests.StatusClosed)

	admin := e.login("admin@test.kz")
	list := func(tab string) requests.ListResult {
		t.Helper()
		resp, body := admin.get("/api/v1/requests?scope=all&tab=" + tab)
		expect(t, resp, body, 200)
		return decode[requests.ListResult](t, body)
	}
	ids := func(r requests.ListResult) map[int64]bool {
		m := map[int64]bool{}
		for _, it := range r.Items {
			m[it.ID] = true
		}
		return m
	}
	same := func(name string, got map[int64]bool, want ...int64) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %v want %v", name, got, want)
		}
		for _, w := range want {
			if !got[w] {
				t.Fatalf("%s: got %v want %v", name, got, want)
			}
		}
	}
	check := func(label string, needs, progress, waiting, closed []int64) {
		t.Helper()
		r := list("needs_action")
		same(label+" needs_action", ids(r), needs...)
		c := r.Counts
		if c.NeedsAction != len(needs) || c.InProgress != len(progress) || c.WaitingAuthor != len(waiting) || c.Closed != len(closed) || c.All != 7 {
			t.Fatalf("%s counts: %+v", label, c)
		}
		same(label+" in_progress", ids(list("in_progress")), progress...)
		same(label+" waiting_author", ids(list("waiting_author")), waiting...)
		same(label+" closed", ids(list("closed")), closed...)
	}
	// fresh: new and decision need action; the in_progress tab covers in_review|decision|in_progress
	check("fresh", []int64{idNew, idDecision}, []int64{idReview, idDecision, idProgress}, []int64{idWaiting}, []int64{idDone, idClosed})

	// past the 120h resolution deadline every open non-paused request needs action; needs_info stays paused
	e.clk.Advance(130 * time.Hour)
	check("overdue", []int64{idNew, idReview, idDecision, idProgress}, []int64{idReview, idDecision, idProgress}, []int64{idWaiting}, []int64{idDone, idClosed})
}
