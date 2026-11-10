package identity

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

type handler struct {
	svc          *Service
	trustedProxy bool
}

// clientIP returns the TCP peer address. Forwarding headers are honoured only when the
// deployment states it runs behind a trusted reverse proxy (TRUSTED_PROXY=true).
func clientIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// the proxy appends the real peer last; earlier entries are client-controlled
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(ip) != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ClientIP resolves the client address with the deployment's proxy policy (used by other modules'
// per-IP throttles, e.g. the anonymous token endpoints).
func (s *Service) ClientIP(r *http.Request) string { return clientIP(r, s.cfg.TrustedProxy) }

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, tok, err := h.svc.Login(r.Context(), in.Email, in.Password, clientIP(r, h.trustedProxy))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: secureRequest(r), MaxAge: int(SessionTTL.Seconds()), Expires: time.Now().Add(SessionTTL),
	})
	httpx.JSON(w, http.StatusOK, view)
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		if err := h.svc.Logout(r.Context(), c.Value); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: secureRequest(r), MaxAge: -1})
	httpx.NoContent(w)
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	a, err := authz.MustActor(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	v, err := h.svc.Me(r.Context(), a)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *handler) demoUsers(w http.ResponseWriter, r *http.Request) {
	hint, users, err := h.svc.DemoUsers(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"password_hint": hint, "users": users})
}

type userInputJSON struct {
	Name         *string          `json:"name"`
	Email        *string          `json:"email"`
	Role         *string          `json:"role"`
	DepartmentID httpx.Opt[int64] `json:"department_id"`
	Active       *bool            `json:"active"`
	Password     *string          `json:"password"`
}

func (j userInputJSON) toInput() UserInput {
	return UserInput{Name: j.Name, Email: j.Email, Role: j.Role, DepartmentID: j.DepartmentID.Ptr(),
		DepartmentSet: j.DepartmentID.Set, Active: j.Active, Password: j.Password}
}

func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	a, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListUsers(r.Context(), a)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
	a, _ := authz.FromContext(r.Context())
	if err := authz.RequireAdmin(a); err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in userInputJSON
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.svc.CreateUser(r.Context(), a, in.toInput())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}

func (h *handler) updateUser(w http.ResponseWriter, r *http.Request) {
	a, _ := authz.FromContext(r.Context())
	if err := authz.RequireAdmin(a); err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in userInputJSON
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.svc.UpdateUser(r.Context(), a, id, in.toInput())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

// RequireAuth resolves the session cookie into an Actor or responds 401.
func RequireAuth(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookie)
			if err != nil {
				httpx.Error(w, r, errUnauth)
				return
			}
			a, err := svc.Authenticate(r.Context(), c.Value)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(authz.WithActor(r.Context(), a)))
		})
	}
}

func mountRoutes(h *handler, pub, priv chi.Router) {
	pub.Post("/auth/login", h.login)
	pub.Post("/auth/logout", h.logout)
	pub.Get("/auth/demo-users", h.demoUsers)
	priv.Get("/auth/me", h.me)
	priv.Get("/admin/users", h.listUsers)
	priv.Post("/admin/users", h.createUser)
	priv.Patch("/admin/users/{id}", h.updateUser)
}
