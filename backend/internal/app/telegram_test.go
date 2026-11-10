package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"modusvoice/internal/modules/requests"
)

const tgTestToken = "424242:AAF-test-token-not-real"

// fakeTG is a minimal Telegram Bot API: getMe, getUpdates (offset aware, short long-poll) and
// sendMessage (recorded).
type fakeTG struct {
	srv *httptest.Server
	mu  sync.Mutex
	ups []map[string]any
	seq int
	out []map[string]any
}

func newFakeTG(t *testing.T) *fakeTG {
	f := &fakeTG{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTG) handle(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/bot"+tgTestToken+"/")
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "getMe":
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"username":"mv_test_bot"}}`))
	case "getUpdates":
		var in struct {
			Offset int `json:"offset"`
		}
		_ = json.Unmarshal(body, &in)
		for i := 0; i < 20; i++ {
			f.mu.Lock()
			var batch []map[string]any
			for _, u := range f.ups {
				if int(u["update_id"].(int)) >= in.Offset {
					batch = append(batch, u)
				}
			}
			f.mu.Unlock()
			if len(batch) > 0 || i == 19 {
				b, _ := json.Marshal(map[string]any{"ok": true, "result": batch})
				_, _ = w.Write(b)
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	case "sendMessage":
		var in map[string]any
		_ = json.Unmarshal(body, &in)
		f.mu.Lock()
		f.out = append(f.out, in)
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeTG) say(chat int64, username, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	f.ups = append(f.ups, map[string]any{"update_id": f.seq, "message": map[string]any{
		"text": text, "chat": map[string]any{"id": chat, "type": "private"}, "from": map[string]any{"id": chat, "username": username},
	}})
}

func (f *fakeTG) messages(chat int64) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, m := range f.out {
		if int64(m["chat_id"].(float64)) == chat {
			out = append(out, m)
		}
	}
	return out
}

// waitFor polls until a message to chat satisfies pred (returns it) or fails the test.
func (f *fakeTG) waitFor(t *testing.T, chat int64, what string, pred func(text string) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range f.messages(chat) {
			if pred(m["text"].(string)) {
				return m
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("no message to chat %d matching %q; got %v", chat, what, f.messages(chat))
	return nil
}

func has(sub string) func(string) bool {
	return func(s string) bool { return strings.Contains(s, sub) }
}

func newTGEnv(t *testing.T) (*testEnv, *fakeTG) {
	t.Helper()
	f := newFakeTG(t)
	e := newEnvCfg(t, func(c *Config) {
		c.TelegramBotToken, c.TelegramAPIBase = tgTestToken, f.srv.URL
		c.TelegramBotURL, c.AppBaseURL = "https://t.me/mv_test_bot", "https://voice.example.com"
	})
	if len(e.app.runners) != 1 {
		t.Fatalf("expected the telegram runner, got %d runners", len(e.app.runners))
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.app.runners[0].Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("telegram runner did not stop on context cancel")
		}
	})
	return e, f
}

type tgStatus struct {
	Enabled          bool    `json:"enabled"`
	BotURL           *string `json:"bot_url"`
	BotUsername      *string `json:"bot_username"`
	Linked           bool    `json:"linked"`
	LinkedAt         *string `json:"linked_at"`
	TelegramUsername *string `json:"telegram_username"`
}

func (c *client) tgStatus() (tgStatus, string) {
	c.e.t.Helper()
	resp, body := c.get("/api/v1/telegram/status")
	if resp.StatusCode != 200 {
		c.e.t.Fatalf("status %d %s", resp.StatusCode, body)
	}
	var s tgStatus
	if err := json.Unmarshal(body, &s); err != nil {
		c.e.t.Fatal(err)
	}
	return s, string(body)
}

func (c *client) linkCode() string {
	c.e.t.Helper()
	resp, body := c.get("/api/v1/telegram/link-code")
	if resp.StatusCode != 200 {
		c.e.t.Fatalf("link-code %d %s", resp.StatusCode, body)
	}
	var v struct {
		Code      string `json:"code"`
		ExpiresIn int    `json:"expires_in_seconds"`
		Period    int    `json:"period_seconds"`
		BotURL    string `json:"bot_url"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		c.e.t.Fatal(err)
	}
	if len(v.Code) != 6 || v.Period != 20 || v.ExpiresIn < 1 || v.ExpiresIn > 20 || v.BotURL != "https://t.me/mv_test_bot" {
		c.e.t.Fatalf("bad link-code payload: %s", body)
	}
	return v.Code
}

func (e *testEnv) linkChat(f *fakeTG, c *client, chat int64, username string) {
	e.t.Helper()
	f.say(chat, username, c.linkCode())
	f.waitFor(e.t, chat, "link confirmation", has("Готово"))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s, _ := c.tgStatus(); s.Linked {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatal("status never became linked")
}

// --- disabled mode ---

func TestTelegramDisabledWithoutToken(t *testing.T) {
	e := newEnv(t)
	if e.app.Cfg.TelegramEnabled() {
		t.Fatal("TelegramEnabled must be false without a token")
	}
	if n := len(e.app.runners); n != 0 {
		t.Fatalf("disabled module must start nothing, runners=%d", n)
	}
	c := e.login("emp1@test.kz")
	s, _ := c.tgStatus()
	if s.Enabled || s.Linked || s.BotURL != nil || s.BotUsername != nil {
		t.Fatalf("disabled status = %+v", s)
	}
	if resp, body := c.get("/api/v1/telegram/link-code"); resp.StatusCode != 404 {
		t.Fatalf("link-code when disabled: %d %s", resp.StatusCode, body)
	}
	if resp, _ := e.anon().get("/api/v1/telegram/status"); resp.StatusCode != 401 {
		t.Fatalf("status without session: %d", resp.StatusCode)
	}
	if resp, _ := e.anon().get("/api/v1/telegram/link-code"); resp.StatusCode != 401 {
		t.Fatalf("link-code without session: %d", resp.StatusCode)
	}
}

func TestTelegramConfigParsing(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "  "+tgTestToken+" ")
	t.Setenv("TELEGRAM_BOT_URL", "https://t.me/x_bot")
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("TELEGRAM_API_BASE", "")
	cfg := LoadConfig()
	if !cfg.TelegramEnabled() || cfg.TelegramBotToken != tgTestToken || cfg.TelegramBotURL != "https://t.me/x_bot" {
		t.Fatalf("parsed %+v", struct{ T, U string }{cfg.TelegramBotToken, cfg.TelegramBotURL})
	}
	if cfg.AppBaseURL != "http://localhost:8080" || cfg.TelegramAPIBase != "https://api.telegram.org" {
		t.Fatalf("defaults: %q %q", cfg.AppBaseURL, cfg.TelegramAPIBase)
	}
	if s := cfg.String() + (&cfg).String(); strings.Contains(s, "AAF-test-token") {
		t.Fatalf("config String leaks the token: %s", s)
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	if LoadConfig().TelegramEnabled() {
		t.Fatal("empty token must disable the bot")
	}
}

// --- link flow + mirroring + commands end to end ---

func TestTelegramEndToEnd(t *testing.T) {
	e, f := newTGEnv(t)
	emp1, emp2, mgrA, admin := e.login("emp1@test.kz"), e.login("emp2@test.kz"), e.login("mgra@test.kz"), e.login("admin@test.kz")

	// status before linking
	s, _ := mgrA.tgStatus()
	if !s.Enabled || s.Linked || s.BotURL == nil || *s.BotURL != "https://t.me/mv_test_bot" {
		t.Fatalf("status = %+v", s)
	}

	// history that exists before linking must not be flooded into the chat
	pre := e.create(e.fx.emp1, e.fx.catA, false)
	var unsentBefore int
	_ = e.app.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND telegram_sent_at IS NULL`, e.fx.mgrA).Scan(&unsentBefore)
	if unsentBefore == 0 {
		t.Fatal("fixture: manager should have a pending notification")
	}

	e.linkChat(f, mgrA, 777, "alpha_mgr")
	var unsentAfter int
	_ = e.app.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND telegram_sent_at IS NULL`, e.fx.mgrA).Scan(&unsentAfter)
	if unsentAfter != 0 {
		t.Fatalf("existing notifications must be marked sent at link time, %d left", unsentAfter)
	}
	time.Sleep(2500 * time.Millisecond) // a full mirror period: still nothing from the past
	for _, m := range f.messages(777) {
		if strings.Contains(m["text"].(string), pre.Number) {
			t.Fatalf("history flooded: %v", m["text"])
		}
	}

	// status after linking exposes the username but never the chat id
	s, raw := mgrA.tgStatus()
	if !s.Linked || s.LinkedAt == nil || s.TelegramUsername == nil || *s.TelegramUsername != "alpha_mgr" || strings.Contains(raw, "777") {
		t.Fatalf("linked status = %s", raw)
	}
	if resp, _ := mgrA.get("/api/v1/telegram/link-code"); resp.StatusCode != 409 {
		t.Fatalf("link-code when linked: %d", resp.StatusCode)
	}
	// the other users are unaffected
	if s, _ := emp1.tgStatus(); s.Linked {
		t.Fatal("emp1 must not appear linked")
	}

	// one user cannot be bound twice, one chat cannot be bound twice
	e.linkChat(f, emp1, 101, "ivan")
	f.say(101, "ivan", emp2.linkCode())
	f.waitFor(t, 101, "chat already linked", has("уже привязан"))
	if s, _ := emp2.tgStatus(); s.Linked {
		t.Fatal("emp2 got linked through emp1's chat")
	}
	f.say(999, "intruder", "ZZZZZ2") // guessing a code
	f.waitFor(t, 999, "reject", has("не найден"))

	// mirroring: a new request routed to the manager arrives with an «Открыть» deep link
	created := e.create(e.fx.emp2, e.fx.catA, false)
	msg := f.waitFor(t, 777, "new request notification", has(created.Number))
	kb := msg["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)[0].(map[string]any)
	wantURL := "https://voice.example.com/requests/" + itoa(created.ID)
	if kb["text"] != "Открыть" || kb["url"] != wantURL {
		t.Fatalf("button = %v, want %s", kb, wantURL)
	}
	if _, plain := msg["parse_mode"]; plain {
		t.Fatal("parse_mode must not be used")
	}
	// delivered exactly once
	time.Sleep(2500 * time.Millisecond)
	n := 0
	for _, m := range f.messages(777) {
		if strings.Contains(m["text"].(string), created.Number) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("notification delivered %d times", n)
	}

	// --- commands and authorization ---
	f.say(101, "ivan", "/inbox")
	f.waitFor(t, 101, "employee /inbox denied", has("руководителям"))
	f.say(101, "ivan", "/admin")
	f.waitFor(t, 101, "employee /admin denied", has("администраторам"))
	f.say(101, "ivan", "/me")
	f.waitFor(t, 101, "/me", has("Сотрудник Первый"))

	f.say(777, "alpha_mgr", "/inbox")
	f.waitFor(t, 777, "manager inbox", has(created.Number))
	f.say(777, "alpha_mgr", "/admin")
	f.waitFor(t, 777, "manager /admin denied", has("администраторам"))

	// emp1 sees own request, and is told "not found" for somebody else's (same as the web API)
	f.say(101, "ivan", "/my")
	f.waitFor(t, 101, "/my", has(pre.Number))
	f.say(101, "ivan", "/request "+created.Number)
	f.waitFor(t, 101, "foreign request hidden", has("Обращение не найдено"))
	f.say(101, "ivan", "/request "+pre.Number)
	f.waitFor(t, 101, "own request card", has("Разработка"))

	// clarification: staff asks, only the author answers (through the same service as HTTP)
	if _, err := e.app.Requests.Service.AddComment(context.Background(), e.actor(e.fx.mgrA), pre.ID, requests.CommentInput{Kind: requests.KindQuestion, Body: "Какой кабинет?"}); err != nil {
		t.Fatalf("question: %v", err)
	}
	f.waitFor(t, 101, "question notification mirrored", has("нужен ваш ответ"))
	emp2Chat := int64(202)
	e.linkChat(f, emp2, emp2Chat, "petr")
	f.say(emp2Chat, "petr", "/answer "+pre.Number+" чужой ответ")
	f.waitFor(t, emp2Chat, "foreign answer refused", has("Обращение не найдено"))
	f.say(777, "alpha_mgr", "/answer "+pre.Number+" ответ руководителя")
	f.waitFor(t, 777, "staff answer refused", has("только автор"))
	f.say(101, "ivan", "/answer "+pre.Number+" Кабинет 5")
	f.waitFor(t, 101, "answer accepted", has("Ответ отправлен"))
	resp, body := emp1.get("/api/v1/requests/" + itoa(pre.ID))
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"status":"in_review"`) || !strings.Contains(string(body), "Кабинет 5") {
		t.Fatalf("answer not applied: %d %s", resp.StatusCode, body)
	}

	// admin: unrouted request notification + /admin summary
	e.linkChat(f, admin, 303, "boss")
	unrouted := e.create(e.fx.emp1, e.fx.catOther, false)
	f.waitFor(t, 303, "unrouted notification for admin", has(unrouted.Number))
	f.say(303, "boss", "/admin")
	f.waitFor(t, 303, "admin summary", has("Не распределено: 1"))

	// anonymous requests are unreachable from the bot
	anon := e.create(e.fx.emp1, e.fx.catA, true)
	f.say(101, "ivan", "/request "+anon.Number)
	f.waitFor(t, 101, "anonymous hidden", func(s string) bool {
		return s == "Обращение не найдено." && countMsgs(f, 101, "Обращение не найдено.") >= 2
	})
	f.say(101, "ivan", "/my")
	time.Sleep(300 * time.Millisecond)
	for _, m := range f.messages(101) {
		if strings.Contains(m["text"].(string), anon.Number) {
			t.Fatalf("anonymous request visible in the bot: %v", m["text"])
		}
	}
}

func countMsgs(f *fakeTG, chat int64, text string) int {
	n := 0
	for _, m := range f.messages(chat) {
		if m["text"].(string) == text {
			n++
		}
	}
	return n
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
