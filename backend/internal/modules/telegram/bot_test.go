package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// --- fakes ---

type sentMsg struct {
	Chat int64
	Text string
	Btn  *Button
}

type fakeAPI struct {
	mu   sync.Mutex
	sent []sentMsg
	fail func(chat int64, text string) error
}

func (f *fakeAPI) GetMe(context.Context) (User, error) { return User{Username: "mv_bot"}, nil }
func (f *fakeAPI) GetUpdates(context.Context, int64, int) ([]Update, error) {
	return nil, nil
}
func (f *fakeAPI) SendMessage(_ context.Context, chat int64, text string, b *Button) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		if err := f.fail(chat, text); err != nil {
			return err
		}
	}
	f.sent = append(f.sent, sentMsg{chat, text, b})
	return nil
}
func (f *fakeAPI) last(chat int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.sent) - 1; i >= 0; i-- {
		if f.sent[i].Chat == chat {
			return f.sent[i].Text
		}
	}
	return ""
}
func (f *fakeAPI) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sent) }

type fakeStore struct {
	mu       sync.Mutex
	byUser   map[int64]Link
	byChat   map[int64]Link
	accounts map[int64]Account
}

func newFakeStore() *fakeStore {
	return &fakeStore{byUser: map[int64]Link{}, byChat: map[int64]Link{}, accounts: map[int64]Account{}}
}

func (s *fakeStore) LinkByUser(_ context.Context, id int64) (Link, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.byUser[id]
	return l, ok, nil
}
func (s *fakeStore) LinkByChat(_ context.Context, id int64) (Link, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.byChat[id]
	return l, ok, nil
}
func (s *fakeStore) CreateLink(ctx context.Context, l Link, inTx func(context.Context, db.Querier) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byChat[l.ChatID]; ok {
		return ErrChatLinked
	}
	if _, ok := s.byUser[l.UserID]; ok {
		return ErrUserLinked
	}
	if inTx != nil {
		if err := inTx(ctx, nil); err != nil {
			return err
		}
	}
	s.byUser[l.UserID], s.byChat[l.ChatID] = l, l
	return nil
}
func (s *fakeStore) Account(_ context.Context, id int64) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return Account{}, ErrNoAccount
	}
	return a, nil
}

type fakeReq struct {
	ID       int64
	Number   string
	AuthorID int64 // 0 = anonymous request (no author)
	Anon     bool
	Status   string
	Title    string
}

type fakeReqs struct {
	reqs      map[string]*fakeReq
	answered  []string
	lastActor authz.Actor
	panicOn   string
}

func (f *fakeReqs) FindID(_ context.Context, number string) (int64, bool, error) {
	if r, ok := f.reqs[number]; ok {
		return r.ID, true, nil
	}
	return 0, false, nil
}
func (f *fakeReqs) byID(id int64) *fakeReq {
	for _, r := range f.reqs {
		if r.ID == id {
			return r
		}
	}
	return nil
}
func (f *fakeReqs) MyRequests(_ context.Context, a authz.Actor, limit int) ([]ReqItem, error) {
	f.lastActor = a
	if f.panicOn == "my" {
		panic("boom")
	}
	var out []ReqItem
	for _, r := range f.reqs {
		if r.AuthorID == a.UserID && !r.Anon {
			out = append(out, ReqItem{ID: r.ID, Number: r.Number, Title: r.Title, Status: r.Status, StatusLabel: r.Status})
		}
	}
	return out, nil
}
func (f *fakeReqs) Inbox(_ context.Context, a authz.Actor, _ int) (InboxView, error) {
	f.lastActor = a
	if !a.IsStaff() {
		return InboxView{}, apperr.Forbidden()
	}
	return InboxView{NeedsAction: 2, Overdue: 1, Items: []ReqItem{{ID: 1, Number: "MV-0001", Title: "Нужно действие", StatusLabel: "Новое"}}}, nil
}
func (f *fakeReqs) Card(_ context.Context, a authz.Actor, id int64) (ReqCard, error) {
	f.lastActor = a
	r := f.byID(id)
	if r == nil || !(r.AuthorID == a.UserID || a.IsStaff()) {
		return ReqCard{}, apperr.NotFound()
	}
	return ReqCard{ReqItem: ReqItem{ID: r.ID, Number: r.Number, Title: r.Title, Status: r.Status, StatusLabel: r.Status, Anonymous: r.Anon}, IsAuthor: r.AuthorID == a.UserID, Question: "Уточните детали"}, nil
}
func (f *fakeReqs) Answer(_ context.Context, a authz.Actor, id int64, text string) error {
	f.lastActor = a
	r := f.byID(id)
	if r == nil || r.AuthorID != a.UserID {
		return apperr.Forbidden()
	}
	f.answered = append(f.answered, fmt.Sprintf("%s:%s", r.Number, text))
	return nil
}
func (f *fakeReqs) AdminStats(_ context.Context, a authz.Actor) (AdminStats, error) {
	if !a.IsAdmin() {
		return AdminStats{}, apperr.Forbidden()
	}
	return AdminStats{Total: 10, Open: 7, Unrouted: 2, Overdue: 3, NewToday: 1}, nil
}

// --- harness ---

const (
	uEmp, uMgr, uAdmin, uOther, uOff int64 = 1, 2, 3, 4, 5
)

type harness struct {
	t     *testing.T
	clk   *fakeClock
	api   *fakeAPI
	store *fakeStore
	reqs  *fakeReqs
	codes *Codes
	bot   *Bot
	marks []int64
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, clk: newFakeClock(), api: &fakeAPI{}, store: newFakeStore(), reqs: &fakeReqs{reqs: map[string]*fakeReq{}}}
	acct := func(id int64, name string, role authz.Role, active bool) {
		h.store.accounts[id] = Account{Actor: authz.Actor{UserID: id, Name: name, Role: role}, Active: active, DepartmentName: "Сервис"}
	}
	acct(uEmp, "Сотрудник Первый", authz.RoleEmployee, true)
	acct(uMgr, "Руководитель Альфа", authz.RoleManager, true)
	acct(uAdmin, "Админ Админов", authz.RoleAdmin, true)
	acct(uOther, "Сотрудник Второй", authz.RoleEmployee, true)
	acct(uOff, "Уволенный", authz.RoleEmployee, false)
	h.reqs.reqs["MV-0001"] = &fakeReq{ID: 1, Number: "MV-0001", AuthorID: uEmp, Status: "needs_info", Title: "Сломался принтер"}
	h.reqs.reqs["MV-0002"] = &fakeReq{ID: 2, Number: "MV-0002", AuthorID: uEmp, Status: "in_review", Title: "Идея"}
	h.reqs.reqs["MV-0003"] = &fakeReq{ID: 3, Number: "MV-0003", AuthorID: uEmp, Anon: true, Status: "needs_info", Title: "Анонимное"}
	h.codes = NewCodes(NewMemoryCache(h.clk.Now), h.clk.Now)
	h.bot = &Bot{
		cfg: Config{AppBaseURL: "https://voice.example.com"}.withDefaults(), api: h.api, store: h.store, codes: h.codes, reqs: h.reqs,
		markSent: func(_ context.Context, _ db.Querier, uid int64) error { h.marks = append(h.marks, uid); return nil },
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)), now: h.clk.Now, fails: newFailureLimiter(h.clk.Now),
		sleep: func(context.Context, time.Duration) error { return nil },
	}
	return h
}

var updSeq int64

func (h *harness) say(chat int64, text string) string {
	h.t.Helper()
	updSeq++
	h.bot.HandleUpdate(context.Background(), Update{UpdateID: updSeq, Message: &Message{Text: text, Chat: Chat{ID: chat, Type: "private"}, From: &User{ID: chat, Username: fmt.Sprintf("tg%d", chat)}}})
	return h.api.last(chat)
}

func (h *harness) link(chat, uid int64) {
	h.t.Helper()
	code, _, err := h.codes.Current(uid)
	if err != nil {
		h.t.Fatal(err)
	}
	if reply := h.say(chat, "/start "+code); !strings.Contains(reply, "Готово") {
		h.t.Fatalf("link failed: %q", reply)
	}
}

// --- link flow ---

func TestLinkStartWithCode(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.codes.Current(uEmp)
	reply := h.say(100, "/start "+code)
	if !strings.Contains(reply, "Сотрудник Первый") || !strings.Contains(reply, "Сотрудник") {
		t.Fatalf("confirmation must name the user and role: %q", reply)
	}
	l, ok, _ := h.store.LinkByUser(context.Background(), uEmp)
	if !ok || l.ChatID != 100 || l.Username != "tg100" {
		t.Fatalf("link = %+v %v", l, ok)
	}
	if len(h.marks) != 1 || h.marks[0] != uEmp {
		t.Fatalf("existing notifications must be marked delivered at link time, marks=%v", h.marks)
	}
}

func TestLinkStartThenPlainCodeAndLowercase(t *testing.T) {
	h := newHarness(t)
	if r := h.say(100, "/start"); !strings.Contains(r, "6-значный код") {
		t.Fatalf("welcome: %q", r)
	}
	code, _, _ := h.codes.Current(uMgr)
	if r := h.say(100, strings.ToLower(code)); !strings.Contains(r, "Готово") || !strings.Contains(r, "Руководитель") {
		t.Fatalf("plain code: %q", r)
	}
}

func TestLinkWrongCode(t *testing.T) {
	h := newHarness(t)
	if r := h.say(100, "ZZZZZZ"); !strings.Contains(r, "не найден") {
		t.Fatalf("wrong code: %q", r)
	}
	if r := h.say(100, "abc"); !strings.Contains(r, "6 символов") {
		t.Fatalf("malformed: %q", r)
	}
	if _, ok, _ := h.store.LinkByChat(context.Background(), 100); ok {
		t.Fatal("chat must not be linked")
	}
}

func TestLinkExpiredCode(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.codes.Current(uEmp)
	h.clk.Advance(CodePeriod + CodeGrace)
	if r := h.say(100, code); !strings.Contains(r, "не найден или устарел") {
		t.Fatalf("expired: %q", r)
	}
}

func TestLinkPreviousCodeWorksInGrace(t *testing.T) {
	h := newHarness(t)
	old, _, _ := h.codes.Current(uEmp)
	h.clk.Advance(CodePeriod + time.Second)
	_, _, _ = h.codes.Current(uEmp) // rotate
	if r := h.say(100, old); !strings.Contains(r, "Готово") {
		t.Fatalf("grace code: %q", r)
	}
}

func TestLinkCodeIsSingleUse(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.codes.Current(uEmp)
	h.say(100, code)
	if r := h.say(200, code); !strings.Contains(r, "не найден") {
		t.Fatalf("reused code: %q", r)
	}
}

func TestLinkChatAlreadyLinkedToAnotherUser(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	code, _, _ := h.codes.Current(uMgr)
	// a linked chat typing a valid code of someone else must not rebind
	h.say(100, "/start "+code)
	l, _, _ := h.store.LinkByChat(context.Background(), 100)
	if l.UserID != uEmp {
		t.Fatalf("chat rebound to %d", l.UserID)
	}
	if _, ok, _ := h.store.LinkByUser(context.Background(), uMgr); ok {
		t.Fatal("manager got linked through a foreign chat")
	}
}

func TestLinkUserAlreadyLinkedElsewhere(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	code, _, _ := h.codes.Current(uEmp) // would be refused by HTTP (409); simulate a code issued before linking
	if r := h.say(200, code); !strings.Contains(r, "уже привязан к другому чату") {
		t.Fatalf("second chat: %q", r)
	}
	if _, ok, _ := h.store.LinkByChat(context.Background(), 200); ok {
		t.Fatal("second chat linked")
	}
}

func TestLinkInactiveUserRefused(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.codes.Current(uOff)
	if r := h.say(100, code); strings.Contains(r, "Готово") {
		t.Fatalf("inactive user linked: %q", r)
	}
	if _, ok, _ := h.store.LinkByUser(context.Background(), uOff); ok {
		t.Fatal("inactive user linked")
	}
}

func TestWrongCodeRateLimit(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < maxCodeFailures; i++ {
		if r := h.say(100, "ZZZZZ"+string(CodeAlphabet[i])); !strings.Contains(r, "не найден") {
			t.Fatalf("attempt %d: %q", i, r)
		}
	}
	// 6th attempt: cooldown message, even with a valid code
	valid, _, _ := h.codes.Current(uEmp)
	if r := h.say(100, valid); !strings.Contains(r, "Слишком много") {
		t.Fatalf("cooldown expected, got %q", r)
	}
	if _, ok, _ := h.store.LinkByChat(context.Background(), 100); ok {
		t.Fatal("linked during cooldown")
	}
	// another chat is not affected
	if r := h.say(200, valid); !strings.Contains(r, "Готово") {
		t.Fatalf("other chat: %q", r)
	}
	// after the window the chat may try again
	h.clk.Advance(failureWindow + time.Second)
	if r := h.say(100, "ZZZZZ2"); !strings.Contains(r, "не найден") {
		t.Fatalf("after cooldown: %q", r)
	}
}

func TestGroupChatsAndEmptyMessagesIgnored(t *testing.T) {
	h := newHarness(t)
	h.bot.HandleUpdate(context.Background(), Update{Message: &Message{Text: "/start", Chat: Chat{ID: -5, Type: "group"}}})
	h.bot.HandleUpdate(context.Background(), Update{})
	h.bot.HandleUpdate(context.Background(), Update{Message: &Message{Text: "   ", Chat: Chat{ID: 7, Type: "private"}}})
	if h.api.count() != 0 {
		t.Fatalf("bot answered %d unexpected messages", h.api.count())
	}
}

func TestUnlinkedCommandsAskToLink(t *testing.T) {
	h := newHarness(t)
	for _, c := range []string{"/my", "/inbox", "/admin", "/request MV-0001", "/answer MV-0001 x"} {
		if r := h.say(100, c); !strings.Contains(r, "Сначала привяжите") {
			t.Fatalf("%s -> %q", c, r)
		}
	}
	if len(h.reqs.answered) != 0 {
		t.Fatal("unlinked chat reached the services")
	}
}

// --- commands ---

func TestMeAndHelp(t *testing.T) {
	h := newHarness(t)
	h.link(100, uMgr)
	r := h.say(100, "/me")
	for _, want := range []string{"Руководитель Альфа", "Руководитель", "Сервис"} {
		if !strings.Contains(r, want) {
			t.Fatalf("/me missing %q: %q", want, r)
		}
	}
	if help := h.say(100, "/help@mv_bot"); !strings.Contains(help, "/inbox") || strings.Contains(help, "/admin") {
		t.Fatalf("manager help: %q", help)
	}
	h.link(101, uAdmin)
	if help := h.say(101, "/help"); !strings.Contains(help, "/admin") {
		t.Fatalf("admin help: %q", help)
	}
	h.link(102, uEmp)
	if help := h.say(102, "/help"); strings.Contains(help, "/inbox") || strings.Contains(help, "/admin") {
		t.Fatalf("employee help advertises staff commands: %q", help)
	}
}

func TestUnknownInputShowsHelp(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	if r := h.say(100, "ZZZZZZ"); r != msgChatAlreadyLinked {
		t.Fatalf("code-shaped text in a linked chat: %q", r)
	}
	for _, in := range []string{"привет", "/unknown"} {
		if r := h.say(100, in); !strings.Contains(r, "Команды:") {
			t.Fatalf("%q -> %q", in, r)
		}
	}
}

func TestMyListsOnlyOwnNonAnonymous(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	r := h.say(100, "/my")
	if !strings.Contains(r, "MV-0001") || !strings.Contains(r, "MV-0002") {
		t.Fatalf("own requests missing: %q", r)
	}
	if strings.Contains(r, "MV-0003") {
		t.Fatalf("anonymous request leaked into /my: %q", r)
	}
	if !strings.Contains(r, "https://voice.example.com/requests/1") {
		t.Fatalf("deep link missing: %q", r)
	}
	if h.reqs.lastActor.UserID != uEmp {
		t.Fatalf("service called as %d, not as the linked user", h.reqs.lastActor.UserID)
	}
}

func TestInboxAndAdminAuthorization(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	h.link(101, uMgr)
	h.link(102, uAdmin)

	if r := h.say(100, "/inbox"); !strings.Contains(r, "руководителям") {
		t.Fatalf("employee /inbox: %q", r)
	}
	if r := h.say(100, "/admin"); !strings.Contains(r, "администраторам") {
		t.Fatalf("employee /admin: %q", r)
	}
	if r := h.say(101, "/inbox"); !strings.Contains(r, "MV-0001") || !strings.Contains(r, "просрочено — 1") {
		t.Fatalf("manager /inbox: %q", r)
	}
	if r := h.say(101, "/admin"); !strings.Contains(r, "администраторам") {
		t.Fatalf("manager /admin: %q", r)
	}
	if r := h.say(102, "/admin"); !strings.Contains(r, "Всего обращений: 10") || !strings.Contains(r, "Не распределено: 2") {
		t.Fatalf("admin /admin: %q", r)
	}
}

func TestRequestCard(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	h.link(101, uOther)
	h.link(102, uMgr)

	r := h.say(100, "/request mv-1")
	for _, want := range []string{"MV-0001", "Сломался принтер", "● Разработка", "○ Принятие решения", "/answer MV-0001"} {
		if !strings.Contains(r, want) {
			t.Fatalf("card missing %q:\n%s", want, r)
		}
	}
	if r := h.say(101, "/request MV-0001"); r != msgRequestNotFound {
		t.Fatalf("foreign employee must get not-found, got %q", r)
	}
	if r := h.say(100, "/request MV-9999"); r != msgRequestNotFound {
		t.Fatalf("missing: %q", r)
	}
	if r := h.say(100, "/request"); !strings.Contains(r, "Укажите номер") {
		t.Fatalf("no arg: %q", r)
	}
	if r := h.say(102, "/request MV-0001"); !strings.Contains(r, "Сломался принтер") {
		t.Fatalf("staff card: %q", r)
	}
	// anonymous request is unreachable for its (pre-CR2 linked) author
	if r := h.say(100, "/request MV-0003"); r != msgRequestNotFound {
		t.Fatalf("anonymous request reachable: %q", r)
	}
}

func TestAnswerAuthorization(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	h.link(101, uOther)
	h.link(102, uMgr)

	if r := h.say(101, "/answer MV-0001 чужой ответ"); r != msgRequestNotFound {
		t.Fatalf("other employee: %q", r)
	}
	if r := h.say(102, "/answer MV-0001 ответ руководителя"); !strings.Contains(r, "только автор") {
		t.Fatalf("staff non-author: %q", r)
	}
	if len(h.reqs.answered) != 0 {
		t.Fatalf("unauthorized answers got through: %v", h.reqs.answered)
	}
	if r := h.say(100, "/answer MV-0002 не тот статус"); !strings.Contains(r, "нет вопроса") {
		t.Fatalf("wrong status: %q", r)
	}
	if r := h.say(100, "/answer MV-0003 анонимное"); r != msgRequestNotFound {
		t.Fatalf("anonymous: %q", r)
	}
	if r := h.say(100, "/answer MV-0001"); !strings.Contains(r, "Формат") {
		t.Fatalf("no text: %q", r)
	}
	if r := h.say(100, "/answer MV-0001 "+strings.Repeat("я", maxAnswerRunes+1)); !strings.Contains(r, "слишком длинный") {
		t.Fatalf("too long: %q", r)
	}
	if r := h.say(100, "/answer MV-0001 Принтер в кабинете 5"); !strings.Contains(r, "Ответ отправлен") {
		t.Fatalf("author answer: %q", r)
	}
	if len(h.reqs.answered) != 1 || h.reqs.answered[0] != "MV-0001:Принтер в кабинете 5" {
		t.Fatalf("answered = %v", h.reqs.answered)
	}
}

func TestInactiveLinkedUserIsLockedOut(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	a := h.store.accounts[uEmp]
	a.Active = false
	h.store.accounts[uEmp] = a
	if r := h.say(100, "/my"); !strings.Contains(r, "отключён") {
		t.Fatalf("%q", r)
	}
}

func TestPanicInHandlerDoesNotKillBot(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	h.reqs.panicOn = "my"
	h.say(100, "/my") // must not panic the test
	h.reqs.panicOn = ""
	if r := h.say(100, "/me"); !strings.Contains(r, "Сотрудник Первый") {
		t.Fatalf("bot dead after panic: %q", r)
	}
}

func TestServiceErrorsDoNotLeakInternals(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	h.bot.reqs = errReqs{h.reqs}
	r := h.say(100, "/my")
	if strings.Contains(r, "sqlite") || strings.Contains(r, "secret") {
		t.Fatalf("internal error leaked: %q", r)
	}
	if r != msgInternal {
		t.Fatalf("want generic message, got %q", r)
	}
}

type errReqs struct{ *fakeReqs }

func (errReqs) MyRequests(context.Context, authz.Actor, int) ([]ReqItem, error) {
	return nil, errors.New("sqlite: secret table dump")
}

func TestMessagesAreTruncated(t *testing.T) {
	h := newHarness(t)
	h.link(100, uEmp)
	h.reqs.reqs["MV-0002"].Title = strings.Repeat("ж", 5000)
	for i := 10; i < 40; i++ {
		n := fmt.Sprintf("MV-%04d", i)
		h.reqs.reqs[n] = &fakeReq{ID: int64(i), Number: n, AuthorID: uEmp, Status: "new", Title: strings.Repeat("я", 90)}
	}
	h.say(100, "/my")
	h.api.mu.Lock()
	defer h.api.mu.Unlock()
	for _, m := range h.api.sent {
		if n := len([]rune(m.Text)); n > MaxMessageRunes {
			t.Fatalf("message of %d runes", n)
		}
	}
	if got := truncate(strings.Repeat("я", 10), 5); len([]rune(got)) != 5 || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncate = %q", got)
	}
}

func TestParseHelpers(t *testing.T) {
	for in, want := range map[string]string{"mv-1": "MV-0001", "MV-0042": "MV-0042", " mv-12345 ": "MV-12345"} {
		if got, ok := parseNumber(in); !ok || got != want {
			t.Errorf("parseNumber(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "MV", "MV-", "MV-abc", "1-2", "MV-0001; DROP TABLE", "MVMVMVMVMV-1"} {
		if _, ok := parseNumber(in); ok {
			t.Errorf("parseNumber(%q) must fail", in)
		}
	}
	if c, a := parseCommand("/Answer@mv_bot MV-1 hello world"); c != "/answer" || a != "MV-1 hello world" {
		t.Errorf("parseCommand = %q %q", c, a)
	}
	if c, a := parseCommand("K7M2QX"); c != "" || a != "K7M2QX" {
		t.Errorf("plain = %q %q", c, a)
	}
	if s := stepper("implemented"); strings.Contains(s, "○") || strings.Contains(s, "●") {
		t.Errorf("done stepper: %s", s)
	}
}
