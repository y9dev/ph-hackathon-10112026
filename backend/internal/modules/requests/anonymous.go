package requests

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/db"
	"modusvoice/internal/platform/eventbus"
)

// Anonymous token (CR2): 28 symbols of Crockford base32 (5 bits each = 140 bits of entropy from
// crypto/rand), shown grouped by 4: XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX. Only SHA-256 of the
// canonical form (upper case, no separators) is stored. The token is never logged.
const (
	tokenAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	tokenSymbols  = 28
	tokenGroup    = 4
)

// NewAnonymousToken returns a fresh grouped token.
func NewAnonymousToken() (string, error) {
	raw := make([]byte, tokenSymbols)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("anonymous token: %w", err)
	}
	var b strings.Builder
	for i, v := range raw {
		if i > 0 && i%tokenGroup == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(tokenAlphabet[v&31]) // 256 is a multiple of 32: unbiased
	}
	return b.String(), nil
}

// CanonicalToken normalizes user input: upper case, separators/spaces removed, Crockford
// look-alikes mapped (O->0, I/L->1). ok=false when the result cannot be a token.
func CanonicalToken(s string) (string, bool) {
	if len(s) > 100 {
		return "", false
	}
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case '-', ' ', '\t', '\n', '\r':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if r > 127 || !strings.ContainsRune(tokenAlphabet, r) {
			return "", false
		}
		b.WriteRune(r)
	}
	c := b.String()
	return c, len(c) == tokenSymbols
}

// HashAnonymousToken returns the stored form (SHA-256 hex of the canonical token).
func HashAnonymousToken(token string) string {
	c, _ := CanonicalToken(token)
	sum := sha256.Sum256([]byte(c))
	return hex.EncodeToString(sum[:])
}

var dummyTokenHash = HashAnonymousToken("0000-0000-0000-0000-0000-0000-0000")

func errAnonNotFound() error { return apperr.NotFound() }

// resolveToken loads the anonymous request for token inside q. A wrong or malformed token yields
// the same 404 as an unknown one; the hash comparison is constant time.
func (s *Service) resolveToken(ctx context.Context, q db.Querier, token string) (DetailRow, error) {
	canon, valid := CanonicalToken(token)
	hash := dummyTokenHash
	if valid {
		sum := sha256.Sum256([]byte(canon))
		hash = hex.EncodeToString(sum[:])
	}
	r, stored, err := s.Store.ByTokenHash(ctx, q, hash)
	if errors.Is(err, ErrNotFound) {
		stored = dummyTokenHash
		r = DetailRow{}
	} else if err != nil {
		return DetailRow{}, err
	}
	match := subtle.ConstantTimeCompare([]byte(stored), []byte(hash)) == 1
	if !valid || !match || r.ID == 0 || !r.Author.IsAnonymous {
		return DetailRow{}, errAnonNotFound()
	}
	r.Author.ID = anonSelfID // in-memory only: the token holder is mapped as the author
	return r, nil
}

func (s *Service) anonAllow(ip, op string) error {
	if !s.anonLimit.Allow("anon:"+op+":"+ip, s.now()) {
		return apperr.TooMany("Слишком много попыток. Подождите минуту и попробуйте снова.")
	}
	return nil
}

// AnonymousLookup returns the author view of an anonymous request by its token (no session).
func (s *Service) AnonymousLookup(ctx context.Context, ip, token string) (Detail, error) {
	if err := s.anonAllow(ip, "lookup"); err != nil {
		return Detail{}, err
	}
	r, err := s.resolveToken(ctx, s.DB, token)
	if err != nil {
		return Detail{}, err
	}
	return s.buildDetail(ctx, anonAuthorViewer(), r)
}

// AnonymousComment lets the token holder write to the request. The kind is chosen by the server:
// answer while needs_info (moves to in_review, owners notified), otherwise comment. Closed: 409.
// No user id is involved anywhere.
func (s *Service) AnonymousComment(ctx context.Context, ip, token, body string) (Detail, error) {
	if err := s.anonAllow(ip, "comment"); err != nil {
		return Detail{}, err
	}
	body = strings.TrimSpace(body)
	if n := utf8.RuneCountInString(body); n < 1 || n > CommentMax || !utf8.ValidString(body) {
		return Detail{}, apperr.Invalid("body", "Текст: от 1 до 4000 символов")
	}
	now := s.now()
	var saved DetailRow
	kind := KindComment
	err := db.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		r, err := s.resolveToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if r.Status.Terminal() {
			return apperr.Conflict("Обращение закрыто. Создайте новое обращение со ссылкой на номер " + r.Number)
		}
		if r.Status == StatusNeedsInfo {
			kind = KindAnswer
		}
		if _, err := s.Store.InsertComment(ctx, tx, NewComment{RequestID: r.ID, ByAuthor: true, Kind: kind, Body: body, At: now}); err != nil {
			return err
		}
		if kind == KindAnswer {
			from := r.Status
			applyTransition(&r.Row, StatusInReview, now)
			to := r.Status
			if err := s.Store.InsertEvent(ctx, tx, Event{RequestID: r.ID, ByAuthor: true, Kind: EvStatusChanged, FromStatus: &from, ToStatus: &to, At: now}); err != nil {
				return err
			}
		}
		r.UpdatedAt = now
		if err := s.Store.Save(ctx, tx, r); err != nil {
			return err
		}
		saved = r
		ns, err := s.authorSideNotes(ctx, r.Row, kind, func(int64) string { return AnonymousLabel }, excerpt(body, 300))
		if err != nil {
			return err
		}
		return s.Notifier.Write(ctx, tx, 0, ns)
	})
	if err != nil {
		return Detail{}, err
	}
	topic := eventbus.RequestCommented
	if kind == KindAnswer {
		topic = eventbus.RequestAnswered
	}
	s.publish(ctx, topic, saved.Row, map[string]any{"kind": kind})
	d, err := s.Store.Get(ctx, s.DB, saved.ID)
	if err != nil {
		return Detail{}, err
	}
	d.Author.ID = anonSelfID
	return s.buildDetail(ctx, anonAuthorViewer(), d)
}

// AnonymousAttachment resolves an attachment of the token holder's request (404 otherwise).
func (s *Service) AnonymousAttachment(ctx context.Context, ip, token string, id int64) (AttachmentRow, error) {
	if err := s.anonAllow(ip, "attachment"); err != nil {
		return AttachmentRow{}, err
	}
	r, err := s.resolveToken(ctx, s.DB, token)
	if err != nil {
		return AttachmentRow{}, err
	}
	at, err := s.Store.Attachment(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && (at.RequestID != r.ID || !validStoredName(at.StoredName))) {
		return AttachmentRow{}, errAnonNotFound()
	}
	if err != nil {
		return AttachmentRow{}, err
	}
	return at, nil
}
