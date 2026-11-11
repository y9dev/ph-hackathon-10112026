package app

import (
	"fmt"
	"net/http"
	"testing"

	"modusvoice/internal/modules/requests"
)

// TestAnonymousSpamBlockAndBrowserStatus: a staff owner hides an anonymous request as spam. The request
// disappears from every view, its token stops working, and the browser that holds the token is told
// (status check, blocked cookie) that anonymous writing is no longer allowed there.
func TestAnonymousSpamBlockAndBrowserStatus(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	author := e.login("emp1@test.kz")
	resp, body := author.multipart(map[string]any{"type": "problem", "category_id": f.catA, "title": "Спам-проба",
		"body": "Описание анонимной проблемы достаточной длины.", "is_anonymous": true}, file{"note.txt", []byte("заметка")})
	expect(t, resp, body, 201)
	d := decode[requests.Detail](t, body)
	token := *d.AnonymousToken

	// the browser holding the token is allowed before any moderation
	resp, body = author.json("POST", "/api/v1/anonymous/status", nil)
	expect(t, resp, body, 200)
	if !decodeAllowed(t, body) {
		t.Fatal("status must allow a clean browser")
	}

	// employees cannot moderate; a non-anonymous request cannot be marked as spam
	resp, body = author.json("POST", fmt.Sprintf("/api/v1/requests/%d/spam", d.ID), nil)
	expect(t, resp, body, 403) // employees cannot moderate
	admin := e.login("admin@test.kz")
	resp, body = admin.json("POST", fmt.Sprintf("/api/v1/requests/%d/spam", d.ID), nil)
	expect(t, resp, body, http.StatusNoContent)

	// hidden everywhere: staff detail, anonymous lookup, and the database row stays but is excluded
	resp, body = admin.get(fmt.Sprintf("/api/v1/requests/%d", d.ID))
	expect(t, resp, body, 404)
	resp, body = e.anon().json("POST", "/api/v1/anonymous/lookup", map[string]string{"token": token})
	expect(t, resp, body, 404)
	if n := countRows(t, e, `SELECT COUNT(*) FROM anon_spam_blocks`); n != 1 {
		t.Fatalf("exactly one blocked token hash expected, got %d", n)
	}
	if n := countRows(t, e, `SELECT COUNT(*) FROM anon_spam_blocks WHERE token_hash LIKE ?`, "%"+token+"%"); n != 0 {
		t.Fatal("raw token must never be stored")
	}

	// the same browser: status says blocked, the blocked cookie is set, anonymous submit is refused
	resp, body = author.json("POST", "/api/v1/anonymous/status", nil)
	expect(t, resp, body, 200)
	if decodeAllowed(t, body) {
		t.Fatal("status must refuse a browser with a blocked token")
	}
	resp, body = author.multipart(map[string]any{"type": "idea", "category_id": f.catA, "title": "Ещё одна анонимная",
		"body": "Описание ещё одной анонимной идеи достаточной длины.", "is_anonymous": true})
	expect(t, resp, body, 400) // validation error on is_anonymous

	// a browser without the token is still allowed (blocking is per token, not per IP or account)
	resp, body = e.anon().json("POST", "/api/v1/anonymous/status", nil)
	expect(t, resp, body, 200)
	if !decodeAllowed(t, body) {
		t.Fatal("clean browser must stay allowed")
	}
}

// TestSpamMarkRejectsNonAnonymous: only anonymous requests can be marked as spam.
func TestSpamMarkRejectsNonAnonymous(t *testing.T) {
	e := newEnv(t)
	f := e.fx
	d := e.create(f.emp1, f.catA, false)
	admin := e.login("admin@test.kz")
	resp, body := admin.json("POST", fmt.Sprintf("/api/v1/requests/%d/spam", d.ID), nil)
	expect(t, resp, body, 409)
	if n := countRows(t, e, `SELECT COUNT(*) FROM anon_spam_blocks`); n != 0 {
		t.Fatal("a named request must not block anything")
	}
}

func decodeAllowed(t *testing.T, body []byte) bool {
	t.Helper()
	v := decode[map[string]bool](t, body)
	return v["anonymous_allowed"]
}
