package identity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// SQLiteStore implements Store on database/sql.
type SQLiteStore struct{ DB *sql.DB }

const userCols = `u.id, u.email, u.name, u.title, u.password_hash, u.role, u.department_id, d.name, u.active, u.created_at`
const userFrom = ` FROM users u LEFT JOIN departments d ON d.id = u.department_id`

func scanUser(sc interface{ Scan(...any) error }) (User, error) {
	var u User
	var role string
	var dept sql.NullInt64
	var deptName sql.NullString
	var active int
	var created int64
	if err := sc.Scan(&u.ID, &u.Email, &u.Name, &u.Title, &u.PasswordHash, &role, &dept, &deptName, &active, &created); err != nil {
		return User{}, err
	}
	u.Role = authz.Role(role)
	u.DepartmentID = db.NullInt(dept)
	u.DepartmentName = db.NullStr(deptName)
	u.Active = active == 1
	u.CreatedAt = db.Time(created)
	return u, nil
}

func (s *SQLiteStore) one(ctx context.Context, where string, arg any) (User, error) {
	u, err := scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+userFrom+` WHERE `+where, arg))
	if db.IsNoRows(err) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}

func (s *SQLiteStore) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.one(ctx, `u.email = ?`, email)
}

func (s *SQLiteStore) UserByID(ctx context.Context, id int64) (User, error) {
	return s.one(ctx, `u.id = ?`, id)
}

func (s *SQLiteStore) many(ctx context.Context, q string, args ...any) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userCols+userFrom+q, args...)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]User, error) {
	return s.many(ctx, ` ORDER BY u.name, u.id`)
}

func (s *SQLiteStore) UsersByEmails(ctx context.Context, emails []string) ([]User, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	args := make([]any, len(emails))
	for i, e := range emails {
		args[i] = e
	}
	return s.many(ctx, ` WHERE u.active = 1 AND u.email IN (`+db.Placeholders(len(emails))+`)`, args...)
}

func (s *SQLiteStore) ActiveStaff(ctx context.Context) ([]User, error) {
	return s.many(ctx, ` WHERE u.active = 1 AND u.role IN ('manager','admin') ORDER BY u.name`)
}

func (s *SQLiteStore) ActiveAdminIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM users WHERE active = 1 AND role = 'admin' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query admins: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *SQLiteStore) LedDepartments(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM departments WHERE head_user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("query led departments: %w", err)
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func mapWriteErr(err error) error {
	switch {
	case db.IsUniqueViolation(err):
		return ErrDuplicateEmail
	case db.IsForeignKeyViolation(err):
		return ErrBadDepartment
	}
	return err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLiteStore) InsertUser(ctx context.Context, u User) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users(email, name, title, password_hash, role, department_id, active, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		u.Email, u.Name, u.Title, u.PasswordHash, string(u.Role), db.IntOrNil(u.DepartmentID), b2i(u.Active), db.Unix(u.CreatedAt))
	if err != nil {
		return 0, mapWriteErr(err)
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) UpdateUser(ctx context.Context, u User) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET email=?, name=?, password_hash=?, role=?, department_id=?, active=? WHERE id=?`,
		u.Email, u.Name, u.PasswordHash, string(u.Role), db.IntOrNil(u.DepartmentID), b2i(u.Active), u.ID)
	return mapWriteErr(err)
}

func (s *SQLiteStore) CreateSession(ctx context.Context, tokenHash string, userID int64, expires, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)`,
		tokenHash, userID, db.Unix(expires), db.Unix(now))
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SessionUserID(ctx context.Context, tokenHash string, now time.Time) (int64, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE token_hash = ? AND expires_at > ?`, tokenHash, db.Unix(now)).Scan(&id)
	if db.IsNoRows(err) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("query session: %w", err)
	}
	return id, nil
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *SQLiteStore) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *SQLiteStore) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, db.Unix(now))
	return err
}
