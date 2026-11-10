package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
)

// Config tunes the identity service.
type Config struct {
	BcryptCost     int
	DemoMode       bool
	DemoEmails     []string
	DemoPassword   string
	AccountMaxFail int           // failed logins per (account, client IP) pair per window (default 5)
	IPMaxFail      int           // failed logins per IP per window (default 20)
	FailWindow     time.Duration // default 15m
	TrustedProxy   bool          // read the client IP from X-Forwarded-For / X-Real-IP (only behind a trusted reverse proxy)
}

// Service implements authentication, sessions and admin user management.
type Service struct {
	store     Store
	cfg       Config
	now       func() time.Time
	byAccount *Throttle
	byIP      *Throttle
	dummyHash []byte
}

func NewService(store Store, cfg Config, now func() time.Time) *Service {
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = bcrypt.DefaultCost
	}
	if cfg.AccountMaxFail == 0 {
		cfg.AccountMaxFail = 5
	}
	if cfg.IPMaxFail == 0 {
		cfg.IPMaxFail = 20
	}
	if cfg.FailWindow == 0 {
		cfg.FailWindow = 15 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	dummy, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), cfg.BcryptCost)
	return &Service{store: store, cfg: cfg, now: now,
		byAccount: NewThrottle(cfg.AccountMaxFail, cfg.FailWindow),
		byIP:      NewThrottle(cfg.IPMaxFail, cfg.FailWindow),
		dummyHash: dummy}
}

// HashPassword hashes p with the configured bcrypt cost.
func (s *Service) HashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), s.cfg.BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

var errBadCreds = apperr.Unauthorized("Неверный email или пароль")

// Login verifies credentials and creates a session. Returns the session token (only its hash is stored).
func (s *Service) Login(ctx context.Context, email, password, ip string) (UserView, string, error) {
	now := s.now()
	email = authz.NormalizeEmail(email)
	// keyed by (account, IP): a third party cannot lock a user out from their own IP
	acctKey := "a:" + email + "|" + ip
	ipKey := "i:" + ip
	if s.byAccount.Blocked(acctKey, now) || s.byIP.Blocked(ipKey, now) {
		return UserView{}, "", apperr.TooMany("Слишком много неудачных попыток входа. Попробуйте через 15 минут.")
	}
	if email == "" || password == "" || len(password) > 128 || len(email) > 254 {
		s.byIP.Fail(ipKey, now)
		return UserView{}, "", errBadCreds
	}
	u, err := s.store.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return UserView{}, "", err
	}
	if errors.Is(err, ErrNotFound) || !u.Active {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password)) // equalize timing
		s.byAccount.Fail(acctKey, now)
		s.byIP.Fail(ipKey, now)
		return UserView{}, "", errBadCreds
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.byAccount.Fail(acctKey, now)
		s.byIP.Fail(ipKey, now)
		return UserView{}, "", errBadCreds
	}
	s.byAccount.Reset(acctKey)

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return UserView{}, "", fmt.Errorf("session token: %w", err)
	}
	tok := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.store.CreateSession(ctx, hashToken(tok), u.ID, now.Add(SessionTTL), now); err != nil {
		return UserView{}, "", err
	}
	_ = s.store.DeleteExpiredSessions(ctx, now)
	view, err := s.view(ctx, u)
	return view, tok, err
}

// Logout deletes the session for token (no-op if unknown).
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(token))
}

// Authenticate resolves a session token to an Actor.
func (s *Service) Authenticate(ctx context.Context, token string) (authz.Actor, error) {
	if token == "" || len(token) > 200 {
		return authz.Actor{}, apperr.Unauthorized("")
	}
	uid, err := s.store.SessionUserID(ctx, hashToken(token), s.now())
	if errors.Is(err, ErrNotFound) {
		return authz.Actor{}, apperr.Unauthorized("")
	}
	if err != nil {
		return authz.Actor{}, err
	}
	u, err := s.store.UserByID(ctx, uid)
	if errors.Is(err, ErrNotFound) || (err == nil && !u.Active) {
		return authz.Actor{}, apperr.Unauthorized("")
	}
	if err != nil {
		return authz.Actor{}, err
	}
	led, err := s.store.LedDepartments(ctx, u.ID)
	if err != nil {
		return authz.Actor{}, err
	}
	return authz.Actor{UserID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, DepartmentID: u.DepartmentID, LedDepartments: led}, nil
}

func (s *Service) view(ctx context.Context, u User) (UserView, error) {
	led, err := s.store.LedDepartments(ctx, u.ID)
	if err != nil {
		return UserView{}, err
	}
	return UserView{ID: u.ID, Name: u.Name, Email: u.Email, Role: string(u.Role), DepartmentID: u.DepartmentID,
		DepartmentName: u.DepartmentName, LedDepartmentIDs: led}, nil
}

// Me returns the current user's view.
func (s *Service) Me(ctx context.Context, a authz.Actor) (UserView, error) {
	u, err := s.store.UserByID(ctx, a.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return UserView{}, apperr.Unauthorized("")
		}
		return UserView{}, err
	}
	return s.view(ctx, u)
}

// DemoUsers lists quick-login accounts; 404 unless DEMO_MODE is on.
func (s *Service) DemoUsers(ctx context.Context) (string, []DemoUser, error) {
	if !s.cfg.DemoMode {
		return "", nil, apperr.NotFound()
	}
	us, err := s.store.UsersByEmails(ctx, s.cfg.DemoEmails)
	if err != nil {
		return "", nil, err
	}
	order := map[string]int{}
	for i, e := range s.cfg.DemoEmails {
		order[strings.ToLower(e)] = i
	}
	out := make([]DemoUser, len(us))
	for _, u := range us {
		out[order[strings.ToLower(u.Email)]] = DemoUser{Email: u.Email, Name: u.Name, Role: string(u.Role), Title: u.Title}
	}
	res := out[:0]
	for _, d := range out {
		if d.Email != "" {
			res = append(res, d)
		}
	}
	return s.cfg.DemoPassword, res, nil
}

// Person returns minimal info about a user (for other modules).
func (s *Service) Person(ctx context.Context, id int64) (Person, error) {
	u, err := s.store.UserByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Person{}, apperr.NotFound()
	}
	if err != nil {
		return Person{}, err
	}
	return Person{ID: u.ID, Name: u.Name, Role: u.Role, DepartmentID: u.DepartmentID, Active: u.Active}, nil
}

// ActiveAdminIDs lists active admins (unrouted queue / escalation recipients).
func (s *Service) ActiveAdminIDs(ctx context.Context) ([]int64, error) {
	return s.store.ActiveAdminIDs(ctx)
}

// AssignableUsers lists active staff.
func (s *Service) AssignableUsers(ctx context.Context) ([]Assignable, error) {
	us, err := s.store.ActiveStaff(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Assignable, 0, len(us))
	for _, u := range us {
		out = append(out, Assignable{ID: u.ID, Name: u.Name, DepartmentID: u.DepartmentID})
	}
	return out, nil
}

// --- admin ---

// UserInput is AdminUserInput; pointer fields are optional for PATCH.
type UserInput struct {
	Name          *string
	Email         *string
	Role          *string
	DepartmentID  *int64
	DepartmentSet bool
	Active        *bool
	Password      *string
}

func (s *Service) ListUsers(ctx context.Context, a authz.Actor) ([]AdminUser, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return nil, err
	}
	us, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdminUser, 0, len(us))
	for _, u := range us {
		out = append(out, toAdmin(u))
	}
	return out, nil
}

func (s *Service) CreateUser(ctx context.Context, a authz.Actor, in UserInput) (AdminUser, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return AdminUser{}, err
	}
	var v apperr.V
	u := User{Active: true, CreatedAt: s.now()}
	if in.Name == nil {
		v.Add("name", "Укажите имя")
	} else {
		validateName(&v, *in.Name)
		u.Name = strings.TrimSpace(*in.Name)
	}
	if in.Email == nil || !validEmail(authz.NormalizeEmail(*in.Email)) {
		v.Add("email", "Укажите корректный email")
	} else {
		u.Email = authz.NormalizeEmail(*in.Email)
	}
	if in.Role == nil || !authz.ValidRole(*in.Role) {
		v.Add("role", "Роль: employee, manager или admin")
	} else {
		u.Role = authz.Role(*in.Role)
	}
	if in.Password == nil {
		v.Add("password", "Укажите начальный пароль")
	} else {
		validatePassword(&v, *in.Password)
	}
	if in.Active != nil {
		u.Active = *in.Active
	}
	u.DepartmentID = in.DepartmentID
	if err := v.Err(); err != nil {
		return AdminUser{}, err
	}
	h, err := s.HashPassword(*in.Password)
	if err != nil {
		return AdminUser{}, err
	}
	u.PasswordHash = h
	id, err := s.store.InsertUser(ctx, u)
	if err != nil {
		return AdminUser{}, s.mapErr(err)
	}
	u.ID = id
	return toAdmin(u), nil
}

func (s *Service) mapErr(err error) error {
	switch {
	case errors.Is(err, ErrDuplicateEmail):
		return apperr.Conflict("Пользователь с таким email уже существует")
	case errors.Is(err, ErrBadDepartment):
		return apperr.Invalid("department_id", "Отдел не найден")
	}
	return err
}

func (s *Service) UpdateUser(ctx context.Context, a authz.Actor, id int64, in UserInput) (AdminUser, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return AdminUser{}, err
	}
	u, err := s.store.UserByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return AdminUser{}, apperr.NotFound()
	}
	if err != nil {
		return AdminUser{}, err
	}
	var v apperr.V
	if in.Name != nil {
		validateName(&v, *in.Name)
		u.Name = strings.TrimSpace(*in.Name)
	}
	if in.Email != nil {
		e := authz.NormalizeEmail(*in.Email)
		if !validEmail(e) {
			v.Add("email", "Укажите корректный email")
		}
		u.Email = e
	}
	if in.Role != nil {
		if !authz.ValidRole(*in.Role) {
			v.Add("role", "Роль: employee, manager или admin")
		}
		u.Role = authz.Role(*in.Role)
	}
	if in.DepartmentSet {
		u.DepartmentID = in.DepartmentID
	}
	wasActive := u.Active
	if in.Active != nil {
		u.Active = *in.Active
	}
	if id == a.UserID && (!u.Active || u.Role != authz.RoleAdmin) {
		v.Add("role", "Нельзя снять с себя роль администратора или деактивировать себя")
	}
	if in.Password != nil {
		validatePassword(&v, *in.Password)
	}
	if err := v.Err(); err != nil {
		return AdminUser{}, err
	}
	if in.Password != nil {
		h, err := s.HashPassword(*in.Password)
		if err != nil {
			return AdminUser{}, err
		}
		u.PasswordHash = h
	}
	if err := s.store.UpdateUser(ctx, u); err != nil {
		return AdminUser{}, s.mapErr(err)
	}
	if (wasActive && !u.Active) || in.Password != nil {
		if err := s.store.DeleteUserSessions(ctx, u.ID); err != nil {
			return AdminUser{}, err
		}
	}
	return toAdmin(u), nil
}
