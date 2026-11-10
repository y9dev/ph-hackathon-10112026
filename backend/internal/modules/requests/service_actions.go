package requests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
	"modusvoice/internal/platform/eventbus"
)

// loadForUpdate loads a request inside tx and checks read access (404) and, if staff is
// required, staff scope (403).
func (s *Service) loadForUpdate(ctx context.Context, tx *sql.Tx, a authz.Actor, id int64, staff bool) (DetailRow, error) {
	r, err := s.Store.Get(ctx, tx, id)
	if errors.Is(err, ErrNotFound) {
		return DetailRow{}, apperr.NotFound()
	}
	if err != nil {
		return DetailRow{}, err
	}
	if !canRead(a, r.Row) {
		return DetailRow{}, apperr.NotFound()
	}
	if staff && !canManage(a, r.Row) {
		return DetailRow{}, apperr.Forbidden()
	}
	return r, nil
}

// markFirstResponse stamps first_responded_at on the first staff action by someone other than the author.
func markFirstResponse(r *Row, a authz.Actor, now time.Time) {
	if r.FirstRespondedAt == nil && a.UserID != r.Author.ID && canManage(a, *r) {
		t := now
		r.FirstRespondedAt = &t
	}
}

func (s *Service) reload(ctx context.Context, a authz.Actor, id int64) (Detail, error) {
	d, err := s.Store.Get(ctx, s.DB, id)
	if err != nil {
		return Detail{}, err
	}
	return s.buildDetail(ctx, a, d)
}

// StatusInput is the POST /requests/{id}/status body.
type StatusInput struct {
	Status        string  `json:"status"`
	ResultText    *string `json:"result_text"`
	DuplicateOfID *int64  `json:"duplicate_of_id"`
}

// ChangeStatus applies a workflow transition (staff in scope).
func (s *Service) ChangeStatus(ctx context.Context, a authz.Actor, id int64, in StatusInput) (Detail, error) {
	to := Status(in.Status)
	if !to.Valid() {
		return Detail{}, apperr.Invalid("status", "Неизвестный статус")
	}
	result := ""
	if in.ResultText != nil {
		result = strings.TrimSpace(*in.ResultText)
	}
	if utf8.RuneCountInString(result) > ResultMax {
		return Detail{}, apperr.Invalid("result_text", "Результат: не более 4000 символов")
	}
	now := s.now()
	var saved DetailRow
	var from Status
	err := db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		r, err := s.loadForUpdate(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if !CanTransition(r.Status, to) {
			return apperr.Conflict(fmt.Sprintf("Переход из статуса «%s» в «%s» недоступен", r.Status.Label(), to.Label()))
		}
		if to.Terminal() && result == "" {
			return apperr.Invalid("result_text", "Укажите результат рассмотрения — его увидит автор")
		}
		if to == StatusDuplicate {
			if in.DuplicateOfID == nil || *in.DuplicateOfID == id {
				return apperr.Invalid("duplicate_of_id", "Укажите обращение-оригинал")
			}
			orig, err := s.Store.Get(ctx, tx, *in.DuplicateOfID)
			if errors.Is(err, ErrNotFound) || (err == nil && !canManage(a, orig.Row)) {
				return apperr.Invalid("duplicate_of_id", "Обращение-оригинал не найдено в вашей зоне ответственности")
			}
			if err != nil {
				return err
			}
			r.DuplicateOfID = &orig.ID
		}
		from = r.Status
		applyTransition(&r.Row, to, now)
		if to.Terminal() {
			r.ResultText = &result
		}
		markFirstResponse(&r.Row, a, now)
		r.UpdatedAt = now
		if err := s.Store.Save(ctx, tx, r); err != nil {
			return err
		}
		var text *string
		if result != "" {
			text = &result
		}
		if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: id, ActorID: &a.UserID, Kind: EvStatusChanged, FromStatus: &from, ToStatus: &to, Text: text, At: now}); err != nil {
			return err
		}
		body := excerpt(r.Title, 200)
		if result != "" {
			body = excerpt(result, 300)
		}
		saved = r
		return s.Notifier.Write(ctx, tx, a.UserID, notes([]int64{r.Author.ID}, notifications.KindStatusChanged, id,
			fmt.Sprintf("%s: статус «%s»", r.Number, to.Label()), body))
	})
	if err != nil {
		return Detail{}, err
	}
	s.publish(ctx, eventbus.RequestStatusChanged, saved.Row, map[string]any{"from": from, "to": to})
	if to.Terminal() {
		s.publish(ctx, eventbus.RequestClosed, saved.Row, map[string]any{"status": to})
	}
	return s.reload(ctx, a, id)
}

// AssignInput is the POST /requests/{id}/assign body (AssigneeSet distinguishes explicit null).
type AssignInput struct {
	DepartmentID *int64
	AssigneeSet  bool
	AssigneeID   *int64
}

// Assign reassigns department and/or assignee (staff in scope).
func (s *Service) Assign(ctx context.Context, a authz.Actor, id int64, in AssignInput) (Detail, error) {
	if in.DepartmentID == nil && !in.AssigneeSet {
		return Detail{}, apperr.Invalid("department_id", "Укажите отдел или ответственного")
	}
	var deptName string
	if in.DepartmentID != nil {
		d, err := s.Org.Department(ctx, *in.DepartmentID)
		if apperr.Is(err, apperr.KindNotFound) {
			return Detail{}, apperr.Invalid("department_id", "Отдел не найден")
		}
		if err != nil {
			return Detail{}, err
		}
		deptName = d.Name
	}
	var assigneeName string
	if in.AssigneeSet && in.AssigneeID != nil {
		o, err := s.owner(ctx, in.AssigneeID)
		if err != nil {
			return Detail{}, err
		}
		if o == nil || !o.Active {
			return Detail{}, apperr.Invalid("assignee_id", "Ответственным может быть только активный руководитель или администратор")
		}
		assigneeName = o.Name
	}
	now := s.now()
	var saved DetailRow
	changed := false
	err := db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		r, err := s.loadForUpdate(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if r.Status.Terminal() {
			return apperr.Conflict("Закрытое обращение нельзя переназначить")
		}
		newDept, newAssignee := r.DepartmentID, r.AssigneeID
		if in.DepartmentID != nil {
			newDept = in.DepartmentID
		} else if r.DepartmentName != nil {
			deptName = *r.DepartmentName
		}
		deptChanged := !eqID(newDept, r.DepartmentID)
		if in.AssigneeSet {
			newAssignee = in.AssigneeID
		} else if deptChanged {
			head, err := s.departmentHead(ctx, newDept)
			if err != nil {
				return err
			}
			newAssignee = nil
			if head != nil && head.Active {
				newAssignee, assigneeName = &head.ID, head.Name
			}
		} else if r.AssigneeName != nil {
			assigneeName = *r.AssigneeName
		}
		if !deptChanged && eqID(newAssignee, r.AssigneeID) {
			saved = r
			return nil
		}
		changed = true
		r.DepartmentID, r.AssigneeID = newDept, newAssignee
		owners, err := s.ownerIDs(ctx, newDept, newAssignee)
		if err != nil {
			return err
		}
		r.RoutingSource = SourceManual
		if newAssignee == nil {
			if head, err := s.departmentHead(ctx, newDept); err != nil {
				return err
			} else if head == nil || !head.Active {
				r.RoutingSource = SourceUnrouted
			}
		}
		markFirstResponse(&r.Row, a, now)
		r.UpdatedAt = now
		if err := s.Store.Save(ctx, tx, r); err != nil {
			return err
		}
		text := "Отдел: " + orDash(deptName) + "; ответственный: " + orDash(assigneeName)
		if newDept == nil {
			text = "Отдел: —; ответственный: " + orDash(assigneeName)
		}
		if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: id, ActorID: &a.UserID, Kind: EvAssigned, Text: &text, Meta: ptr(r.RoutingSource), At: now}); err != nil {
			return err
		}
		saved = r
		return s.Notifier.Write(ctx, tx, a.UserID, notes(owners, notifications.KindAssigned, id,
			fmt.Sprintf("Вам передано обращение %s", r.Number), excerpt(r.Title, 200)))
	})
	if err != nil {
		return Detail{}, err
	}
	if changed {
		s.publish(ctx, eventbus.RequestAssigned, saved.Row, map[string]any{"department_id": saved.DepartmentID, "assignee_id": saved.AssigneeID})
	}
	// The actor may have handed the request out of their own scope; still return the resulting card.
	return s.reload(ctx, a, id)
}

func eqID(x, y *int64) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	return *x == *y
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// CommentInput is the POST /requests/{id}/comments body.
type CommentInput struct {
	Kind string `json:"kind"`
	Body string `json:"body"`
}

// AddComment adds a comment/question/answer/internal note with the workflow side effects.
func (s *Service) AddComment(ctx context.Context, a authz.Actor, id int64, in CommentInput) (Detail, error) {
	if !ValidCommentKind(in.Kind) {
		return Detail{}, apperr.Invalid("kind", "Тип: comment, question, answer или internal")
	}
	body := strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(body); n < 1 || n > CommentMax || !utf8.ValidString(body) {
		return Detail{}, apperr.Invalid("body", "Текст: от 1 до 4000 символов")
	}
	now := s.now()
	var saved DetailRow
	topic := eventbus.RequestCommented
	err := db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		r, err := s.loadForUpdate(ctx, tx, a, id, false)
		if err != nil {
			return err
		}
		staff := canManage(a, r.Row)
		isAuthor := r.Author.ID == a.UserID
		switch in.Kind {
		case KindComment:
			if !staff && !(isAuthor && !r.Status.Terminal()) {
				if isAuthor {
					return apperr.Conflict("Обращение закрыто. Создайте новое обращение со ссылкой на номер " + r.Number)
				}
				return apperr.Forbidden()
			}
		case KindQuestion:
			if !staff {
				return apperr.Forbidden()
			}
			if r.Status != StatusNew && r.Status != StatusInReview && r.Status != StatusNeedsInfo {
				return apperr.Conflict("Запросить уточнение можно только у нового обращения или обращения на рассмотрении")
			}
		case KindAnswer:
			if !isAuthor {
				return apperr.Forbidden()
			}
			if r.Status != StatusNeedsInfo {
				return apperr.Conflict("Ответить на уточнение можно, только когда оно запрошено")
			}
		case KindInternal:
			if !staff || isAuthor {
				return apperr.Forbidden()
			}
		}
		if _, err := s.Store.InsertComment(ctx, tx, id, a.UserID, in.Kind, body, now); err != nil {
			return err
		}
		from := r.Status
		switch {
		case in.Kind == KindQuestion && r.Status != StatusNeedsInfo:
			applyTransition(&r.Row, StatusNeedsInfo, now)
		case in.Kind == KindAnswer:
			applyTransition(&r.Row, StatusInReview, now)
		}
		if r.Status != from {
			to := r.Status
			if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: id, ActorID: &a.UserID, Kind: EvStatusChanged, FromStatus: &from, ToStatus: &to, At: now}); err != nil {
				return err
			}
		}
		if in.Kind != KindInternal {
			markFirstResponse(&r.Row, a, now)
		}
		r.UpdatedAt = now
		if err := s.Store.Save(ctx, tx, r); err != nil {
			return err
		}
		saved = r
		// notification texts are rendered with the actor masked for each recipient (R2)
		actorName := func(recipient int64) string {
			return ActorView(&a.UserID, a.Name, a.Role, r.Author, recipientViewer(recipient)).Name
		}
		short := excerpt(body, 300)
		var ns []notifications.Note
		switch {
		case in.Kind == KindInternal:
		case in.Kind == KindQuestion:
			topic = eventbus.RequestQuestionAsked
			ns = notes([]int64{r.Author.ID}, notifications.KindQuestion, id, fmt.Sprintf("%s: нужен ваш ответ", r.Number), short)
		case in.Kind == KindAnswer || (in.Kind == KindComment && isAuthor && !staff):
			owners, err := s.ownerIDs(ctx, r.DepartmentID, r.AssigneeID)
			if err != nil {
				return err
			}
			kind, title := notifications.KindComment, "%s: новый комментарий от %s"
			if in.Kind == KindAnswer {
				topic = eventbus.RequestAnswered
				kind, title = notifications.KindAnswer, "%s: ответ на уточнение от %s"
			}
			for _, o := range owners {
				ns = append(ns, notifications.Note{UserID: o, Kind: kind, RequestID: id, Title: fmt.Sprintf(title, r.Number, actorName(o)), Body: short})
			}
		default: // staff comment -> author
			ns = notes([]int64{r.Author.ID}, notifications.KindComment, id,
				fmt.Sprintf("%s: новый комментарий от %s", r.Number, actorName(r.Author.ID)), short)
		}
		return s.Notifier.Write(ctx, tx, a.UserID, ns)
	})
	if err != nil {
		return Detail{}, err
	}
	data := map[string]any{"kind": in.Kind}
	if saved.Author.ID != a.UserID || !saved.Author.IsAnonymous {
		data["actor_id"] = a.UserID
	}
	s.publish(ctx, topic, saved.Row, data)
	return s.reload(ctx, a, id)
}

// ThemeInput is the POST /requests/{id}/theme body.
type ThemeInput struct {
	ThemeID *int64  `json:"theme_id"`
	Title   *string `json:"title"`
}

// LinkTheme links the request to an existing theme or a new one by title (staff in scope).
func (s *Service) LinkTheme(ctx context.Context, a authz.Actor, id int64, in ThemeInput) (Detail, error) {
	title := ""
	if in.Title != nil {
		title = strings.TrimSpace(*in.Title)
	}
	if in.ThemeID == nil {
		if n := utf8.RuneCountInString(title); n < 2 || n > ThemeTitleMax {
			return Detail{}, apperr.Invalid("title", "Название темы: от 2 до 140 символов")
		}
	}
	now := s.now()
	err := db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		r, err := s.loadForUpdate(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		var t Theme
		if in.ThemeID != nil {
			t, err = s.Store.ThemeByID(ctx, tx, *in.ThemeID)
			if errors.Is(err, ErrNotFound) {
				return apperr.Invalid("theme_id", "Тема не найдена")
			}
			if err != nil {
				return err
			}
		} else {
			t, err = s.Store.ThemeByTitle(ctx, tx, title)
			if errors.Is(err, ErrNotFound) {
				tid, err := s.Store.InsertTheme(ctx, tx, title, a.UserID, now)
				if err != nil {
					return err
				}
				t = Theme{ID: tid, Title: title}
			} else if err != nil {
				return err
			}
		}
		if r.ThemeID != nil && *r.ThemeID == t.ID {
			return nil
		}
		r.ThemeID = &t.ID
		r.UpdatedAt = now
		if err := s.Store.Save(ctx, tx, r); err != nil {
			return err
		}
		return s.Store.InsertEvent(ctx, tx, Event{RequestID: id, ActorID: &a.UserID, Kind: EvThemeLinked, Text: ptr("Тема: " + t.Title), At: now})
	})
	if err != nil {
		return Detail{}, err
	}
	return s.reload(ctx, a, id)
}

// AttachmentFile resolves a downloadable attachment the actor may read.
func (s *Service) AttachmentFile(ctx context.Context, a authz.Actor, id int64) (AttachmentRow, error) {
	at, err := s.Store.Attachment(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return AttachmentRow{}, apperr.NotFound()
	}
	if err != nil {
		return AttachmentRow{}, err
	}
	r, err := s.Store.Get(ctx, s.DB, at.RequestID)
	if errors.Is(err, ErrNotFound) {
		return AttachmentRow{}, apperr.NotFound()
	}
	if err != nil {
		return AttachmentRow{}, err
	}
	if !canRead(a, r.Row) || !validStoredName(at.StoredName) {
		return AttachmentRow{}, apperr.NotFound()
	}
	return at, nil
}
