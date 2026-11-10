package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "123456:SECRET-TOKEN-VALUE"

// fakeTelegram is a tiny Bot API server recording calls.
type fakeTelegram struct {
	srv *httptest.Server
	mu  sync.Mutex

	sends   []map[string]any
	updates [][]Update // each getUpdates call pops one batch
	// sendHook may fail a sendMessage: (callNo starting at 1) -> (status, body); status 0 = ok.
	sendHook func(n int) (int, string)
	getMeErr bool
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	f := &fakeTelegram{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTelegram) handle(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/bot"+testToken+"/") {
		http.NotFound(w, r)
		return
	}
	method := strings.TrimPrefix(r.URL.Path, "/bot"+testToken+"/")
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "getMe":
		if f.getMeErr {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":99,"username":"mv_test_bot","first_name":"MV"}}`))
	case "getUpdates":
		var batch []Update
		if len(f.updates) > 0 {
			batch, f.updates = f.updates[0], f.updates[1:]
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "result": batch})
		_, _ = w.Write(b)
	case "sendMessage":
		var in map[string]any
		_ = json.Unmarshal(body, &in)
		n := len(f.sends) + 1
		if f.sendHook != nil {
			if status, resp := f.sendHook(n); status != 0 {
				f.sends = append(f.sends, map[string]any{"_failed": true, "chat_id": in["chat_id"]})
				w.WriteHeader(status)
				_, _ = w.Write([]byte(resp))
				return
			}
		}
		f.sends = append(f.sends, in)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeTelegram) delivered() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, s := range f.sends {
		if s["_failed"] == nil {
			out = append(out, s)
		}
	}
	return out
}

func (f *fakeTelegram) client() *Client {
	c := NewClient(Config{Token: testToken, APIBase: f.srv.URL}.withDefaults())
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

func TestClientGetMeAndUpdates(t *testing.T) {
	f := newFakeTelegram(t)
	f.updates = [][]Update{{{UpdateID: 5, Message: &Message{Text: "/start", Chat: Chat{ID: 42, Type: "private"}, From: &User{ID: 42, Username: "ivan"}}}}}
	c := f.client()
	me, err := c.GetMe(context.Background())
	if err != nil || me.Username != "mv_test_bot" {
		t.Fatalf("getMe = %+v, %v", me, err)
	}
	ups, err := c.GetUpdates(context.Background(), 0, 0)
	if err != nil || len(ups) != 1 || ups[0].UpdateID != 5 || ups[0].Message.Chat.ID != 42 || ups[0].Message.From.Username != "ivan" {
		t.Fatalf("updates = %+v, %v", ups, err)
	}
}

func TestSendMessagePlainTextWithButton(t *testing.T) {
	f := newFakeTelegram(t)
	c := f.client()
	if err := c.SendMessage(context.Background(), 42, "<b>raw</b> text", &Button{Text: "Открыть", URL: "https://example.com/requests/1"}); err != nil {
		t.Fatal(err)
	}
	got := f.delivered()
	if len(got) != 1 {
		t.Fatalf("sent %d", len(got))
	}
	if _, has := got[0]["parse_mode"]; has {
		t.Fatal("parse_mode must not be set")
	}
	if got[0]["text"] != "<b>raw</b> text" || got[0]["chat_id"].(float64) != 42 {
		t.Fatalf("payload %v", got[0])
	}
	kb := got[0]["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)[0].(map[string]any)
	if kb["text"] != "Открыть" || kb["url"] != "https://example.com/requests/1" {
		t.Fatalf("keyboard %v", kb)
	}
}

func TestSendMessageRetriesOn429(t *testing.T) {
	f := newFakeTelegram(t)
	f.sendHook = func(n int) (int, string) {
		if n <= 2 {
			return 429, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":3}}`
		}
		return 0, ""
	}
	c := f.client()
	var slept []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	if err := c.SendMessage(context.Background(), 1, "hi", nil); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(f.delivered()) != 1 {
		t.Fatalf("delivered %d", len(f.delivered()))
	}
	n3 := 0
	for _, d := range slept {
		if d == 3*time.Second {
			n3++
		}
	}
	if n3 != 2 {
		t.Fatalf("retry_after sleeps = %v, want two 3s waits", slept)
	}
}

func TestSendMessageGivesUpAfterRepeated429(t *testing.T) {
	f := newFakeTelegram(t)
	f.sendHook = func(int) (int, string) {
		return 429, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`
	}
	err := f.client().SendMessage(context.Background(), 1, "hi", nil)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != 429 {
		t.Fatalf("err = %v", err)
	}
}

func TestClientErrorsNeverContainToken(t *testing.T) {
	f := newFakeTelegram(t)
	f.srv.Close() // connection refused; net/http would embed the URL (with the token)
	c := NewClient(Config{Token: testToken, APIBase: f.srv.URL}.withDefaults())
	_, err := c.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN-VALUE") || strings.Contains(err.Error(), testToken) {
		t.Fatalf("token leaked in error: %v", err)
	}
	if s := (Config{Token: testToken}).String(); strings.Contains(s, "SECRET") {
		t.Fatalf("config String leaks token: %s", s)
	}
}

func TestSendLimiterSpacing(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newSendLimiter(GlobalSendInterval, ChatSendInterval, func() time.Time { return now })

	// 25 messages fit into one second: the 26th waits a full second (<= 25 msg/s overall)
	var last time.Duration
	for i := 0; i < 26; i++ {
		last = l.reserveGlobal()
	}
	if want := 25 * GlobalSendInterval; last != want {
		t.Fatalf("26th message waits %v, want %v", last, want)
	}

	// same chat: at most one per second; other chats are not delayed by its backlog
	if w := l.reserveChat(5); w != 0 {
		t.Fatalf("first wait %v", w)
	}
	if w := l.reserveChat(5); w != ChatSendInterval {
		t.Fatalf("second message to the same chat waits %v, want 1s", w)
	}
	if w := l.reserveChat(5); w != 2*ChatSendInterval {
		t.Fatalf("third waits %v, want 2s", w)
	}
	if w := l.reserveChat(6); w != 0 {
		t.Fatalf("other chat delayed by %v", w)
	}
}

func TestSendMessageAppliesChatLimit(t *testing.T) {
	f := newFakeTelegram(t)
	c := f.client()
	var slept []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	for i := 0; i < 3; i++ {
		if err := c.SendMessage(context.Background(), 9, "x", nil); err != nil {
			t.Fatal(err)
		}
	}
	var max time.Duration
	for _, d := range slept {
		if d > max {
			max = d
		}
	}
	if max < ChatSendInterval {
		t.Fatalf("per-chat limit not applied, sleeps=%v", slept)
	}
}

func TestButtonURLAllowed(t *testing.T) {
	for u, want := range map[string]bool{
		"https://voice.example.com/requests/1": true,
		"http://localhost:8080/requests/1":     false,
		"http://127.0.0.1:8080/x":              false,
		"http://192.168.1.5/x":                 false,
		"ftp://example.com/x":                  false,
		"not a url":                            false,
	} {
		if got := buttonURLAllowed(u); got != want {
			t.Errorf("buttonURLAllowed(%q) = %v, want %v", u, got, want)
		}
	}
}
