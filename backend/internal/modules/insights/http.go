package insights

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

type handler struct{ svc *Service }

func actor(r *http.Request) authz.Actor { a, _ := authz.FromContext(r.Context()); return a }

func (h *handler) summary(w http.ResponseWriter, r *http.Request) {
	a := actor(r)
	if err := authz.RequireStaff(a); err != nil {
		httpx.Error(w, r, err)
		return
	}
	var f SummaryFilter
	var err error
	if f.From, err = h.svc.ParseDate("from", r.URL.Query().Get("from")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if f.To, err = h.svc.ParseDate("to", r.URL.Query().Get("to")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if f.DepartmentID, err = httpx.QueryInt(r, "department_id"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Summary(r.Context(), a, f)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *handler) groups(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.SuggestedGroups(r.Context(), actor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) themes(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Themes(r.Context(), actor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func mountRoutes(h *handler, priv chi.Router) {
	priv.Get("/analytics/summary", h.summary)
	priv.Get("/analytics/suggested-groups", h.groups)
	priv.Get("/themes", h.themes)
}
