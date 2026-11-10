package telegram

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/platform/db"
)

// Deps are the collaborators of the module.
type Deps struct {
	DB       *sql.DB
	Cfg      Config
	Requests RequestsAPI
	// Notifications is the notification service (mirroring + marking history as delivered on link).
	Notifications *notifications.Service
	Cache         Cache // nil: in-memory
	API           API   // nil: HTTP client for Cfg (tests inject their own)
	Now           func() time.Time
	Log           *slog.Logger
}

// Module wires the bot, the notification mirror and the HTTP routes. With no token it only serves
// the routes (status enabled=false, link-code 404) and starts nothing.
type Module struct {
	cfg    Config
	store  Store
	codes  *Codes
	cache  Cache
	bot    *Bot
	mirror *Mirror
	log    *slog.Logger
	now    func() time.Time
	h      *handler
	once   sync.Once
}

// New builds the module. It performs no network calls.
func New(d Deps) *Module {
	cfg := d.Cfg.withDefaults()
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	m := &Module{cfg: cfg, log: d.Log.With("module", "telegram"), now: d.Now}
	m.store = &SQLiteStore{DB: d.DB}
	m.h = &handler{m: m}
	if !cfg.Enabled() {
		return m
	}
	m.cache = d.Cache
	if m.cache == nil {
		m.cache = NewMemoryCache(d.Now)
	}
	m.codes = NewCodes(m.cache, d.Now)
	var api API = d.API
	if api == nil {
		api = NewClient(cfg)
	}
	var markSent func(ctx context.Context, q db.Querier, userID int64) error
	var outbox Outbox
	if d.Notifications != nil {
		markSent = d.Notifications.MarkAllTelegramSent
		outbox = d.Notifications
	}
	m.bot = &Bot{cfg: cfg, api: api, store: m.store, codes: m.codes, reqs: d.Requests, markSent: markSent,
		log: m.log, now: d.Now, fails: newFailureLimiter(d.Now), sleep: sleepCtx}
	if outbox != nil {
		m.mirror = &Mirror{api: api, outbox: outbox, cfg: cfg, log: m.log, now: d.Now}
	}
	return m
}

// Enabled reports whether the bot is configured (token set). The meta handler uses this for
// features.telegram.
func (m *Module) Enabled() bool { return m.cfg.Enabled() }

// Name implements the HTTPModule contract.
func (m *Module) Name() string { return "telegram" }

// Routes mounts the session-protected endpoints.
func (m *Module) Routes(_, priv chi.Router) {
	priv.Get("/telegram/status", m.h.status)
	priv.Get("/telegram/link-code", m.h.linkCode)
}

// Bot exposes the update handler (tests, diagnostics).
func (m *Module) Bot() *Bot { return m.bot }

// Mirror exposes the notification mirror (tests).
func (m *Module) Mirror() *Mirror { return m.mirror }

// Codes exposes the link code service (tests).
func (m *Module) Codes() *Codes { return m.codes }

// Run starts the long polling bot, the mirroring worker and the cache janitor, and returns when
// ctx is cancelled and all of them stopped. It returns immediately when the module is disabled.
func (m *Module) Run(ctx context.Context) {
	if !m.Enabled() || m.bot == nil {
		return
	}
	var wg sync.WaitGroup
	start := func(f func(context.Context)) {
		wg.Add(1)
		go func() { defer wg.Done(); f(ctx) }()
	}
	if mc, ok := m.cache.(*MemoryCache); ok {
		start(func(c context.Context) { mc.RunJanitor(c, 30*time.Second) })
	}
	start(m.bot.Run)
	if m.mirror != nil {
		start(m.mirror.Run)
	}
	wg.Wait()
}

// botURL is the t.me link shown to users: the configured URL, else derived from the bot username.
func (m *Module) botURL() string {
	if m.cfg.BotURL != "" {
		return m.cfg.BotURL
	}
	if m.bot != nil {
		if u := m.bot.Username(); u != "" {
			return "https://t.me/" + strings.TrimPrefix(u, "@")
		}
	}
	return ""
}

// active reports whether the bot is configured and Telegram has not rejected the token.
func (m *Module) active() bool {
	return m.Enabled() && m.bot != nil && !m.bot.disabled.Load()
}
