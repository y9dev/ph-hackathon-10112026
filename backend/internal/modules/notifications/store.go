package notifications

import (
	"context"
	"time"

	"modusvoice/internal/platform/db"
)

// Store persists notifications.
type Store interface {
	// Insert writes ns using q (the caller's transaction).
	Insert(ctx context.Context, q db.Querier, now time.Time, ns []Note) error
	List(ctx context.Context, userID int64, unreadOnly bool, limit int) ([]View, error)
	UnreadCount(ctx context.Context, userID int64) (int, error)
	MarkRead(ctx context.Context, userID int64, ids []int64, all bool, now time.Time) error
}
