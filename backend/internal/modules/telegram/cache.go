package telegram

import (
	"context"
	"sync"
	"time"
)

// Cache is a small string key/value store with per-entry TTL. The link codes live here. The
// in-memory implementation is enough for a single node; for several instances implement this
// interface on top of Redis (GET / SET key value PX ttl / DEL) and pass it via Deps.Cache.
type Cache interface {
	// Get returns the value if present and not expired.
	Get(key string) (string, bool)
	// Set stores value for ttl (ttl <= 0 is treated as already expired).
	Set(key, value string, ttl time.Duration)
	// Delete removes key (no-op if absent).
	Delete(key string)
}

type cacheEntry struct {
	value   string
	expires time.Time
}

// MemoryCache is a goroutine-safe Cache with lazy expiry and an optional janitor.
type MemoryCache struct {
	mu  sync.Mutex
	m   map[string]cacheEntry
	now func() time.Time
}

// NewMemoryCache builds an empty cache; now may be nil (time.Now).
func NewMemoryCache(now func() time.Time) *MemoryCache {
	if now == nil {
		now = time.Now
	}
	return &MemoryCache{m: map[string]cacheEntry{}, now: now}
}

func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return "", false
	}
	if !c.now().Before(e.expires) {
		delete(c.m, key)
		return "", false
	}
	return e.value, true
}

func (c *MemoryCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{value: value, expires: c.now().Add(ttl)}
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

// Len returns the number of stored entries (expired ones included until swept); for tests.
func (c *MemoryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// Sweep removes expired entries.
func (c *MemoryCache) Sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for k, e := range c.m {
		if !now.Before(e.expires) {
			delete(c.m, k)
		}
	}
}

// RunJanitor sweeps expired entries every interval until ctx is cancelled.
func (c *MemoryCache) RunJanitor(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.Sweep()
		}
	}
}
