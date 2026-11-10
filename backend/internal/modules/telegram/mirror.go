package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/requests"
)

const (
	// MirrorInterval is how often the worker looks for unsent notifications.
	MirrorInterval = 2 * time.Second
	// MaxDeliveryAttempts is how many failed sends are tolerated before a message is dropped.
	MaxDeliveryAttempts = 5
	mirrorBatch         = 50
	openButtonText      = "Открыть"
)

// Outbox is the notification side of mirroring (implemented by *notifications.Service).
type Outbox interface {
	PendingTelegram(ctx context.Context, limit int) ([]notifications.TelegramItem, error)
	MarkTelegramSent(ctx context.Context, id int64) error
	RecordTelegramFailure(ctx context.Context, id int64) (int, error)
}

// Mirror delivers in-app notifications of linked users to their Telegram chats. Delivery is
// at-least-once: the telegram_sent_at marker is written only after Telegram acknowledged the
// message, so a crash between send and mark re-sends it after restart. A message that failed
// MaxDeliveryAttempts times is marked sent with an error log so it cannot block the queue.
type Mirror struct {
	api     API
	outbox  Outbox
	cfg     Config
	log     *slog.Logger
	now     func() time.Time
	retryAt map[int64]time.Time // in-memory backoff per notification id
	// Interval overrides MirrorInterval (tests).
	Interval time.Duration
}

// Run ticks until ctx is cancelled.
func (m *Mirror) Run(ctx context.Context) {
	iv := m.Interval
	if iv <= 0 {
		iv = MirrorInterval
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Tick(ctx)
		}
	}
}

// Tick performs one delivery pass.
func (m *Mirror) Tick(ctx context.Context) {
	if m.retryAt == nil {
		m.retryAt = map[int64]time.Time{}
	}
	items, err := m.outbox.PendingTelegram(ctx, mirrorBatch)
	if err != nil {
		if ctx.Err() == nil {
			m.log.Error("telegram mirror: load pending", "err", err)
		}
		return
	}
	held := map[int64]bool{} // chats with a message in backoff keep their order
	for _, it := range items {
		if ctx.Err() != nil {
			return
		}
		if held[it.ChatID] {
			continue
		}
		if t, ok := m.retryAt[it.ID]; ok && m.now().Before(t) {
			held[it.ChatID] = true
			continue
		}
		if err := m.deliver(ctx, it); err != nil {
			if ctx.Err() != nil {
				return
			}
			n, ferr := m.outbox.RecordTelegramFailure(ctx, it.ID)
			if ferr != nil {
				m.log.Error("telegram mirror: record failure", "err", ferr)
			}
			if n >= MaxDeliveryAttempts {
				m.log.Error("telegram mirror: giving up on notification", "notification_id", it.ID, "attempts", n, "err", err)
				if merr := m.outbox.MarkTelegramSent(ctx, it.ID); merr != nil {
					m.log.Error("telegram mirror: mark abandoned", "err", merr)
				}
				delete(m.retryAt, it.ID)
				continue
			}
			m.log.Warn("telegram mirror: send failed, will retry", "notification_id", it.ID, "attempt", n, "err", err)
			m.retryAt[it.ID] = m.now().Add(time.Duration(1<<max(n-1, 0)) * 2 * time.Second)
			held[it.ChatID] = true
			continue
		}
		delete(m.retryAt, it.ID)
		if err := m.outbox.MarkTelegramSent(ctx, it.ID); err != nil {
			m.log.Error("telegram mirror: mark sent (message may be re-sent)", "notification_id", it.ID, "err", err)
			held[it.ChatID] = true
		}
	}
}

func (m *Mirror) deliver(ctx context.Context, it notifications.TelegramItem) error {
	link := fmt.Sprintf("%s/requests/%d", m.cfg.AppBaseURL, it.RequestID)
	var sb strings.Builder
	sb.WriteString(it.Title)
	if it.Body != "" {
		sb.WriteString("\n")
		sb.WriteString(it.Body)
	}
	fmt.Fprintf(&sb, "\n\nОбращение %s · %s", it.RequestNumber, requests.Status(it.RequestStatus).Label())
	text := truncate(sb.String(), MaxMessageRunes-len(link)-16)
	if buttonURLAllowed(link) {
		err := m.api.SendMessage(ctx, it.ChatID, text, &Button{Text: openButtonText, URL: link})
		var ae *APIError
		if err == nil || !errors.As(err, &ae) || ae.Code != http.StatusBadRequest {
			return err
		}
		// Telegram refused the button URL: fall back to a plain link in the text.
	}
	return m.api.SendMessage(ctx, it.ChatID, text+"\nОткрыть: "+link, nil)
}
