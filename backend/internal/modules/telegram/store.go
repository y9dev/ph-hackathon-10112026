package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// Link binds a user to exactly one Telegram chat, permanently.
type Link struct {
	UserID   int64
	ChatID   int64
	Username string
	LinkedAt time.Time
}

// Account is the linked user as the bot needs to know them. Actor is built exactly like the
// session actor of the identity module so the same authorization rules apply.
type Account struct {
	Actor          authz.Actor
	Active         bool
	DepartmentName string
}

// Errors returned by Store.CreateLink.
var (
	ErrUserLinked = errors.New("user already linked")
	ErrChatLinked = errors.New("chat already linked")
	ErrNoAccount  = errors.New("account not found")
)

// Store persists links and reads the user directory.
type Store interface {
	LinkByUser(ctx context.Context, userID int64) (Link, bool, error)
	LinkByChat(ctx context.Context, chatID int64) (Link, bool, error)
	// CreateLink inserts l and runs inTx in the same transaction (used to mark the user's existing
	// notifications as delivered). ErrUserLinked / ErrChatLinked on uniqueness violations.
	CreateLink(ctx context.Context, l Link, inTx func(ctx context.Context, q db.Querier) error) error
	Account(ctx context.Context, userID int64) (Account, error)
}

// SQLiteStore implements Store.
type SQLiteStore struct{ DB *sql.DB }

func (s *SQLiteStore) link(ctx context.Context, where string, arg int64) (Link, bool, error) {
	var l Link
	var uname sql.NullString
	var at int64
	err := s.DB.QueryRowContext(ctx, `SELECT user_id, chat_id, tg_username, linked_at FROM telegram_links WHERE `+where+` = ?`, arg).
		Scan(&l.UserID, &l.ChatID, &uname, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, false, nil
	}
	if err != nil {
		return Link{}, false, fmt.Errorf("query telegram link: %w", err)
	}
	l.Username, l.LinkedAt = uname.String, db.Time(at)
	return l, true, nil
}

func (s *SQLiteStore) LinkByUser(ctx context.Context, userID int64) (Link, bool, error) {
	return s.link(ctx, "user_id", userID)
}

func (s *SQLiteStore) LinkByChat(ctx context.Context, chatID int64) (Link, bool, error) {
	return s.link(ctx, "chat_id", chatID)
}

func (s *SQLiteStore) CreateLink(ctx context.Context, l Link, inTx func(ctx context.Context, q db.Querier) error) error {
	err := db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		var uname any
		if l.Username != "" {
			uname = l.Username
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_links(user_id, chat_id, tg_username, linked_at) VALUES (?,?,?,?)`,
			l.UserID, l.ChatID, uname, db.Unix(l.LinkedAt)); err != nil {
			return err
		}
		if inTx != nil {
			return inTx(ctx, tx)
		}
		return nil
	})
	if err != nil && db.IsUniqueViolation(err) {
		if _, ok, qerr := s.LinkByChat(ctx, l.ChatID); qerr == nil && ok {
			return ErrChatLinked
		}
		return ErrUserLinked
	}
	if err != nil {
		return fmt.Errorf("create telegram link: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Account(ctx context.Context, userID int64) (Account, error) {
	var a Account
	var dept sql.NullInt64
	var deptName sql.NullString
	var role string
	var active int
	err := s.DB.QueryRowContext(ctx, `SELECT u.name, u.email, u.role, u.department_id, d.name, u.active
		FROM users u LEFT JOIN departments d ON d.id = u.department_id WHERE u.id = ?`, userID).
		Scan(&a.Actor.Name, &a.Actor.Email, &role, &dept, &deptName, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNoAccount
	}
	if err != nil {
		return Account{}, fmt.Errorf("query account: %w", err)
	}
	a.Actor.UserID, a.Actor.Role, a.Actor.DepartmentID = userID, authz.Role(role), db.NullInt(dept)
	a.Active, a.DepartmentName = active == 1, deptName.String
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM departments WHERE head_user_id = ? ORDER BY id`, userID)
	if err != nil {
		return Account{}, fmt.Errorf("query led departments: %w", err)
	}
	defer rows.Close()
	a.Actor.LedDepartments = []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return Account{}, err
		}
		a.Actor.LedDepartments = append(a.Actor.LedDepartments, id)
	}
	return a, rows.Err()
}
