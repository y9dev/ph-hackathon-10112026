package main

import (
	"testing"

	"modusvoice/internal/app"
)

func TestSeedDecision(t *testing.T) {
	cases := []struct {
		name     string
		explicit bool
		cfg      app.Config
		want     seedAction
	}{
		{"nothing requested", false, app.Config{}, seedNone},
		{"nothing requested, demo mode", false, app.Config{DemoMode: true}, seedNone},
		{"--seed without demo mode refuses", true, app.Config{}, seedRefuse},
		{"--seed with SEED_DEMO but no demo mode refuses", true, app.Config{SeedDemo: true}, seedRefuse},
		{"SEED_DEMO without demo mode is skipped", false, app.Config{SeedDemo: true}, seedSkipWarn},
		{"--seed with demo mode runs", true, app.Config{DemoMode: true}, seedRun},
		{"SEED_DEMO with demo mode runs (docker-compose)", false, app.Config{DemoMode: true, SeedDemo: true}, seedRun},
	}
	for _, c := range cases {
		if got := seedDecision(c.explicit, c.cfg); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
