package org

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

type handler struct{ svc *Service }

func actor(r *http.Request) authz.Actor { a, _ := authz.FromContext(r.Context()); return a }

func items[T any](w http.ResponseWriter, r *http.Request, list []T, err error) {
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *handler) departments(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.AdminDepartments(r.Context(), actor(r))
	items(w, r, l, err)
}

func (h *handler) categories(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.AdminCategories(r.Context(), actor(r))
	items(w, r, l, err)
}

func (h *handler) rules(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.AdminRules(r.Context(), actor(r))
	items(w, r, l, err)
}

// decodeAdmin checks admin rights before reading the body so non-admins get 403, not 400.
func decodeAdmin(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := authz.RequireAdmin(actor(r)); err != nil {
		httpx.Error(w, r, err)
		return false
	}
	if err := httpx.Decode(r, dst); err != nil {
		httpx.Error(w, r, err)
		return false
	}
	return true
}

func (h *handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var in CategoryInput
	if !decodeAdmin(w, r, &in) {
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), actor(r), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in CategoryInput
	if !decodeAdmin(w, r, &in) {
		return
	}
	c, err := h.svc.UpdateCategory(r.Context(), actor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *handler) createRule(w http.ResponseWriter, r *http.Request) {
	var in RuleInput
	if !decodeAdmin(w, r, &in) {
		return
	}
	rule, err := h.svc.CreateRule(r.Context(), actor(r), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rule)
}

func (h *handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in RuleInput
	if !decodeAdmin(w, r, &in) {
		return
	}
	rule, err := h.svc.UpdateRule(r.Context(), actor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rule)
}

func (h *handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.DeleteRule(r.Context(), actor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func mountRoutes(h *handler, priv chi.Router) {
	priv.Get("/admin/departments", h.departments)
	priv.Get("/admin/categories", h.categories)
	priv.Post("/admin/categories", h.createCategory)
	priv.Patch("/admin/categories/{id}", h.updateCategory)
	priv.Get("/admin/routing-rules", h.rules)
	priv.Post("/admin/routing-rules", h.createRule)
	priv.Patch("/admin/routing-rules/{id}", h.updateRule)
	priv.Delete("/admin/routing-rules/{id}", h.deleteRule)
}
