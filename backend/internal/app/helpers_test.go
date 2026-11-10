package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/authz"
)

const testPassword = "Passw0rd!"

// testClock is a mutable clock shared by all services of a test app.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fixture ids
type fx struct {
	deptA, deptB, deptMgmt                 int64
	admin, mgrA, mgrB, emp1, emp2, mgrNone int64
	catA, catB, catOther                   int64
	ruleA                                  int64
}

type testEnv struct {
	t   *testing.T
	app *App
	clk *testClock
	srv *httptest.Server
	fx  fx
}

func newEnv(t *testing.T) *testEnv { return newEnvCfg(t, nil) }

// newEnvCfg builds a test app; tweak may adjust the config (rate limits are disabled by default).
func newEnvCfg(t *testing.T, tweak func(*Config)) *testEnv {
	t.Helper()
	clk := &testClock{t: time.Date(2026, 11, 10, 6, 0, 0, 0, time.UTC)}
	cfg := Config{DataDir: t.TempDir(), BcryptCost: bcrypt.MinCost, DisableTicker: true, DisplayTZ: "UTC", Now: clk.Now, DemoMode: true, CreatePerHour: -1, CommentPerHour: -1}
	if tweak != nil {
		tweak(&cfg)
	}
	a, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	e := &testEnv{t: t, app: a, clk: clk}
	e.fixture()
	e.srv = httptest.NewServer(a.Handler)
	t.Cleanup(func() { e.srv.Close(); _ = a.Close() })
	return e
}

func (e *testEnv) exec(q string, args ...any) int64 {
	e.t.Helper()
	res, err := e.app.DB.Exec(q, args...)
	if err != nil {
		e.t.Fatalf("fixture %q: %v", q, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func (e *testEnv) fixture() {
	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	now := e.clk.Now().Unix()
	f := &e.fx
	f.deptA = e.exec(`INSERT INTO departments(name) VALUES ('Отдел А')`)
	f.deptB = e.exec(`INSERT INTO departments(name) VALUES ('Отдел Б')`)
	f.deptMgmt = e.exec(`INSERT INTO departments(name) VALUES ('Руководство')`)
	user := func(email, name, role string, dept int64) int64 {
		return e.exec(`INSERT INTO users(email, name, password_hash, role, department_id, created_at) VALUES (?,?,?,?,?,?)`, email, name, string(hash), role, dept, now)
	}
	f.admin = user("admin@test.kz", "Админ Админов", "admin", f.deptMgmt)
	f.mgrA = user("mgra@test.kz", "Руководитель Альфа", "manager", f.deptA)
	f.mgrB = user("mgrb@test.kz", "Руководитель Бета", "manager", f.deptB)
	f.emp1 = user("emp1@test.kz", "Сотрудник Первый", "employee", f.deptA)
	f.emp2 = user("emp2@test.kz", "Сотрудник Второй", "employee", f.deptB)
	f.mgrNone = user("mgrnone@test.kz", "Руководитель Без Отдела", "manager", f.deptB)
	e.exec(`UPDATE departments SET head_user_id = ? WHERE id = ?`, f.mgrA, f.deptA)
	e.exec(`UPDATE departments SET head_user_id = ? WHERE id = ?`, f.mgrB, f.deptB)
	e.exec(`UPDATE departments SET head_user_id = ? WHERE id = ?`, f.admin, f.deptMgmt)
	f.catA = e.exec(`INSERT INTO categories(name, description, icon, sort) VALUES ('Категория А', 'а', 'wrench', 1)`)
	f.catB = e.exec(`INSERT INTO categories(name, description, icon, sort) VALUES ('Категория Б', 'б', 'package', 2)`)
	f.catOther = e.exec(`INSERT INTO categories(name, description, icon, sort) VALUES ('Другое', 'прочее', 'circle-help', 3)`)
	f.ruleA = e.exec(`INSERT INTO routing_rules(category_id, department_id, assignee_user_id, first_response_hours, resolution_hours) VALUES (?,?,?,24,120)`, f.catA, f.deptA, f.mgrA)
	e.exec(`INSERT INTO routing_rules(category_id, department_id, first_response_hours, resolution_hours) VALUES (?,?,48,240)`, f.catB, f.deptB) // no assignee -> head
}

// actor builds the Actor exactly as the auth middleware would.
func (e *testEnv) actor(id int64) authz.Actor {
	e.t.Helper()
	p, err := e.app.Identity.Service.Person(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	var led []int64
	rows, _ := e.app.DB.Query(`SELECT id FROM departments WHERE head_user_id = ?`, id)
	for rows.Next() {
		var d int64
		_ = rows.Scan(&d)
		led = append(led, d)
	}
	_ = rows.Close()
	return authz.Actor{UserID: p.ID, Name: p.Name, Role: p.Role, DepartmentID: p.DepartmentID, LedDepartments: led}
}

func (e *testEnv) create(author int64, cat int64, anon bool) requests.Detail {
	e.t.Helper()
	d, err := e.app.Requests.Service.Create(context.Background(), e.actor(author), requests.CreateInput{
		Type: requests.TypeProblem, CategoryID: cat, Title: "Тестовое обращение", Body: "Подробное описание тестовой проблемы для проверки.", IsAnonymous: anon,
	}, nil)
	if err != nil {
		e.t.Fatalf("create: %v", err)
	}
	return d
}

// --- HTTP client ---

type client struct {
	e  *testEnv
	hc *http.Client
}

func (e *testEnv) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, hc: &http.Client{Jar: jar}}
}

func (e *testEnv) login(email string) *client {
	e.t.Helper()
	c := e.anon()
	resp, body := c.json("POST", "/api/v1/auth/login", map[string]string{"email": email, "password": testPassword})
	if resp.StatusCode != 200 {
		e.t.Fatalf("login %s: %d %s", email, resp.StatusCode, body)
	}
	return c
}

func (c *client) do(req *http.Request) (*http.Response, []byte) {
	c.e.t.Helper()
	resp, err := c.hc.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func (c *client) json(method, path string, body any) (*http.Response, []byte) {
	c.e.t.Helper()
	var r io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		r = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}

func (c *client) get(path string) (*http.Response, []byte) { return c.json("GET", path, nil) }

type file struct {
	name string
	data []byte
}

func (c *client) multipart(payload any, files ...file) (*http.Response, []byte) {
	c.e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	p, _ := json.Marshal(payload)
	_ = w.WriteField("payload", string(p))
	for _, f := range files {
		fw, _ := w.CreateFormFile("files", f.name)
		_, _ = fw.Write(f.data)
	}
	_ = w.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/api/v1/requests", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return c.do(req)
}

func decode[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return v
}

func expect(t *testing.T, resp *http.Response, body []byte, status int) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("%s %s: status %d want %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, status, body)
	}
}
