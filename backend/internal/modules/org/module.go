package org

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
)

// Module wires the org service and admin routes.
type Module struct {
	Service *Service
	h       *handler
}

func New(d *sql.DB, people People) *Module {
	svc := NewService(&SQLiteStore{DB: d}, people)
	return &Module{Service: svc, h: &handler{svc: svc}}
}

func (m *Module) Name() string { return "org" }

func (m *Module) Routes(_, priv chi.Router) { mountRoutes(m.h, priv) }
