package app

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/requests"
)

// TestCRAnonymousCreateLeavesNoLinkInDB (CR2): after an anonymous request is created over HTTP
// (with an attachment), questioned by staff and answered by token, NO table other than the
// identity tables holds the author's id, name or e-mail, and no log line mentions them or the token.
func TestCRAnonymousCreateLeavesNoLinkInDB(t *testing.T) {
	e, logs := newEnvWithLogs(t, nil)
	f := e.fx
	author := e.login("emp1@test.kz")
	resp, body := author.multipart(map[string]any{"type": "problem", "category_id": f.catA, "title": "Анонимная проблема",
		"body": "Описание анонимной проблемы достаточной длины.", "is_anonymous": true, "priority": "high"}, file{"scan.pdf", []byte("%PDF-1.4 x")})
	expect(t, resp, body, 201)
	d := decode[requests.Detail](t, body)
	if d.AnonymousToken == nil || len(*d.AnonymousToken) != 34 || d.Priority != "high" {
		t.Fatalf("token/priority in 201: %v %s", d.AnonymousToken, d.Priority)
	}
	token := *d.AnonymousToken
	// the token is returned ONLY in the 201 response
	_, again := e.login("mgra@test.kz").get(fmt.Sprintf("/api/v1/requests/%d", d.ID))
	if strings.Contains(string(again), "anonymous_token") || strings.Contains(string(again), token) {
		t.Fatal("token must not appear in later responses")
	}
	mgr := e.login("mgra@test.kz")
	resp, body = mgr.json("POST", fmt.Sprintf("/api/v1/requests/%d/comments", d.ID), map[string]any{"kind": "question", "body": "Уточните участок"})
	expect(t, resp, body, 201)
	resp, body = e.anon().json("POST", "/api/v1/anonymous/comments", map[string]any{"token": token, "body": "Участок монтажа"})
	expect(t, resp, body, 201)

	// direct column checks
	if n := countRows(t, e, `SELECT COUNT(*) FROM requests WHERE id = ? AND author_id IS NULL AND is_anonymous = 1 AND length(anon_token_hash) = 64`, d.ID); n != 1 {
		t.Fatal("requests row must have no author and a token hash")
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM requests WHERE anon_token_hash LIKE ?`, "%"+token+"%"); n != 0 {
		t.Fatal("raw token stored")
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM comments WHERE request_id = ? AND by_author = 1 AND author_id IS NULL AND kind = 'answer'`, d.ID); n != 1 {
		t.Fatal("author answer must be stored with by_author=1 and no author_id")
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM request_events WHERE request_id = ? AND by_author = 1 AND actor_id IS NULL`, d.ID); n < 2 {
		t.Fatalf("author-side events (created, answer status change) must have no actor: %d", n)
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM attachments WHERE request_id = ? AND uploaded_by IS NULL`, d.ID); n != 1 {
		t.Fatal("attachment must have no uploader")
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM notifications WHERE user_id = ?`, f.emp1); n != 0 {
		t.Fatal("anonymous author must get no notification rows")
	}
	assertNoUserLinkInDB(t, e, f.emp1, "Сотрудник Первый", "emp1@test.kz")

	// logs: the access log line for POST /requests exists but carries no user identity or token
	out := logs.String()
	if !strings.Contains(out, "path=/api/v1/requests") {
		t.Fatalf("access log line expected: %s", out)
	}
	mustNotContain(t, "logs", out, "emp1@test.kz", "Сотрудник Первый", token, strings.ReplaceAll(token, "-", ""))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "method=POST path=/api/v1/requests ") &&
			(strings.Contains(line, "user") || strings.Contains(line, "actor") || strings.Contains(line, "author")) {
			t.Errorf("POST /requests access log carries a user reference: %s", line)
		}
	}
	// staff were notified about the anonymous answer, with the anonymous label
	n, _ := e.app.Notifications.Service.List(t.Context(), e.actor(f.mgrA), false, 50)
	found := false
	for _, it := range n.Items {
		if it.RequestID == d.ID && it.Kind == notifications.KindAnswer {
			found = strings.Contains(it.Title, requests.AnonymousLabel)
		}
	}
	if !found {
		t.Fatalf("owner must be notified about the anonymous answer: %+v", n.Items)
	}
}

// TestCRAnonymousTokenFlow covers lookup, comment/answer kinds, closed 409, attachments,
// wrong tokens (identical 404) and scope=mine.
func TestCRAnonymousTokenFlow(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	author := e.login("emp1@test.kz")
	resp, body := author.multipart(map[string]any{"type": "idea", "category_id": f.catA, "title": "Анонимная идея",
		"body": "Описание анонимной идеи достаточной длины.", "is_anonymous": true}, file{"note.txt", []byte("заметка")})
	expect(t, resp, body, 201)
	d := decode[requests.Detail](t, body)
	token := *d.AnonymousToken
	other := e.create(f.emp2, f.catA, false)
	_ = other
	c := e.anon() // no session at all: "another device"

	// lookup (token is case/separator insensitive)
	resp, body = c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": strings.ToLower(strings.ReplaceAll(token, "-", " "))})
	expect(t, resp, body, 200)
	v := decode[requests.Detail](t, body)
	if v.ID != d.ID || !v.Author.IsMe || !v.Author.Anonymous || v.AccessLevel != requests.AccessAuthor || v.AnonymousToken != nil {
		t.Fatalf("lookup view: %+v", v.Summary)
	}
	if len(v.AllowedCommentKinds) != 1 || v.AllowedCommentKinds[0] != "comment" || len(v.AllowedStatuses) != 0 || v.CanAssign {
		t.Fatalf("token holder actions: %v %v %v", v.AllowedCommentKinds, v.AllowedStatuses, v.CanAssign)
	}
	// wrong / malformed / missing token: identical 404 bodies
	var bodies []string
	for _, bad := range []string{"0000-0000-0000-0000-0000-0000-0000", "not-a-token", "", token + "0"} {
		resp, body = c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": bad})
		expect(t, resp, body, 404)
		bodies = append(bodies, string(body))
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Fatalf("404 bodies differ: %q vs %q", b, bodies[0])
		}
	}
	// plain comment while not needs_info
	resp, body = c.json("POST", "/api/v1/anonymous/comments", map[string]any{"token": token, "body": "Дополнение"})
	expect(t, resp, body, 201)
	if v := decode[requests.Detail](t, body); v.Status != requests.StatusNew || v.Timeline[len(v.Timeline)-1].Kind != "comment" {
		t.Fatalf("comment kind: %s %s", v.Status, v.Timeline[len(v.Timeline)-1].Kind)
	}
	resp, body = c.json("POST", "/api/v1/anonymous/comments", map[string]any{"token": token, "body": " "})
	expect(t, resp, body, 400)
	// staff question + internal note -> token holder answers (answer kind, back to in_review), never sees internal
	mgr := e.login("mgra@test.kz")
	p := fmt.Sprintf("/api/v1/requests/%d", d.ID)
	resp, body = mgr.json("POST", p+"/comments", map[string]any{"kind": "internal", "body": "Секретная заметка"})
	expect(t, resp, body, 201)
	resp, body = mgr.json("POST", p+"/comments", map[string]any{"kind": "question", "body": "Уточните"})
	expect(t, resp, body, 201)
	resp, body = c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": token})
	expect(t, resp, body, 200)
	v = decode[requests.Detail](t, body)
	if strings.Contains(string(body), "Секретная заметка") || len(v.AllowedCommentKinds) != 1 || v.AllowedCommentKinds[0] != "answer" {
		t.Fatalf("needs_info view: %v", v.AllowedCommentKinds)
	}
	resp, body = c.json("POST", "/api/v1/anonymous/comments", map[string]any{"token": token, "body": "Ответ на уточнение"})
	expect(t, resp, body, 201)
	if v := decode[requests.Detail](t, body); v.Status != requests.StatusInReview {
		t.Fatalf("answer must move to in_review: %s", v.Status)
	}
	// attachments: right token ok, wrong token / foreign attachment 404, session-less
	attPath := fmt.Sprintf("/api/v1/anonymous/attachments/%d", d.Attachments[0].ID)
	get := func(path, tok string) (*http.Response, []byte) {
		req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
		if tok != "" {
			req.Header.Set("X-Anonymous-Token", tok)
		}
		return c.do(req)
	}
	resp, body = get(attPath, token)
	expect(t, resp, body, 200)
	if string(body) != "заметка" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;") {
		t.Fatalf("download: %q %v", body, resp.Header)
	}
	resp, body = get(attPath, "0000-0000-0000-0000-0000-0000-0000")
	expect(t, resp, body, 404)
	resp, body = get(attPath, "")
	expect(t, resp, body, 404)
	resp, body = get("/api/v1/anonymous/attachments/999999", token)
	expect(t, resp, body, 404)
	// the creator's session: not in scope=mine, card 404, regular attachment route 404
	resp, body = author.get("/api/v1/requests?scope=mine")
	expect(t, resp, body, 200)
	if decode[requests.ListResult](t, body).Total != 0 {
		t.Fatalf("scope=mine must never list anonymous requests: %s", body)
	}
	resp, body = author.get(p)
	expect(t, resp, body, 404)
	// closed -> 409
	if _, err := e.app.Requests.Service.ChangeStatus(t.Context(), e.actor(f.mgrA), d.ID, requests.StatusInput{Status: "closed", ResultText: ptrS("Решено")}); err != nil {
		t.Fatal(err)
	}
	resp, body = c.json("POST", "/api/v1/anonymous/comments", map[string]any{"token": token, "body": "Ещё вопрос"})
	expect(t, resp, body, 409)
	resp, body = c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": token})
	expect(t, resp, body, 200)
	if v := decode[requests.Detail](t, body); len(v.AllowedCommentKinds) != 0 || v.ResultText == nil {
		t.Fatalf("closed view: %v %v", v.AllowedCommentKinds, v.ResultText)
	}
}

// TestCRAnonymousThrottle: the token endpoints are limited per client IP (10/min by default).
func TestCRAnonymousThrottle(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for i := 0; i < requests.DefaultAnonPerMinute; i++ {
		resp, body := c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": "0000-0000-0000-0000-0000-0000-0000"})
		expect(t, resp, body, 404)
	}
	resp, body := c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": "0000-0000-0000-0000-0000-0000-0000"})
	expect(t, resp, body, 429)
	if !strings.Contains(string(body), "too_many_requests") {
		t.Fatalf("429 envelope: %s", body)
	}
	e.clk.Advance(61 * time.Second)
	resp, body = c.json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": "0000-0000-0000-0000-0000-0000-0000"})
	expect(t, resp, body, 404)
}
