// Package apperr defines the business error type shared by services and mapped to HTTP by httpx.
package apperr

import "errors"

// Kind classifies an application error; httpx maps it to a status code.
type Kind int

const (
	KindValidation Kind = iota + 1
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindTooMany
)

// Error is a safe-to-show error with a Russian message.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  map[string]string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// As extracts *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Is reports whether err is an *Error of the given kind.
func Is(err error, k Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == k
}

func Validation(fields map[string]string) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Message: "Проверьте правильность заполнения полей", Fields: fields}
}

func Invalid(field, msg string) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Message: msg, Fields: map[string]string{field: msg}}
}

func BadRequest(msg string) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Message: msg}
}

func Unauthorized(msg string) *Error {
	if msg == "" {
		msg = "Требуется вход в систему"
	}
	return &Error{Kind: KindUnauthorized, Code: "unauthorized", Message: msg}
}

func Forbidden() *Error {
	return &Error{Kind: KindForbidden, Code: "forbidden", Message: "Недостаточно прав для этого действия"}
}

func NotFound() *Error {
	return &Error{Kind: KindNotFound, Code: "not_found", Message: "Не найдено"}
}

func Conflict(msg string) *Error {
	return &Error{Kind: KindConflict, Code: "conflict", Message: msg}
}

func TooMany(msg string) *Error {
	return &Error{Kind: KindTooMany, Code: "too_many_requests", Message: msg}
}

// V collects field validation messages.
type V struct{ fields map[string]string }

// Add records msg for field (first message per field wins).
func (v *V) Add(field, msg string) {
	if v.fields == nil {
		v.fields = map[string]string{}
	}
	if _, ok := v.fields[field]; !ok {
		v.fields[field] = msg
	}
}

// Err returns a validation error if any field failed.
func (v *V) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	e := Validation(v.fields)
	if len(v.fields) == 1 {
		for _, m := range v.fields {
			e.Message = m
		}
	}
	return e
}
