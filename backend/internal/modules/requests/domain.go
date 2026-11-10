// Package requests is the core bounded context: requests, workflow, comments, timeline,
// attachments, routing engine and SLA.
package requests

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"modusvoice/internal/platform/apperr"
)

// Status of a request.
type Status string

const (
	StatusNew         Status = "new"
	StatusInReview    Status = "in_review"
	StatusNeedsInfo   Status = "needs_info"
	StatusDecision    Status = "decision"
	StatusInProgress  Status = "in_progress"
	StatusImplemented Status = "implemented"
	StatusDeclined    Status = "declined"
	StatusDuplicate   Status = "duplicate"
	StatusClosed      Status = "closed"
)

// AllStatuses in display order.
var AllStatuses = []Status{StatusNew, StatusInReview, StatusNeedsInfo, StatusDecision, StatusInProgress, StatusImplemented, StatusDeclined, StatusDuplicate, StatusClosed}

var statusLabels = map[Status]string{
	StatusNew:         "Новое",
	StatusInReview:    "В проработке",
	StatusNeedsInfo:   "Нужно уточнение",
	StatusDecision:    "Принятие решения",
	StatusInProgress:  "Реализация",
	StatusImplemented: "Реализовано",
	StatusDeclined:    "Отклонено",
	StatusDuplicate:   "Дубликат",
	StatusClosed:      "Закрыто",
}

// Label returns the Russian label.
func (s Status) Label() string { return statusLabels[s] }

// Valid reports whether s is a known status.
func (s Status) Valid() bool { _, ok := statusLabels[s]; return ok }

// Terminal reports whether s closes the request.
func (s Status) Terminal() bool {
	return s == StatusImplemented || s == StatusDeclined || s == StatusDuplicate || s == StatusClosed
}

// transitions is the R10 table.
var transitions = map[Status][]Status{
	StatusNew:        {StatusInReview, StatusNeedsInfo, StatusDecision, StatusDeclined, StatusDuplicate, StatusClosed},
	StatusInReview:   {StatusNeedsInfo, StatusDecision, StatusDeclined, StatusDuplicate, StatusClosed},
	StatusNeedsInfo:  {StatusInReview, StatusClosed},
	StatusDecision:   {StatusInProgress, StatusInReview, StatusDeclined, StatusClosed},
	StatusInProgress: {StatusImplemented, StatusDeclined, StatusDecision, StatusClosed},
}

// CanTransition reports whether from -> to is allowed.
func CanTransition(from, to Status) bool { return slices.Contains(transitions[from], to) }

// AllowedTransitions lists statuses reachable from s.
func AllowedTransitions(s Status) []Status { return append([]Status{}, transitions[s]...) }

// Request types (CR1: feedback = Положительный отзыв).
const (
	TypeProblem  = "problem"
	TypeIdea     = "idea"
	TypeFeedback = "feedback"
)

// AllTypes in display order.
var AllTypes = []string{TypeProblem, TypeIdea, TypeFeedback}

var typeLabels = map[string]string{TypeProblem: "Проблема", TypeIdea: "Предложение", TypeFeedback: "Положительный отзыв"}

// TypeLabel returns the Russian label of a request type.
func TypeLabel(t string) string { return typeLabels[t] }

// ValidType reports whether t is a known request type.
func ValidType(t string) bool { _, ok := typeLabels[t]; return ok }

const errTypeMsg = "Выберите тип: проблема, предложение или положительный отзыв"

// Priorities (CR1).
const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

// AllPriorities in display order (lowest first).
var AllPriorities = []string{PriorityLow, PriorityMedium, PriorityHigh}

var priorityLabels = map[string]string{PriorityLow: "Низкий", PriorityMedium: "Средний", PriorityHigh: "Высокий"}

// PriorityLabel returns the Russian label of a priority.
func PriorityLabel(p string) string { return priorityLabels[p] }

// ValidPriority reports whether p is a known priority.
func ValidPriority(p string) bool { _, ok := priorityLabels[p]; return ok }

// Access levels of the viewer to one request (CR6).
const (
	AccessOwner    = "owner"    // admin, or manager owning the request's department / assigned to it
	AccessObserver = "observer" // manager of another department: read + comment/internal only
	AccessAuthor   = "author"   // the author (named, or anonymous via token)
)

// Comment kinds.
const (
	KindComment  = "comment"
	KindQuestion = "question"
	KindAnswer   = "answer"
	KindInternal = "internal"
	KindReport   = "report"
)

// ValidCommentKind reports whether k is a known comment kind.
func ValidCommentKind(k string) bool {
	return k == KindComment || k == KindQuestion || k == KindAnswer || k == KindInternal || k == KindReport
}

// Routing sources.
const (
	SourceRule     = "rule"
	SourceManual   = "manual"
	SourceUnrouted = "unrouted"
)

// Event kinds stored in request_events (subset shown in timeline).
const (
	EvCreated          = "created"
	EvRouted           = "routed"
	EvAssigned         = "assigned"
	EvStatusChanged    = "status_changed"
	EvThemeLinked      = "theme_linked"
	EvOverdue          = "overdue"
	EvOverdueEscalated = "overdue_escalated"
)

// CanReport reports whether a progress report may be posted in status s.
func CanReport(s Status) bool {
	return s == StatusInReview || s == StatusDecision || s == StatusInProgress
}

// Limits from the contract.
const (
	TitleMin       = 5
	TitleMax       = 140
	BodyMin        = 20
	BodyMax        = 4000
	SolutionMax    = 2000
	CommentMax     = 4000
	ResultMax      = 4000
	ThemeTitleMax  = 140
	MaxFiles       = 5
	MaxFileBytes   = 10 << 20
	MaxTotalBytes  = 10 << 20
	AnonymousLabel = "Автор (анонимно)"
	SystemLabel    = "Система"
)

// CreateInput is the CreateRequest payload.
type CreateInput struct {
	Type             string `json:"type"`
	CategoryID       int64  `json:"category_id"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ProposedSolution string `json:"proposed_solution"`
	IsAnonymous      bool   `json:"is_anonymous"`
	Priority         string `json:"priority"`
}

// Normalize trims text fields and applies the default priority.
func (in *CreateInput) Normalize() {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	in.ProposedSolution = strings.TrimSpace(in.ProposedSolution)
	in.Priority = strings.TrimSpace(in.Priority)
	if in.Priority == "" {
		in.Priority = PriorityMedium
	}
}

// Validate checks field limits (category existence is checked by the service).
func (in CreateInput) Validate() error {
	var v apperr.V
	if !ValidType(in.Type) {
		v.Add("type", errTypeMsg)
	}
	if in.Priority != "" && !ValidPriority(in.Priority) { // "" = default (medium), set by Normalize
		v.Add("priority", "Приоритет: low, medium или high")
	}
	if in.CategoryID <= 0 {
		v.Add("category_id", "Выберите категорию")
	}
	if n := utf8.RuneCountInString(in.Title); n < TitleMin || n > TitleMax {
		v.Add("title", "Заголовок: от 5 до 140 символов")
	}
	if n := utf8.RuneCountInString(in.Body); n < BodyMin || n > BodyMax {
		v.Add("body", "Описание: от 20 до 4000 символов")
	}
	if utf8.RuneCountInString(in.ProposedSolution) > SolutionMax {
		v.Add("proposed_solution", "Предлагаемое решение: не более 2000 символов")
	}
	if !utf8.ValidString(in.Title) || !utf8.ValidString(in.Body) || !utf8.ValidString(in.ProposedSolution) {
		v.Add("body", "Текст содержит недопустимые символы")
	}
	return v.Err()
}

// FormatNumber renders the public request number.
func FormatNumber(n int64) string {
	return fmt.Sprintf("MV-%04d", n)
}
