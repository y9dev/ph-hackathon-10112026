// Package notifications owns in-app notifications. Creation is a synchronous call inside the
// caller's transaction (AC-08); texts arrive already masked by the requests AuthorView mapper.
package notifications

import "time"

// Kinds of notifications.
const (
	KindCreated          = "created"
	KindStatusChanged    = "status_changed"
	KindQuestion         = "question"
	KindAnswer           = "answer"
	KindComment          = "comment"
	KindAssigned         = "assigned"
	KindOverdue          = "overdue"
	KindOverdueEscalated = "overdue_escalated"
)

// Note is a notification to be written.
type Note struct {
	UserID    int64
	Kind      string
	RequestID int64
	Title     string
	Body      string
}

// View is the API representation.
type View struct {
	ID            int64     `json:"id"`
	Kind          string    `json:"kind"`
	RequestID     int64     `json:"request_id"`
	RequestNumber string    `json:"request_number"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	Read          bool      `json:"read"`
	CreatedAt     time.Time `json:"created_at"`
}

// List is the GET /notifications response.
type List struct {
	UnreadCount int    `json:"unread_count"`
	Items       []View `json:"items"`
}

// Dedupe keeps one entry per (user, request) in a batch and drops entries for skipUser (the actor).
func Dedupe(ns []Note, skipUser int64) []Note {
	seen := map[[2]int64]bool{}
	out := ns[:0:0]
	for _, n := range ns {
		if n.UserID == 0 || n.UserID == skipUser {
			continue
		}
		k := [2]int64{n.UserID, n.RequestID}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, n)
	}
	return out
}
