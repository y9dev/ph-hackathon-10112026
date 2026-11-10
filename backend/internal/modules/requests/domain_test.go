package requests

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
)

func TestTransitionTable(t *testing.T) {
	allowed := map[Status][]Status{
		StatusNew:        {StatusInReview, StatusNeedsInfo, StatusDecision, StatusDeclined, StatusDuplicate, StatusClosed},
		StatusInReview:   {StatusNeedsInfo, StatusDecision, StatusDeclined, StatusDuplicate, StatusClosed},
		StatusNeedsInfo:  {StatusInReview, StatusClosed},
		StatusDecision:   {StatusInProgress, StatusInReview, StatusDeclined, StatusClosed},
		StatusInProgress: {StatusImplemented, StatusDeclined, StatusDecision, StatusClosed},
	}
	for _, from := range AllStatuses {
		for _, to := range AllStatuses {
			want := false
			for _, a := range allowed[from] {
				if a == to {
					want = true
				}
			}
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s,%s)=%v want %v", from, to, got, want)
			}
		}
	}
	for _, s := range []Status{StatusImplemented, StatusDeclined, StatusDuplicate, StatusClosed} {
		if !s.Terminal() || len(AllowedTransitions(s)) != 0 {
			t.Errorf("%s must be terminal without transitions (no reopen in MVP)", s)
		}
	}
}

func TestComputeSLA(t *testing.T) {
	created := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	fr := created.Add(24 * time.Hour)
	res := created.Add(120 * time.Hour)
	responded := created.Add(2 * time.Hour)
	tests := []struct {
		name      string
		status    Status
		responded *time.Time
		now       time.Time
		state     string
		kind      string
		hours     int
	}{
		{"fresh new is ok", StatusNew, nil, created.Add(time.Hour), SLAOk, "", 0},
		{"new close to first-response due is soon", StatusNew, nil, created.Add(20 * time.Hour), SLASoon, "", 0},
		{"new past first-response due", StatusNew, nil, created.Add(30 * time.Hour), SLAOverdue, OverdueFirstResponse, 6},
		{"partial hour rounds up", StatusNew, nil, fr.Add(10 * time.Minute), SLAOverdue, OverdueFirstResponse, 1},
		{"responded, before resolution due", StatusInReview, &responded, created.Add(30 * time.Hour), SLAOk, "", 0},
		{"in_progress past resolution due", StatusInProgress, &responded, res.Add(5 * time.Hour), SLAOverdue, OverdueResolution, 5},
		{"needs_info is paused even when late", StatusNeedsInfo, &responded, res.Add(50 * time.Hour), SLAPaused, "", 0},
		{"closed is done", StatusImplemented, &responded, res.Add(500 * time.Hour), SLADone, "", 0},
		{"no first response and resolution passed reports first response", StatusInReview, nil, res.Add(time.Hour), SLAOverdue, OverdueFirstResponse, 97},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := ComputeSLA(SLAInput{Status: tc.status, CreatedAt: created, FirstResponseDue: fr, ResolutionDue: res, FirstRespondedAt: tc.responded}, tc.now)
			if st.State != tc.state {
				t.Fatalf("state=%s want %s", st.State, tc.state)
			}
			kind := ""
			if st.OverdueKind != nil {
				kind = *st.OverdueKind
			}
			if kind != tc.kind || st.OverdueHours != tc.hours {
				t.Fatalf("kind=%q hours=%d want %q %d", kind, st.OverdueHours, tc.kind, tc.hours)
			}
		})
	}
}

func TestApplyTransitionPausesAndShiftsDeadlines(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	r := Row{Status: StatusInReview, FirstResponseDue: t0.Add(24 * time.Hour), ResolutionDue: t0.Add(120 * time.Hour)}
	applyTransition(&r, StatusNeedsInfo, t0.Add(2*time.Hour))
	if r.PausedAt == nil || !r.PausedAt.Equal(t0.Add(2*time.Hour)) {
		t.Fatalf("paused_at not set: %v", r.PausedAt)
	}
	applyTransition(&r, StatusInReview, t0.Add(12*time.Hour)) // 10h paused
	if r.PausedAt != nil {
		t.Fatal("paused_at must be cleared on resume")
	}
	if want := t0.Add(130 * time.Hour); !r.ResolutionDue.Equal(want) {
		t.Fatalf("resolution due %v want %v", r.ResolutionDue, want)
	}
	if want := t0.Add(34 * time.Hour); !r.FirstResponseDue.Equal(want) {
		t.Fatalf("first response due %v want %v", r.FirstResponseDue, want)
	}
	applyTransition(&r, StatusInProgress, t0.Add(13*time.Hour))
	if r.ClosedAt != nil {
		t.Fatal("in_progress is not terminal")
	}
	applyTransition(&r, StatusImplemented, t0.Add(14*time.Hour))
	if r.ClosedAt == nil {
		t.Fatal("terminal status must stamp closed_at")
	}
}

func TestResolveRouteFallback(t *testing.T) {
	ruleWithAssignee := &RuleCandidate{ID: 1, DepartmentID: 10, DepartmentName: "IT", AssigneeID: ptr(int64(5)), FirstResponseHours: 24, ResolutionHours: 120}
	catRule := &RuleCandidate{ID: 2, DepartmentID: 20, DepartmentName: "Сервис", FirstResponseHours: 48, ResolutionHours: 240}
	active := &Owner{ID: 5, Name: "Елена", Active: true}
	inactive := &Owner{ID: 5, Name: "Елена", Active: false}
	head := &Owner{ID: 7, Name: "Сергей", Active: true}
	headInactive := &Owner{ID: 7, Name: "Сергей", Active: false}
	tests := []struct {
		name           string
		exact, wide    *RuleCandidate
		assignee, head *Owner
		source         string
		dept, owner    int64
		fr             int
	}{
		{"exact rule, active assignee", ruleWithAssignee, catRule, active, head, SourceRule, 10, 5, 24},
		{"exact rule wins over category rule", ruleWithAssignee, catRule, active, nil, SourceRule, 10, 5, 24},
		{"category rule when no exact", nil, catRule, nil, head, SourceRule, 20, 7, 48},
		{"inactive assignee falls back to head", ruleWithAssignee, nil, inactive, head, SourceRule, 10, 7, 24},
		{"no assignee and inactive head is unrouted", catRule, nil, nil, headInactive, SourceUnrouted, 20, 0, 48},
		{"no rule is unrouted with default SLA", nil, nil, nil, nil, SourceUnrouted, 0, 0, UnroutedFirstResponseHours},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ResolveRoute(tc.exact, tc.wide, tc.assignee, tc.head)
			if r.Source != tc.source || r.FirstResponseHours != tc.fr {
				t.Fatalf("source=%s fr=%d want %s %d", r.Source, r.FirstResponseHours, tc.source, tc.fr)
			}
			var dept, owner int64
			if r.DepartmentID != nil {
				dept = *r.DepartmentID
			}
			if r.AssigneeID != nil {
				owner = *r.AssigneeID
			}
			if dept != tc.dept || owner != tc.owner {
				t.Fatalf("dept=%d owner=%d want %d %d", dept, owner, tc.dept, tc.owner)
			}
		})
	}
}

func TestAuthorView(t *testing.T) {
	// CR2: anonymous requests carry no author id/name at all
	author := AuthorRef{IsAnonymous: true}
	tests := []struct {
		name     string
		author   AuthorRef
		viewer   authz.Actor
		wantName string
		wantAnon bool
		wantMe   bool
	}{
		{"admin sees label", author, authz.Actor{UserID: 9, Role: authz.RoleAdmin}, AnonymousLabel, true, false},
		{"zero-id viewer is never the author", author, authz.Actor{UserID: 0}, AnonymousLabel, true, false},
		{"token holder sees label as me", AuthorRef{ID: anonSelfID, IsAnonymous: true}, anonAuthorViewer(), AnonymousLabel, true, true},
		{"named author sees self", AuthorRef{ID: 3, Name: "Иван Сидоров"}, authz.Actor{UserID: 3}, "Иван Сидоров", false, true},
		{"named visible to others", AuthorRef{ID: 3, Name: "Иван Сидоров"}, authz.Actor{UserID: 9}, "Иван Сидоров", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := AuthorView(tc.author, tc.viewer)
			if v.Name != tc.wantName || v.Anonymous != tc.wantAnon || v.IsMe != tc.wantMe {
				t.Fatalf("got %+v", v)
			}
		})
	}
	// author-side rows (by_author) of anonymous requests are masked in the timeline too
	if v := ActorView(nil, true, "", "", author, authz.Actor{UserID: 9}); v.Name != AnonymousLabel || !v.Anonymous {
		t.Fatalf("author actor must be masked: %+v", v)
	}
	named := AuthorRef{ID: 3, Name: "Иван Сидоров"}
	if v := ActorView(ptr(int64(3)), false, "Иван Сидоров", authz.RoleEmployee, named, authz.Actor{UserID: 9}); v.Name != "Иван Сидоров" {
		t.Fatalf("legacy author row by id: %+v", v)
	}
	if v := ActorView(ptr(int64(9)), false, "Елена", authz.RoleManager, author, authz.Actor{UserID: 3}); v.Name != "Елена" || v.Role == nil {
		t.Fatalf("staff actor carries role: %+v", v)
	}
	if v := ActorView(nil, false, "", "", author, authz.Actor{UserID: 3}); v.Name != SystemLabel {
		t.Fatalf("system actor: %+v", v)
	}
}

func TestAnonymousToken(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := NewAnonymousToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != tokenSymbols+tokenSymbols/tokenGroup-1 || seen[tok] {
			t.Fatalf("bad or repeated token %q", tok)
		}
		seen[tok] = true
		c, ok := CanonicalToken(strings.ToLower(tok))
		if !ok || len(c) != tokenSymbols || HashAnonymousToken(tok) != HashAnonymousToken(" "+strings.ToLower(tok)+" ") {
			t.Fatalf("canonical form of %q: %q %v", tok, c, ok)
		}
	}
	tests := []struct {
		in string
		ok bool
	}{
		{"0000-0000-0000-0000-0000-0000-0000", true},
		{"oooo-iiii-llll-0000-0000-0000-0000", true},  // look-alikes
		{"0000-0000-0000-0000-0000-0000-000", false},  // too short
		{"0000-0000-0000-0000-0000-0000-000U", false}, // U not in alphabet
		{"ЖЖЖЖ-0000-0000-0000-0000-0000-0000", false},
		{strings.Repeat("0", 200), false},
	}
	for _, tc := range tests {
		if _, ok := CanonicalToken(tc.in); ok != tc.ok {
			t.Errorf("CanonicalToken(%q) ok=%v want %v", tc.in, ok, tc.ok)
		}
	}
	if HashAnonymousToken("oooo-iiii-llll-0000-0000-0000-0000") != HashAnonymousToken("0000-1111-1111-0000-0000-0000-0000") {
		t.Fatal("look-alikes must hash identically")
	}
}

func TestPriorityAndTypeValidation(t *testing.T) {
	base := CreateInput{Type: TypeFeedback, CategoryID: 1, Title: "Заголовок", Body: strings.Repeat("а", 20)}
	tests := []struct {
		priority string
		wantErr  bool
		want     string
	}{
		{"", false, PriorityMedium},
		{"low", false, PriorityLow},
		{" high ", false, PriorityHigh},
		{"urgent", true, ""},
	}
	for _, tc := range tests {
		in := base
		in.Priority = tc.priority
		in.Normalize()
		err := in.Validate()
		if (err != nil) != tc.wantErr || (!tc.wantErr && in.Priority != tc.want) {
			t.Errorf("priority %q: err=%v priority=%q", tc.priority, err, in.Priority)
		}
	}
	for _, typ := range AllTypes {
		if !ValidType(typ) || TypeLabel(typ) == "" {
			t.Errorf("type %s must be valid and labelled", typ)
		}
	}
	if TypeLabel(TypeFeedback) != "Положительный отзыв" {
		t.Fatal("feedback label")
	}
}

func TestCreateInputValidation(t *testing.T) {
	ok := CreateInput{Type: TypeProblem, CategoryID: 1, Title: "Заголовок", Body: strings.Repeat("а", 20)}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	tests := []struct {
		name  string
		mod   func(*CreateInput)
		field string
	}{
		{"bad type", func(c *CreateInput) { c.Type = "bug" }, "type"},
		{"no category", func(c *CreateInput) { c.CategoryID = 0 }, "category_id"},
		{"short title", func(c *CreateInput) { c.Title = "abc" }, "title"},
		{"long title", func(c *CreateInput) { c.Title = strings.Repeat("я", 141) }, "title"},
		{"short body", func(c *CreateInput) { c.Body = "коротко" }, "body"},
		{"long solution", func(c *CreateInput) { c.ProposedSolution = strings.Repeat("я", 2001) }, "proposed_solution"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := ok
			tc.mod(&in)
			e, isApp := apperr.As(in.Validate())
			if !isApp || e.Fields[tc.field] == "" {
				t.Fatalf("want field error %q, got %v", tc.field, e)
			}
		})
	}
}

func TestSniffAndUploadValidation(t *testing.T) {
	sniff := []struct {
		ext  string
		head []byte
		ok   bool
	}{
		{"png", []byte("\x89PNG\r\n\x1a\nrest"), true},
		{"png", []byte("MZ\x90\x00"), false},
		{"jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, true},
		{"jpeg", []byte("GIF89a"), false},
		{"pdf", []byte("%PDF-1.7"), true},
		{"docx", []byte("PK\x03\x04"), true},
		{"xlsx", []byte("%PDF-"), false},
		{"txt", []byte("Привет, мир"), true},
		{"txt", []byte("bin\x00ary"), false},
		{"exe", []byte("MZ"), false},
	}
	for _, s := range sniff {
		if got := SniffMatches(s.ext, s.head); got != s.ok {
			t.Errorf("SniffMatches(%s, %q)=%v want %v", s.ext, s.head, got, s.ok)
		}
	}
	// a UTF-8 rune cut at the sniff boundary is still text
	cut := []byte("ааа")[:5]
	if !SniffMatches("txt", cut) {
		t.Error("truncated UTF-8 must pass")
	}
	files := func(n int, size int64, name string) []Upload {
		out := make([]Upload, n)
		for i := range out {
			out[i] = Upload{Filename: name, Size: size}
		}
		return out
	}
	if err := ValidateUploads(files(5, 1000, "a.png")); err != nil {
		t.Fatalf("5 files allowed: %v", err)
	}
	for name, in := range map[string][]Upload{
		"six files":       files(6, 10, "a.png"),
		"bad extension":   files(1, 10, "virus.exe"),
		"file over 10 MB": files(1, MaxFileBytes+1, "a.pdf"),
		"total over 10MB": files(2, 6<<20, "a.pdf"),
		"empty file":      files(1, 0, "a.txt"),
	} {
		if err := ValidateUploads(in); !apperr.Is(err, apperr.KindValidation) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}
	if got := SanitizeFilename(`..\..\evil"<name>.pdf`); strings.ContainsAny(got, `\/"<>`) {
		t.Errorf("unsafe filename kept: %q", got)
	}
	if !bytes.Equal([]byte(FormatNumber(42)), []byte("MV-0042")) || FormatNumber(12345) != "MV-12345" {
		t.Error("FormatNumber")
	}
}

func TestCSVCellInjection(t *testing.T) {
	for in, want := range map[string]string{"=SUM(A1)": "'=SUM(A1)", "+7": "'+7", "обычный": "обычный", "": ""} {
		if got := csvCell(in); got != want {
			t.Errorf("csvCell(%q)=%q want %q", in, got, want)
		}
	}
}
