package insights

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
)

// OverdueLister supplies overdue request summaries (already passed through AuthorView).
type OverdueLister interface {
	ListOverdue(ctx context.Context, a authz.Actor, departmentID *int64, limit int) ([]requests.Summary, error)
}

// Service implements similarity and analytics use cases.
type Service struct {
	store   Store
	overdue OverdueLister
	now     func() time.Time
	loc     *time.Location
}

func NewService(store Store, overdue OverdueLister, now func() time.Time, loc *time.Location) *Service {
	if now == nil {
		now = time.Now
	}
	if loc == nil {
		loc = time.UTC
	}
	return &Service{store: store, overdue: overdue, now: now, loc: loc}
}

// SetOverdueLister breaks the construction cycle (requests needs insights for similarity).
func (s *Service) SetOverdueLister(o OverdueLister) { s.overdue = o }

const (
	similarLookback = 365 * 24 * time.Hour
	similarMaxDocs  = 1500                // newest candidates scored per FindSimilar call
	countLookback   = 90 * 24 * time.Hour // aggregate-only count for non-staff: same category, recent
	groupLookback   = 180 * 24 * time.Hour
	groupMaxDocs    = 600
)

// staffScope is the OWNER scope (suggested groups: actionable, a theme can only be linked by owners).
func staffScope(a authz.Actor) Scope {
	sql, args := authz.ScopeSQL(a, "r.department_id", "r.assignee_id")
	return Scope{SQL: sql, Args: args}
}

// readScope is what staff may READ: CR6 lets managers (and admins) see every request, so
// analytics and themes are company-wide; the department_id filter narrows them.
func readScope(a authz.Actor) Scope {
	if a.IsStaff() {
		return Scope{SQL: "1=1"}
	}
	return Scope{SQL: "0=1"}
}

// FindSimilar implements requests.SimilarFinder: all requests scoring >= Threshold, best first.
// The caller filters items by scope; the count is an aggregate.
func (s *Service) FindSimilar(ctx context.Context, requestID int64) ([]requests.SimilarMatch, error) {
	target, err := s.store.Doc(ctx, requestID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	docs, err := s.store.Candidates(ctx, nil, s.now().Add(-similarLookback), similarMaxDocs)
	if err != nil {
		return nil, err
	}
	var out []requests.SimilarMatch
	for i := range docs {
		d := &docs[i]
		if d.ID == target.ID {
			continue
		}
		if sc := Score(&target, d); sc >= Threshold {
			out = append(out, requests.SimilarMatch{ID: d.ID, Number: d.Number, Title: d.Title, Status: requests.Status(d.Status),
				DepartmentID: d.DepartmentID, AssigneeID: d.AssigneeID, Score: round2(sc)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

// CountSimilar implements requests.SimilarFinder for non-staff viewers: an aggregate count of similar
// requests in the SAME category within the last 90 days. Counts below 2 are reported as 0 so a
// small number never reveals that one specific other request exists.
func (s *Service) CountSimilar(ctx context.Context, requestID int64) (int, error) {
	target, err := s.store.Doc(ctx, requestID)
	if errors.Is(err, ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	docs, err := s.store.Candidates(ctx, &target.CategoryID, s.now().Add(-countLookback), similarMaxDocs)
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range docs {
		if docs[i].ID != target.ID && Score(&target, &docs[i]) >= Threshold {
			n++
		}
	}
	if n < 2 {
		return 0, nil
	}
	return n, nil
}

// GroupRequest is a request inside a suggested group.
type GroupRequest struct {
	ID     int64           `json:"id"`
	Number string          `json:"number"`
	Title  string          `json:"title"`
	Status requests.Status `json:"status"`
}

// SuggestedGroup is the contract SuggestedGroup.
type SuggestedGroup struct {
	Key        string         `json:"key"`
	Title      string         `json:"title"`
	Count      int            `json:"count"`
	Score      float64        `json:"score"`
	Categories []string       `json:"categories"`
	Requests   []GroupRequest `json:"requests"`
}

// SuggestedGroups clusters recent requests in the actor's staff scope (R8: a manager confirms).
// Groups whose members all already share one theme are omitted.
func (s *Service) SuggestedGroups(ctx context.Context, a authz.Actor) ([]SuggestedGroup, error) {
	if err := authz.RequireStaff(a); err != nil {
		return nil, err
	}
	docs, err := s.store.Docs(ctx, staffScope(a), s.now().Add(-groupLookback), groupMaxDocs)
	if err != nil {
		return nil, err
	}
	out := []SuggestedGroup{}
	for _, g := range Cluster(docs, Threshold, MinGroupSize) {
		if sameTheme(docs, g.Members) {
			continue
		}
		minID := docs[g.Members[0]].ID
		catCount := map[string]int{}
		members := append([]int{}, g.Members...)
		sort.Slice(members, func(i, j int) bool { return docs[members[i]].CreatedAt.After(docs[members[j]].CreatedAt) })
		sg := SuggestedGroup{Title: docs[g.Center].Title, Count: len(members), Score: g.Score, Requests: []GroupRequest{}}
		for _, i := range members {
			d := docs[i]
			minID = min(minID, d.ID)
			catCount[d.CategoryName]++
			sg.Requests = append(sg.Requests, GroupRequest{ID: d.ID, Number: d.Number, Title: d.Title, Status: requests.Status(d.Status)})
		}
		sg.Key = fmt.Sprintf("g-%d", minID)
		sg.Categories = sortedKeys(catCount)
		out = append(out, sg)
		if len(out) == 10 {
			break
		}
	}
	return out, nil
}

func sameTheme(docs []Doc, members []int) bool {
	first := docs[members[0]].ThemeID
	if first == nil {
		return false
	}
	for _, i := range members[1:] {
		if t := docs[i].ThemeID; t == nil || *t != *first {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

// ThemeView is the contract Theme.
type ThemeView struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Count      int      `json:"count"`
	OpenCount  int      `json:"open_count"`
	Categories []string `json:"categories"`
}

// Themes lists confirmed themes with counts over requests in the actor's scope.
func (s *Service) Themes(ctx context.Context, a authz.Actor) ([]ThemeView, error) {
	if err := authz.RequireStaff(a); err != nil {
		return nil, err
	}
	rows, err := s.store.ThemeRows(ctx, readScope(a))
	if err != nil {
		return nil, err
	}
	byID := map[int64]*ThemeView{}
	cats := map[int64]map[string]int{}
	var order []int64
	for _, r := range rows {
		t, ok := byID[r.ThemeID]
		if !ok {
			t = &ThemeView{ID: r.ThemeID, Title: r.Title}
			byID[r.ThemeID] = t
			cats[r.ThemeID] = map[string]int{}
			order = append(order, r.ThemeID)
		}
		t.Count++
		if !requests.Status(r.Status).Terminal() {
			t.OpenCount++
		}
		cats[r.ThemeID][r.Category]++
	}
	out := make([]ThemeView, 0, len(order))
	for _, id := range order {
		t := byID[id]
		t.Categories = sortedKeys(cats[id])
		out = append(out, *t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}

// --- analytics ---

// SummaryFilter is the parsed /analytics/summary query.
type SummaryFilter struct {
	From         *time.Time // inclusive day start in display TZ
	To           *time.Time // inclusive day (end handled internally)
	DepartmentID *int64
}

// KPIs are the TZ metrics (CR5: p90 removed; on-time share and resolution time added).
type KPIs struct {
	Total                    int      `json:"total"`
	ReviewedShare            float64  `json:"reviewed_share"`
	MedianFirstResponseHours *float64 `json:"median_first_response_hours"`
	FirstResponseOnTimeShare *float64 `json:"first_response_on_time_share"`
	FirstResponseSample      int      `json:"first_response_sample"`
	MedianResolutionHours    *float64 `json:"median_resolution_hours"`
	OutcomeShare             float64  `json:"outcome_share"`
	ImplementedCount         int      `json:"implemented_count"`
	OverdueCount             int      `json:"overdue_count"`
	AutoRoutedShare          float64  `json:"auto_routed_share"`
}

type StatusCount struct {
	Status requests.Status `json:"status"`
	Count  int             `json:"count"`
}

type CategoryStat struct {
	Category    string `json:"category"`
	Count       int    `json:"count"`
	Implemented int    `json:"implemented"`
	Declined    int    `json:"declined"`
	Open        int    `json:"open"`
}

type DepartmentStat struct {
	Department string `json:"department"`
	Count      int    `json:"count"`
	Open       int    `json:"open"`
	Overdue    int    `json:"overdue"`
}

type Bucket struct {
	Date    string `json:"date"`
	Created int    `json:"created"`
	Closed  int    `json:"closed"`
}

// Summary is the contract AnalyticsSummary.
type Summary struct {
	KPIs         KPIs               `json:"kpis"`
	ByStatus     []StatusCount      `json:"by_status"`
	ByCategory   []CategoryStat     `json:"by_category"`
	ByDepartment []DepartmentStat   `json:"by_department"`
	Timeseries   []Bucket           `json:"timeseries"`
	Overdue      []requests.Summary `json:"overdue"`
}

// UnassignedDepartment labels requests without a department.
const UnassignedDepartment = "Не распределено"

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(rank-float64(lo))
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }

// median returns the rounded median of xs (nil when empty). xs is sorted in place.
func median(xs []float64) *float64 {
	if len(xs) == 0 {
		return nil
	}
	sort.Float64s(xs)
	m := round1(percentile(xs, 0.5))
	return &m
}

// FirstResponseAndResolution returns, in hours, created->first response for requests with a
// first response and created->closed for closed requests among facts.
func FirstResponseAndResolution(facts []Fact) (first, resolution []float64) {
	for _, x := range facts {
		if x.FirstRespondedAt != nil {
			first = append(first, x.FirstRespondedAt.Sub(x.CreatedAt).Hours())
		}
		if x.ClosedAt != nil && requests.Status(x.Status).Terminal() {
			resolution = append(resolution, x.ClosedAt.Sub(x.CreatedAt).Hours())
		}
	}
	return first, resolution
}

// OnTimeShare (CR5) is the share of requests answered no later than their first-response deadline
// among requests that have a first response. The stored deadline already includes needs_info
// pause shifts. nil when nothing was answered yet.
func OnTimeShare(facts []Fact) *float64 {
	answered, onTime := 0, 0
	for _, x := range facts {
		if x.FirstRespondedAt == nil {
			continue
		}
		answered++
		if !x.FirstRespondedAt.After(x.FirstResponseDue) {
			onTime++
		}
	}
	if answered == 0 {
		return nil
	}
	v := share(onTime, answered)
	return &v
}

func share(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return math.Round(float64(n)/float64(d)*1000) / 1000
}

func (s *Service) dayStart(t time.Time) time.Time {
	t = t.In(s.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, s.loc)
}

// Summary computes KPIs company-wide for staff (CR6: managers and admins), optionally for one department.
func (s *Service) Summary(ctx context.Context, a authz.Actor, f SummaryFilter) (Summary, error) {
	if err := authz.RequireStaff(a); err != nil {
		return Summary{}, err
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		return Summary{}, apperr.Invalid("to", "Дата окончания раньше даты начала")
	}
	scope := readScope(a)
	now := s.now()
	var toExcl *time.Time
	if f.To != nil {
		t := s.dayStart(*f.To).AddDate(0, 0, 1)
		toExcl = &t
	}
	facts, err := s.store.Facts(ctx, scope, f.From, toExcl, f.DepartmentID)
	if err != nil {
		return Summary{}, err
	}

	var res Summary
	k := &res.KPIs
	k.Total = len(facts)
	statusCount := map[requests.Status]int{}
	cats := map[string]*CategoryStat{}
	depts := map[string]*DepartmentStat{}
	var catOrder, deptOrder []string
	frHours, resHours := FirstResponseAndResolution(facts)
	reviewed, closed, withResult, auto := 0, 0, 0, 0
	for _, x := range facts {
		st := requests.Status(x.Status)
		statusCount[st]++
		open := !st.Terminal()
		overdue := requests.OverdueKindAt(requests.SLAInput{Status: st, CreatedAt: x.CreatedAt, FirstResponseDue: x.FirstResponseDue,
			ResolutionDue: x.ResolutionDue, FirstRespondedAt: x.FirstRespondedAt}, now) != ""
		if x.FirstRespondedAt != nil {
			reviewed++
		}
		if st.Terminal() {
			closed++
			if x.HasResult {
				withResult++
			}
		}
		if st == requests.StatusImplemented {
			k.ImplementedCount++
		}
		if overdue {
			k.OverdueCount++
		}
		if x.AutoRouted {
			auto++
		}
		c, ok := cats[x.Category]
		if !ok {
			c = &CategoryStat{Category: x.Category}
			cats[x.Category] = c
			catOrder = append(catOrder, x.Category)
		}
		c.Count++
		switch {
		case st == requests.StatusImplemented:
			c.Implemented++
		case st == requests.StatusDeclined:
			c.Declined++
		case open:
			c.Open++
		}
		dn := UnassignedDepartment
		if x.Department != nil {
			dn = *x.Department
		}
		d, ok := depts[dn]
		if !ok {
			d = &DepartmentStat{Department: dn}
			depts[dn] = d
			deptOrder = append(deptOrder, dn)
		}
		d.Count++
		if open {
			d.Open++
		}
		if overdue {
			d.Overdue++
		}
	}
	k.ReviewedShare = share(reviewed, k.Total)
	k.OutcomeShare = share(withResult, closed)
	k.AutoRoutedShare = share(auto, k.Total)
	k.FirstResponseSample = len(frHours)
	k.MedianFirstResponseHours = median(frHours)
	k.MedianResolutionHours = median(resHours)
	k.FirstResponseOnTimeShare = OnTimeShare(facts)
	for _, st := range requests.AllStatuses {
		res.ByStatus = append(res.ByStatus, StatusCount{Status: st, Count: statusCount[st]})
	}
	res.ByCategory = make([]CategoryStat, 0, len(catOrder))
	for _, c := range catOrder {
		res.ByCategory = append(res.ByCategory, *cats[c])
	}
	sort.SliceStable(res.ByCategory, func(i, j int) bool { return res.ByCategory[i].Count > res.ByCategory[j].Count })
	res.ByDepartment = make([]DepartmentStat, 0, len(deptOrder))
	for _, d := range deptOrder {
		res.ByDepartment = append(res.ByDepartment, *depts[d])
	}
	sort.SliceStable(res.ByDepartment, func(i, j int) bool { return res.ByDepartment[i].Count > res.ByDepartment[j].Count })

	if res.Timeseries, err = s.timeseries(ctx, scope, f, facts, now); err != nil {
		return Summary{}, err
	}
	if s.overdue != nil {
		if res.Overdue, err = s.overdue.ListOverdue(ctx, a, f.DepartmentID, 10); err != nil {
			return Summary{}, err
		}
	}
	if res.Overdue == nil {
		res.Overdue = []requests.Summary{}
	}
	return res, nil
}

func (s *Service) timeseries(ctx context.Context, scope Scope, f SummaryFilter, facts []Fact, now time.Time) ([]Bucket, error) {
	end := s.dayStart(now)
	if f.To != nil {
		end = s.dayStart(*f.To)
	}
	var start time.Time
	switch {
	case f.From != nil:
		start = s.dayStart(*f.From)
	default:
		first, err := s.store.FirstCreated(ctx, scope)
		if err != nil {
			return nil, err
		}
		start = end.AddDate(0, 0, -29)
		if first != nil && s.dayStart(*first).Before(start) {
			start = s.dayStart(*first)
		}
	}
	if start.Before(end.AddDate(-1, 0, 0)) {
		start = end.AddDate(-1, 0, 0) // cap the chart at one year
	}
	weekly := end.Sub(start) > 62*24*time.Hour
	bucketOf := func(t time.Time) time.Time {
		d := s.dayStart(t)
		if weekly {
			off := (int(d.Weekday()) + 6) % 7 // Monday-based weeks
			d = d.AddDate(0, 0, -off)
		}
		return d
	}
	start = bucketOf(start)
	idx := map[string]int{}
	var out []Bucket
	step := 1
	if weekly {
		step = 7
	}
	for d := start; !d.After(end); d = d.AddDate(0, 0, step) {
		key := d.Format("2006-01-02")
		idx[key] = len(out)
		out = append(out, Bucket{Date: key})
	}
	for _, x := range facts {
		if i, ok := idx[bucketOf(x.CreatedAt).Format("2006-01-02")]; ok {
			out[i].Created++
		}
		if x.ClosedAt != nil {
			if i, ok := idx[bucketOf(*x.ClosedAt).Format("2006-01-02")]; ok {
				out[i].Closed++
			}
		}
	}
	if out == nil {
		out = []Bucket{}
	}
	return out, nil
}

// ParseDate parses YYYY-MM-DD in the display location.
func (s *Service) ParseDate(field, v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", v, s.loc)
	if err != nil {
		return nil, apperr.Invalid(field, "Дата в формате ГГГГ-ММ-ДД")
	}
	return &t, nil
}
