package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"modusvoice/internal/platform/db"
)

// TelegramItem is a notification waiting to be mirrored to the Telegram chat of its recipient.
type TelegramItem struct {
	ID            int64
	UserID        int64
	ChatID        int64
	Kind          string
	RequestID     int64
	RequestNumber string
	RequestStatus string
	Title         string
	Body          string
	Attempts      int
}

// PendingTelegram returns up to limit unsent notifications of users that linked a Telegram chat
// (active users only), oldest first. It reads telegram_links, owned by the telegram module.
func (s *Service) PendingTelegram(ctx context.Context, limit int) ([]TelegramItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.PendingTelegram(ctx, limit)
}

// MarkTelegramSent records that notification id was delivered (or abandoned).
func (s *Service) MarkTelegramSent(ctx context.Context, id int64) error {
	return s.store.MarkTelegramSent(ctx, id, s.now())
}

// RecordTelegramFailure increments the delivery attempt counter and returns the new value.
func (s *Service) RecordTelegramFailure(ctx context.Context, id int64) (int, error) {
	return s.store.RecordTelegramFailure(ctx, id)
}

// MarkAllTelegramSent marks every unsent notification of a user as delivered, inside q (the
// transaction that creates the link), so a freshly linked chat does not receive the history.
func (s *Service) MarkAllTelegramSent(ctx context.Context, q db.Querier, userID int64) error {
	return s.store.MarkAllTelegramSent(ctx, q, userID, s.now())
}

func (s *SQLiteStore) PendingTelegram(ctx context.Context, limit int) ([]TelegramItem, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT n.id, n.user_id, l.chat_id, n.kind, n.request_id, r.number, r.status, n.title, n.body, n.telegram_attempts
		FROM telegram_links l
		JOIN users u ON u.id = l.user_id AND u.active = 1
		JOIN notifications n ON n.user_id = l.user_id AND n.telegram_sent_at IS NULL
		JOIN requests r ON r.id = n.request_id
		ORDER BY n.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending telegram notifications: %w", err)
	}
	defer rows.Close()
	var out []TelegramItem
	for rows.Next() {
		var it TelegramItem
		if err := rows.Scan(&it.ID, &it.UserID, &it.ChatID, &it.Kind, &it.RequestID, &it.RequestNumber, &it.RequestStatus, &it.Title, &it.Body, &it.Attempts); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) MarkTelegramSent(ctx context.Context, id int64, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET telegram_sent_at = ? WHERE id = ? AND telegram_sent_at IS NULL`, db.Unix(now), id)
	return err
}

func (s *SQLiteStore) RecordTelegramFailure(ctx context.Context, id int64) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `UPDATE notifications SET telegram_attempts = telegram_attempts + 1 WHERE id = ? RETURNING telegram_attempts`, id).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (s *SQLiteStore) MarkAllTelegramSent(ctx context.Context, q db.Querier, userID int64, now time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE notifications SET telegram_sent_at = ? WHERE user_id = ? AND telegram_sent_at IS NULL`, db.Unix(now), userID)
	return err
}
