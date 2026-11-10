package requests

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"modusvoice/internal/platform/authz"
)

const exportMaxRows = 10000

const utf8BOM = string(rune(0xFEFF))

var typeLabels = map[string]string{TypeProblem: "Проблема", TypeIdea: "Предложение"}

// csvCell neutralizes spreadsheet formula injection.
func csvCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func fmtTime(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	return t.In(loc).Format("02.01.2006 15:04")
}

// ExportCSV writes the staff inbox (with list filters) as Excel-friendly CSV: UTF-8 BOM,
// semicolon separator, CRLF. Authors are mapped through AuthorView (anonymous masked).
func (s *Service) ExportCSV(ctx context.Context, a authz.Actor, q ListQuery, loc *time.Location, w io.Writer) error {
	if err := authz.RequireStaff(a); err != nil {
		return err
	}
	if q.Scope == "" || q.Scope == "mine" {
		q.Scope = "inbox"
	}
	f, err := s.filter(a, q)
	if err != nil {
		return err
	}
	f.Limit, f.Offset = exportMaxRows, 0
	rows, _, err := s.Store.List(ctx, f)
	if err != nil {
		return err
	}
	if loc == nil {
		loc = time.UTC
	}
	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.UseCRLF = true
	_ = cw.Write([]string{"Номер", "Тип", "Категория", "Заголовок", "Статус", "Автор", "Отдел", "Ответственный",
		"Создано", "Срок первого ответа", "Срок решения", "Первый ответ", "Просрочено, ч", "Закрыто"})
	for _, r := range rows {
		v := toSummary(r, a, f.Now)
		dept, asg := "", ""
		if v.Department != nil {
			dept = v.Department.Name
		}
		if v.Assignee != nil {
			asg = v.Assignee.Name
		}
		overdue := ""
		if v.SLA.State == SLAOverdue {
			overdue = strconv.Itoa(v.SLA.OverdueHours)
		}
		rec := []string{v.Number, typeLabels[v.Type], v.Category.Name, v.Title, v.Status.Label(), v.Author.Name, dept, asg,
			fmtTime(&v.CreatedAt, loc), fmtTime(&v.SLA.FirstResponseDue, loc), fmtTime(&v.SLA.ResolutionDue, loc),
			fmtTime(v.SLA.FirstRespondedAt, loc), overdue, fmtTime(v.ClosedAt, loc)}
		for i := range rec {
			rec[i] = csvCell(rec[i])
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
