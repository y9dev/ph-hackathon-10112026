package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

type ctxKey int

const requestIDKey ctxKey = 1

// RequestID returns the request id stored by the RequestID middleware.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// WithRequestID assigns a request id (X-Request-ID echoed back).
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Logger logs one line per request (no bodies, no query strings to avoid leaking data).
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			log.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
				"bytes", rec.bytes, "dur_ms", time.Since(start).Milliseconds(), "request_id", RequestID(r.Context()))
		})
	}
}

// Recover converts panics into a 500 envelope and logs the stack.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					if p == http.ErrAbortHandler {
						panic(p)
					}
					log.Error("panic", "panic", p, "stack", string(debug.Stack()), "request_id", RequestID(r.Context()))
					JSON(w, http.StatusInternalServerError, ErrorBody{Error: ErrorPayload{Code: "internal", Message: "Внутренняя ошибка сервера. Попробуйте позже."}})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets baseline security headers for every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func isUnsafe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// OriginCheck rejects unsafe-method requests whose Origin (or Referer) does not match the
// request host or an explicitly allowed origin. Requests without either header (non-browser
// clients) pass; browsers always send Origin on cross-origin POSTs. Complements SameSite=Lax.
func OriginCheck(allowed []string) func(http.Handler) http.Handler {
	allow := map[string]bool{}
	for _, a := range allowed {
		a = strings.TrimRight(strings.TrimSpace(a), "/")
		if a != "" {
			allow[strings.ToLower(a)] = true
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isUnsafe(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
				forbidOrigin(w)
				return
			}
			src := r.Header.Get("Origin")
			if src == "" || src == "null" {
				if ref := r.Header.Get("Referer"); ref != "" {
					src = ref
				}
			}
			if src == "" {
				next.ServeHTTP(w, r)
				return
			}
			u, err := url.Parse(src)
			if err != nil || u.Host == "" {
				forbidOrigin(w)
				return
			}
			origin := strings.ToLower(u.Scheme + "://" + u.Host)
			if strings.EqualFold(u.Host, r.Host) || allow[origin] {
				next.ServeHTTP(w, r)
				return
			}
			forbidOrigin(w)
		})
	}
}

func forbidOrigin(w http.ResponseWriter) {
	JSON(w, http.StatusForbidden, ErrorBody{Error: ErrorPayload{Code: "forbidden", Message: "Запрос отклонён: недопустимый источник"}})
}

// BodyLimit caps request bodies at n bytes, except requests for which bigger(r) returns a
// larger limit (multipart uploads).
func BodyLimit(n int64, bigger func(*http.Request) int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := n
			if bigger != nil {
				if b := bigger(r); b > 0 {
					limit = b
				}
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
