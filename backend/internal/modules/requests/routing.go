package requests

import "time"

// Default SLA for unrouted requests (admin queue) so every request still has deadlines (TZ 2.5).
const (
	UnroutedFirstResponseHours = 24
	UnroutedResolutionHours    = 168
)

// RuleCandidate is a routing rule as seen by the routing engine.
type RuleCandidate struct {
	ID                 int64
	DepartmentID       int64
	DepartmentName     string
	AssigneeID         *int64
	FirstResponseHours int
	ResolutionHours    int
}

// Owner is a potential assignee with its active flag.
type Owner struct {
	ID     int64
	Name   string
	Active bool
}

// Route is the routing decision for a new request.
type Route struct {
	Source             string // rule | unrouted
	RuleID             *int64
	DepartmentID       *int64
	DepartmentName     string
	AssigneeID         *int64
	AssigneeName       string
	FirstResponseHours int
	ResolutionHours    int
}

// Routed reports whether the request has an owner (not in the admin queue).
func (r Route) Routed() bool { return r.Source == SourceRule }

// Due returns (first response due, resolution due) for a request created at t.
func (r Route) Due(t time.Time) (time.Time, time.Time) {
	return t.Add(time.Duration(r.FirstResponseHours) * time.Hour), t.Add(time.Duration(r.ResolutionHours) * time.Hour)
}

// ResolveRoute is the routing engine (pure): rule by (category,type) -> rule by category -> unrouted.
// Assignee fallback: rule assignee if active -> department head if active -> none (=> unrouted, admin queue).
// ruleAssignee and head are the resolved people for the chosen rule (nil when absent).
func ResolveRoute(exact, categoryWide *RuleCandidate, ruleAssignee, head *Owner) Route {
	rule := exact
	if rule == nil {
		rule = categoryWide
	}
	if rule == nil {
		return Route{Source: SourceUnrouted, FirstResponseHours: UnroutedFirstResponseHours, ResolutionHours: UnroutedResolutionHours}
	}
	dept := rule.DepartmentID
	id := rule.ID
	rt := Route{Source: SourceRule, RuleID: &id, DepartmentID: &dept, DepartmentName: rule.DepartmentName,
		FirstResponseHours: rule.FirstResponseHours, ResolutionHours: rule.ResolutionHours}
	switch {
	case ruleAssignee != nil && ruleAssignee.Active:
		a := ruleAssignee.ID
		rt.AssigneeID, rt.AssigneeName = &a, ruleAssignee.Name
	case head != nil && head.Active:
		h := head.ID
		rt.AssigneeID, rt.AssigneeName = &h, head.Name
	default:
		rt.Source = SourceUnrouted // department known but nobody can own it: admin queue
	}
	return rt
}
