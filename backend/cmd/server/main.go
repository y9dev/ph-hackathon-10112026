// Command server runs the Modus Voice API and embedded UI.
//
//	go run ./cmd/server            # serve on $ADDR (default :8080)
//	go run ./cmd/server --seed     # seed demo data into an empty DB, then serve
//	go run ./cmd/server --seed-only
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"modusvoice/internal/app"
	"modusvoice/internal/seed"
)

func main() {
	seedFlag := flag.Bool("seed", false, "seed synthetic demo data if the database is empty")
	seedOnly := flag.Bool("seed-only", false, "seed (if empty) and exit")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg := app.LoadConfig()
	a, err := app.New(cfg, log)
	if err != nil {
		log.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	switch seedDecision(*seedFlag || *seedOnly, cfg) {
	case seedRefuse:
		log.Error("refusing to seed demo data: DEMO_MODE is not true (demo accounts share a public password)")
		a.Close()
		os.Exit(1)
	case seedSkipWarn:
		log.Warn("SEED_DEMO ignored: demo data is seeded only when DEMO_MODE=true")
	case seedRun:
		if err := runSeed(a, cfg); err != nil {
			log.Error("seed failed", "err", err)
			os.Exit(1)
		}
	}
	if *seedOnly {
		return
	}
	if cfg.DemoMode {
		log.Warn("DEMO_MODE is on: demo accounts are listed on the login screen")
	}

	startupPing()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := a.Run(ctx); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func runSeed(a *app.App, cfg app.Config) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(seed.DemoPassword), cfg.BcryptCost)
	if err != nil {
		return err
	}
	err = seed.Run(context.Background(), a.DB, time.Now(), string(hash))
	if errors.Is(err, seed.ErrAlreadySeeded) {
		a.Log.Info("seed skipped: database already has data")
		return nil
	}
	if err == nil {
		a.Log.Info("demo data seeded", "password", "see backend/README.md")
	}
	return err
}

type seedAction int

const (
	seedNone     seedAction = iota // nothing requested
	seedRun                        // seed (if the DB is empty)
	seedSkipWarn                   // SEED_DEMO without DEMO_MODE: skip with a warning
	seedRefuse                     // explicit --seed/--seed-only without DEMO_MODE: exit non-zero
)

// seedDecision gates seeding: demo accounts share a public password, so seeding is allowed only
// when DEMO_MODE=true.
func seedDecision(explicit bool, cfg app.Config) seedAction {
	switch {
	case !explicit && !cfg.SeedDemo:
		return seedNone
	case !cfg.DemoMode && explicit:
		return seedRefuse
	case !cfg.DemoMode:
		return seedSkipWarn
	}
	return seedRun
}
