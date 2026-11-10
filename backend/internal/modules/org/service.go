package org

import (
	"context"
	"errors"

	"modusvoice/internal/modules/identity"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

// People is the slice of identity the org module needs (assignee validation).
type People interface {
	Person(ctx context.Context, id int64) (identity.Person, error)
}

// Service exposes org reference data and admin configuration use cases.
type Service struct {
	store  Store
	people People
}

func NewService(store Store, people People) *Service { return &Service{store: store, people: people} }

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperr.NotFound()
	}
	return err
}

// --- reference data (used by other modules) ---

func (s *Service) Departments(ctx context.Context) ([]Department, error) {
	return s.store.Departments(ctx)
}

func (s *Service) Department(ctx context.Context, id int64) (Department, error) {
	d, err := s.store.Department(ctx, id)
	return d, notFound(err)
}

func (s *Service) Categories(ctx context.Context, activeOnly bool) ([]Category, error) {
	return s.store.Categories(ctx, activeOnly)
}

func (s *Service) Category(ctx context.Context, id int64) (Category, error) {
	c, err := s.store.Category(ctx, id)
	return c, notFound(err)
}

// MatchRules returns the active exact (category, type) rule and the category-wide rule, either may be nil.
func (s *Service) MatchRules(ctx context.Context, categoryID int64, typ string) (exact, categoryWide *Rule, err error) {
	exact, err = s.store.ActiveRule(ctx, categoryID, &typ)
	if err != nil {
		return nil, nil, err
	}
	categoryWide, err = s.store.ActiveRule(ctx, categoryID, nil)
	return exact, categoryWide, err
}

// --- admin ---

func (s *Service) AdminDepartments(ctx context.Context, a authz.Actor) ([]Department, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return nil, err
	}
	return s.store.Departments(ctx)
}

func (s *Service) AdminCategories(ctx context.Context, a authz.Actor) ([]Category, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return nil, err
	}
	return s.store.Categories(ctx, false)
}

func (s *Service) CreateCategory(ctx context.Context, a authz.Actor, in CategoryInput) (Category, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return Category{}, err
	}
	c := Category{Icon: "circle", Active: true}
	if err := in.apply(&c, true); err != nil {
		return Category{}, err
	}
	id, err := s.store.InsertCategory(ctx, c)
	if errors.Is(err, ErrDuplicate) {
		return Category{}, apperr.Conflict("Категория с таким названием уже существует")
	}
	if err != nil {
		return Category{}, err
	}
	c.ID = id
	return c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, a authz.Actor, id int64, in CategoryInput) (Category, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return Category{}, err
	}
	c, err := s.store.Category(ctx, id)
	if err != nil {
		return Category{}, notFound(err)
	}
	if err := in.apply(&c, false); err != nil {
		return Category{}, err
	}
	if err := s.store.UpdateCategory(ctx, c); err != nil {
		if errors.Is(err, ErrDuplicate) {
			return Category{}, apperr.Conflict("Категория с таким названием уже существует")
		}
		return Category{}, err
	}
	return c, nil
}

// RuleInput is RoutingRuleInput with tri-state nullable fields.
type RuleInput struct {
	CategoryID         *int64            `json:"category_id"`
	RequestType        httpx.Opt[string] `json:"request_type"`
	DepartmentID       *int64            `json:"department_id"`
	AssigneeUserID     httpx.Opt[int64]  `json:"assignee_user_id"`
	FirstResponseHours *int              `json:"first_response_hours"`
	ResolutionHours    *int              `json:"resolution_hours"`
	Active             *bool             `json:"active"`
}

func (s *Service) applyRule(ctx context.Context, r *Rule, in RuleInput, create bool) error {
	var v apperr.V
	if in.CategoryID != nil {
		if _, err := s.store.Category(ctx, *in.CategoryID); err != nil {
			if !errors.Is(err, ErrNotFound) {
				return err
			}
			v.Add("category_id", "Категория не найдена")
		}
		r.CategoryID = *in.CategoryID
	} else if create {
		v.Add("category_id", "Укажите категорию")
	}
	if in.RequestType.Set {
		if in.RequestType.Null || in.RequestType.V == "" {
			r.RequestType = nil
		} else if !ValidRequestType(in.RequestType.V) {
			v.Add("request_type", "Тип: problem, idea, feedback или пусто (любой)")
		} else {
			t := in.RequestType.V
			r.RequestType = &t
		}
	}
	if in.DepartmentID != nil {
		if _, err := s.store.Department(ctx, *in.DepartmentID); err != nil {
			if !errors.Is(err, ErrNotFound) {
				return err
			}
			v.Add("department_id", "Отдел не найден")
		}
		r.DepartmentID = *in.DepartmentID
	} else if create {
		v.Add("department_id", "Укажите отдел")
	}
	if in.AssigneeUserID.Set {
		if in.AssigneeUserID.Null {
			r.AssigneeUserID = nil
		} else {
			p, err := s.people.Person(ctx, in.AssigneeUserID.V)
			if err != nil && !apperr.Is(err, apperr.KindNotFound) {
				return err
			}
			if err != nil || !p.Active || (p.Role != authz.RoleManager && p.Role != authz.RoleAdmin) {
				v.Add("assignee_user_id", "Ответственным может быть только активный руководитель или администратор")
			}
			id := in.AssigneeUserID.V
			r.AssigneeUserID = &id
		}
	}
	if in.FirstResponseHours != nil {
		r.FirstResponseHours = *in.FirstResponseHours
	} else if create {
		r.FirstResponseHours = 48
	}
	if in.ResolutionHours != nil {
		r.ResolutionHours = *in.ResolutionHours
	} else if create {
		r.ResolutionHours = 240
	}
	if r.FirstResponseHours < 1 || r.FirstResponseHours > maxSLAHours {
		v.Add("first_response_hours", "Срок первого ответа: от 1 до 8760 часов")
	}
	if r.ResolutionHours < 1 || r.ResolutionHours > maxSLAHours {
		v.Add("resolution_hours", "Срок решения: от 1 до 8760 часов")
	} else if r.ResolutionHours < r.FirstResponseHours {
		v.Add("resolution_hours", "Срок решения не может быть меньше срока первого ответа")
	}
	if in.Active != nil {
		r.Active = *in.Active
	}
	return v.Err()
}

func (s *Service) AdminRules(ctx context.Context, a authz.Actor) ([]Rule, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return nil, err
	}
	return s.store.Rules(ctx)
}

var errRuleConflict = apperr.Conflict("Активное правило для этой категории и типа уже существует")

func (s *Service) CreateRule(ctx context.Context, a authz.Actor, in RuleInput) (Rule, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return Rule{}, err
	}
	r := Rule{Active: true}
	if err := s.applyRule(ctx, &r, in, true); err != nil {
		return Rule{}, err
	}
	id, err := s.store.InsertRule(ctx, r)
	if errors.Is(err, ErrDuplicate) {
		return Rule{}, errRuleConflict
	}
	if err != nil {
		return Rule{}, err
	}
	return s.store.Rule(ctx, id)
}

func (s *Service) UpdateRule(ctx context.Context, a authz.Actor, id int64, in RuleInput) (Rule, error) {
	if err := authz.RequireAdmin(a); err != nil {
		return Rule{}, err
	}
	r, err := s.store.Rule(ctx, id)
	if err != nil {
		return Rule{}, notFound(err)
	}
	if err := s.applyRule(ctx, &r, in, false); err != nil {
		return Rule{}, err
	}
	if err := s.store.UpdateRule(ctx, r); err != nil {
		if errors.Is(err, ErrDuplicate) {
			return Rule{}, errRuleConflict
		}
		return Rule{}, err
	}
	return s.store.Rule(ctx, id)
}

func (s *Service) DeleteRule(ctx context.Context, a authz.Actor, id int64) error {
	if err := authz.RequireAdmin(a); err != nil {
		return err
	}
	return notFound(s.store.DeleteRule(ctx, id))
}
