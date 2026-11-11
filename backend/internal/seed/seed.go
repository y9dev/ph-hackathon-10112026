// Package seed loads the deterministic demo data set of CR1: exactly the 20 requests A-001…A-020
// of docs/DATASET.md (MV-0001 = A-001 … MV-0020 = A-020) with the dataset's categories, conditional
// roles as users, routing rules derived from the conditional responsibles, and added test dates
// (created over ~60 days, first responses, a few overdue requests). A-005, A-017 and A-018 are
// anonymous (CR2): no author link is stored; they are reachable with the demo tokens.
package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/db"
)

// ErrAlreadySeeded is returned when the database already has users.
var ErrAlreadySeeded = errors.New("seed: database is not empty")

// Domain is the e-mail domain of demo accounts.
const Domain = "@modus.demo"

type seeder struct {
	ctx     context.Context
	tx      *sql.Tx
	now     time.Time
	userIDs map[string]int64
	names   map[string]string
	deptIDs []int64
	catIDs  []int64
	adminID int64
}

// Run seeds an empty database. passwordHash is the bcrypt hash of DemoPassword.
func Run(ctx context.Context, d *sql.DB, now time.Time, passwordHash string) error {
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return fmt.Errorf("seed check: %w", err)
	}
	if n > 0 {
		return ErrAlreadySeeded
	}
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		s := &seeder{ctx: ctx, tx: tx, now: now.UTC().Truncate(time.Second), userIDs: map[string]int64{}, names: map[string]string{}}
		if err := s.org(); err != nil {
			return err
		}
		if err := s.people(passwordHash); err != nil {
			return err
		}
		if err := s.config(); err != nil {
			return err
		}
		return s.requests()
	})
	if err == nil {
		slog.Info("demo anonymous tokens (DEMO ONLY, see backend/README.md)",
			"A-005/MV-0005", DemoTokenA005, "A-017/MV-0017", DemoTokenA017, "A-018/MV-0018", DemoTokenA018)
	}
	return err
}

func (s *seeder) exec(q string, args ...any) (int64, error) {
	res, err := s.tx.ExecContext(s.ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("seed exec %.60q: %w", q, err)
	}
	return res.LastInsertId()
}

func (s *seeder) org() error {
	for _, d := range departments {
		id, err := s.exec(`INSERT INTO departments(name) VALUES (?)`, d.name)
		if err != nil {
			return err
		}
		s.deptIDs = append(s.deptIDs, id)
	}
	return nil
}

func (s *seeder) people(hash string) error {
	created := db.Unix(s.now.AddDate(0, 0, -90))
	for _, u := range users {
		id, err := s.exec(`INSERT INTO users(email, name, title, password_hash, role, department_id, active, created_at) VALUES (?,?,?,?,?,?,1,?)`,
			u.login+Domain, fullNames[u.login], u.name, hash, u.role, s.deptIDs[u.dept], created)
		if err != nil {
			return err
		}
		s.userIDs[u.login] = id
		s.names[u.login] = fullNames[u.login]
		if u.role == "admin" && s.adminID == 0 {
			s.adminID = id
		}
	}
	for i, d := range departments {
		if d.head == "" {
			continue
		}
		if _, err := s.exec(`UPDATE departments SET head_user_id = ? WHERE id = ?`, s.userIDs[d.head], s.deptIDs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) config() error {
	for i, c := range categories {
		id, err := s.exec(`INSERT INTO categories(name, description, icon, sort, active) VALUES (?,?,?,?,1)`, c.name, c.desc, c.icon, (i+1)*10)
		if err != nil {
			return err
		}
		s.catIDs = append(s.catIDs, id)
	}
	for _, r := range rules {
		var typ any
		if r.typ != "" {
			typ = r.typ
		}
		if _, err := s.exec(`INSERT INTO routing_rules(category_id, request_type, department_id, assignee_user_id, first_response_hours, resolution_hours, active) VALUES (?,?,?,?,?,?,1)`,
			s.catIDs[r.cat], typ, s.deptIDs[r.dept], s.userIDs[r.assignee], r.fr, r.res); err != nil {
			return err
		}
	}
	return nil
}

// rule mirrors the routing engine: (category, type) first, then category-wide.
func rule(cat int, typ string) (ruleSeed, bool) {
	var wide *ruleSeed
	for i := range rules {
		if rules[i].cat != cat {
			continue
		}
		if rules[i].typ == typ {
			return rules[i], true
		}
		if rules[i].typ == "" {
			wide = &rules[i]
		}
	}
	if wide != nil {
		return *wide, true
	}
	return ruleSeed{}, false
}

// reqState mirrors the mutable request columns while the history is replayed.
type reqState struct {
	id                int64
	number, title     string
	author            *int64 // nil for anonymous requests
	status            string
	frd, rd           time.Time
	firstResp, paused *time.Time
	closed            *time.Time
	result            *string
	dept, assignee    *int64
	source            string
	updated           time.Time
	staff             int64 // the responsible acting on the request
	created           time.Time
}

func (s *seeder) requests() error {
	for i, t := range requestSeeds {
		if err := s.request(i+1, t); err != nil {
			return fmt.Errorf("%s: %w", t.code, err)
		}
	}
	_, err := s.exec(`UPDATE counters SET value = ? WHERE name = 'request'`, len(requestSeeds))
	return err
}

func (s *seeder) at(st *reqState, h float64) time.Time {
	t := st.created.Add(time.Duration(h * float64(time.Hour)))
	if limit := s.now.Add(-10 * time.Minute); t.After(limit) {
		t = limit
	}
	return t
}

func (s *seeder) request(seq int, t reqSeed) error {
	created := s.now.Add(-time.Duration(t.ageH * float64(time.Hour)))
	st := &reqState{number: fmt.Sprintf("MV-%04d", seq), title: t.title, status: "new", created: created, updated: created}
	if !t.anon {
		id := s.userIDs[t.author]
		st.author = &id
	}
	frH, resH := requests.UnroutedFirstResponseHours, requests.UnroutedResolutionHours
	routedText := "Подходящего правила маршрутизации нет — обращение передано администратору"
	st.source, st.staff = "unrouted", s.adminID
	if r, ok := rule(t.cat, t.typ); ok {
		frH, resH = r.fr, r.res
		dept, asg := s.deptIDs[r.dept], s.userIDs[r.assignee]
		st.dept, st.assignee, st.source, st.staff = &dept, &asg, "rule", asg
		routedText = fmt.Sprintf("Передано: %s, ответственный: %s. Срок первого ответа — %d ч", departments[r.dept].name, s.names[r.assignee], frH)
	}
	st.frd, st.rd = created.Add(time.Duration(frH)*time.Hour), created.Add(time.Duration(resH)*time.Hour)

	var tokenHash any
	if t.anon {
		tokenHash = requests.HashAnonymousToken(t.token)
	}
	id, err := s.exec(`INSERT INTO requests(number, type, priority, category_id, title, body, author_id, is_anonymous, anon_token_hash,
		department_id, assignee_id, status, routing_source, first_response_due, resolution_due, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,'new',?,?,?,?,?)`,
		st.number, t.typ, t.prio, s.catIDs[t.cat], t.title, t.body, db.IntOrNil(st.author), b2i(t.anon), tokenHash,
		db.IntOrNil(st.dept), db.IntOrNil(st.assignee), st.source, db.Unix(st.frd), db.Unix(st.rd), db.Unix(created), db.Unix(created))
	if err != nil {
		return err
	}
	st.id = id
	if err := s.event(st.id, st.author, true, "created", "", "new", nil, nil, created); err != nil {
		return err
	}
	if err := s.event(st.id, nil, false, "routed", "", "", &routedText, &st.source, created); err != nil {
		return err
	}
	title := "Новое обращение " + st.number
	if st.source == "unrouted" {
		title = "Нераспределённое обращение " + st.number
	}
	if err := s.notify(st.staff, "created", st.id, title, t.title, created); err != nil {
		return err
	}
	if err := s.history(st, t); err != nil {
		return err
	}
	_, err = s.exec(`UPDATE requests SET status=?, first_response_due=?, resolution_due=?, first_responded_at=?, paused_at=?, closed_at=?,
		result_text=?, department_id=?, assignee_id=?, routing_source=?, updated_at=? WHERE id=?`,
		st.status, db.Unix(st.frd), db.Unix(st.rd), db.TimeOrNil(st.firstResp), db.TimeOrNil(st.paused), db.TimeOrNil(st.closed),
		st.result, db.IntOrNil(st.dept), db.IntOrNil(st.assignee), st.source, db.Unix(st.updated), st.id)
	return err
}

func (s *seeder) firstResponse(st *reqState, at time.Time) {
	if st.firstResp == nil {
		st.firstResp = &at
	}
}

// history replays the dataset status with test dates.
func (s *seeder) history(st *reqState, t reqSeed) error {
	if t.source() == "unrouted" {
		// A-019: no rule for «Хранение документов» -> admin queue -> the admin assigns the dataset's
		// conditional responsible manually (dataset scenario: alternative when nobody is assigned).
		at := s.at(st, t.frH)
		dept, asg := s.deptIDs[dAdminHR], s.userIDs[t.responsible]
		st.dept, st.assignee, st.source = &dept, &asg, "manual"
		text := fmt.Sprintf("Отдел: %s; ответственный: %s", departments[dAdminHR].name, s.names[t.responsible])
		admin := s.adminID
		if err := s.event(st.id, &admin, false, "assigned", "", "", &text, ptr("manual"), at); err != nil {
			return err
		}
		if err := s.notify(asg, "assigned", st.id, "Вам передано обращение "+st.number, t.title, at); err != nil {
			return err
		}
		s.firstResponse(st, at)
		st.updated = at
		st.staff = asg
		t.frH += 1 // the responsible starts the review an hour later
	}
	if t.note != "" && t.question == "" {
		// «Пример дальнейшего действия» -> internal note by the responsible (never shown to the author)
		h := t.frH + 0.5
		if t.frH == 0 {
			h = min(2, t.ageH/2)
		}
		if err := s.comment(st, st.staff, "internal", t.note, s.at(st, h)); err != nil {
			return err
		}
	}
	if t.status == "new" {
		return nil
	}
	first := s.at(st, t.frH)
	if t.status == "needs_info" {
		// «Ожидает уточнения» -> staff question; the SLA clock is paused
		if err := s.comment(st, st.staff, "question", t.question, first); err != nil {
			return err
		}
		s.firstResponse(st, first)
		if err := s.setStatus(st, "needs_info", nil, first); err != nil {
			return err
		}
		return s.notifyAuthor(st, "question", st.number+": нужен ваш ответ", t.question, first)
	}
	s.firstResponse(st, first)
	if err := s.setStatus(st, "in_review", nil, first); err != nil {
		return err
	}
	switch t.status {
	case "in_review":
		return nil
	case "in_progress", "implemented":
		end := t.ageH
		if t.status == "implemented" {
			end = t.closeH
		}
		span := end - t.frH
		if err := s.setStatus(st, "decision", nil, s.at(st, t.frH+span*0.3)); err != nil {
			return err
		}
		if err := s.setStatus(st, "in_progress", nil, s.at(st, t.frH+span*0.5)); err != nil {
			return err
		}
		if t.status == "in_progress" {
			return nil
		}
		return s.closeWith(st, "implemented", t.result, s.at(st, t.closeH))
	case "declined", "closed":
		return s.closeWith(st, t.status, t.result, s.at(st, t.closeH))
	}
	return fmt.Errorf("seed: unknown status %q", t.status)
}

func (t reqSeed) source() string {
	if _, ok := rule(t.cat, t.typ); ok {
		return "rule"
	}
	return "unrouted"
}

var statusLabel = map[string]string{"new": "Новое", "in_review": "В проработке", "needs_info": "Нужно уточнение", "decision": "Принятие решения",
	"in_progress": "Реализация", "implemented": "Реализовано", "declined": "Отклонено", "duplicate": "Дубликат", "closed": "Закрыто"}

func (s *seeder) setStatus(st *reqState, to string, text *string, at time.Time) error {
	from := st.status
	if from == "needs_info" && st.paused != nil {
		d := at.Sub(*st.paused)
		st.frd, st.rd = st.frd.Add(d), st.rd.Add(d)
		st.paused = nil
	}
	if to == "needs_info" {
		st.paused = &at
	}
	st.status = to
	if to == "implemented" || to == "declined" || to == "duplicate" || to == "closed" {
		st.closed = &at
	}
	st.updated = at
	staff := st.staff
	if err := s.event(st.id, &staff, false, "status_changed", from, to, text, nil, at); err != nil {
		return err
	}
	body := st.title
	if text != nil {
		body = *text
	}
	return s.notifyAuthor(st, "status_changed", fmt.Sprintf("%s: статус «%s»", st.number, statusLabel[to]), body, at)
}

func (s *seeder) closeWith(st *reqState, status, result string, at time.Time) error {
	if strings.TrimSpace(result) == "" {
		return fmt.Errorf("seed: %s needs a result text", status)
	}
	st.result = &result
	return s.setStatus(st, status, &result, at)
}

func (s *seeder) comment(st *reqState, author int64, kind, body string, at time.Time) error {
	if at.After(st.updated) {
		st.updated = at
	}
	_, err := s.exec(`INSERT INTO comments(request_id, author_id, by_author, kind, body, created_at) VALUES (?,?,0,?,?,?)`, st.id, author, kind, body, db.Unix(at))
	return err
}

func (s *seeder) event(reqID int64, actor *int64, byAuthor bool, kind, from, to string, text, meta *string, at time.Time) error {
	_, err := s.exec(`INSERT INTO request_events(request_id, actor_id, by_author, kind, from_status, to_status, text, meta, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		reqID, db.IntOrNil(actor), b2i(byAuthor), kind, nullStr(from), nullStr(to), text, meta, db.Unix(at))
	return err
}

// notifyAuthor writes an in-app notification for a NAMED author; anonymous authors get none (CR2).
func (s *seeder) notifyAuthor(st *reqState, kind, title, body string, at time.Time) error {
	if st.author == nil {
		return nil
	}
	return s.notify(*st.author, kind, st.id, title, body, at)
}

func (s *seeder) notify(userID int64, kind string, reqID int64, title, body string, at time.Time) error {
	var readAt any
	if s.now.Sub(at) > 72*time.Hour {
		readAt = db.Unix(at.Add(2 * time.Hour))
	}
	_, err := s.exec(`INSERT INTO notifications(user_id, kind, request_id, title, body, read_at, created_at) VALUES (?,?,?,?,?,?,?)`,
		userID, kind, reqID, title, body, readAt, db.Unix(at))
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func ptr[T any](v T) *T { return &v }
