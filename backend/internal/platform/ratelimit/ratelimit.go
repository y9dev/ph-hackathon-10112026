// Package ratelimit is a small in-memory sliding-window limiter keyed by an arbitrary string.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most max events per key within window.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	events map[string][]time.Time
}

// New returns a limiter; max <= 0 disables limiting.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, events: map[string][]time.Time{}}
}

// Allow records an event for key at now and reports whether it is within the limit.
// A rejected event is not recorded.
func (l *Limiter) Allow(key string, now time.Time) bool {
	if l == nil || l.max <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.events[key]
	cut := 0
	for cut < len(list) && now.Sub(list[cut]) >= l.window {
		cut++
	}
	list = list[cut:]
	if len(list) >= l.max {
		l.events[key] = list
		return false
	}
	l.events[key] = append(list, now)
	if len(l.events) > 100000 { // memory guard
		l.events = map[string][]time.Time{key: l.events[key]}
	}
	return true
}
