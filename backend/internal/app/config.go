package app

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is read from the environment (and an optional .env file in the working directory).
type Config struct {
	Addr           string        // ADDR, default :8080
	DataDir        string        // DATA_DIR, default ./data (SQLite file + uploads)
	DemoMode       bool          // DEMO_MODE, default false: exposes /auth/demo-users
	SeedDemo       bool          // SEED_DEMO, default false: seed demo data if the DB is empty
	TrustedProxy   bool          // TRUSTED_PROXY, default false: trust X-Forwarded-For / X-Real-IP for the client IP
	CreatePerHour  int           // RATE_CREATE_PER_HOUR, default 20: POST /requests per user (negative disables)
	CommentPerHour int           // RATE_COMMENT_PER_HOUR, default 60: POST comments per user (negative disables)
	AllowedOrigins []string      // ALLOWED_ORIGINS, comma-separated extra origins for unsafe methods (e.g. Vite dev server)
	DisplayTZ      string        // DISPLAY_TZ, default Asia/Almaty: CSV export and analytics day buckets
	BcryptCost     int           // BCRYPT_COST, default 10 (tests use 4)
	OverdueEvery   time.Duration // overdue ticker period (fixed 1m; tests disable the ticker)
	DisableTicker  bool
	Now            func() time.Time // clock override for tests
}

// LoadConfig reads configuration from the environment.
func LoadConfig() Config {
	loadDotEnv(".env")
	return Config{
		Addr:           env("ADDR", ":8080"),
		DataDir:        env("DATA_DIR", "./data"),
		DemoMode:       envBool("DEMO_MODE", false),
		SeedDemo:       envBool("SEED_DEMO", false),
		TrustedProxy:   envBool("TRUSTED_PROXY", false),
		CreatePerHour:  envInt("RATE_CREATE_PER_HOUR", 0),
		CommentPerHour: envInt("RATE_COMMENT_PER_HOUR", 0),
		AllowedOrigins: splitList(os.Getenv("ALLOWED_ORIGINS")),
		DisplayTZ:      env("DISPLAY_TZ", "Asia/Almaty"),
		BcryptCost:     envInt("BCRYPT_COST", 10),
		OverdueEvery:   time.Minute,
	}
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(k)))
	if err != nil {
		return def
	}
	return v
}

func envInt(k string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(k)))
	if err != nil {
		return def
	}
	return v
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv sets KEY=VALUE pairs from path without overriding existing variables.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
