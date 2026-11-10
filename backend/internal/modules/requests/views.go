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
	Priority       string     `json:"priority"`
	AccessLevel    string     `json:"access_level"`
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
	// AnonymousToken is set ONLY in the 201 response of an anonymous POST /requests (shown once).
	AnonymousToken      *string          `json:"anonymous_token,omitempty"`
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

// canManage reports whether viewer OWNS r (full staff powers): admin, or manager heading the
// request's department or assigned to it.
func canManage(viewer authz.Actor, r Row) bool { return viewer.CanManage(r.DepartmentID, r.AssigneeID) }

// isObserver: a manager who can read r (CR6: managers read every request) but does not own it.
func isObserver(viewer authz.Actor, r Row) bool {
	return viewer.Role == authz.RoleManager && !canManage(viewer, r) && !isAuthorOf(viewer, r.Author)
}

// canRead: the named author, or any staff member (CR6: managers read all requests, admin all).
// Anonymous requests have no author link; their author reads them only via the token endpoints.
func canRead(viewer authz.Actor, r Row) bool {
	return isAuthorOf(viewer, r.Author) || viewer.IsStaff()
}

// seesInternal: staff who are not the author see internal notes (owners and observers).
func seesInternal(viewer authz.Actor, r Row) bool {
	return viewer.IsStaff() && !isAuthorOf(viewer, r.Author)
}

// accessLevel is the contract AccessLevel of viewer for r.
func accessLevel(viewer authz.Actor, r Row) string {
	switch {
	case isAuthorOf(viewer, r.Author):
		return AccessAuthor
	case canManage(viewer, r):
		return AccessOwner
	case viewer.Role == authz.RoleManager:
		return AccessObserver
	}
	return AccessAuthor // unreachable for authorized reads
}

// toSummary maps a row for viewer; author identity passes through AuthorView.
func toSummary(r Row, viewer authz.Actor, now time.Time) Summary {
	cc := r.CommentsPublic
	if seesInternal(viewer, r) {
		cc = r.CommentsAll
	}
	return Summary{
		ID: r.ID, Number: r.Number, Type: r.Type, Category: Ref{ID: r.CategoryID, Name: r.CategoryName}, Title: r.Title,
		Status: r.Status, Author: AuthorView(r.Author, viewer), Department: refOf(r.DepartmentID, r.DepartmentName),
		Assignee: refOf(r.AssigneeID, r.AssigneeName), RoutingSource: r.RoutingSource, Priority: r.Priority,
		AccessLevel: accessLevel(viewer, r), SLA: ComputeSLA(r.SLAInput(), now),
		CommentsCount: cc, AwaitingAuthor: r.Status == StatusNeedsInfo, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ClosedAt: r.ClosedAt,
	}
}

// buildTimeline merges events and comments; internal notes only for staff who are not the author.
func buildTimeline(r Row, events []EventRow, comments []CommentRow, viewer authz.Actor) []TimelineItem {
	showInternal := seesInternal(viewer, r)
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
			Actor: ActorView(e.ActorID, e.ByAuthor, e.ActorName, e.ActorRole, r.Author, viewer), FromStatus: e.FromStatus, ToStatus: e.ToStatus, Text: e.Text})
	}
	for _, c := range comments {
		if c.Kind == KindInternal && !showInternal {
			continue
		}
		body := c.Body
		var id *int64
		if c.AuthorID != 0 {
			v := c.AuthorID
			id = &v
		}
		out = append(out, TimelineItem{ID: "c-" + strconv.FormatInt(c.ID, 10), At: c.CreatedAt, Kind: c.Kind,
			Actor: ActorView(id, c.ByAuthor, c.Name, c.Role, r.Author, viewer), Text: &body})
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

// allowedFor computes the actions viewer may perform now. Observers (CR6) may only add
// comments and internal notes; owners have full staff powers; authors comment/answer.
func allowedFor(r Row, viewer authz.Actor) (statuses []Status, kinds []string, canAssign bool) {
	staff := canManage(viewer, r)
	isAuthor := isAuthorOf(viewer, r.Author)
	statuses = []Status{}
	kinds = []string{}
	if isObserver(viewer, r) {
		return statuses, []string{KindComment, KindInternal}, false
	}
	if staff {
		statuses = AllowedTransitions(r.Status)
		canAssign = !r.Status.Terminal()
		kinds = append(kinds, KindComment)
		if r.Status == StatusNew || r.Status == StatusInReview || r.Status == StatusNeedsInfo {
			kinds = append(kinds, KindQuestion)
		}
		if !isAuthor && CanReport(r.Status) {
			kinds = append(kinds, KindReport)
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
