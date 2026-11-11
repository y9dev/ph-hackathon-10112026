package requests

import (
	"context"
	"database/sql"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/db"
)

// MaxBrowserTokens is how many anonymous tokens a browser keeps for the status check.
const MaxBrowserTokens = 5

// MarkSpam hides an anonymous request as spam and puts its token on the block list.
// Allowed for admins and for the current owner of the request (CR6 canManage). Only the token hash
// is stored; the request itself stays in the database, hidden from every list and detail view.
func (s *Service) MarkSpam(ctx context.Context, a authz.Actor, id int64) error {
	if err := authz.RequireStaff(a); err != nil {
		return err
	}
	return db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		r, err := s.loadForUpdate(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if !r.Author.IsAnonymous {
			return apperr.Conflict("Отметить как спам можно только анонимное обращение")
		}
		hash, err := s.Store.MarkSpam(ctx, tx, id)
		if err != nil {
			return err
		}
		if hash == "" {
			return nil
		}
		return s.Store.BlockTokens(ctx, tx, []string{hash}, s.now())
	})
}

// AnonymousStatus checks anonymous tokens sent back by the browser (its cookie). Nothing is written:
// the tokens are hashed and compared with the block list. allowed=false when any of them is blocked.
func (s *Service) AnonymousStatus(ctx context.Context, ip string, tokens []string) (bool, error) {
	if err := s.anonAllow(ip, "status"); err != nil {
		return false, err
	}
	hashes := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := CanonicalToken(t); ok {
			hashes = append(hashes, HashAnonymousToken(t))
		}
	}
	blocked, err := s.Store.AnyTokenBlocked(ctx, s.DB, hashes)
	if err != nil {
		return false, err
	}
	return !blocked, nil
}
