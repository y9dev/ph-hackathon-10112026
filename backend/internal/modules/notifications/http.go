package notifications

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

type handler struct{ svc *Service }

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	a, _ := authz.FromContext(r.Context())
	unread, err := httpx.QueryBool(r, "unread")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	limit, err := httpx.QueryInt(r, "limit")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	l := 20
	if limit != nil {
		l = int(*limit)
	}
	res, err := h.svc.List(r.Context(), a, unread != nil && *unread, l)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *handler) markRead(w http.ResponseWriter, r *http.Request) {
	a, _ := authz.FromContext(r.Context())
	var in struct {
		IDs []int64 `json:"ids"`
		All bool    `json:"all"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.MarkRead(r.Context(), a, in.IDs, in.All); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func mountRoutes(h *handler, priv chi.Router) {
	priv.Get("/notifications", h.list)
	priv.Post("/notifications/read", h.markRead)
}
