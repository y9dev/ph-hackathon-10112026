package identity

import (
	"database/sql"
	"time"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/apperr"
)

var errUnauth = apperr.Unauthorized("")

// Module wires the identity service and its HTTP routes.
type Module struct {
	Service *Service
	h       *handler
}

// New builds the module on the given database.
func New(d *sql.DB, cfg Config, now func() time.Time) *Module {
	svc := NewService(&SQLiteStore{DB: d}, cfg, now)
	return &Module{Service: svc, h: &handler{svc: svc}}
}

func (m *Module) Name() string { return "identity" }

// Routes mounts public (no session) and private (session required) routes.
func (m *Module) Routes(pub, priv chi.Router) { mountRoutes(m.h, pub, priv) }
