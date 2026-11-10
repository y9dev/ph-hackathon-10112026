package requests

import (
	"context"
	"time"

	"modusvoice/internal/modules/identity"
	"modusvoice/internal/modules/org"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
)

// StatusMeta describes a status for forms.
type StatusMeta struct {
	ID       Status `json:"id"`
	Label    string `json:"label"`
	Terminal bool   `json:"terminal"`
}

// Limits are the validation limits published to the UI.
type Limits struct {
	MaxFiles          int      `json:"max_files"`
	MaxFileMB         int      `json:"max_file_mb"`
	MaxTotalMB        int      `json:"max_total_mb"`
	AllowedExtensions []string `json:"allowed_extensions"`
	TitleMax          int      `json:"title_max"`
	BodyMax           int      `json:"body_max"`
}

// Meta is the contract Meta.
type Meta struct {
	Categories      []org.Category        `json:"categories"`
	Departments     []org.Department      `json:"departments"`
	Statuses        []StatusMeta          `json:"statuses"`
	Limits          Limits                `json:"limits"`
	AssignableUsers []identity.Assignable `json:"assignable_users"`
}

// Meta returns reference data for forms; assignable users only for staff.
func (s *Service) Meta(ctx context.Context, a authz.Actor) (Meta, error) {
	cats, err := s.Org.Categories(ctx, true)
	if err != nil {
		return Meta{}, err
	}
	depts, err := s.Org.Departments(ctx)
	if err != nil {
		return Meta{}, err
	}
	m := Meta{Categories: cats, Departments: depts, AssignableUsers: []identity.Assignable{},
		Limits: Limits{MaxFiles: MaxFiles, MaxFileMB: MaxFileBytes >> 20, MaxTotalMB: MaxTotalBytes >> 20,
			AllowedExtensions: AllowedExtensions, TitleMax: TitleMax, BodyMax: BodyMax}}
	for _, st := range AllStatuses {
		m.Statuses = append(m.Statuses, StatusMeta{ID: st, Label: st.Label(), Terminal: st.Terminal()})
	}
	if a.IsStaff() {
		if m.AssignableUsers, err = s.People.AssignableUsers(ctx); err != nil {
			return Meta{}, err
		}
	}
	return m, nil
}

// Preview is the contract RoutingPreview.
type Preview struct {
	Routed             bool            `json:"routed"`
	Department         *org.Department `json:"department"`
	AssigneeName       *string         `json:"assignee_name"`
	FirstResponseHours int             `json:"first_response_hours"`
	ResolutionHours    int             `json:"resolution_hours"`
	FirstResponseDue   time.Time       `json:"first_response_due"`
	SimilarCount       int             `json:"similar_count"`
}

// similarWindow bounds the aggregate "N colleagues reported something similar" count.
const similarWindow = 90 * 24 * time.Hour

// RoutingPreview tells the submit form what will happen to a request (any authenticated user).
func (s *Service) RoutingPreview(ctx context.Context, _ authz.Actor, categoryID int64, typ string) (Preview, error) {
	var v apperr.V
	if !ValidType(typ) {
		v.Add("type", "Выберите тип: проблема или предложение")
	}
	if categoryID <= 0 {
		v.Add("category_id", "Выберите категорию")
	}
	if err := v.Err(); err != nil {
		return Preview{}, err
	}
	cat, err := s.Org.Category(ctx, categoryID)
	if apperr.Is(err, apperr.KindNotFound) || (err == nil && !cat.Active) {
		return Preview{}, apperr.Invalid("category_id", "Категория не найдена")
	}
	if err != nil {
		return Preview{}, err
	}
	route, err := s.resolveRoute(ctx, categoryID, typ)
	if err != nil {
		return Preview{}, err
	}
	now := s.now()
	frd, _ := route.Due(now)
	p := Preview{Routed: route.Routed(), FirstResponseHours: route.FirstResponseHours, ResolutionHours: route.ResolutionHours, FirstResponseDue: frd}
	if route.DepartmentID != nil {
		d, err := s.Org.Department(ctx, *route.DepartmentID)
		if err != nil {
			return Preview{}, err
		}
		p.Department = &d
	}
	if route.AssigneeName != "" {
		p.AssigneeName = &route.AssigneeName
	}
	if p.SimilarCount, err = s.Store.CountInCategory(ctx, categoryID, now.Add(-similarWindow)); err != nil {
		return Preview{}, err
	}
	return p, nil
}
