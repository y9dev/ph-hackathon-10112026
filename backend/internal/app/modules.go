package app

import (
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/integrations"
	"modusvoice/internal/modules/identity"
	"modusvoice/internal/modules/insights"
	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/org"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/modules/telegram"
	"modusvoice/internal/seed"
)

// HTTPModule is implemented by every module: it mounts its routes on the public (no session)
// and private (session required) /api/v1 routers.
type HTTPModule interface {
	Name() string
	Routes(pub, priv chi.Router)
}

// registerModules is THE place modules are constructed, wired and registered.
// To add a module: build it here, append it to the returned list, done.
func (a *App) registerModules() []HTTPModule {
	now := a.Cfg.Now

	a.Identity = identity.New(a.DB, identity.Config{
		BcryptCost: a.Cfg.BcryptCost, DemoMode: a.Cfg.DemoMode,
		DemoEmails: seed.DemoEmails, DemoPassword: seed.DemoPassword, TrustedProxy: a.Cfg.TrustedProxy,
	}, now)
	a.Org = org.New(a.DB, a.Identity.Service)
	a.Notifications = notifications.New(a.DB, now)
	a.Insights = insights.New(a.DB, now, a.Loc)
	a.Requests = requests.New(requests.Deps{
		DB: a.DB, Org: a.Org.Service, People: a.Identity.Service, Notifier: a.Notifications.Service,
		Similar: a.Insights.Service, Bus: a.Bus, Now: now, UploadDir: filepath.Join(a.Cfg.DataDir, "uploads"), Log: a.Log,
		CreatePerHour: a.Cfg.CreatePerHour, CommentPerHour: a.Cfg.CommentPerHour,
	}, a.Loc)
	a.Insights.Service.SetOverdueLister(a.Requests.Service)
	tg := telegram.New(telegram.Deps{
		DB: a.DB, Now: now, Log: a.Log, Notifications: a.Notifications.Service,
		Cfg:      telegram.Config{Token: a.Cfg.TelegramBotToken, BotURL: a.Cfg.TelegramBotURL, AppBaseURL: a.Cfg.AppBaseURL, APIBase: a.Cfg.TelegramAPIBase},
		Requests: &telegram.RequestsAdapter{Svc: a.Requests.Service, DB: a.DB, Loc: a.Loc, Now: now},
	})

	// non-critical event subscribers
	a.Bus.Subscribe("*", logSubscriber(a.Log))
	integrations.SubscribeNotifier(a.Bus, integrations.LogNotifier{Log: a.Log.With("adapter", "log-notifier")})

	if !a.Cfg.DisableTicker {
		a.runners = append(a.runners, a.Requests.Service)
	}
	if tg.Enabled() { // no token: no goroutines at all (routes still answer enabled=false / 404)
		a.runners = append(a.runners, tg)
	}
	return []HTTPModule{a.Identity, a.Org, a.Notifications, a.Requests, a.Insights, tg}
}
