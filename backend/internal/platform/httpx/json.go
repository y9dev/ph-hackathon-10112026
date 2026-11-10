// Package httpx holds HTTP helpers: JSON encoding, the single error envelope and middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/apperr"
)

// ErrorBody is the single error envelope: {"error":{"code","message","fields"}}.
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

type ErrorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// JSON writes v with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// NoContent writes 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

func statusFor(k apperr.Kind) int {
	switch k {
	case apperr.KindValidation:
		return http.StatusBadRequest
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindTooMany:
		return http.StatusTooManyRequests
	}
	return http.StatusInternalServerError
}

// Error writes err as the error envelope. Unknown errors are logged with context and
// reported as a generic internal error without details.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := apperr.As(err); ok {
		JSON(w, statusFor(e.Kind), ErrorBody{Error: ErrorPayload{Code: e.Code, Message: e.Message, Fields: e.Fields}})
		return
	}
	slog.ErrorContext(r.Context(), "request failed", "err", err, "method", r.Method, "path", r.URL.Path, "request_id", RequestID(r.Context()))
	JSON(w, http.StatusInternalServerError, ErrorBody{Error: ErrorPayload{Code: "internal", Message: "Внутренняя ошибка сервера. Попробуйте позже."}})
}

// Decode reads a JSON body into dst. The body size is bounded by the BodyLimit middleware.
func Decode(r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(ct); err != nil || mt != "application/json" {
		return apperr.BadRequest("Ожидается тело запроса в формате JSON")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return apperr.BadRequest("Слишком большой запрос")
		}
		if errors.Is(err, io.EOF) {
			return apperr.BadRequest("Пустое тело запроса")
		}
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) && te.Field != "" {
			return apperr.Invalid(te.Field, "Неверный тип значения")
		}
		return apperr.BadRequest("Некорректный JSON")
	}
	return nil
}

// IDParam parses a positive integer path parameter; invalid values are reported as 404.
func IDParam(r *http.Request, name string) (int64, error) {
	v, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || v <= 0 {
		return 0, apperr.NotFound()
	}
	return v, nil
}

// QueryInt parses an optional integer query parameter.
func QueryInt(r *http.Request, name string) (*int64, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, apperr.Invalid(name, "Ожидается целое число")
	}
	return &v, nil
}

// QueryBool parses an optional boolean query parameter.
func QueryBool(r *http.Request, name string) (*bool, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return nil, apperr.Invalid(name, "Ожидается true или false")
	}
	return &v, nil
}
