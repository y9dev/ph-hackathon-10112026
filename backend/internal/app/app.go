// Package app is the composition root: config, database, modules, router and server lifecycle.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/modules/identity"
	"modusvoice/internal/modules/insights"
	"modusvoice/internal/modules/notifications"
	"modusvoice/internal/modules/org"
	"modusvoice/internal/modules/requests"
	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/db"
	"modusvoice/internal/platform/eventbus"
	"modusvoice/internal/platform/httpx"
	"modusvoice/internal/platform/migrate"
	"modusvoice/internal/webui"
	"modusvoice/migrations"

	_ "time/tzdata" // DISPLAY_TZ works on hosts without a tz database (Windows, scratch images)
)

// Runner is a background job started with the server.
type Runner interface{ Run(ctx context.Context) }

// App holds the wired application.
type App struct {
	Cfg     Config
	DB      *sql.DB
	Log     *slog.Logger
	Bus     *eventbus.Bus
	Loc     *time.Location
	Handler http.Handler

	Identity      *identity.Module
	Org           *org.Module
	Notifications *notifications.Module
	Requests      *requests.Module
	Insights      *insights.Module

	runners []Runner
}

// New opens the database, applies migrations and wires modules and routes.
func New(cfg Config, log *slog.Logger) (*App, error) {
	if log == nil {
		log = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	loc, err := time.LoadLocation(cfg.DisplayTZ)
	if err != nil || cfg.DisplayTZ == "" {
		loc = time.UTC
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "uploads"), 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	d, err := db.Open(filepath.Join(cfg.DataDir, "modusvoice.db"))
	if err != nil {
		return nil, err
	}
	applied, err := migrate.Up(context.Background(), d, migrations.FS)
	if err != nil {
		_ = d.Close()
		return nil, err
	}
	if len(applied) > 0 {
		log.Info("migrations applied", "versions", applied)
	}
	a := &App{Cfg: cfg, DB: d, Log: log, Bus: eventbus.New(log), Loc: loc}
	a.Handler = a.router(a.registerModules())
	return a, nil
}

func (a *App) router(mods []HTTPModule) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.WithRequestID, httpx.Recover(a.Log), httpx.Logger(a.Log), httpx.SecurityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.PingContext(r.Context()); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_unavailable"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(httpx.OriginCheck(a.Cfg.AllowedOrigins))
		api.Use(httpx.BodyLimit(1<<20, func(r *http.Request) int64 {
			if r.Method == http.MethodPost && strings.TrimSuffix(r.URL.Path, "/") == "/api/v1/requests" {
				return requests.MaxCreateBody
			}
			return 0
		}))
		api.NotFound(func(w http.ResponseWriter, r *http.Request) { httpx.Error(w, r, apperr.NotFound()) })
		api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			httpx.JSON(w, http.StatusMethodNotAllowed, httpx.ErrorBody{Error: httpx.ErrorPayload{Code: "method_not_allowed", Message: "Метод не поддерживается"}})
		})
		priv := api.With(identity.RequireAuth(a.Identity.Service))
		for _, m := range mods {
			m.Routes(api, priv)
		}
	})
	r.Handle("/*", webui.Handler())
	return r
}

// Run serves HTTP until ctx is cancelled, with background runners and graceful shutdown.
func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr: a.Cfg.Addr, Handler: a.Handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	jobCtx, stopJobs := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, r := range a.runners {
		wg.Add(1)
		go func(r Runner) { defer wg.Done(); r.Run(jobCtx) }(r)
	}
	errc := make(chan error, 1)
	go func() {
		a.Log.Info("listening", "addr", a.Cfg.Addr)
		errc <- srv.ListenAndServe()
	}()
	var err error
	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	stopJobs()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	wg.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return err
}

// Close releases the database.
func (a *App) Close() error { return a.DB.Close() }

func logSubscriber(log *slog.Logger) eventbus.Handler {
	return func(_ context.Context, e eventbus.Event) error {
		log.Info("event", "topic", e.Topic, "request_id", e.RequestID, "number", e.Number)
		return nil
	}
}
