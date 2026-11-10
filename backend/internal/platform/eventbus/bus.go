// Package eventbus is an in-process publish/subscribe bus for non-critical subscribers
// (logging, external notifier ports). Critical side effects (in-app notifications) are
// written synchronously inside the request transaction instead (R6 / AC-08).
package eventbus

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Topics published by the requests module.
const (
	RequestCreated       = "request.created"
	RequestAssigned      = "request.assigned"
	RequestStatusChanged = "request.status_changed"
	RequestCommented     = "request.commented"
	RequestQuestionAsked = "request.question_asked"
	RequestAnswered      = "request.answered"
	RequestOverdue       = "request.overdue"
	RequestClosed        = "request.closed"
)

// Event payloads carry ids, numbers and statuses only; never anonymous author identity.
type Event struct {
	Topic     string         `json:"topic"`
	RequestID int64          `json:"request_id"`
	Number    string         `json:"number"`
	At        time.Time      `json:"at"`
	Data      map[string]any `json:"data,omitempty"`
}

// Handler handles an event; errors are logged, never propagated to the publisher.
type Handler func(ctx context.Context, e Event) error

// Bus dispatches events synchronously after the publisher's transaction committed.
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
	log  *slog.Logger
}

func New(log *slog.Logger) *Bus { return &Bus{subs: map[string][]Handler{}, log: log} }

// Subscribe registers h for topic ("*" = all topics).
func (b *Bus) Subscribe(topic string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[topic] = append(b.subs[topic], h)
}

// Publish delivers e to subscribers, isolating panics and errors.
func (b *Bus) Publish(ctx context.Context, e Event) {
	if b == nil {
		return
	}
	b.mu.RLock()
	hs := append(append([]Handler{}, b.subs[e.Topic]...), b.subs["*"]...)
	b.mu.RUnlock()
	for _, h := range hs {
		func() {
			defer func() {
				if p := recover(); p != nil {
					b.log.Error("event subscriber panic", "topic", e.Topic, "panic", p)
				}
			}()
			if err := h(ctx, e); err != nil {
				b.log.Warn("event subscriber failed", "topic", e.Topic, "err", err)
			}
		}()
	}
}
