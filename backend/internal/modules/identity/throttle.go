package identity

import (
	"sync"
	"time"
)

// Throttle limits failed login attempts per key (IP or account) within a sliding window.
type Throttle struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
}

func NewThrottle(max int, window time.Duration) *Throttle {
	return &Throttle{max: max, window: window, fails: map[string][]time.Time{}}
}

func (t *Throttle) prune(key string, now time.Time) []time.Time {
	list := t.fails[key]
	cut := 0
	for cut < len(list) && now.Sub(list[cut]) >= t.window {
		cut++
	}
	list = list[cut:]
	if len(list) == 0 {
		delete(t.fails, key)
	} else {
		t.fails[key] = list
	}
	return list
}

// Blocked reports whether key has reached the failure limit in the window.
func (t *Throttle) Blocked(key string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.prune(key, now)) >= t.max
}

// Fail records a failed attempt.
func (t *Throttle) Fail(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fails[key] = append(t.prune(key, now), now)
	if len(t.fails) > 100000 { // memory guard against key flooding
		t.fails = map[string][]time.Time{}
	}
}

// Reset clears failures for key (after a successful login).
func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.fails, key)
}
