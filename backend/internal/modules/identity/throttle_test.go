package identity

import (
	"testing"
	"time"
)

func TestThrottleWindow(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	th := NewThrottle(3, 15*time.Minute)
	for i := 0; i < 3; i++ {
		if th.Blocked("k", t0) {
			t.Fatalf("blocked after %d failures", i)
		}
		th.Fail("k", t0.Add(time.Duration(i)*time.Minute))
	}
	if !th.Blocked("k", t0.Add(3*time.Minute)) {
		t.Fatal("must block at the limit")
	}
	if th.Blocked("other", t0) {
		t.Fatal("keys are independent")
	}
	if th.Blocked("k", t0.Add(16*time.Minute)) {
		t.Fatal("first failure left the window, must unblock")
	}
	th.Reset("k")
	if th.Blocked("k", t0.Add(3*time.Minute)) {
		t.Fatal("reset clears")
	}
}

func TestValidEmail(t *testing.T) {
	for e, ok := range map[string]bool{"a@b.kz": true, "a.b@modus.demo": true, "ab": false, "a@b": false, "a b@c.kz": false, "@c.kz": false} {
		if validEmail(e) != ok {
			t.Errorf("validEmail(%q) != %v", e, ok)
		}
	}
}
