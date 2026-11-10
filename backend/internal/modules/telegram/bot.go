package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

const (
	// MaxMessageRunes caps every outgoing message (Telegram allows 4096).
	MaxMessageRunes  = 4000
	maxIncomingRunes = 4096
	maxAnswerRunes   = 4000
	listLimit        = 10

	maxCodeFailures = 5
	failureWindow   = time.Minute
)

// Bot handles incoming updates for linked and not yet linked chats.
type Bot struct {
	cfg      Config
	api      API
	store    Store
	codes    *Codes
	reqs     RequestsAPI
	markSent func(ctx context.Context, q db.Querier, userID int64) error // marks existing notifications delivered at link time
	log      *slog.Logger
	now      func() time.Time
	fails    *failureLimiter
	username atomic.Value // string
	disabled atomic.Bool  // set when Telegram rejects the token
	sleep    func(ctx context.Context, d time.Duration) error
}

// Username returns the bot username known after getMe ("" before).
func (b *Bot) Username() string {
	s, _ := b.username.Load().(string)
	return s
}

// failureLimiter allows maxCodeFailures wrong codes per chat per window.
type failureLimiter struct {
	mu  sync.Mutex
	now func() time.Time
	m   map[int64][]time.Time
}

func newFailureLimiter(now func() time.Time) *failureLimiter {
	return &failureLimiter{now: now, m: map[int64][]time.Time{}}
}

func (f *failureLimiter) prune(chat int64) []time.Time {
	cut := f.now().Add(-failureWindow)
	ts := f.m[chat]
	i := 0
	for i < len(ts) && !ts[i].After(cut) {
		i++
	}
	ts = ts[i:]
	if len(ts) == 0 {
		delete(f.m, chat)
	} else {
		f.m[chat] = ts
	}
	return ts
}

// blocked returns how long the chat must wait (0 = may try).
func (f *failureLimiter) blocked(chat int64) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	ts := f.prune(chat)
	if len(ts) < maxCodeFailures {
		return 0
	}
	return ts[0].Add(failureWindow).Sub(f.now())
}

func (f *failureLimiter) fail(chat int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.m) > 4096 { // bound memory under chat-id spraying
		for id := range f.m {
			f.prune(id)
		}
	}
	f.m[chat] = append(f.prune(chat), f.now())
}

func (f *failureLimiter) reset(chat int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, chat)
}

// Run validates the token (getMe) and then long-polls until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		me, err := b.api.GetMe(ctx)
		if err == nil {
			b.username.Store(me.Username)
			b.log.Info("telegram bot started", "bot", "@"+me.Username)
			break
		}
		var ae *APIError
		if errors.As(err, &ae) && (ae.Code == http.StatusUnauthorized || ae.Code == http.StatusNotFound) {
			b.disabled.Store(true)
			b.log.Error("telegram token rejected by Telegram, bot disabled", "code", ae.Code)
			return
		}
		if ctx.Err() != nil {
			return
		}
		b.log.Warn("telegram getMe failed, retrying", "err", err, "in", backoff)
		if b.sleep(ctx, backoff) != nil {
			return
		}
		backoff = nextBackoff(backoff)
	}
	var offset int64
	backoff = time.Second
	for ctx.Err() == nil {
		ups, err := b.api.GetUpdates(ctx, offset, PollTimeoutSeconds)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Warn("telegram getUpdates failed", "err", err, "retry_in", backoff)
			if b.sleep(ctx, backoff) != nil {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		backoff = time.Second
		for _, u := range ups {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			b.HandleUpdate(ctx, u)
		}
	}
}

func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > time.Minute {
		d = time.Minute
	}
	return d
}

// HandleUpdate processes one update; a panic is logged and never stops the polling loop.
func (b *Bot) HandleUpdate(ctx context.Context, u Update) {
	defer func() {
		if r := recover(); r != nil {
			b.log.Error("telegram update handler panic", "update_id", u.UpdateID, "panic", fmt.Sprint(r))
		}
	}()
	m := u.Message
	if m == nil || m.Chat.Type != "private" {
		return
	}
	text := strings.TrimSpace(m.Text)
	if text == "" || utf8.RuneCountInString(text) > maxIncomingRunes {
		return
	}
	cmd, args := parseCommand(text)
	link, linked, err := b.store.LinkByChat(ctx, m.Chat.ID)
	if err != nil {
		b.log.Error("telegram link lookup failed", "err", err)
		b.reply(ctx, m.Chat.ID, msgInternal)
		return
	}
	if !linked {
		b.handleUnlinked(ctx, m, cmd, args, text)
		return
	}
	b.handleLinked(ctx, m.Chat.ID, link, cmd, args)
}

// parseCommand splits "/cmd@bot args" into ("/cmd", "args"); plain text yields ("", text).
func parseCommand(text string) (cmd, args string) {
	if !strings.HasPrefix(text, "/") {
		return "", text
	}
	head, rest, _ := strings.Cut(text, " ")
	if i := strings.IndexByte(head, '@'); i > 0 {
		head = head[:i]
	}
	return strings.ToLower(head), strings.TrimSpace(rest)
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	b.send(ctx, chatID, text, nil)
}

func (b *Bot) send(ctx context.Context, chatID int64, text string, btn *Button) {
	if err := b.api.SendMessage(ctx, chatID, truncate(text, MaxMessageRunes), btn); err != nil && ctx.Err() == nil {
		b.log.Warn("telegram reply failed", "err", err)
	}
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// ---- linking ----

func (b *Bot) handleUnlinked(ctx context.Context, m *Message, cmd, args, text string) {
	chat := m.Chat.ID
	switch cmd {
	case "/start":
		if args == "" {
			b.reply(ctx, chat, msgWelcomeUnlinked)
			return
		}
		b.tryLink(ctx, m, args)
	case "/help":
		b.reply(ctx, chat, msgWelcomeUnlinked)
	case "":
		b.tryLink(ctx, m, text)
	default:
		b.reply(ctx, chat, msgLinkFirst)
	}
}

func (b *Bot) tryLink(ctx context.Context, m *Message, raw string) {
	chat := m.Chat.ID
	if wait := b.fails.blocked(chat); wait > 0 {
		b.reply(ctx, chat, fmt.Sprintf("Слишком много неверных попыток. Подождите %d с и пришлите новый код.", int(wait.Seconds())+1))
		return
	}
	code, ok := NormalizeCode(raw)
	if !ok {
		b.reply(ctx, chat, msgBadCodeShape)
		return
	}
	uid, ok := b.codes.Resolve(code)
	if !ok {
		b.fails.fail(chat)
		b.reply(ctx, chat, msgCodeInvalid)
		return
	}
	acc, err := b.store.Account(ctx, uid)
	if err != nil || !acc.Active {
		b.fails.fail(chat)
		b.reply(ctx, chat, msgCodeInvalid)
		return
	}
	l := Link{UserID: uid, ChatID: chat, LinkedAt: b.now()}
	if m.From != nil {
		l.Username = m.From.Username
	}
	err = b.store.CreateLink(ctx, l, func(ctx context.Context, q db.Querier) error {
		if b.markSent == nil {
			return nil
		}
		return b.markSent(ctx, q, uid)
	})
	switch {
	case errors.Is(err, ErrChatLinked):
		b.reply(ctx, chat, msgChatAlreadyLinked)
		return
	case errors.Is(err, ErrUserLinked):
		b.codes.Consume(uid, code)
		b.reply(ctx, chat, msgUserAlreadyLinked)
		return
	case err != nil:
		b.log.Error("telegram link failed", "err", err)
		b.reply(ctx, chat, msgInternal)
		return
	}
	b.codes.Consume(uid, code)
	b.fails.reset(chat)
	b.log.Info("telegram account linked", "user_id", uid)
	b.reply(ctx, chat, fmt.Sprintf("Готово! Аккаунт привязан: %s (%s).\n\nТеперь уведомления по обращениям будут приходить сюда. Список команд: /help", acc.Actor.Name, roleLabel(acc.Actor.Role)))
}

// ---- commands of linked chats ----

func (b *Bot) handleLinked(ctx context.Context, chat int64, link Link, cmd, args string) {
	acc, err := b.store.Account(ctx, link.UserID)
	if err != nil {
		b.log.Error("telegram account lookup failed", "err", err)
		b.reply(ctx, chat, msgInternal)
		return
	}
	if !acc.Active {
		b.reply(ctx, chat, "Ваш аккаунт отключён. Обратитесь к администратору.")
		return
	}
	a := acc.Actor
	switch cmd {
	case "/start":
		b.reply(ctx, chat, fmt.Sprintf("Вы уже подключены: %s (%s).\n\n%s", a.Name, roleLabel(a.Role), helpText(a)))
	case "/help":
		b.reply(ctx, chat, helpText(a))
	case "/me":
		dept := acc.DepartmentName
		if dept == "" {
			dept = "не указан"
		}
		b.reply(ctx, chat, fmt.Sprintf("Вы: %s\nРоль: %s\nОтдел: %s", a.Name, roleLabel(a.Role), dept))
	case "/my":
		b.cmdMy(ctx, chat, a)
	case "/inbox":
		b.cmdInbox(ctx, chat, a)
	case "/request":
		b.cmdRequest(ctx, chat, a, args)
	case "/answer":
		b.cmdAnswer(ctx, chat, a, args)
	case "/admin":
		b.cmdAdmin(ctx, chat, a)
	default:
		if _, isCode := NormalizeCode(args); cmd == "" && isCode {
			b.reply(ctx, chat, msgChatAlreadyLinked)
			return
		}
		b.reply(ctx, chat, "Не понял сообщение.\n\n"+helpText(a))
	}
}

func (b *Bot) cmdMy(ctx context.Context, chat int64, a authz.Actor) {
	items, err := b.reqs.MyRequests(ctx, a, listLimit)
	if err != nil {
		b.fail(ctx, chat, err)
		return
	}
	if len(items) == 0 {
		b.reply(ctx, chat, "У вас пока нет своих обращений. Создать обращение можно на сайте.")
		return
	}
	var sb strings.Builder
	sb.WriteString("Ваши последние обращения:\n")
	for _, it := range items {
		b.writeItem(&sb, it)
	}
	b.reply(ctx, chat, sb.String())
}

func (b *Bot) cmdInbox(ctx context.Context, chat int64, a authz.Actor) {
	if !a.IsStaff() {
		b.reply(ctx, chat, msgStaffOnly)
		return
	}
	v, err := b.reqs.Inbox(ctx, a, listLimit)
	if err != nil {
		b.fail(ctx, chat, err)
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Входящие: требуют действия — %d, просрочено — %d, ждут ответа автора — %d, всего в работе — %d.\n", v.NeedsAction, v.Overdue, v.WaitingAuthor, v.All)
	if len(v.Items) == 0 {
		sb.WriteString("\nНет обращений, требующих действия.")
	} else {
		sb.WriteString("\nТребуют действия:\n")
		for _, it := range v.Items {
			b.writeItem(&sb, it)
		}
	}
	b.reply(ctx, chat, sb.String())
}

func (b *Bot) cmdAdmin(ctx context.Context, chat int64, a authz.Actor) {
	if !a.IsAdmin() {
		b.reply(ctx, chat, msgAdminOnly)
		return
	}
	s, err := b.reqs.AdminStats(ctx, a)
	if err != nil {
		b.fail(ctx, chat, err)
		return
	}
	text := fmt.Sprintf("Сводка:\nВсего обращений: %d\nОткрыто: %d\nНе распределено: %d\nПросрочено: %d\nНовых сегодня: %d\n\nОткрыть аналитику", s.Total, s.Open, s.Unrouted, s.Overdue, s.NewToday)
	if err := b.api.SendLinked(ctx, chat, text, TextLink{Text: "Открыть аналитику", URL: b.cfg.AppBaseURL + "/analytics"}); err != nil && ctx.Err() == nil {
		b.log.Warn("telegram reply failed", "err", err)
	}
}

func (b *Bot) cmdRequest(ctx context.Context, chat int64, a authz.Actor, args string) {
	number, ok := parseNumber(args)
	if !ok {
		b.reply(ctx, chat, "Укажите номер обращения, например: /request MV-0001")
		return
	}
	c, found := b.loadCard(ctx, chat, a, number)
	if !found {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s · %s\n", c.Number, c.Title)
	fmt.Fprintf(&sb, "Статус: %s\n%s\n", c.StatusLabel, stepper(c.Status))
	if c.Category != "" {
		fmt.Fprintf(&sb, "Категория: %s\n", c.Category)
	}
	if c.Department != "" {
		fmt.Fprintf(&sb, "Отдел: %s\n", c.Department)
	}
	if c.Assignee != "" {
		fmt.Fprintf(&sb, "Ответственный: %s\n", c.Assignee)
	}
	if c.Overdue {
		sb.WriteString("Срок нарушен.\n")
	}
	if c.Status == "needs_info" && c.Question != "" {
		fmt.Fprintf(&sb, "\nВопрос: %s\n", truncate(c.Question, 600))
		if c.IsAuthor {
			fmt.Fprintf(&sb, "Ответить: /answer %s ваш ответ\n", c.Number)
		}
	}
	if c.LatestReport != "" {
		fmt.Fprintf(&sb, "\nПоследний отчёт: %s\n", truncate(c.LatestReport, 800))
	}
	if c.Result != "" {
		fmt.Fprintf(&sb, "\nРезультат: %s\n", truncate(c.Result, 800))
	}
	fmt.Fprintf(&sb, "\n%s/requests/%d", b.cfg.AppBaseURL, c.ID)
	b.reply(ctx, chat, sb.String())
}

func (b *Bot) cmdAnswer(ctx context.Context, chat int64, a authz.Actor, args string) {
	num, text, _ := strings.Cut(args, " ")
	number, ok := parseNumber(num)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		b.reply(ctx, chat, "Формат: /answer MV-0001 текст ответа")
		return
	}
	if utf8.RuneCountInString(text) > maxAnswerRunes {
		b.reply(ctx, chat, fmt.Sprintf("Ответ слишком длинный: не более %d символов.", maxAnswerRunes))
		return
	}
	c, found := b.loadCard(ctx, chat, a, number)
	if !found {
		return
	}
	if !c.IsAuthor {
		b.reply(ctx, chat, "Отвечать на уточнение может только автор обращения.")
		return
	}
	if c.Status != "needs_info" {
		b.reply(ctx, chat, "По этому обращению сейчас нет вопроса, на который нужно ответить.")
		return
	}
	if err := b.reqs.Answer(ctx, a, c.ID, text); err != nil {
		b.fail(ctx, chat, err)
		return
	}
	b.reply(ctx, chat, fmt.Sprintf("Ответ отправлен. Обращение %s вернулось в работу.", c.Number))
}

// loadCard resolves number and loads the card with the actor's rights. Anonymous requests and
// requests the actor may not read are reported identically as not found.
func (b *Bot) loadCard(ctx context.Context, chat int64, a authz.Actor, number string) (ReqCard, bool) {
	id, ok, err := b.reqs.FindID(ctx, number)
	if err != nil {
		b.fail(ctx, chat, err)
		return ReqCard{}, false
	}
	if !ok {
		b.reply(ctx, chat, msgRequestNotFound)
		return ReqCard{}, false
	}
	c, err := b.reqs.Card(ctx, a, id)
	if err != nil {
		b.fail(ctx, chat, err)
		return ReqCard{}, false
	}
	if c.Anonymous && c.IsAuthor {
		b.reply(ctx, chat, msgRequestNotFound)
		return ReqCard{}, false
	}
	return c, true
}

func (b *Bot) writeItem(sb *strings.Builder, it ReqItem) {
	flag := ""
	if it.Overdue {
		flag = " · просрочено"
	}
	fmt.Fprintf(sb, "\n%s · %s\n%s%s\n%s/requests/%d\n", it.Number, truncate(it.Title, 90), it.StatusLabel, flag, b.cfg.AppBaseURL, it.ID)
}

// fail maps a service error to a short Russian reply; internal details are only logged.
func (b *Bot) fail(ctx context.Context, chat int64, err error) {
	if e, ok := apperr.As(err); ok {
		switch e.Kind {
		case apperr.KindNotFound:
			b.reply(ctx, chat, msgRequestNotFound)
		case apperr.KindForbidden:
			b.reply(ctx, chat, "Недостаточно прав для этого действия.")
		default:
			b.reply(ctx, chat, e.Message)
		}
		return
	}
	b.log.Error("telegram command failed", "err", err)
	b.reply(ctx, chat, msgInternal)
}

// parseNumber accepts MV-0001 / mv-1 style numbers.
func parseNumber(s string) (string, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	prefix, digits, ok := strings.Cut(s, "-")
	if !ok || len(prefix) < 1 || len(prefix) > 6 || len(digits) < 1 || len(digits) > 8 {
		return "", false
	}
	for _, r := range prefix {
		if r < 'A' || r > 'Z' {
			return "", false
		}
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	if len(digits) < 4 {
		digits = strings.Repeat("0", 4-len(digits)) + digits
	}
	return prefix + "-" + digits, true
}

// stepper renders the three customer stages and where the request is.
func stepper(status string) string {
	stage := 0
	switch status {
	case "decision":
		stage = 1
	case "in_progress":
		stage = 2
	case "implemented", "declined", "duplicate", "closed":
		stage = 3
	}
	names := []string{"Разработка", "Принятие решения", "Реализация"}
	parts := make([]string, 3)
	for i, n := range names {
		switch {
		case i < stage:
			parts[i] = "✓ " + n
		case i == stage:
			parts[i] = "● " + n
		default:
			parts[i] = "○ " + n
		}
	}
	return strings.Join(parts, "  →  ")
}

func roleLabel(r authz.Role) string { return authz.RoleLabel(r) }

func helpText(a authz.Actor) string {
	var sb strings.Builder
	sb.WriteString("Команды:\n/me — кто я\n/my — мои последние обращения\n/request MV-0001 — карточка обращения\n/answer MV-0001 текст — ответить на уточнение\n")
	if a.IsStaff() {
		sb.WriteString("/inbox — что требует действия\n")
	}
	if a.IsAdmin() {
		sb.WriteString("/admin — сводка по компании\n")
	}
	sb.WriteString("/help — эта справка\n\nУведомления по обращениям приходят сюда автоматически.")
	return sb.String()
}

const (
	msgWelcomeUnlinked   = "Здравствуйте! Это бот Modus Voice: он присылает уведомления по обращениям и помогает быстро их проверить.\n\nЧтобы привязать аккаунт, откройте Modus Voice, перейдите в раздел «Telegram» и пришлите сюда 6-значный код. Код обновляется каждые 20 секунд."
	msgLinkFirst         = "Сначала привяжите аккаунт: пришлите 6-значный код из раздела «Telegram» в Modus Voice."
	msgBadCodeShape      = "Код состоит из 6 символов (буквы и цифры). Скопируйте его из раздела «Telegram» в Modus Voice."
	msgCodeInvalid       = "Код не найден или устарел. Он обновляется каждые 20 секунд — возьмите свежий код на сайте."
	msgChatAlreadyLinked = "Этот чат уже привязан к аккаунту."
	msgUserAlreadyLinked = "Этот аккаунт уже привязан к другому чату Telegram."
	msgInternal          = "Что-то пошло не так. Попробуйте позже."
	msgStaffOnly         = "Команда доступна руководителям и администраторам."
	msgAdminOnly         = "Команда доступна только администраторам."
	msgRequestNotFound   = "Обращение не найдено."
)
