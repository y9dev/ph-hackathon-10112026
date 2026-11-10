package telegram

import (
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 11, 10, 6, 0, 0, 0, time.UTC)} }
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestCodeAlphabetIsUnambiguous(t *testing.T) {
	if strings.ContainsAny(CodeAlphabet, "01OIL") {
		t.Fatalf("ambiguous characters in alphabet %q", CodeAlphabet)
	}
	clk := newFakeClock()
	c := NewCodes(NewMemoryCache(clk.Now), clk.Now)
	for uid := int64(1); uid <= 300; uid++ {
		code, left, err := c.Current(uid)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != CodeLen {
			t.Fatalf("code %q has length %d", code, len(code))
		}
		if _, ok := NormalizeCode(code); !ok {
			t.Fatalf("generated code %q does not pass NormalizeCode", code)
		}
		if left != CodePeriod {
			t.Fatalf("first remaining = %v", left)
		}
	}
}

func TestCodeRotationAndGrace(t *testing.T) {
	clk := newFakeClock()
	c := NewCodes(NewMemoryCache(clk.Now), clk.Now)

	first, _, _ := c.Current(7)
	clk.Advance(5 * time.Second)
	again, left, _ := c.Current(7)
	if again != first {
		t.Fatalf("code changed inside the window: %s -> %s", first, again)
	}
	if left != 15*time.Second {
		t.Fatalf("remaining = %v, want 15s", left)
	}
	if uid, ok := c.Resolve(first); !ok || uid != 7 {
		t.Fatalf("resolve current = %d %v", uid, ok)
	}

	clk.Advance(15 * time.Second) // window over: rotate
	second, left, _ := c.Current(7)
	if second == first {
		t.Fatal("code did not rotate after 20s")
	}
	if left != CodePeriod {
		t.Fatalf("remaining after rotation = %v", left)
	}
	// previous code: valid during the 3 s grace, gone afterwards
	if uid, ok := c.Resolve(first); !ok || uid != 7 {
		t.Fatalf("previous code must stay valid in grace, got %d %v", uid, ok)
	}
	clk.Advance(CodeGrace)
	if _, ok := c.Resolve(first); ok {
		t.Fatal("previous code still valid after grace")
	}
	if uid, ok := c.Resolve(second); !ok || uid != 7 {
		t.Fatal("new code must be valid")
	}
}

func TestCodeExpiresWhenNotRotated(t *testing.T) {
	clk := newFakeClock()
	c := NewCodes(NewMemoryCache(clk.Now), clk.Now)
	code, _, _ := c.Current(1)
	clk.Advance(CodePeriod + CodeGrace)
	if _, ok := c.Resolve(code); ok {
		t.Fatal("code valid after period+grace")
	}
}

func TestCodeSingleUse(t *testing.T) {
	clk := newFakeClock()
	c := NewCodes(NewMemoryCache(clk.Now), clk.Now)
	old, _, _ := c.Current(3)
	clk.Advance(21 * time.Second)
	cur, _, _ := c.Current(3) // old is in grace
	c.Consume(3, cur)
	if _, ok := c.Resolve(cur); ok {
		t.Fatal("consumed code still resolves")
	}
	if _, ok := c.Resolve(old); ok {
		t.Fatal("previous code of the same user must be invalidated too")
	}
	next, _, _ := c.Current(3)
	if next == cur {
		t.Fatal("a consumed code must not be re-issued as current")
	}
}

func TestCodesArePerUser(t *testing.T) {
	clk := newFakeClock()
	c := NewCodes(NewMemoryCache(clk.Now), clk.Now)
	a, _, _ := c.Current(1)
	b, _, _ := c.Current(2)
	if a == b {
		t.Fatal("two users got the same code")
	}
	if uid, _ := c.Resolve(a); uid != 1 {
		t.Fatalf("code a -> %d", uid)
	}
	if uid, _ := c.Resolve(b); uid != 2 {
		t.Fatalf("code b -> %d", uid)
	}
}

func TestNormalizeCode(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"k7m2qx", "K7M2QX", true},
		{"  K7M2QX \n", "K7M2QX", true},
		{"K7M2Q", "", false},
		{"K7M2QXX", "", false},
		{"K7M2Q0", "", false}, // 0 not in alphabet
		{"K7M2QI", "", false},
		{"/start", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeCode(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("NormalizeCode(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMemoryCacheTTLAndSweep(t *testing.T) {
	clk := newFakeClock()
	c := NewMemoryCache(clk.Now)
	c.Set("a", "1", 10*time.Second)
	c.Set("b", "2", 20*time.Second)
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatal("get a")
	}
	clk.Advance(10 * time.Second)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a must be expired")
	}
	c.Set("c", "3", time.Second)
	clk.Advance(2 * time.Second)
	c.Sweep()
	if c.Len() != 1 {
		t.Fatalf("len after sweep = %d, want 1 (b)", c.Len())
	}
	c.Delete("b")
	if _, ok := c.Get("b"); ok {
		t.Fatal("deleted key present")
	}
}
