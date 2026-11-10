package requests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"modusvoice/internal/modules/identity"
	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/org"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
	"modusvoice/internal/platform/eventbus"
)

// OrgReader is the slice of the org module this module needs.
type OrgReader interface {
	Category(ctx context.Context, id int64) (org.Category, error)
	Categories(ctx context.Context, activeOnly bool) ([]org.Category, error)
	Department(ctx context.Context, id int64) (org.Department, error)
	Departments(ctx context.Context) ([]org.Department, error)
	MatchRules(ctx context.Context, categoryID int64, typ string) (exact, categoryWide *org.Rule, err error)
}

// PeopleReader is the slice of the identity module this module needs.
type PeopleReader interface {
	Person(ctx context.Context, id int64) (identity.Person, error)
	ActiveAdminIDs(ctx context.Context) ([]int64, error)
	AssignableUsers(ctx context.Context) ([]identity.Assignable, error)
}

// NotificationWriter writes in-app notifications inside the caller's transaction (AC-08).
type NotificationWriter interface {
	Write(ctx context.Context, q db.Querier, actorID int64, ns []notifications.Note) error
}

// SimilarMatch is a request similar to a given one (any scope; the service filters by scope).
type SimilarMatch struct {
	ID           int64
	Number       string
	Title        string
	Status       Status
	DepartmentID *int64
	AssigneeID   *int64
	Score        float64
}

// SimilarFinder finds similar requests (implemented by the insights module).
type SimilarFinder interface {
	FindSimilar(ctx context.Context, requestID int64) ([]SimilarMatch, error)
}

// Deps are the collaborators wired in app/modules.go.
type Deps struct {
	DB        *sql.DB
	Store     Store
	Org       OrgReader
	People    PeopleReader
	Notifier  NotificationWriter
	Similar   SimilarFinder
	Bus       *eventbus.Bus
	Now       func() time.Time
	UploadDir string
	Log       *slog.Logger
}

// Service implements the request use cases. Every method takes the acting Actor.
type Service struct{ Deps }

func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{Deps: d}
}

func (s *Service) now() time.Time { return s.Now().UTC().Truncate(time.Second) }

func isStaffRole(r authz.Role) bool { return r == authz.RoleManager || r == authz.RoleAdmin }

func excerpt(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func ptr[T any](v T) *T { return &v }

// --- routing ---

func candidate(r *org.Rule) *RuleCandidate {
	if r == nil {
		return nil
	}
	return &RuleCandidate{ID: r.ID, DepartmentID: r.DepartmentID, DepartmentName: r.DepartmentName, AssigneeID: r.AssigneeUserID,
		FirstResponseHours: r.FirstResponseHours, ResolutionHours: r.ResolutionHours}
}

func (s *Service) owner(ctx context.Context, id *int64) (*Owner, error) {
	if id == nil {
		return nil, nil
	}
	p, err := s.People.Person(ctx, *id)
	if apperr.Is(err, apperr.KindNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Owner{ID: p.ID, Name: p.Name, Active: p.Active && isStaffRole(p.Role)}, nil
}

func (s *Service) departmentHead(ctx context.Context, deptID *int64) (*Owner, error) {
	if deptID == nil {
		return nil, nil
	}
	d, err := s.Org.Department(ctx, *deptID)
	if apperr.Is(err, apperr.KindNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.owner(ctx, d.HeadUserID)
}

func (s *Service) resolveRoute(ctx context.Context, categoryID int64, typ string) (Route, error) {
	exact, wide, err := s.Org.MatchRules(ctx, categoryID, typ)
	if err != nil {
		return Route{}, err
	}
	rule := exact
	if rule == nil {
		rule = wide
	}
	var ra, head *Owner
	if rule != nil {
		if ra, err = s.owner(ctx, rule.AssigneeUserID); err != nil {
			return Route{}, err
		}
		if head, err = s.departmentHead(ctx, &rule.DepartmentID); err != nil {
			return Route{}, err
		}
	}
	return ResolveRoute(candidate(exact), candidate(wide), ra, head), nil
}

// ownerIDs returns who owns a request: assignee, else active department head, else all admins.
func (s *Service) ownerIDs(ctx context.Context, deptID, assigneeID *int64) ([]int64, error) {
	if assigneeID != nil {
		return []int64{*assigneeID}, nil
	}
	head, err := s.departmentHead(ctx, deptID)
	if err != nil {
		return nil, err
	}
	if head != nil && head.Active {
		return []int64{head.ID}, nil
	}
	return s.People.ActiveAdminIDs(ctx)
}

func notes(ids []int64, kind string, requestID int64, title, body string) []notifications.Note {
	out := make([]notifications.Note, 0, len(ids))
	for _, id := range ids {
		out = append(out, notifications.Note{UserID: id, Kind: kind, RequestID: requestID, Title: title, Body: body})
	}
	return out
}

func (s *Service) publish(ctx context.Context, topic string, r Row, data map[string]any) {
	s.Bus.Publish(ctx, eventbus.Event{Topic: topic, RequestID: r.ID, Number: r.Number, At: s.now(), Data: data})
}

// --- create ---

// Create validates, routes and stores a new request with attachments, timeline and notifications in one tx.
func (s *Service) Create(ctx context.Context, a authz.Actor, in CreateInput, files []Upload) (Detail, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Detail{}, err
	}
	if err := ValidateUploads(files); err != nil {
		return Detail{}, err
	}
	cat, err := s.Org.Category(ctx, in.CategoryID)
	if apperr.Is(err, apperr.KindNotFound) || (err == nil && !cat.Active) {
		return Detail{}, apperr.Invalid("category_id", "Категория не найдена")
	}
	if err != nil {
		return Detail{}, err
	}
	route, err := s.resolveRoute(ctx, cat.ID, in.Type)
	if err != nil {
		return Detail{}, err
	}
	recipients, err := s.ownerIDs(ctx, route.DepartmentID, route.AssigneeID)
	if err != nil {
		return Detail{}, err
	}
	stored, err := storeUploads(s.UploadDir, files)
	if err != nil {
		return Detail{}, err
	}
	now := s.now()
	frd, rd := route.Due(now)
	var id int64
	var number string
	err = db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		n, err := s.Store.NextNumber(ctx, tx)
		if err != nil {
			return err
		}
		number = FormatNumber(n)
		var prop *string
		if in.ProposedSolution != "" {
			prop = &in.ProposedSolution
		}
		id, err = s.Store.InsertRequest(ctx, tx, NewRequest{Number: number, Type: in.Type, CategoryID: cat.ID, Title: in.Title, Body: in.Body,
			ProposedSolution: prop, AuthorID: a.UserID, IsAnonymous: in.IsAnonymous, DepartmentID: route.DepartmentID,
			AssigneeID: route.AssigneeID, RoutingSource: route.Source, FirstResponseDue: frd, ResolutionDue: rd, CreatedAt: now})
		if err != nil {
			return err
		}
		if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: id, ActorID: &a.UserID, Kind: EvCreated, ToStatus: ptr(StatusNew), At: now}); err != nil {
			return err
		}
		if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: id, Kind: EvRouted, Text: ptr(routedText(route)), Meta: ptr(route.Source), At: now}); err != nil {
			return err
		}
		for _, f := range stored {
			if err := s.Store.InsertAttachment(ctx, tx, id, a.UserID, f, now); err != nil {
				return err
			}
		}
		title := fmt.Sprintf("Новое обращение %s", number)
		if !route.Routed() {
			title = fmt.Sprintf("Нераспределённое обращение %s", number)
		}
		return s.Notifier.Write(ctx, tx, a.UserID, notes(recipients, notifications.KindCreated, id, title, excerpt(in.Title, 200)))
	})
	if err != nil {
		removeStored(s.UploadDir, stored)
		return Detail{}, err
	}
	d, err := s.Store.Get(ctx, s.DB, id)
	if err != nil {
		return Detail{}, err
	}
	s.publish(ctx, eventbus.RequestCreated, d.Row, map[string]any{"category_id": cat.ID, "routing_source": route.Source, "department_id": route.DepartmentID})
	return s.buildDetail(ctx, a, d)
}

func routedText(r Route) string {
	if r.Routed() {
		who := ""
		if r.AssigneeName != "" {
			who = ", ответственный: " + r.AssigneeName
		}
		return fmt.Sprintf("Передано: %s%s. Срок первого ответа — %d ч", r.DepartmentName, who, r.FirstResponseHours)
	}
	if r.DepartmentID != nil {
		return fmt.Sprintf("Для отдела «%s» нет активного ответственного — обращение передано администратору", r.DepartmentName)
	}
	return "Подходящего правила маршрутизации нет — обращение передано администратору"
}

// --- read ---

// Get returns a request the actor may read (404 otherwise, never 403, to avoid existence leaks).
func (s *Service) Get(ctx context.Context, a authz.Actor, id int64) (Detail, error) {
	r, err := s.Store.Get(ctx, s.DB, id)
	if errors.Is(err, ErrNotFound) {
		return Detail{}, apperr.NotFound()
	}
	if err != nil {
		return Detail{}, err
	}
	if !canRead(a, r.Row) {
		return Detail{}, apperr.NotFound()
	}
	return s.buildDetail(ctx, a, r)
}

func (s *Service) buildDetail(ctx context.Context, a authz.Actor, r DetailRow) (Detail, error) {
	now := s.now()
	events, err := s.Store.Events(ctx, r.ID)
	if err != nil {
		return Detail{}, err
	}
	comments, err := s.Store.Comments(ctx, r.ID)
	if err != nil {
		return Detail{}, err
	}
	atts, err := s.Store.Attachments(ctx, r.ID)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Summary: toSummary(r.Row, a, now), Body: r.Body, ProposedSolution: r.ProposedSolution, ResultText: r.ResultText,
		Timeline: buildTimeline(r.Row, events, comments, a), Attachments: make([]AttachmentView, 0, len(atts)),
		Similar: SimilarView{Items: []SimilarItem{}}}
	if r.DuplicateOfID != nil {
		num := ""
		if r.DuplicateOfNumber != nil {
			num = *r.DuplicateOfNumber
		}
		d.DuplicateOf = &NumRef{ID: *r.DuplicateOfID, Number: num}
	}
	if r.ThemeID != nil && r.ThemeTitle != nil {
		d.Theme = &ThemeRef{ID: *r.ThemeID, Title: *r.ThemeTitle}
	}
	for _, at := range atts {
		d.Attachments = append(d.Attachments, AttachmentView{ID: at.ID, Filename: at.Filename, MIME: at.MIME, Size: at.Size})
	}
	d.AllowedStatuses, d.AllowedCommentKinds, d.CanAssign = allowedFor(r.Row, a)
	if s.Similar != nil {
		matches, err := s.Similar.FindSimilar(ctx, r.ID)
		if err != nil {
			return Detail{}, err
		}
		d.Similar.Count = len(matches)
		if a.IsStaff() {
			for _, m := range matches {
				if len(d.Similar.Items) >= 5 {
					break
				}
				if a.CanManage(m.DepartmentID, m.AssigneeID) {
					d.Similar.Items = append(d.Similar.Items, SimilarItem{ID: m.ID, Number: m.Number, Title: m.Title, Status: m.Status, Score: m.Score})
				}
			}
		}
	}
	return d, nil
}

// ListQuery is the parsed GET /requests query.
type ListQuery struct {
	Scope        string
	Tab          string
	Status       string
	CategoryID   *int64
	DepartmentID *int64
	Assignee     string
	Overdue      *bool
	Q            string
	Sort         string
	Page         int
	PageSize     int
}

func (s *Service) filter(a authz.Actor, q ListQuery) (ListFilter, error) {
	var v apperr.V
	f := ListFilter{Now: s.now(), CategoryID: q.CategoryID, DepartmentID: q.DepartmentID, Overdue: q.Overdue, Query: q.Q, Sort: q.Sort, Tab: q.Tab}
	switch q.Scope {
	case "mine":
		f.ScopeSQL, f.ScopeArgs = "r.author_id = ?", []any{a.UserID}
	case "inbox", "all":
		if err := authz.RequireStaff(a); err != nil {
			return ListFilter{}, err
		}
		f.ScopeSQL, f.ScopeArgs = authz.ScopeSQL(a, "r.department_id", "r.assignee_id")
	default:
		v.Add("scope", "scope: mine, inbox или all")
	}
	switch q.Tab {
	case "", "all", "needs_action", "in_progress", "waiting_author", "closed":
	default:
		v.Add("tab", "Неизвестная вкладка")
	}
	if q.Status != "" {
		st := Status(q.Status)
		if !st.Valid() {
			v.Add("status", "Неизвестный статус")
		}
		f.Status = &st
	}
	switch q.Assignee {
	case "":
	case "me":
		f.AssigneeID = &a.UserID
	case "none":
		f.AssigneeNone = true
	default:
		v.Add("assignee", "assignee: me или none")
	}
	switch q.Sort {
	case "", "deadline", "newest", "oldest", "updated":
	default:
		v.Add("sort", "sort: deadline, newest, oldest или updated")
	}
	if utf8.RuneCountInString(q.Q) > 200 {
		v.Add("q", "Поисковый запрос: не более 200 символов")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	if q.Page > 100000 {
		v.Add("page", "Слишком большой номер страницы")
	}
	f.Limit, f.Offset = q.PageSize, (q.Page-1)*q.PageSize
	return f, v.Err()
}

// List returns a page of requests in the requested scope with tab counters.
func (s *Service) List(ctx context.Context, a authz.Actor, q ListQuery) (ListResult, error) {
	f, err := s.filter(a, q)
	if err != nil {
		return ListResult{}, err
	}
	rows, total, err := s.Store.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	counts, err := s.Store.Counts(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]Summary, 0, len(rows))
	for _, r := range rows {
		items = append(items, toSummary(r, a, f.Now))
	}
	return ListResult{Items: items, Total: total, Page: f.Offset/f.Limit + 1, PageSize: f.Limit, Counts: counts}, nil
}

// ListOverdue returns currently overdue requests in the actor's staff scope (analytics).
func (s *Service) ListOverdue(ctx context.Context, a authz.Actor, departmentID *int64, limit int) ([]Summary, error) {
	res, err := s.List(ctx, a, ListQuery{Scope: "inbox", Overdue: ptr(true), DepartmentID: departmentID, Sort: "deadline", PageSize: limit})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}
