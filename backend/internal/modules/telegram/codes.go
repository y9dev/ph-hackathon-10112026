package telegram

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// CodePeriod is how long the code shown to a user stays current before it rotates.
	CodePeriod = 20 * time.Second
	// CodeGrace is how long the previous code stays valid after rotation.
	CodeGrace = 3 * time.Second
	// CodeLen is the number of characters of a link code.
	CodeLen = 6
	// CodeAlphabet has no 0/O/1/I/L: 23 letters + digits 2-9.
	CodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
)

// Codes issues and resolves short-lived link codes (user -> code and code -> user) in a Cache.
type Codes struct {
	cache Cache
	now   func() time.Time
	mu    sync.Mutex // serializes check-then-set on the cache
}

// NewCodes builds the code service.
func NewCodes(cache Cache, now func() time.Time) *Codes {
	if now == nil {
		now = time.Now
	}
	return &Codes{cache: cache, now: now}
}

func curKey(uid int64) string    { return "tg:cur:" + strconv.FormatInt(uid, 10) }
func codeKey(code string) string { return "tg:code:" + code }

// NormalizeCode upper-cases and trims user input; ok is false if it cannot be a code.
func NormalizeCode(s string) (string, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != CodeLen {
		return "", false
	}
	for _, r := range s {
		if !strings.ContainsRune(CodeAlphabet, r) {
			return "", false
		}
	}
	return s, true
}

func randomCode() (string, error) {
	out := make([]byte, 0, CodeLen)
	buf := make([]byte, 16)
	limit := byte(256 - 256%len(CodeAlphabet)) // rejection sampling: no modulo bias
	for len(out) < CodeLen {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("random code: %w", err)
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, CodeAlphabet[int(b)%len(CodeAlphabet)])
			if len(out) == CodeLen {
				break
			}
		}
	}
	return string(out), nil
}

// Current returns the user's current code and the time left in its window. A new code is issued
// when there is none or the window has passed; the previous code remains resolvable for CodeGrace.
func (c *Codes) Current(userID int64) (code string, remaining time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	prev := ""
	if v, ok := c.cache.Get(curKey(userID)); ok {
		// value: code|issuedAtUnixNano|previousCode
		parts := strings.Split(v, "|")
		if len(parts) == 3 {
			if ns, perr := strconv.ParseInt(parts[1], 10, 64); perr == nil {
				if left := time.Unix(0, ns).Add(CodePeriod).Sub(now); left > 0 {
					return parts[0], left, nil
				}
			}
			prev = parts[0]
		}
	}
	for i := 0; i < 20; i++ {
		cand, err := randomCode()
		if err != nil {
			return "", 0, err
		}
		if _, taken := c.cache.Get(codeKey(cand)); taken {
			continue
		}
		c.cache.Set(codeKey(cand), strconv.FormatInt(userID, 10), CodePeriod+CodeGrace)
		// the cur entry outlives the window by the grace period so Consume still finds the previous code
		c.cache.Set(curKey(userID), cand+"|"+strconv.FormatInt(now.UnixNano(), 10)+"|"+prev, CodePeriod+CodeGrace)
		return cand, CodePeriod, nil
	}
	return "", 0, fmt.Errorf("could not allocate a unique link code")
}

// Resolve maps a (normalized) code to its user id if the code is still valid.
func (c *Codes) Resolve(code string) (int64, bool) {
	v, ok := c.cache.Get(codeKey(code))
	if !ok {
		return 0, false
	}
	uid, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return uid, true
}

// Consume invalidates the code and the user's current code (single use).
func (c *Codes) Consume(userID int64, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Delete(codeKey(code))
	if v, ok := c.cache.Get(curKey(userID)); ok {
		parts := strings.Split(v, "|")
		if len(parts) == 3 {
			c.cache.Delete(codeKey(parts[0]))
			if parts[2] != "" {
				c.cache.Delete(codeKey(parts[2]))
			}
		}
	}
	c.cache.Delete(curKey(userID))
}
