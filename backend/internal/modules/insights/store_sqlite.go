package insights

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modusvoice/internal/platform/db"
)

// ErrNotFound is returned when a request does not exist.
var ErrNotFound = errors.New("insights: not found")

// SQLiteStore implements the read-only Store.
type SQLiteStore struct{ DB *sql.DB }

const docSelect = `SELECT r.id, r.number, r.title, r.body, r.category_id, c.name, r.status, r.department_id, r.assignee_id, r.theme_id, r.created_at
	FROM requests r JOIN categories c ON c.id = r.category_id`

func scanDoc(sc interface{ Scan(...any) error }) (Doc, error) {
	var d Doc
	var dept, asg, theme sql.NullInt64
	var created int64
	if err := sc.Scan(&d.ID, &d.Number, &d.Title, &d.Body, &d.CategoryID, &d.CategoryName, &d.Status, &dept, &asg, &theme, &created); err != nil {
		return Doc{}, err
	}
	d.DepartmentID, d.AssigneeID, d.ThemeID = db.NullInt(dept), db.NullInt(asg), db.NullInt(theme)
	d.CreatedAt = db.Time(created)
	return d, nil
}

func (s *SQLiteStore) Doc(ctx context.Context, id int64) (Doc, error) {
	d, err := scanDoc(s.DB.QueryRowContext(ctx, docSelect+` WHERE r.id = ?`, id))
	if db.IsNoRows(err) {
		return Doc{}, ErrNotFound
	}
	return d, err
}

func (s *SQLiteStore) Docs(ctx context.Context, scope Scope, since time.Time, limit int) ([]Doc, error) {
	args := append(append([]any{}, scope.Args...), db.Unix(since), limit)
	rows, err := s.DB.QueryContext(ctx, docSelect+` WHERE (`+scope.SQL+`) AND r.created_at >= ? ORDER BY r.created_at DESC, r.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("query docs: %w", err)
	}
	defer rows.Close()
	var out []Doc
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Facts(ctx context.Context, scope Scope, from, to *time.Time, departmentID *int64) ([]Fact, error) {
	q := `SELECT r.id, r.status, c.name, d.name, r.created_at, r.first_response_due, r.resolution_due, r.first_responded_at, r.closed_at,
		CASE WHEN r.result_text IS NOT NULL AND length(trim(r.result_text)) > 0 THEN 1 ELSE 0 END,
		EXISTS(SELECT 1 FROM request_events e WHERE e.request_id = r.id AND e.kind = 'routed' AND e.meta = 'rule')
		FROM requests r JOIN categories c ON c.id = r.category_id LEFT JOIN departments d ON d.id = r.department_id
		WHERE (` + scope.SQL + `)`
	args := append([]any{}, scope.Args...)
	if from != nil {
		q += ` AND r.created_at >= ?`
		args = append(args, db.Unix(*from))
	}
	if to != nil {
		q += ` AND r.created_at < ?`
		args = append(args, db.Unix(*to))
	}
	if departmentID != nil {
		q += ` AND r.department_id = ?`
		args = append(args, *departmentID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY r.created_at`, args...)
	if err != nil {
		return nil, fmt.Errorf("query facts: %w", err)
	}
	defer rows.Close()
	var out []Fact
	for rows.Next() {
		var f Fact
		var dept sql.NullString
		var created, frd, rd int64
		var fra, closed sql.NullInt64
		var hasResult, auto int
		if err := rows.Scan(&f.ID, &f.Status, &f.Category, &dept, &created, &frd, &rd, &fra, &closed, &hasResult, &auto); err != nil {
			return nil, err
		}
		f.Department = db.NullStr(dept)
		f.CreatedAt, f.FirstResponseDue, f.ResolutionDue = db.Time(created), db.Time(frd), db.Time(rd)
		f.FirstRespondedAt, f.ClosedAt = db.NullTime(fra), db.NullTime(closed)
		f.HasResult, f.AutoRouted = hasResult == 1, auto == 1
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) FirstCreated(ctx context.Context, scope Scope) (*time.Time, error) {
	var v sql.NullInt64
	if err := s.DB.QueryRowContext(ctx, `SELECT MIN(r.created_at) FROM requests r WHERE (`+scope.SQL+`)`, scope.Args...).Scan(&v); err != nil {
		return nil, err
	}
	return db.NullTime(v), nil
}

func (s *SQLiteStore) ThemeRows(ctx context.Context, scope Scope) ([]ThemeRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT t.id, t.title, r.status, c.name FROM themes t
		JOIN requests r ON r.theme_id = t.id JOIN categories c ON c.id = r.category_id
		WHERE (`+scope.SQL+`) ORDER BY t.id`, scope.Args...)
	if err != nil {
		return nil, fmt.Errorf("query themes: %w", err)
	}
	defer rows.Close()
	var out []ThemeRow
	for rows.Next() {
		var t ThemeRow
		if err := rows.Scan(&t.ThemeID, &t.Title, &t.Status, &t.Category); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
