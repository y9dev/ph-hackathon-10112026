package requests

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// SQLiteStore implements Store. Category/department/user names are read via joins (read model);
// writes touch only tables owned by this module.
type SQLiteStore struct{ DB *sql.DB }

const terminalSQL = `('implemented','declined','duplicate','closed')`

// overdueSQL mirrors OverdueKindAt; it takes two ? (now, now).
const overdueSQL = `(r.status IN ('new','in_review','decision','in_progress') AND ((r.first_responded_at IS NULL AND r.first_response_due < ?) OR r.resolution_due < ?))`

const rowCols = `r.id, r.number, r.type, r.priority, r.category_id, c.name, r.title, r.status, IFNULL(r.author_id, 0), IFNULL(au.name, ''), r.is_anonymous,
	r.department_id, d.name, r.assignee_id, asg.name, r.routing_source, r.first_response_due, r.resolution_due,
	r.first_responded_at, r.paused_at, r.closed_at, r.created_at, r.updated_at,
	(SELECT COUNT(*) FROM comments cm WHERE cm.request_id = r.id),
	(SELECT COUNT(*) FROM comments cm WHERE cm.request_id = r.id AND cm.kind <> 'internal')`

const rowFrom = ` FROM requests r
	JOIN categories c ON c.id = r.category_id
	LEFT JOIN users au ON au.id = r.author_id
	LEFT JOIN departments d ON d.id = r.department_id
	LEFT JOIN users asg ON asg.id = r.assignee_id`

type scanner interface{ Scan(...any) error }

func scanRow(sc scanner, extra ...any) (Row, error) {
	var r Row
	var status string
	var anon int
	var dept, asg, fra, paused, closed sql.NullInt64
	var deptName, asgName sql.NullString
	var frd, rd, created, updated int64
	dest := []any{&r.ID, &r.Number, &r.Type, &r.Priority, &r.CategoryID, &r.CategoryName, &r.Title, &status, &r.Author.ID, &r.Author.Name, &anon,
		&dept, &deptName, &asg, &asgName, &r.RoutingSource, &frd, &rd, &fra, &paused, &closed, &created, &updated,
		&r.CommentsAll, &r.CommentsPublic}
	if err := sc.Scan(append(dest, extra...)...); err != nil {
		return Row{}, err
	}
	r.Status = Status(status)
	r.Author.IsAnonymous = anon == 1
	r.DepartmentID, r.DepartmentName = db.NullInt(dept), db.NullStr(deptName)
	r.AssigneeID, r.AssigneeName = db.NullInt(asg), db.NullStr(asgName)
	r.FirstResponseDue, r.ResolutionDue = db.Time(frd), db.Time(rd)
	r.FirstRespondedAt, r.PausedAt, r.ClosedAt = db.NullTime(fra), db.NullTime(paused), db.NullTime(closed)
	r.CreatedAt, r.UpdatedAt = db.Time(created), db.Time(updated)
	return r, nil
}

func (s *SQLiteStore) NextNumber(ctx context.Context, q db.Querier) (int64, error) {
	var n int64
	if err := q.QueryRowContext(ctx, `UPDATE counters SET value = value + 1 WHERE name = 'request' RETURNING value`).Scan(&n); err != nil {
		return 0, fmt.Errorf("next request number: %w", err)
	}
	return n, nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLiteStore) InsertRequest(ctx context.Context, q db.Querier, r NewRequest) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO requests(number, type, priority, category_id, title, body, proposed_solution, author_id, is_anonymous,
		anon_token_hash, department_id, assignee_id, status, routing_source, first_response_due, resolution_due, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,'new',?,?,?,?,?)`,
		r.Number, r.Type, r.Priority, r.CategoryID, r.Title, r.Body, r.ProposedSolution, db.IntOrNil(r.AuthorID), b2i(r.IsAnonymous), r.AnonTokenHash,
		db.IntOrNil(r.DepartmentID), db.IntOrNil(r.AssigneeID), r.RoutingSource, db.Unix(r.FirstResponseDue), db.Unix(r.ResolutionDue),
		db.Unix(r.CreatedAt), db.Unix(r.CreatedAt))
	if err != nil {
		return 0, fmt.Errorf("insert request: %w", err)
	}
	return res.LastInsertId()
}

func statusPtr(s *Status) any {
	if s == nil {
		return nil
	}
	return string(*s)
}

func (s *SQLiteStore) InsertEvent(ctx context.Context, q db.Querier, e Event) error {
	_, err := q.ExecContext(ctx, `INSERT INTO request_events(request_id, actor_id, by_author, kind, from_status, to_status, text, meta, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		e.RequestID, db.IntOrNil(e.ActorID), b2i(e.ByAuthor), e.Kind, statusPtr(e.FromStatus), statusPtr(e.ToStatus), e.Text, e.Meta, db.Unix(e.At))
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *SQLiteStore) InsertComment(ctx context.Context, q db.Querier, c NewComment) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO comments(request_id, author_id, by_author, kind, body, created_at) VALUES (?,?,?,?,?,?)`,
		c.RequestID, db.IntOrNil(c.AuthorID), b2i(c.ByAuthor), c.Kind, c.Body, db.Unix(c.At))
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) InsertAttachment(ctx context.Context, q db.Querier, requestID int64, uploadedBy *int64, f storedFile, at time.Time) error {
	_, err := q.ExecContext(ctx, `INSERT INTO attachments(request_id, filename, stored_name, mime, size, uploaded_by, created_at) VALUES (?,?,?,?,?,?,?)`,
		requestID, f.Filename, f.StoredName, f.MIME, f.Size, db.IntOrNil(uploadedBy), db.Unix(at))
	if err != nil {
		return fmt.Errorf("insert attachment: %w", err)
	}
	return nil
}

const detailSelect = `SELECT ` + rowCols + `, r.body, r.proposed_solution, r.result_text, r.duplicate_of, dr.number, r.theme_id, t.title, IFNULL(r.anon_token_hash, '')` +
	rowFrom + ` LEFT JOIN requests dr ON dr.id = r.duplicate_of LEFT JOIN themes t ON t.id = r.theme_id`

func scanDetail(sc scanner) (DetailRow, string, error) {
	var d DetailRow
	var prop, result, dupNum, themeTitle sql.NullString
	var dup, theme sql.NullInt64
	var hash string
	row, err := scanRow(sc, &d.Body, &prop, &result, &dup, &dupNum, &theme, &themeTitle, &hash)
	if err != nil {
		return DetailRow{}, "", err
	}
	d.Row = row
	d.ProposedSolution, d.ResultText = db.NullStr(prop), db.NullStr(result)
	d.DuplicateOfID, d.DuplicateOfNumber = db.NullInt(dup), db.NullStr(dupNum)
	d.ThemeID, d.ThemeTitle = db.NullInt(theme), db.NullStr(themeTitle)
	return d, hash, nil
}

func (s *SQLiteStore) Get(ctx context.Context, q db.Querier, id int64) (DetailRow, error) {
	d, _, err := scanDetail(q.QueryRowContext(ctx, detailSelect+` WHERE r.id = ?`, id))
	if db.IsNoRows(err) {
		return DetailRow{}, ErrNotFound
	}
	if err != nil {
		return DetailRow{}, fmt.Errorf("get request: %w", err)
	}
	return d, nil
}

func (s *SQLiteStore) ByTokenHash(ctx context.Context, q db.Querier, hash string) (DetailRow, string, error) {
	d, stored, err := scanDetail(q.QueryRowContext(ctx, detailSelect+` WHERE r.anon_token_hash = ? AND r.is_anonymous = 1`, hash))
	if db.IsNoRows(err) {
		return DetailRow{}, "", ErrNotFound
	}
	if err != nil {
		return DetailRow{}, "", fmt.Errorf("get request by token: %w", err)
	}
	return d, stored, nil
}

func (s *SQLiteStore) Save(ctx context.Context, q db.Querier, r DetailRow) error {
	_, err := q.ExecContext(ctx, `UPDATE requests SET status=?, result_text=?, duplicate_of=?, closed_at=?, first_responded_at=?, paused_at=?,
		first_response_due=?, resolution_due=?, department_id=?, assignee_id=?, routing_source=?, theme_id=?, updated_at=? WHERE id=?`,
		string(r.Status), r.ResultText, db.IntOrNil(r.DuplicateOfID), db.TimeOrNil(r.ClosedAt), db.TimeOrNil(r.FirstRespondedAt),
		db.TimeOrNil(r.PausedAt), db.Unix(r.FirstResponseDue), db.Unix(r.ResolutionDue), db.IntOrNil(r.DepartmentID),
		db.IntOrNil(r.AssigneeID), r.RoutingSource, db.IntOrNil(r.ThemeID), db.Unix(r.UpdatedAt), r.ID)
	if err != nil {
		return fmt.Errorf("save request: %w", err)
	}
	return nil
}

func (s *SQLiteStore) HasEvent(ctx context.Context, q db.Querier, requestID int64, kind, meta string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM request_events WHERE request_id = ? AND kind = ? AND IFNULL(meta,'') = ?`, requestID, kind, meta).Scan(&n)
	return n > 0, err
}

// where builds the WHERE clause; includeTab/includeStatus control tab/status filters (counters ignore them).
func where(f ListFilter, includeTabStatus bool) (string, []any) {
	conds := []string{"(" + f.ScopeSQL + ")"}
	args := append([]any{}, f.ScopeArgs...)
	now := db.Unix(f.Now)
	if includeTabStatus {
		if f.Status != nil {
			conds = append(conds, "r.status = ?")
			args = append(args, string(*f.Status))
		}
		switch f.Tab {
		case "needs_action":
			conds = append(conds, "(r.status = 'new' OR r.status = 'decision' OR "+overdueSQL+")")
			args = append(args, now, now)
		case "in_progress":
			conds = append(conds, "r.status IN ('in_review','decision','in_progress')")
		case "waiting_author":
			conds = append(conds, "r.status = 'needs_info'")
		case "closed":
			conds = append(conds, "r.status IN "+terminalSQL)
		}
	}
	if f.CategoryID != nil {
		conds = append(conds, "r.category_id = ?")
		args = append(args, *f.CategoryID)
	}
	if f.DepartmentID != nil {
		conds = append(conds, "r.department_id = ?")
		args = append(args, *f.DepartmentID)
	}
	if f.AssigneeID != nil {
		conds = append(conds, "r.assignee_id = ?")
		args = append(args, *f.AssigneeID)
	}
	if f.AssigneeNone {
		conds = append(conds, "r.assignee_id IS NULL")
	}
	if f.Priority != "" {
		conds = append(conds, "r.priority = ?")
		args = append(args, f.Priority)
	}
	if f.RequestType != "" {
		conds = append(conds, "r.type = ?")
		args = append(args, f.RequestType)
	}
	if f.Overdue != nil {
		if *f.Overdue {
			conds = append(conds, overdueSQL)
		} else {
			conds = append(conds, "NOT "+overdueSQL)
		}
		args = append(args, now, now)
	}
	if f.CreatedFrom != nil {
		conds = append(conds, "r.created_at >= ?")
		args = append(args, db.Unix(*f.CreatedFrom))
	}
	if f.CreatedTo != nil {
		conds = append(conds, "r.created_at < ?")
		args = append(args, db.Unix(*f.CreatedTo))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		lq := strings.ToLower(q)
		conds = append(conds, "(instr(mv_lower(r.number), ?) > 0 OR instr(mv_lower(r.title), ?) > 0 OR instr(mv_lower(r.body), ?) > 0 OR instr(mv_lower(IFNULL(r.result_text,'')), ?) > 0)")
		args = append(args, lq, lq, lq, lq)
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func orderBy(sort string) string {
	switch sort {
	case "newest":
		return " ORDER BY r.created_at DESC, r.id DESC"
	case "oldest":
		return " ORDER BY r.created_at ASC, r.id ASC"
	case "updated":
		return " ORDER BY r.updated_at DESC, r.id DESC"
	case "priority":
		// high first; within a priority the deadline order below
		return ` ORDER BY CASE r.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, ` + deadlineOrder
	}
	return " ORDER BY " + deadlineOrder
}

// deadlineOrder: open by nearest effective deadline (overdue first), waiting-for-author next, closed last (newest closed first).
const deadlineOrder = `CASE WHEN r.status IN ` + terminalSQL + ` THEN 2 WHEN r.status = 'needs_info' THEN 1 ELSE 0 END,
		CASE WHEN r.status IN ` + terminalSQL + ` THEN -IFNULL(r.closed_at, r.updated_at)
		     WHEN r.first_responded_at IS NULL THEN r.first_response_due ELSE r.resolution_due END ASC, r.id DESC`

func (s *SQLiteStore) List(ctx context.Context, f ListFilter) ([]Row, int, error) {
	w, args := where(f, true)
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM requests r`+w, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count requests: %w", err)
	}
	q := `SELECT ` + rowCols + rowFrom + w + orderBy(f.Sort)
	if f.Limit > 0 {
		q += ` LIMIT ? OFFSET ?`
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan request: %w", err)
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *SQLiteStore) Counts(ctx context.Context, f ListFilter) (Counts, error) {
	w, args := where(f, false)
	now := db.Unix(f.Now)
	q := `SELECT
		IFNULL(SUM(CASE WHEN r.status = 'new' OR r.status = 'decision' OR ` + overdueSQL + ` THEN 1 ELSE 0 END),0),
		IFNULL(SUM(CASE WHEN r.status IN ('in_review','decision','in_progress') THEN 1 ELSE 0 END),0),
		IFNULL(SUM(CASE WHEN r.status = 'needs_info' THEN 1 ELSE 0 END),0),
		IFNULL(SUM(CASE WHEN r.status IN ` + terminalSQL + ` THEN 1 ELSE 0 END),0),
		COUNT(*),
		IFNULL(SUM(CASE WHEN ` + overdueSQL + ` THEN 1 ELSE 0 END),0)
		FROM requests r` + w
	all := append([]any{now, now, now, now}, args...)
	var c Counts
	err := s.DB.QueryRowContext(ctx, q, all...).Scan(&c.NeedsAction, &c.InProgress, &c.WaitingAuthor, &c.Closed, &c.All, &c.Overdue)
	if err != nil {
		return Counts{}, fmt.Errorf("count tabs: %w", err)
	}
	return c, nil
}

func (s *SQLiteStore) Events(ctx context.Context, requestID int64) ([]EventRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT e.id, e.request_id, e.actor_id, e.by_author, IFNULL(u.name,''), IFNULL(u.role,''), e.kind, e.from_status, e.to_status, e.text, e.meta, e.created_at
		FROM request_events e LEFT JOIN users u ON u.id = e.actor_id WHERE e.request_id = ? ORDER BY e.created_at, e.id`, requestID)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		var actor sql.NullInt64
		var byAuthor int
		var role string
		var from, to, text, meta sql.NullString
		var at int64
		if err := rows.Scan(&e.ID, &e.RequestID, &actor, &byAuthor, &e.ActorName, &role, &e.Kind, &from, &to, &text, &meta, &at); err != nil {
			return nil, err
		}
		e.ActorID = db.NullInt(actor)
		e.ByAuthor = byAuthor == 1
		e.ActorRole = authz.Role(role)
		if from.Valid {
			st := Status(from.String)
			e.FromStatus = &st
		}
		if to.Valid {
			st := Status(to.String)
			e.ToStatus = &st
		}
		e.Text, e.Meta = db.NullStr(text), db.NullStr(meta)
		e.At = db.Time(at)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Comments(ctx context.Context, requestID int64) ([]CommentRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id, IFNULL(c.author_id, 0), c.by_author, IFNULL(u.name, ''), IFNULL(u.role, ''), c.kind, c.body, c.created_at
		FROM comments c LEFT JOIN users u ON u.id = c.author_id WHERE c.request_id = ? ORDER BY c.created_at, c.id`, requestID)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()
	var out []CommentRow
	for rows.Next() {
		var c CommentRow
		var role string
		var byAuthor int
		var at int64
		if err := rows.Scan(&c.ID, &c.AuthorID, &byAuthor, &c.Name, &role, &c.Kind, &c.Body, &at); err != nil {
			return nil, err
		}
		c.ByAuthor = byAuthor == 1
		c.Role = authz.Role(role)
		c.CreatedAt = db.Time(at)
		out = append(out, c)
	}
	return out, rows.Err()
}

const attCols = `id, request_id, filename, stored_name, mime, size`

func (s *SQLiteStore) Attachments(ctx context.Context, requestID int64) ([]AttachmentRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+attCols+` FROM attachments WHERE request_id = ? ORDER BY id`, requestID)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer rows.Close()
	out := []AttachmentRow{}
	for rows.Next() {
		var a AttachmentRow
		if err := rows.Scan(&a.ID, &a.RequestID, &a.Filename, &a.StoredName, &a.MIME, &a.Size); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Attachment(ctx context.Context, id int64) (AttachmentRow, error) {
	var a AttachmentRow
	err := s.DB.QueryRowContext(ctx, `SELECT `+attCols+` FROM attachments WHERE id = ?`, id).Scan(&a.ID, &a.RequestID, &a.Filename, &a.StoredName, &a.MIME, &a.Size)
	if db.IsNoRows(err) {
		return AttachmentRow{}, ErrNotFound
	}
	return a, err
}

func (s *SQLiteStore) OverdueCandidates(ctx context.Context, now time.Time) ([]Row, error) {
	n := db.Unix(now)
	rows, err := s.DB.QueryContext(ctx, `SELECT `+rowCols+rowFrom+` WHERE `+overdueSQL+` ORDER BY r.id`, n, n)
	if err != nil {
		return nil, fmt.Errorf("query overdue: %w", err)
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CountInCategory(ctx context.Context, categoryID int64, since time.Time) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM requests WHERE category_id = ? AND created_at >= ?`, categoryID, db.Unix(since)).Scan(&n)
	return n, err
}

func (s *SQLiteStore) ThemeByID(ctx context.Context, q db.Querier, id int64) (Theme, error) {
	var t Theme
	err := q.QueryRowContext(ctx, `SELECT id, title FROM themes WHERE id = ?`, id).Scan(&t.ID, &t.Title)
	if db.IsNoRows(err) {
		return Theme{}, ErrNotFound
	}
	return t, err
}

func (s *SQLiteStore) ThemeByTitle(ctx context.Context, q db.Querier, title string) (Theme, error) {
	var t Theme
	err := q.QueryRowContext(ctx, `SELECT id, title FROM themes WHERE title = ?`, title).Scan(&t.ID, &t.Title)
	if db.IsNoRows(err) {
		return Theme{}, ErrNotFound
	}
	return t, err
}

func (s *SQLiteStore) InsertTheme(ctx context.Context, q db.Querier, title string, by int64, at time.Time) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO themes(title, created_by, created_at) VALUES (?,?,?)`, title, by, db.Unix(at))
	if err != nil {
		return 0, fmt.Errorf("insert theme: %w", err)
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ThemeAccessible(ctx context.Context, q db.Querier, themeID, userID int64, scopeSQL string, scopeArgs []any) (bool, error) {
	args := append([]any{themeID, userID, themeID}, scopeArgs...)
	var n int
	err := q.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM themes WHERE id = ? AND created_by = ?)
		OR EXISTS(SELECT 1 FROM requests r WHERE r.theme_id = ? AND (`+scopeSQL+`))`, args...).Scan(&n)
	return n == 1, err
}
