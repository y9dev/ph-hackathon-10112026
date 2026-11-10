package insights

import (
	"database/sql"
	"time"

	"github.com/go-chi/chi/v5"
)

// Module wires the insights service and routes.
type Module struct {
	Service *Service
	h       *handler
}

// New builds the module; the overdue lister is attached later via Service.SetOverdueLister.
func New(d *sql.DB, now func() time.Time, loc *time.Location) *Module {
	svc := NewService(&SQLiteStore{DB: d}, nil, now, loc)
	return &Module{Service: svc, h: &handler{svc: svc}}
}

func (m *Module) Name() string { return "insights" }

func (m *Module) Routes(_, priv chi.Router) { mountRoutes(m.h, priv) }
