package notifications

import (
	"context"
	"time"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// Service creates and lists in-app notifications.
type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

// Write inserts notifications inside the caller's transaction q, skipping the acting user.
func (s *Service) Write(ctx context.Context, q db.Querier, actorID int64, ns []Note) error {
	ns = Dedupe(ns, actorID)
	if len(ns) == 0 {
		return nil
	}
	return s.store.Insert(ctx, q, s.now(), ns)
}

// List returns the actor's notifications (limit 1..100, default 20).
func (s *Service) List(ctx context.Context, a authz.Actor, unreadOnly bool, limit int) (List, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, err := s.store.List(ctx, a.UserID, unreadOnly, limit)
	if err != nil {
		return List{}, err
	}
	n, err := s.store.UnreadCount(ctx, a.UserID)
	if err != nil {
		return List{}, err
	}
	return List{UnreadCount: n, Items: items}, nil
}

// MarkRead marks the actor's own notifications read (ids or all).
func (s *Service) MarkRead(ctx context.Context, a authz.Actor, ids []int64, all bool) error {
	if !all && len(ids) == 0 {
		return apperr.Invalid("ids", "Укажите ids или all=true")
	}
	if len(ids) > 500 {
		return apperr.Invalid("ids", "Не более 500 идентификаторов за раз")
	}
	return s.store.MarkRead(ctx, a.UserID, ids, all, s.now())
}
