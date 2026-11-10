package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/modules/notifications"
)

type fakeOutbox struct {
	items    []notifications.TelegramItem
	sent     map[int64]bool
	attempts map[int64]int
}

func newFakeOutbox(items ...notifications.TelegramItem) *fakeOutbox {
	return &fakeOutbox{items: items, sent: map[int64]bool{}, attempts: map[int64]int{}}
}

func (o *fakeOutbox) PendingTelegram(_ context.Context, limit int) ([]notifications.TelegramItem, error) {
	var out []notifications.TelegramItem
	for _, it := range o.items {
		if !o.sent[it.ID] {
			it.Attempts = o.attempts[it.ID]
			out = append(out, it)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (o *fakeOutbox) MarkTelegramSent(_ context.Context, id int64) error {
	o.sent[id] = true
	return nil
}
func (o *fakeOutbox) RecordTelegramFailure(_ context.Context, id int64) (int, error) {
	o.attempts[id]++
	return o.attempts[id], nil
}

func item(id, chat int64, title string) notifications.TelegramItem {
	return notifications.TelegramItem{ID: id, UserID: 1, ChatID: chat, Kind: "status_changed", RequestID: 10, RequestNumber: "MV-0010",
		RequestStatus: "in_review", Title: title, Body: "Тело уведомления"}
}

func newMirror(api API, o Outbox, clk *fakeClock, base string) *Mirror {
	return &Mirror{api: api, outbox: o, cfg: Config{AppBaseURL: base}.withDefaults(), log: slog.New(slog.NewTextHandler(io.Discard, nil)), now: clk.Now}
}

func TestMirrorSendsAndMarksSent(t *testing.T) {
	api, clk := &fakeAPI{}, newFakeClock()
	o := newFakeOutbox(item(1, 500, "Статус изменён"), item(2, 500, "Новый комментарий"))
	m := newMirror(api, o, clk, "https://voice.example.com")
	m.Tick(context.Background())

	if api.count() != 2 || !o.sent[1] || !o.sent[2] {
		t.Fatalf("sent=%d marks=%v", api.count(), o.sent)
	}
	first := api.sent[0]
	for _, want := range []string{"Статус изменён", "Тело уведомления", "MV-0010", "В проработке"} {
		if !strings.Contains(first.Text, want) {
			t.Fatalf("message missing %q: %q", want, first.Text)
		}
	}
	if first.Btn == nil || first.Btn.Text != "Открыть" || first.Btn.URL != "https://voice.example.com/requests/10" {
		t.Fatalf("button = %+v", first.Btn)
	}
	if !strings.HasPrefix(api.sent[1].Text, "Новый комментарий") {
		t.Fatalf("order not preserved: %q", api.sent[1].Text)
	}
	// nothing is re-sent on the next tick
	m.Tick(context.Background())
	if api.count() != 2 {
		t.Fatalf("duplicate delivery: %d", api.count())
	}
}

func TestMirrorLocalhostUsesLinkInText(t *testing.T) {
	api, clk := &fakeAPI{}, newFakeClock()
	o := newFakeOutbox(item(1, 500, "T"))
	newMirror(api, o, clk, "http://localhost:8080").Tick(context.Background())
	if len(api.sent) != 1 || api.sent[0].Btn == nil || api.sent[0].Btn.URL != "http://localhost:8080/requests/10" || !strings.HasSuffix(api.sent[0].Text, "\nОткрыть") {
		t.Fatalf("sent = %+v", api.sent)
	}
}

func TestMirrorFallsBackWhenTelegramRejectsButton(t *testing.T) {
	clk := newFakeClock()
	api := &fakeAPI{}
	calls := 0
	api.fail = func(int64, string) error {
		calls++
		if calls == 1 {
			return &APIError{Code: 400, Description: "Bad Request: wrong HTTP URL"}
		}
		return nil
	}
	o := newFakeOutbox(item(1, 500, "T"))
	newMirror(api, o, clk, "https://voice.example.com").Tick(context.Background())
	if !o.sent[1] || len(api.sent) != 1 || api.sent[0].Btn == nil || api.sent[0].Btn.URL != "https://voice.example.com/requests/10" || !strings.HasSuffix(api.sent[0].Text, "\nОткрыть") {
		t.Fatalf("fallback failed: %+v sent=%v", api.sent, o.sent)
	}
}

func TestMirrorRetriesWithBackoffThenSucceeds(t *testing.T) {
	clk := newFakeClock()
	api := &fakeAPI{}
	fails := 2
	api.fail = func(int64, string) error {
		if fails > 0 {
			fails--
			return errors.New("network down")
		}
		return nil
	}
	o := newFakeOutbox(item(1, 500, "A"), item(2, 500, "B"), item(3, 600, "C"))
	m := newMirror(api, o, clk, "https://voice.example.com")

	m.Tick(context.Background()) // A fails, B held back (same chat, order), C (other chat) is delivered
	if o.sent[1] || o.sent[2] {
		t.Fatalf("A/B must wait: %v", o.sent)
	}
	m.Tick(context.Background()) // inside backoff: no new attempt for chat 500
	if o.attempts[1] != 1 {
		t.Fatalf("attempts during backoff = %d", o.attempts[1])
	}
	clk.Advance(10 * time.Second)
	m.Tick(context.Background())
	clk.Advance(10 * time.Second)
	m.Tick(context.Background())
	if !o.sent[1] || !o.sent[2] || !o.sent[3] {
		t.Fatalf("not eventually delivered: %v attempts=%v", o.sent, o.attempts)
	}
	var texts []string
	for _, s := range api.sent {
		if s.Chat == 500 {
			texts = append(texts, s.Text[:1])
		}
	}
	if strings.Join(texts, "") != "AB" {
		t.Fatalf("chat 500 order = %v", texts)
	}
}

func TestMirrorPoisonMessageIsDropped(t *testing.T) {
	clk := newFakeClock()
	api := &fakeAPI{fail: func(_ int64, text string) error {
		if strings.HasPrefix(text, "POISON") {
			return &APIError{Code: 403, Description: "Forbidden: bot was blocked by the user"}
		}
		return nil
	}}
	o := newFakeOutbox(item(1, 500, "POISON"), item(2, 500, "after"))
	m := newMirror(api, o, clk, "https://voice.example.com")
	for i := 0; i < MaxDeliveryAttempts+2; i++ {
		m.Tick(context.Background())
		clk.Advance(5 * time.Minute)
	}
	if o.attempts[1] != MaxDeliveryAttempts {
		t.Fatalf("attempts = %d, want %d", o.attempts[1], MaxDeliveryAttempts)
	}
	if !o.sent[1] {
		t.Fatal("poison message must be marked sent after the attempt limit")
	}
	if !o.sent[2] {
		t.Fatal("queue stayed blocked behind the poison message")
	}
}

func TestMirrorStopsOnContextCancel(t *testing.T) {
	api, clk := &fakeAPI{}, newFakeClock()
	o := newFakeOutbox(item(1, 500, "A"))
	m := newMirror(api, o, clk, "https://voice.example.com")
	m.Interval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for api.count() == 0 {
		select {
		case <-deadline:
			t.Fatal("worker did not deliver")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}
