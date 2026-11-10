package telegram

import (
	"math"
	"net/http"
	"time"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

type handler struct{ m *Module }

type statusView struct {
	Enabled          bool       `json:"enabled"`
	BotURL           *string    `json:"bot_url"`
	BotUsername      *string    `json:"bot_username"`
	Linked           bool       `json:"linked"`
	LinkedAt         *time.Time `json:"linked_at"`
	TelegramUsername *string    `json:"telegram_username"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// status answers GET /telegram/status for the current user. It never exposes chat ids.
func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	a, _ := authz.FromContext(r.Context())
	v := statusView{Enabled: h.m.active()}
	if !v.Enabled {
		httpx.JSON(w, http.StatusOK, v)
		return
	}
	v.BotURL = strPtr(h.m.botURL())
	v.BotUsername = strPtr(h.m.bot.Username())
	l, ok, err := h.m.store.LinkByUser(r.Context(), a.UserID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if ok {
		v.Linked = true
		t := l.LinkedAt
		v.LinkedAt = &t
		v.TelegramUsername = strPtr(l.Username)
	}
	httpx.JSON(w, http.StatusOK, v)
}

type linkCodeView struct {
	Code             string `json:"code"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	PeriodSeconds    int    `json:"period_seconds"`
	BotURL           string `json:"bot_url"`
}

// linkCode answers GET /telegram/link-code: 404 when disabled, 409 when already linked.
func (h *handler) linkCode(w http.ResponseWriter, r *http.Request) {
	if !h.m.active() {
		httpx.Error(w, r, apperr.NotFound())
		return
	}
	a, _ := authz.FromContext(r.Context())
	if _, linked, err := h.m.store.LinkByUser(r.Context(), a.UserID); err != nil {
		httpx.Error(w, r, err)
		return
	} else if linked {
		httpx.Error(w, r, apperr.Conflict("Аккаунт уже привязан к Telegram"))
		return
	}
	code, left, err := h.m.codes.Current(a.UserID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	secs := int(math.Ceil(left.Seconds()))
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, linkCodeView{Code: code, ExpiresInSeconds: secs, PeriodSeconds: int(CodePeriod / time.Second), BotURL: h.m.botURL()})
}
