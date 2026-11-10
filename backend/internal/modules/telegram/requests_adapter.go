package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// RequestsAdapter implements RequestsAPI on top of requests.Service. All reads and the answer
// go through the service, so visibility, masking and workflow rules are exactly those of the web
// API. The only direct SQL is the number lookup and the admin counters.
type RequestsAdapter struct {
	Svc *requests.Service
	DB  *sql.DB
	Loc *time.Location
	Now func() time.Time
}

func toItem(s requests.Summary) ReqItem {
	return ReqItem{
		ID: s.ID, Number: s.Number, Title: s.Title, Status: string(s.Status), StatusLabel: s.Status.Label(),
		Overdue: s.SLA.State == "overdue", Anonymous: s.Author.Anonymous,
	}
}

func (r *RequestsAdapter) FindID(ctx context.Context, number string) (int64, bool, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `SELECT id FROM requests WHERE number = ?`, number).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find request by number: %w", err)
	}
	return id, true, nil
}

func (r *RequestsAdapter) MyRequests(ctx context.Context, a authz.Actor, limit int) ([]ReqItem, error) {
	res, err := r.Svc.List(ctx, a, requests.ListQuery{Scope: "mine", Sort: "newest", PageSize: limit * 3})
	if err != nil {
		return nil, err
	}
	out := make([]ReqItem, 0, limit)
	for _, s := range res.Items {
		if s.Author.Anonymous { // anonymous requests are never reachable from the bot
			continue
		}
		out = append(out, toItem(s))
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (r *RequestsAdapter) Inbox(ctx context.Context, a authz.Actor, limit int) (InboxView, error) {
	res, err := r.Svc.List(ctx, a, requests.ListQuery{Scope: "inbox", Tab: "needs_action", Sort: "deadline", PageSize: limit})
	if err != nil {
		return InboxView{}, err
	}
	v := InboxView{NeedsAction: res.Counts.NeedsAction, Overdue: res.Counts.Overdue, WaitingAuthor: res.Counts.WaitingAuthor, All: res.Counts.All}
	for _, s := range res.Items {
		v.Items = append(v.Items, toItem(s))
	}
	return v, nil
}

func (r *RequestsAdapter) Card(ctx context.Context, a authz.Actor, id int64) (ReqCard, error) {
	d, err := r.Svc.Get(ctx, a, id)
	if err != nil {
		return ReqCard{}, err
	}
	c := ReqCard{ReqItem: toItem(d.Summary), Type: d.Type, Category: d.Category.Name, IsAuthor: d.Author.IsMe, CreatedAt: d.CreatedAt}
	if d.Department != nil {
		c.Department = d.Department.Name
	}
	if d.Assignee != nil {
		c.Assignee = d.Assignee.Name
	}
	if d.ResultText != nil {
		c.Result = *d.ResultText
	}
	for _, t := range d.Timeline {
		if t.Text == nil {
			continue
		}
		switch t.Kind {
		case requests.KindReport:
			c.LatestReport = *t.Text
		case requests.KindQuestion:
			c.Question = *t.Text
		}
	}
	return c, nil
}

func (r *RequestsAdapter) Answer(ctx context.Context, a authz.Actor, id int64, text string) error {
	_, err := r.Svc.AddComment(ctx, a, id, requests.CommentInput{Kind: requests.KindAnswer, Body: text})
	return err
}

func (r *RequestsAdapter) AdminStats(ctx context.Context, a authz.Actor) (AdminStats, error) {
	if !a.IsAdmin() {
		return AdminStats{}, apperr.Forbidden()
	}
	now := r.Now()
	loc := r.Loc
	if loc == nil {
		loc = time.UTC
	}
	ln := now.In(loc)
	dayStart := time.Date(ln.Year(), ln.Month(), ln.Day(), 0, 0, 0, 0, loc)
	var s AdminStats
	err := r.DB.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN status NOT IN ('implemented','declined','duplicate','closed') THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN routing_source = 'unrouted' AND status NOT IN ('implemented','declined','duplicate','closed') THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0)
		FROM requests`, db.Unix(dayStart)).Scan(&s.Total, &s.Open, &s.Unrouted, &s.NewToday)
	if err != nil {
		return AdminStats{}, fmt.Errorf("admin stats: %w", err)
	}
	res, err := r.Svc.List(ctx, a, requests.ListQuery{Scope: "all", PageSize: 1})
	if err != nil {
		return AdminStats{}, err
	}
	s.Overdue = res.Counts.Overdue
	return s, nil
}
