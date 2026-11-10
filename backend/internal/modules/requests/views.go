package requests

import (
	"strconv"
	"time"

	"modusvoice/internal/platform/authz"
)

// Ref is {id, name}.
type Ref struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Summary is the contract RequestSummary.
type Summary struct {
	ID             int64      `json:"id"`
	Number         string     `json:"number"`
	Type           string     `json:"type"`
	Category       Ref        `json:"category"`
	Title          string     `json:"title"`
	Status         Status     `json:"status"`
	Author         PersonView `json:"author"`
	Department     *Ref       `json:"department"`
	Assignee       *Ref       `json:"assignee"`
	RoutingSource  string     `json:"routing_source"`
	SLA            SLAState   `json:"sla"`
	CommentsCount  int        `json:"comments_count"`
	AwaitingAuthor bool       `json:"awaiting_author"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at"`
}

// AttachmentView is the contract Attachment.
type AttachmentView struct {
	ID       int64  `json:"id"`
	Filename string `json:"filename"`
	MIME     string `json:"mime"`
	Size     int64  `json:"size"`
}

// TimelineItem is the contract TimelineItem.
type TimelineItem struct {
	ID         string     `json:"id"`
	At         time.Time  `json:"at"`
	Actor      PersonView `json:"actor"`
	Kind       string     `json:"kind"`
	FromStatus *Status    `json:"from_status,omitempty"`
	ToStatus   *Status    `json:"to_status,omitempty"`
	Text       *string    `json:"text"`
}

// SimilarItem is a similar request inside the staff scope.
type SimilarItem struct {
	ID     int64   `json:"id"`
	Number string  `json:"number"`
	Title  string  `json:"title"`
	Status Status  `json:"status"`
	Score  float64 `json:"score"`
}

// SimilarView is {count, items}.
type SimilarView struct {
	Count int           `json:"count"`
	Items []SimilarItem `json:"items"`
}

// NumRef is {id, number}.
type NumRef struct {
	ID     int64  `json:"id"`
	Number string `json:"number"`
}

// ThemeRef is {id, title}.
type ThemeRef struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// Detail is the contract RequestDetail.
type Detail struct {
	Summary
	Body                string           `json:"body"`
	ProposedSolution    *string          `json:"proposed_solution"`
	ResultText          *string          `json:"result_text"`
	DuplicateOf         *NumRef          `json:"duplicate_of"`
	Theme               *ThemeRef        `json:"theme"`
	Attachments         []AttachmentView `json:"attachments"`
	Timeline            []TimelineItem   `json:"timeline"`
	AllowedStatuses     []Status         `json:"allowed_statuses"`
	AllowedCommentKinds []string         `json:"allowed_comment_kinds"`
	CanAssign           bool             `json:"can_assign"`
	Similar             SimilarView      `json:"similar"`
}

// ListResult is the contract RequestList.
type ListResult struct {
	Items    []Summary `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Counts   Counts    `json:"counts"`
}

func refOf(id *int64, name *string) *Ref {
	if id == nil {
		return nil
	}
	n := ""
	if name != nil {
		n = *name
	}
	return &Ref{ID: *id, Name: n}
}

// canManage reports whether viewer has staff powers over r.
func canManage(viewer authz.Actor, r Row) bool { return viewer.CanManage(r.DepartmentID, r.AssigneeID) }

// canRead: author, or staff in scope.
func canRead(viewer authz.Actor, r Row) bool {
	return r.Author.ID == viewer.UserID || canManage(viewer, r)
}

// toSummary maps a row for viewer; author identity passes through AuthorView.
func toSummary(r Row, viewer authz.Actor, now time.Time) Summary {
	cc := r.CommentsPublic
	if canManage(viewer, r) && r.Author.ID != viewer.UserID {
		cc = r.CommentsAll
	}
	return Summary{
		ID: r.ID, Number: r.Number, Type: r.Type, Category: Ref{ID: r.CategoryID, Name: r.CategoryName}, Title: r.Title,
		Status: r.Status, Author: AuthorView(r.Author, viewer), Department: refOf(r.DepartmentID, r.DepartmentName),
		Assignee: refOf(r.AssigneeID, r.AssigneeName), RoutingSource: r.RoutingSource, SLA: ComputeSLA(r.SLAInput(), now),
		CommentsCount: cc, AwaitingAuthor: r.Status == StatusNeedsInfo, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ClosedAt: r.ClosedAt,
	}
}

// buildTimeline merges events and comments; internal notes only for staff who are not the author.
func buildTimeline(r Row, events []EventRow, comments []CommentRow, viewer authz.Actor) []TimelineItem {
	showInternal := canManage(viewer, r) && r.Author.ID != viewer.UserID
	out := make([]TimelineItem, 0, len(events)+len(comments))
	for _, e := range events {
		kind := e.Kind
		switch kind {
		case EvCreated, EvRouted, EvAssigned, EvStatusChanged, EvThemeLinked, EvOverdue:
		case EvOverdueEscalated:
			kind = EvOverdue
		default:
			continue
		}
		out = append(out, TimelineItem{ID: "e-" + strconv.FormatInt(e.ID, 10), At: e.At, Kind: kind,
			Actor: ActorView(e.ActorID, e.ActorName, e.ActorRole, r.Author, viewer), FromStatus: e.FromStatus, ToStatus: e.ToStatus, Text: e.Text})
	}
	for _, c := range comments {
		if c.Kind == KindInternal && !showInternal {
			continue
		}
		body := c.Body
		id := c.AuthorID
		out = append(out, TimelineItem{ID: "c-" + strconv.FormatInt(c.ID, 10), At: c.CreatedAt, Kind: c.Kind,
			Actor: ActorView(&id, c.Name, c.Role, r.Author, viewer), Text: &body})
	}
	// stable merge by time; events before comments at the same second keeps status changes readable
	sortTimeline(out)
	return out
}

func sortTimeline(items []TimelineItem) {
	// insertion sort keeps relative order for equal timestamps (lists are short)
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].At.Before(items[j-1].At); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// allowedFor computes the actions viewer may perform now.
func allowedFor(r Row, viewer authz.Actor) (statuses []Status, kinds []string, canAssign bool) {
	staff := canManage(viewer, r)
	isAuthor := r.Author.ID == viewer.UserID
	statuses = []Status{}
	kinds = []string{}
	if staff {
		statuses = AllowedTransitions(r.Status)
		canAssign = !r.Status.Terminal()
		kinds = append(kinds, KindComment)
		if r.Status == StatusNew || r.Status == StatusInReview || r.Status == StatusNeedsInfo {
			kinds = append(kinds, KindQuestion)
		}
		if !isAuthor { // authors never see internal notes, so they cannot write them either
			kinds = append(kinds, KindInternal)
		}
	}
	if isAuthor {
		if !staff && !r.Status.Terminal() {
			kinds = append(kinds, KindComment)
		}
		if r.Status == StatusNeedsInfo {
			kinds = append(kinds, KindAnswer)
		}
	}
	return statuses, kinds, canAssign
}
