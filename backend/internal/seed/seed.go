// Package seed loads deterministic synthetic demo data: 7 departments, 21 users, 9 categories,
// routing rules (none for "Другое"), ~70 requests over the last 90 days with full timelines,
// 4 deliberate similarity clusters, one confirmed theme and notifications.
package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"modusvoice/internal/platform/db"
)

// ErrAlreadySeeded is returned when the database already has users.
var ErrAlreadySeeded = errors.New("seed: database is not empty")

// Domain is the e-mail domain of demo accounts.
const Domain = "@modus.demo"

type seeder struct {
	ctx      context.Context
	tx       *sql.Tx
	now      time.Time
	rnd      *rand.Rand
	userIDs  map[string]int64
	userDept map[string]int
	deptIDs  []int64
	catIDs   []int64
	rules    map[[2]string]ruleSeed // (cat index, type) -> rule
	adminID  int64
	keyIDs   map[string]int64
	keyNums  map[string]string
	rotation map[int]int
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
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		s := &seeder{ctx: ctx, tx: tx, now: now.UTC().Truncate(time.Second), rnd: rand.New(rand.NewPCG(42, 2026)),
			userIDs: map[string]int64{}, userDept: map[string]int{}, rules: map[[2]string]ruleSeed{},
			keyIDs: map[string]int64{}, keyNums: map[string]string{}, rotation: map[int]int{}}
		for _, step := range []func(string) error{s.org, s.people, s.config} {
			if err := step(passwordHash); err != nil {
				return err
			}
		}
		return s.requests()
	})
}

func (s *seeder) exec(q string, args ...any) (int64, error) {
	res, err := s.tx.ExecContext(s.ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("seed exec %.60q: %w", q, err)
	}
	return res.LastInsertId()
}

func (s *seeder) org(string) error {
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
	created := db.Unix(s.now.AddDate(0, 0, -120))
	for _, u := range users {
		id, err := s.exec(`INSERT INTO users(email, name, title, password_hash, role, department_id, active, created_at) VALUES (?,?,?,?,?,?,1,?)`,
			u.login+Domain, u.name, u.title, hash, u.role, s.deptIDs[u.dept], created)
		if err != nil {
			return err
		}
		s.userIDs[u.login] = id
		s.userDept[u.login] = u.dept
		if u.role == "admin" && s.adminID == 0 {
			s.adminID = id
		}
	}
	for i, d := range departments {
		if _, err := s.exec(`UPDATE departments SET head_user_id = ? WHERE id = ?`, s.userIDs[d.head], s.deptIDs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) config(string) error {
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
		head := departments[r.dept].head
		if _, err := s.exec(`INSERT INTO routing_rules(category_id, request_type, department_id, assignee_user_id, first_response_hours, resolution_hours, active) VALUES (?,?,?,?,?,?,1)`,
			s.catIDs[r.cat], typ, s.deptIDs[r.dept], s.userIDs[head], r.fr, r.res); err != nil {
			return err
		}
		s.rules[[2]string{fmt.Sprint(r.cat), r.typ}] = r
	}
	return nil
}

func (s *seeder) rule(cat int, typ string) (ruleSeed, bool) {
	if r, ok := s.rules[[2]string{fmt.Sprint(cat), typ}]; ok {
		return r, true
	}
	r, ok := s.rules[[2]string{fmt.Sprint(cat), ""}]
	return r, ok
}

// fallback authors per category when a template has none
var rotationAuthors = []string{"i.sidorov", "a.petrova", "a.fedorov", "t.zhukov", "n.morozova", "p.sokolov", "r.akhmetov"}

func (s *seeder) author(t reqSeed) string {
	if t.au != "" {
		return t.au
	}
	i := s.rotation[t.cat]
	s.rotation[t.cat]++
	return rotationAuthors[(t.cat+i)%len(rotationAuthors)]
}

func pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }

// reqState mirrors the mutable request columns while the history is simulated.
type reqState struct {
	id, authorID, actorID int64
	number, title         string
	status                string
	frd, rd               time.Time
	firstResp, paused     *time.Time
	closed                *time.Time
	result                *string
	dupOf                 *int64
	cur                   time.Time
}

func (s *seeder) requests() error {
	tpls := append([]reqSeed{}, requestSeeds...)
	sort.SliceStable(tpls, func(i, j int) bool { return tpls[i].d > tpls[j].d }) // oldest first => MV-0001 is oldest
	for n, t := range tpls {
		if err := s.request(n+1, t); err != nil {
			return err
		}
	}
	if _, err := s.exec(`UPDATE counters SET value = ? WHERE name = 'request'`, len(tpls)); err != nil {
		return err
	}
	return s.theme()
}

func (s *seeder) request(seq int, t reqSeed) error {
	login := s.author(t)
	authorID := s.userIDs[login]
	created := s.now.Add(-time.Duration(t.d*24)*time.Hour - time.Duration(1+s.rnd.IntN(7))*time.Hour - time.Duration(s.rnd.IntN(60))*time.Minute)
	rule, routed := s.rule(t.cat, t.typ)
	frH, resH := 24, 168
	var dept, assignee any
	staff := s.adminID
	source := "unrouted"
	routedText := "Подходящего правила маршрутизации нет — обращение передано администратору"
	if routed {
		frH, resH = rule.fr, rule.res
		head := departments[rule.dept].head
		dept, assignee, staff, source = s.deptIDs[rule.dept], s.userIDs[head], s.userIDs[head], "rule"
		name := ""
		for _, u := range users {
			if u.login == head {
				name = u.name
			}
		}
		routedText = fmt.Sprintf("Передано: %s, ответственный: %s. Срок первого ответа — %d ч", departments[rule.dept].name, name, frH)
	}
	st := &reqState{authorID: authorID, actorID: staff, number: fmt.Sprintf("MV-%04d", seq), title: t.title, status: "new",
		frd: created.Add(time.Duration(frH) * time.Hour), rd: created.Add(time.Duration(resH) * time.Hour), cur: created}
	var sol any
	if t.sol != "" {
		sol = t.sol
	}
	id, err := s.exec(`INSERT INTO requests(number, type, category_id, title, body, proposed_solution, author_id, is_anonymous, department_id, assignee_id,
		status, routing_source, first_response_due, resolution_due, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,'new',?,?,?,?,?)`,
		st.number, t.typ, s.catIDs[t.cat], t.title, t.body, sol, authorID, b2i(t.anon), dept, assignee, source,
		db.Unix(st.frd), db.Unix(st.rd), db.Unix(created), db.Unix(created))
	if err != nil {
		return err
	}
	st.id = id
	if t.key != "" {
		s.keyIDs[t.key], s.keyNums[t.key] = id, st.number
	}
	if err := s.event(st.id, &authorID, "created", "", "new", nil, nil, created); err != nil {
		return err
	}
	if err := s.event(st.id, nil, "routed", "", "", &routedText, &source, created); err != nil {
		return err
	}
	title := "Новое обращение " + st.number
	if !routed {
		title = "Нераспределённое обращение " + st.number
	}
	if err := s.notify(staff, "created", st.id, title, t.title, created); err != nil {
		return err
	}
	if err := s.history(st, t, frH, seq); err != nil {
		return err
	}
	_, err = s.exec(`UPDATE requests SET status=?, first_response_due=?, resolution_due=?, first_responded_at=?, paused_at=?, closed_at=?,
		result_text=?, duplicate_of=?, updated_at=? WHERE id=?`,
		st.status, db.Unix(st.frd), db.Unix(st.rd), db.TimeOrNil(st.firstResp), db.TimeOrNil(st.paused), db.TimeOrNil(st.closed),
		st.result, db.IntOrNil(st.dupOf), db.Unix(st.cur), st.id)
	return err
}

// advance moves the simulated clock forward by h hours, never past "now - 20 min".
func (s *seeder) advance(st *reqState, h float64) time.Time {
	st.cur = st.cur.Add(time.Duration(h * float64(time.Hour)))
	if limit := s.now.Add(-20 * time.Minute); st.cur.After(limit) {
		st.cur = limit
	}
	return st.cur
}

func (s *seeder) staffAction(st *reqState) {
	if st.firstResp == nil {
		t := st.cur
		st.firstResp = &t
	}
}

func (s *seeder) setStatus(st *reqState, actor int64, to string, text *string) error {
	from := st.status
	if from == "needs_info" && st.paused != nil {
		d := st.cur.Sub(*st.paused)
		st.frd, st.rd = st.frd.Add(d), st.rd.Add(d)
		st.paused = nil
	}
	if to == "needs_info" {
		t := st.cur
		st.paused = &t
	}
	st.status = to
	if to == "implemented" || to == "declined" || to == "duplicate" || to == "closed" {
		t := st.cur
		st.closed = &t
	}
	if err := s.event(st.id, &actor, "status_changed", from, to, text, nil, st.cur); err != nil {
		return err
	}
	if actor == st.authorID {
		return nil
	}
	body := st.title
	if text != nil {
		body = *text
	}
	return s.notify(st.authorID, "status_changed", st.id, fmt.Sprintf("%s: статус «%s»", st.number, statusLabel[to]), body, st.cur)
}

var statusLabel = map[string]string{"new": "Новое", "in_review": "В проработке", "needs_info": "Нужно уточнение", "decision": "Принятие решения",
	"in_progress": "Реализация", "implemented": "Реализовано", "declined": "Отклонено", "duplicate": "Дубликат", "closed": "Закрыто"}

func (s *seeder) comment(st *reqState, author int64, kind, body string) error {
	_, err := s.exec(`INSERT INTO comments(request_id, author_id, kind, body, created_at) VALUES (?,?,?,?,?)`, st.id, author, kind, body, db.Unix(st.cur))
	return err
}

// firstResponseDelay: mostly within SLA, sometimes late (realistic median/p90).
func (s *seeder) firstResponseDelay(frH int) float64 {
	f := 0.1 + s.rnd.Float64()*0.9
	if s.rnd.IntN(5) == 0 {
		f = 1.05 + s.rnd.Float64()*0.6
	}
	return float64(frH) * f
}

func (s *seeder) history(st *reqState, t reqSeed, frH, seq int) error {
	staff := st.actorID
	ask := func() error {
		s.advance(st, 2+s.rnd.Float64()*20)
		s.staffAction(st)
		q := pick(s.rnd, questions)
		if err := s.comment(st, staff, "question", q); err != nil {
			return err
		}
		if err := s.setStatus(st, staff, "needs_info", nil); err != nil {
			return err
		}
		return s.notify(st.authorID, "question", st.id, st.number+": нужен ваш ответ", q, st.cur)
	}
	answer := func() error {
		s.advance(st, 2+s.rnd.Float64()*30)
		a := pick(s.rnd, answers)
		if err := s.comment(st, st.authorID, "answer", a); err != nil {
			return err
		}
		if err := s.setStatus(st, st.authorID, "in_review", nil); err != nil {
			return err
		}
		return s.notify(staff, "answer", st.id, st.number+": ответ на уточнение", a, st.cur)
	}
	review := func() error {
		s.advance(st, s.firstResponseDelay(frH))
		s.staffAction(st)
		if err := s.setStatus(st, staff, "in_review", nil); err != nil {
			return err
		}
		if s.rnd.IntN(2) == 0 {
			s.advance(st, 0.2)
			c := pick(s.rnd, staffComments)
			if err := s.comment(st, staff, "comment", c); err != nil {
				return err
			}
			if err := s.notify(st.authorID, "comment", st.id, st.number+": новый комментарий", c, st.cur); err != nil {
				return err
			}
		}
		if s.rnd.IntN(4) == 0 {
			s.advance(st, 0.5)
			return s.comment(st, staff, "internal", pick(s.rnd, internalNotes))
		}
		return nil
	}
	result := func(def string) *string {
		r := t.res
		if r == "" {
			r = def
		}
		return &r
	}
	switch t.st {
	case "new":
		return nil
	case "in_review":
		return review()
	case "needs_info":
		s.advance(st, s.firstResponseDelay(frH)*0.6)
		return ask()
	case "decision", "in_progress", "implemented":
		if err := review(); err != nil {
			return err
		}
		if seq%3 == 0 {
			if err := ask(); err != nil {
				return err
			}
			if err := answer(); err != nil {
				return err
			}
		}
		s.advance(st, 8+s.rnd.Float64()*60)
		if err := s.setStatus(st, staff, "decision", ptr(pick(s.rnd, decisionTexts))); err != nil {
			return err
		}
		if t.st == "decision" {
			return nil
		}
		s.advance(st, 6+s.rnd.Float64()*40)
		if err := s.setStatus(st, staff, "in_progress", ptr(pick(s.rnd, inProgressTexts))); err != nil {
			return err
		}
		reports := 1 + (seq+len(t.title))%3
		for i := 0; i < reports; i++ {
			s.advance(st, 12+s.rnd.Float64()*48)
			txt := progressReports[(seq*3+i*5)%len(progressReports)]
			if err := s.comment(st, staff, "report", txt); err != nil {
				return err
			}
			if err := s.notify(st.authorID, "report", st.id, st.number+": отчёт о ходе работ", txt, st.cur); err != nil {
				return err
			}
		}
		if t.st == "implemented" {
			s.advance(st, 72+s.rnd.Float64()*400)
			return s.closeWith(st, staff, "implemented", result("Реализовано: изменения внедрены, ответственные проинформированы."))
		}
		return nil
	case "declined":
		if err := review(); err != nil {
			return err
		}
		s.advance(st, 10+s.rnd.Float64()*70)
		return s.closeWith(st, staff, "declined", result("Отклонено после рассмотрения: предложенное решение сейчас нецелесообразно."))
	case "closed":
		if err := review(); err != nil {
			return err
		}
		s.advance(st, 6+s.rnd.Float64()*48)
		return s.closeWith(st, staff, "closed", result("Закрыто: вопрос решён в рабочем порядке, дополнительных действий не требуется."))
	case "duplicate":
		s.advance(st, s.firstResponseDelay(frH))
		s.staffAction(st)
		orig, ok := s.keyIDs[t.dup]
		if !ok {
			return fmt.Errorf("seed: unknown duplicate original %q", t.dup)
		}
		st.dupOf = &orig
		return s.closeWith(st, staff, "duplicate", ptr(fmt.Sprintf("Объединено с обращением %s — ход решения смотрите там.", s.keyNums[t.dup])))
	}
	return fmt.Errorf("seed: unknown status %q", t.st)
}

func (s *seeder) closeWith(st *reqState, staff int64, status string, text *string) error {
	st.result = text
	return s.setStatus(st, staff, status, text)
}

func (s *seeder) theme() error {
	at := s.now.AddDate(0, 0, -20)
	tid, err := s.exec(`INSERT INTO themes(title, created_by, created_at) VALUES (?,?,?)`, ThemeTitle, s.userIDs["e.kuznetsova"], db.Unix(at))
	if err != nil {
		return err
	}
	text := "Тема: " + ThemeTitle
	for _, k := range themeMembers {
		id := s.keyIDs[k]
		if _, err := s.exec(`UPDATE requests SET theme_id = ? WHERE id = ?`, tid, id); err != nil {
			return err
		}
		actor := s.userIDs["e.kuznetsova"]
		if err := s.event(id, &actor, "theme_linked", "", "", &text, nil, at); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) event(reqID int64, actor *int64, kind, from, to string, text, meta *string, at time.Time) error {
	_, err := s.exec(`INSERT INTO request_events(request_id, actor_id, kind, from_status, to_status, text, meta, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		reqID, db.IntOrNil(actor), kind, nullStr(from), nullStr(to), text, meta, db.Unix(at))
	return err
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
