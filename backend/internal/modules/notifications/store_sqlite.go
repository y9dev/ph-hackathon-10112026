package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"modusvoice/internal/platform/db"
)

// SQLiteStore implements Store. Request numbers are read via a join (read model).
type SQLiteStore struct{ DB *sql.DB }

func (s *SQLiteStore) Insert(ctx context.Context, q db.Querier, now time.Time, ns []Note) error {
	for _, n := range ns {
		if _, err := q.ExecContext(ctx, `INSERT INTO notifications(user_id, kind, request_id, title, body, created_at) VALUES (?,?,?,?,?,?)`,
			n.UserID, n.Kind, n.RequestID, n.Title, n.Body, db.Unix(now)); err != nil {
			return fmt.Errorf("insert notification: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) List(ctx context.Context, userID int64, unreadOnly bool, limit int) ([]View, error) {
	q := `SELECT n.id, n.kind, n.request_id, r.number, n.title, n.body, n.read_at, n.created_at
		FROM notifications n JOIN requests r ON r.id = n.request_id WHERE n.user_id = ?`
	if unreadOnly {
		q += ` AND n.read_at IS NULL`
	}
	q += ` ORDER BY n.created_at DESC, n.id DESC LIMIT ?`
	rows, err := s.DB.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()
	out := []View{}
	for rows.Next() {
		var v View
		var readAt sql.NullInt64
		var created int64
		if err := rows.Scan(&v.ID, &v.Kind, &v.RequestID, &v.RequestNumber, &v.Title, &v.Body, &readAt, &created); err != nil {
			return nil, err
		}
		v.Read = readAt.Valid
		v.CreatedAt = db.Time(created)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) UnreadCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

func (s *SQLiteStore) MarkRead(ctx context.Context, userID int64, ids []int64, all bool, now time.Time) error {
	if all {
		_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE user_id = ? AND read_at IS NULL`, db.Unix(now), userID)
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	args := []any{db.Unix(now), userID}
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE user_id = ? AND read_at IS NULL AND id IN (`+db.Placeholders(len(ids))+`)`, args...)
	return err
}
