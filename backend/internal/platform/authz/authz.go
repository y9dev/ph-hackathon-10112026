// Package authz is the shared kernel for the authenticated principal (Actor) and scope rules
// from ARCHITECTURE §5. Every service method takes an Actor.
package authz

import (
	"context"
	"slices"
	"strings"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/db"
)

type Role string

const (
	RoleEmployee Role = "employee"
	RoleManager  Role = "manager"
	RoleAdmin    Role = "admin"
)

// ValidRole reports whether r is a known role.
func ValidRole(r string) bool {
	switch Role(r) {
	case RoleEmployee, RoleManager, RoleAdmin:
		return true
	}
	return false
}

// RoleLabel is the Russian label for staff roles shown in timelines.
func RoleLabel(r Role) string {
	switch r {
	case RoleAdmin:
		return "Администратор"
	case RoleManager:
		return "Руководитель"
	}
	return "Сотрудник"
}

// Actor is the authenticated user performing an action.
type Actor struct {
	UserID         int64
	Name           string
	Email          string
	Role           Role
	DepartmentID   *int64
	LedDepartments []int64
}

func (a Actor) IsAdmin() bool { return a.Role == RoleAdmin }
func (a Actor) IsStaff() bool { return a.Role == RoleAdmin || a.Role == RoleManager }

// Leads reports whether the actor heads department id.
func (a Actor) Leads(id int64) bool { return slices.Contains(a.LedDepartments, id) }

// CanManage reports whether a staff actor has a request (by department/assignee) in scope:
// admin = all; manager = routed to a department they head or assigned to them.
func (a Actor) CanManage(departmentID, assigneeID *int64) bool {
	switch a.Role {
	case RoleAdmin:
		return true
	case RoleManager:
		if assigneeID != nil && *assigneeID == a.UserID {
			return true
		}
		return departmentID != nil && a.Leads(*departmentID)
	}
	return false
}

// ScopeSQL returns a SQL predicate (with args) restricting requests to the actor's staff scope.
// deptCol/assigneeCol are trusted column expressions supplied by code, never user input.
func ScopeSQL(a Actor, deptCol, assigneeCol string) (string, []any) {
	switch a.Role {
	case RoleAdmin:
		return "1=1", nil
	case RoleManager:
		if len(a.LedDepartments) == 0 {
			return assigneeCol + " = ?", []any{a.UserID}
		}
		args := make([]any, 0, len(a.LedDepartments)+1)
		for _, d := range a.LedDepartments {
			args = append(args, d)
		}
		args = append(args, a.UserID)
		return "(" + deptCol + " IN (" + db.Placeholders(len(a.LedDepartments)) + ") OR " + assigneeCol + " = ?)", args
	}
	return "0=1", nil
}

// RequireStaff returns forbidden unless the actor is manager or admin.
func RequireStaff(a Actor) error {
	if !a.IsStaff() {
		return apperr.Forbidden()
	}
	return nil
}

// RequireAdmin returns forbidden unless the actor is admin.
func RequireAdmin(a Actor) error {
	if !a.IsAdmin() {
		return apperr.Forbidden()
	}
	return nil
}

type ctxKey struct{}

// WithActor stores the actor in ctx.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// FromContext returns the actor stored by the auth middleware.
func FromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxKey{}).(Actor)
	return a, ok
}

// MustActor returns the actor or an unauthorized error.
func MustActor(ctx context.Context) (Actor, error) {
	a, ok := FromContext(ctx)
	if !ok || a.UserID == 0 {
		return Actor{}, apperr.Unauthorized("")
	}
	return a, nil
}

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
