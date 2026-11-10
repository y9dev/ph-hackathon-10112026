package requests

import (
	"time"

	"github.com/go-chi/chi/v5"
)

// Module wires the requests service, its routes and the overdue background job.
type Module struct {
	Service *Service
	h       *handler
}

// New builds the module. loc is the display time zone for CSV export.
func New(d Deps, loc *time.Location) *Module {
	if d.Store == nil {
		d.Store = &SQLiteStore{DB: d.DB}
	}
	svc := NewService(d)
	if loc == nil {
		loc = time.UTC
	}
	return &Module{Service: svc, h: &handler{svc: svc, loc: loc}}
}

func (m *Module) Name() string { return "requests" }

func (m *Module) Routes(pub, priv chi.Router) { mountRoutes(m.h, pub, priv) }
