package notifications

import (
	"database/sql"
	"time"

	"github.com/go-chi/chi/v5"
)

// Module wires the notifications service and routes.
type Module struct {
	Service *Service
	h       *handler
}

func New(d *sql.DB, now func() time.Time) *Module {
	svc := NewService(&SQLiteStore{DB: d}, now)
	return &Module{Service: svc, h: &handler{svc: svc}}
}

func (m *Module) Name() string { return "notifications" }

func (m *Module) Routes(_, priv chi.Router) { mountRoutes(m.h, priv) }
