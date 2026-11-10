package requests

import (
	"context"
	"errors"
	"time"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// ErrNotFound is returned by the store for missing rows.
var ErrNotFound = errors.New("requests: not found")

// Row is a request with joined display names (read model). Author identity stays internal.
type Row struct {
	ID               int64
	Number           string
	Type             string
	CategoryID       int64
	CategoryName     string
	Title            string
	Status           Status
	Author           AuthorRef
	DepartmentID     *int64
	DepartmentName   *string
	AssigneeID       *int64
	AssigneeName     *string
	RoutingSource    string
	FirstResponseDue time.Time
	ResolutionDue    time.Time
	FirstRespondedAt *time.Time
	PausedAt         *time.Time
	ClosedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CommentsAll      int
	CommentsPublic   int
}

// SLAInput returns the SLA input for the row.
func (r Row) SLAInput() SLAInput {
	return SLAInput{Status: r.Status, CreatedAt: r.CreatedAt, FirstResponseDue: r.FirstResponseDue,
		ResolutionDue: r.ResolutionDue, FirstRespondedAt: r.FirstRespondedAt}
}

// DetailRow adds the long fields.
type DetailRow struct {
	Row
	Body              string
	ProposedSolution  *string
	ResultText        *string
	DuplicateOfID     *int64
	DuplicateOfNumber *string
	ThemeID           *int64
	ThemeTitle        *string
}

// NewRequest is inserted on create.
type NewRequest struct {
	Number           string
	Type             string
	CategoryID       int64
	Title            string
	Body             string
	ProposedSolution *string
	AuthorID         int64
	IsAnonymous      bool
	DepartmentID     *int64
	AssigneeID       *int64
	RoutingSource    string
	FirstResponseDue time.Time
	ResolutionDue    time.Time
	CreatedAt        time.Time
}

// Event is a request_events row.
type Event struct {
	RequestID  int64
	ActorID    *int64
	Kind       string
	FromStatus *Status
	ToStatus   *Status
	Text       *string
	Meta       *string
	At         time.Time
}

// EventRow is an event with actor info for the timeline.
type EventRow struct {
	Event
	ID        int64
	ActorName string
	ActorRole authz.Role
}

// CommentRow is a comment with author info.
type CommentRow struct {
	ID        int64
	AuthorID  int64
	Name      string
	Role      authz.Role
	Kind      string
	Body      string
	CreatedAt time.Time
}

// AttachmentRow is a stored attachment.
type AttachmentRow struct {
	ID         int64
	RequestID  int64
	Filename   string
	StoredName string
	MIME       string
	Size       int64
}

// Theme is a confirmed grouping of requests.
type Theme struct {
	ID    int64
	Title string
}

// ListFilter is a fully-resolved list query (scope predicate built from authz.ScopeSQL).
type ListFilter struct {
	ScopeSQL     string
	ScopeArgs    []any
	Status       *Status
	CategoryID   *int64
	DepartmentID *int64
	AssigneeID   *int64
	AssigneeNone bool
	Overdue      *bool
	Query        string
	Tab          string
	Sort         string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	Limit        int
	Offset       int
	Now          time.Time
}

// Counts are the inbox tab counters.
type Counts struct {
	NeedsAction   int `json:"needs_action"`
	InProgress    int `json:"in_progress"`
	WaitingAuthor int `json:"waiting_author"`
	Closed        int `json:"closed"`
	All           int `json:"all"`
	Overdue       int `json:"overdue"`
}

// Store persists requests and their history. Methods taking q run inside the caller's tx.
type Store interface {
	NextNumber(ctx context.Context, q db.Querier) (int64, error)
	InsertRequest(ctx context.Context, q db.Querier, r NewRequest) (int64, error)
	InsertEvent(ctx context.Context, q db.Querier, e Event) error
	InsertComment(ctx context.Context, q db.Querier, requestID, authorID int64, kind, body string, at time.Time) (int64, error)
	InsertAttachment(ctx context.Context, q db.Querier, requestID, uploadedBy int64, f storedFile, at time.Time) error
	Get(ctx context.Context, q db.Querier, id int64) (DetailRow, error)
	Save(ctx context.Context, q db.Querier, r DetailRow) error
	HasEvent(ctx context.Context, q db.Querier, requestID int64, kind, meta string) (bool, error)

	List(ctx context.Context, f ListFilter) ([]Row, int, error)
	Counts(ctx context.Context, f ListFilter) (Counts, error)
	Events(ctx context.Context, requestID int64) ([]EventRow, error)
	Comments(ctx context.Context, requestID int64) ([]CommentRow, error)
	Attachments(ctx context.Context, requestID int64) ([]AttachmentRow, error)
	Attachment(ctx context.Context, id int64) (AttachmentRow, error)
	OverdueCandidates(ctx context.Context, now time.Time) ([]Row, error)
	CountInCategory(ctx context.Context, categoryID int64, since time.Time) (int, error)

	ThemeByID(ctx context.Context, q db.Querier, id int64) (Theme, error)
	ThemeByTitle(ctx context.Context, q db.Querier, title string) (Theme, error)
	InsertTheme(ctx context.Context, q db.Querier, title string, by int64, at time.Time) (int64, error)
}
