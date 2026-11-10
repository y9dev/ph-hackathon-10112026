// Package identity owns users, sessions, login (with throttling) and admin user management.
package identity

import (
	"strings"
	"time"
	"unicode/utf8"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
)

const (
	SessionCookie = "mv_session"
	SessionTTL    = 7 * 24 * time.Hour
)

// User is the stored user record.
type User struct {
	ID             int64
	Email          string
	Name           string
	Title          string
	PasswordHash   string
	Role           authz.Role
	DepartmentID   *int64
	DepartmentName *string
	Active         bool
	CreatedAt      time.Time
}

// Person is the minimal view other modules need about a user.
type Person struct {
	ID           int64
	Name         string
	Role         authz.Role
	DepartmentID *int64
	Active       bool
}

// Assignable is a staff user that may own requests.
type Assignable struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DepartmentID *int64 `json:"department_id"`
}

// UserView is the /auth/me and /auth/login response.
type UserView struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Email            string  `json:"email"`
	Role             string  `json:"role"`
	DepartmentID     *int64  `json:"department_id"`
	DepartmentName   *string `json:"department_name"`
	LedDepartmentIDs []int64 `json:"led_department_ids"`
}

// AdminUser is the admin list/detail view.
type AdminUser struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	DepartmentID *int64 `json:"department_id"`
	Active       bool   `json:"active"`
}

// DemoUser is listed on the login screen in DEMO_MODE.
type DemoUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Title string `json:"title"`
}

func toAdmin(u User) AdminUser {
	return AdminUser{ID: u.ID, Name: u.Name, Email: u.Email, Role: string(u.Role), DepartmentID: u.DepartmentID, Active: u.Active}
}

func validEmail(s string) bool {
	if len(s) < 3 || len(s) > 254 || strings.ContainsAny(s, " \t\r\n<>\"") {
		return false
	}
	at := strings.LastIndex(s, "@")
	return at > 0 && at < len(s)-1 && strings.Contains(s[at+1:], ".")
}

func validateName(v *apperr.V, name string) {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	if n < 2 || n > 120 {
		v.Add("name", "Имя: от 2 до 120 символов")
	}
}

func validatePassword(v *apperr.V, p string) {
	if len(p) < 8 || len(p) > 128 {
		v.Add("password", "Пароль: от 8 до 128 символов")
	}
}
