package integrations

import (
	"context"
	"log/slog"

	"modusvoice/internal/platform/eventbus"
)

// LogNotifier is the demo Notifier adapter: it logs what an external channel would receive.
type LogNotifier struct{ Log *slog.Logger }

func (n LogNotifier) Notify(_ context.Context, m Message) error {
	n.Log.Info("integration notify", "topic", m.Topic, "request_id", m.RequestID, "number", m.Number, "text", m.Text)
	return nil
}

// SubscribeNotifier forwards request events from the bus to a Notifier port.
func SubscribeNotifier(bus *eventbus.Bus, n Notifier) {
	bus.Subscribe("*", func(ctx context.Context, e eventbus.Event) error {
		return n.Notify(ctx, Message{Topic: e.Topic, RequestID: e.RequestID, Number: e.Number, Text: e.Topic + " " + e.Number, At: e.At})
	})
}
