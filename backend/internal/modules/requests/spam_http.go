package requests

import (
	"net/http"
	"strings"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/httpx"
)

// Browser cookies for anonymous requests. The server never stores the cookie values: tokens are read
// on each status check and only their hashes are compared with the block list.
const (
	anonTokensCookie  = "mv_anon_tokens"  // canonical tokens of this browser's anonymous requests, dot-separated
	anonBlockedCookie = "mv_anon_blocked" // "1" once the browser was found to hold a blocked token
	anonCookieMaxAge  = 30 * 24 * 60 * 60
)

func anonCookie(r *http.Request, name, value string) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: anonCookieMaxAge, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil}
}

func anonBlockedByCookie(r *http.Request) bool {
	c, err := r.Cookie(anonBlockedCookie)
	return err == nil && c.Value == "1"
}

func anonTokensFromCookie(r *http.Request) []string {
	c, err := r.Cookie(anonTokensCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	return strings.Split(c.Value, ".")
}

// rememberAnonToken appends the token of a fresh anonymous request to the browser cookie (last 5 kept).
func rememberAnonToken(w http.ResponseWriter, r *http.Request, token string) {
	canon, ok := CanonicalToken(token)
	if !ok {
		return
	}
	list := []string{}
	for _, t := range anonTokensFromCookie(r) {
		if t != canon {
			list = append(list, t)
		}
	}
	list = append(list, canon)
	if len(list) > MaxBrowserTokens {
		list = list[len(list)-MaxBrowserTokens:]
	}
	http.SetCookie(w, anonCookie(r, anonTokensCookie, strings.Join(list, ".")))
}

// anonStatus is called when the site opens: the browser sends its cookies, the server answers whether
// anonymous writing is allowed here. A blocked browser gets the blocked cookie.
func (h *handler) anonStatus(w http.ResponseWriter, r *http.Request) {
	allowed, err := h.svc.AnonymousStatus(r.Context(), h.clientIP(r), anonTokensFromCookie(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if anonBlockedByCookie(r) {
		allowed = false
	}
	if !allowed {
		http.SetCookie(w, anonCookie(r, anonBlockedCookie, "1"))
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"anonymous_allowed": allowed})
}

// markSpam: POST /requests/{id}/spam (admin or the current owner).
func (h *handler) markSpam(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.MarkSpam(r.Context(), actor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// spamBlockedError is returned when a browser with the blocked cookie tries to write anonymously.
func spamBlockedError() error {
	return apperr.Invalid("is_anonymous", "Анонимная подача с этого браузера недоступна")
}
