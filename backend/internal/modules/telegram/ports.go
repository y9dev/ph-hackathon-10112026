package telegram

import (
	"context"
	"time"

	"modusvoice/internal/platform/authz"
)

// ReqItem is one line of a request list as the bot shows it.
type ReqItem struct {
	ID          int64
	Number      string
	Title       string
	Status      string // machine status, e.g. in_review
	StatusLabel string // Russian label
	Overdue     bool
	Anonymous   bool
}

// ReqCard is the request summary of /request.
type ReqCard struct {
	ReqItem
	Type         string
	Category     string
	Department   string
	Assignee     string
	IsAuthor     bool // the viewer is the author
	CreatedAt    time.Time
	Result       string // result text of a closed request
	LatestReport string // newest progress report
	Question     string // newest staff question (shown while the status is needs_info)
}

// InboxView is the staff inbox: counters plus the first items needing action.
type InboxView struct {
	NeedsAction   int
	Overdue       int
	WaitingAuthor int
	All           int
	Items         []ReqItem
}

// AdminStats is the /admin summary.
type AdminStats struct {
	Total    int
	Open     int
	Unrouted int
	Overdue  int
	NewToday int
}

// RequestsAPI is everything the bot needs from the requests module. Every method takes the linked
// user as Actor and must enforce the same authorization as the HTTP API (the adapter delegates to
// requests.Service). Implemented by RequestsAdapter.
type RequestsAPI interface {
	// FindID resolves a request number (MV-0001) to its id without any access decision.
	FindID(ctx context.Context, number string) (int64, bool, error)
	// MyRequests returns up to limit newest own, non-anonymous requests.
	MyRequests(ctx context.Context, a authz.Actor, limit int) ([]ReqItem, error)
	// Inbox is staff only.
	Inbox(ctx context.Context, a authz.Actor, limit int) (InboxView, error)
	// Card returns the request if the actor may read it (apperr NotFound otherwise).
	Card(ctx context.Context, a authz.Actor, id int64) (ReqCard, error)
	// Answer posts the author's answer to a clarification question.
	Answer(ctx context.Context, a authz.Actor, id int64, text string) error
	// AdminStats is admin only.
	AdminStats(ctx context.Context, a authz.Actor) (AdminStats, error)
}
