package org

import (
	"context"
	"database/sql"
	"fmt"

	"modusvoice/internal/platform/db"
)

// SQLiteStore implements Store. Department head names are read via a join on users (read model).
type SQLiteStore struct{ DB *sql.DB }

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case db.IsUniqueViolation(err):
		return ErrDuplicate
	case db.IsForeignKeyViolation(err):
		return ErrBadRef
	}
	return err
}

const deptSelect = `SELECT d.id, d.name, d.head_user_id, u.name, IFNULL(u.active, 0) FROM departments d LEFT JOIN users u ON u.id = d.head_user_id`

func scanDept(sc interface{ Scan(...any) error }) (Department, error) {
	var d Department
	var head sql.NullInt64
	var headName sql.NullString
	var active int
	if err := sc.Scan(&d.ID, &d.Name, &head, &headName, &active); err != nil {
		return Department{}, err
	}
	d.HeadUserID = db.NullInt(head)
	d.HeadName = db.NullStr(headName)
	d.HeadActive = active == 1
	return d, nil
}

func (s *SQLiteStore) Departments(ctx context.Context) ([]Department, error) {
	rows, err := s.DB.QueryContext(ctx, deptSelect+` ORDER BY d.name`)
	if err != nil {
		return nil, fmt.Errorf("query departments: %w", err)
	}
	defer rows.Close()
	out := []Department{}
	for rows.Next() {
		d, err := scanDept(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Department(ctx context.Context, id int64) (Department, error) {
	d, err := scanDept(s.DB.QueryRowContext(ctx, deptSelect+` WHERE d.id = ?`, id))
	if db.IsNoRows(err) {
		return Department{}, ErrNotFound
	}
	return d, err
}

const catSelect = `SELECT id, name, description, icon, sort, active FROM categories`

func scanCat(sc interface{ Scan(...any) error }) (Category, error) {
	var c Category
	var active int
	err := sc.Scan(&c.ID, &c.Name, &c.Description, &c.Icon, &c.Sort, &active)
	c.Active = active == 1
	return c, err
}

func (s *SQLiteStore) Categories(ctx context.Context, activeOnly bool) ([]Category, error) {
	q := catSelect
	if activeOnly {
		q += ` WHERE active = 1`
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY sort, name`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		c, err := scanCat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Category(ctx context.Context, id int64) (Category, error) {
	c, err := scanCat(s.DB.QueryRowContext(ctx, catSelect+` WHERE id = ?`, id))
	if db.IsNoRows(err) {
		return Category{}, ErrNotFound
	}
	return c, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLiteStore) InsertCategory(ctx context.Context, c Category) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO categories(name, description, icon, sort, active) VALUES (?,?,?,?,?)`,
		c.Name, c.Description, c.Icon, c.Sort, b2i(c.Active))
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) UpdateCategory(ctx context.Context, c Category) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE categories SET name=?, description=?, icon=?, sort=?, active=? WHERE id=?`,
		c.Name, c.Description, c.Icon, c.Sort, b2i(c.Active), c.ID)
	return mapErr(err)
}

const ruleSelect = `SELECT r.id, r.category_id, c.name, r.request_type, r.department_id, d.name, r.assignee_user_id, u.name,
	r.first_response_hours, r.resolution_hours, r.active
	FROM routing_rules r JOIN categories c ON c.id = r.category_id JOIN departments d ON d.id = r.department_id
	LEFT JOIN users u ON u.id = r.assignee_user_id`

func scanRule(sc interface{ Scan(...any) error }) (Rule, error) {
	var r Rule
	var typ, aname sql.NullString
	var assignee sql.NullInt64
	var active int
	if err := sc.Scan(&r.ID, &r.CategoryID, &r.CategoryName, &typ, &r.DepartmentID, &r.DepartmentName, &assignee, &aname,
		&r.FirstResponseHours, &r.ResolutionHours, &active); err != nil {
		return Rule{}, err
	}
	r.RequestType = db.NullStr(typ)
	r.AssigneeUserID = db.NullInt(assignee)
	r.AssigneeName = db.NullStr(aname)
	r.Active = active == 1
	return r, nil
}

func (s *SQLiteStore) Rules(ctx context.Context) ([]Rule, error) {
	rows, err := s.DB.QueryContext(ctx, ruleSelect+` ORDER BY c.sort, c.name, r.request_type, r.id`)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Rule(ctx context.Context, id int64) (Rule, error) {
	r, err := scanRule(s.DB.QueryRowContext(ctx, ruleSelect+` WHERE r.id = ?`, id))
	if db.IsNoRows(err) {
		return Rule{}, ErrNotFound
	}
	return r, err
}

func (s *SQLiteStore) ActiveRule(ctx context.Context, categoryID int64, typ *string) (*Rule, error) {
	var row *sql.Row
	if typ == nil {
		row = s.DB.QueryRowContext(ctx, ruleSelect+` WHERE r.active = 1 AND r.category_id = ? AND r.request_type IS NULL`, categoryID)
	} else {
		row = s.DB.QueryRowContext(ctx, ruleSelect+` WHERE r.active = 1 AND r.category_id = ? AND r.request_type = ?`, categoryID, *typ)
	}
	r, err := scanRule(row)
	if db.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active rule: %w", err)
	}
	return &r, nil
}

func (s *SQLiteStore) InsertRule(ctx context.Context, r Rule) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO routing_rules(category_id, request_type, department_id, assignee_user_id, first_response_hours, resolution_hours, active) VALUES (?,?,?,?,?,?,?)`,
		r.CategoryID, r.RequestType, r.DepartmentID, db.IntOrNil(r.AssigneeUserID), r.FirstResponseHours, r.ResolutionHours, b2i(r.Active))
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) UpdateRule(ctx context.Context, r Rule) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE routing_rules SET category_id=?, request_type=?, department_id=?, assignee_user_id=?, first_response_hours=?, resolution_hours=?, active=? WHERE id=?`,
		r.CategoryID, r.RequestType, r.DepartmentID, db.IntOrNil(r.AssigneeUserID), r.FirstResponseHours, r.ResolutionHours, b2i(r.Active), r.ID)
	return mapErr(err)
}

func (s *SQLiteStore) DeleteRule(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM routing_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
