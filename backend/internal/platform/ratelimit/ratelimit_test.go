package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterWindow(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	l := New(2, time.Hour)
	if !l.Allow("u", t0) || !l.Allow("u", t0.Add(time.Minute)) {
		t.Fatal("first two must pass")
	}
	if l.Allow("u", t0.Add(2*time.Minute)) {
		t.Fatal("third within the window must be rejected")
	}
	if !l.Allow("v", t0) {
		t.Fatal("keys are independent")
	}
	if !l.Allow("u", t0.Add(61*time.Minute)) {
		t.Fatal("oldest event left the window")
	}
	if !New(0, time.Hour).Allow("u", t0) {
		t.Fatal("max<=0 disables")
	}
}
