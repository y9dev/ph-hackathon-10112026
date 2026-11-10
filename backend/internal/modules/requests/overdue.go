package requests

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/platform/db"
	"modusvoice/internal/platform/eventbus"
)

// EscalationDelay: admins are notified this long after a deadline was missed (R3).
const EscalationDelay = 24 * time.Hour

// CheckOverdue emits overdue events/notifications once per (request, deadline kind, due timestamp):
// the owner immediately, admins after EscalationDelay. Idempotent via request_events.meta ("kind@due_unix");
// a deadline shifted by a needs_info pause and missed again fires again.
func (s *Service) CheckOverdue(ctx context.Context) (int, error) {
	now := s.now()
	rows, err := s.Store.OverdueCandidates(ctx, now)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	admins, err := s.People.ActiveAdminIDs(ctx)
	if err != nil {
		return 0, err
	}
	emitted := 0
	for _, r := range rows {
		kind := OverdueKindAt(r.SLAInput(), now)
		if kind == "" {
			continue
		}
		due, label := r.ResolutionDue, "Просрочен срок решения"
		if kind == OverdueFirstResponse {
			due, label = r.FirstResponseDue, "Просрочен срок первого ответа"
		}
		meta := fmt.Sprintf("%s@%d", kind, due.Unix())
		owners, err := s.ownerIDs(ctx, r.DepartmentID, r.AssigneeID)
		if err != nil {
			return emitted, err
		}
		newEvent := false
		err = db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
			has, err := s.Store.HasEvent(ctx, tx, r.ID, EvOverdue, meta)
			if err != nil {
				return err
			}
			if !has {
				newEvent = true
				if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: r.ID, Kind: EvOverdue, Text: ptr(label), Meta: ptr(meta), At: now}); err != nil {
					return err
				}
				if err := s.Notifier.Write(ctx, tx, 0, notes(owners, notifications.KindOverdue, r.ID,
					fmt.Sprintf("%s: %s", r.Number, label), excerpt(r.Title, 200))); err != nil {
					return err
				}
			}
			if now.Sub(due) < EscalationDelay {
				return nil
			}
			has, err = s.Store.HasEvent(ctx, tx, r.ID, EvOverdueEscalated, meta)
			if err != nil || has {
				return err
			}
			newEvent = true
			if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: r.ID, Kind: EvOverdueEscalated, Text: ptr(label + " более чем на 24 ч — уведомлены администраторы"), Meta: ptr(meta), At: now}); err != nil {
				return err
			}
			return s.Notifier.Write(ctx, tx, 0, notes(admins, notifications.KindOverdueEscalated, r.ID,
				fmt.Sprintf("Эскалация %s: %s", r.Number, label), excerpt(r.Title, 200)))
		})
		if err != nil {
			return emitted, err
		}
		if newEvent {
			emitted++
			s.publish(ctx, eventbus.RequestOverdue, r, map[string]any{"overdue_kind": kind})
		}
	}
	return emitted, nil
}

// Run starts the overdue ticker (every minute) until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	tick := func() {
		if n, err := s.CheckOverdue(ctx); err != nil {
			if ctx.Err() == nil {
				s.Log.Error("overdue check failed", "err", err)
			}
		} else if n > 0 {
			s.Log.Info("overdue notifications emitted", "requests", n)
		}
	}
	tick()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}
