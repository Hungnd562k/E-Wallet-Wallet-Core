// Package apperr defines transport-neutral business errors.
//
// Modules declare sentinel errors with New and return them (optionally wrapped
// with fmt.Errorf("...: %w", err)). The HTTP layer (platform/httpx) maps the
// Kind to a status code, so domain code never imports net/http.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error by how the caller should react, not by where it came from.
type Kind int

const (
	KindInternal      Kind = iota // bug or infrastructure failure → 5xx
	KindInvalid                   // malformed request → 400
	KindUnauthorized              // missing/invalid credentials → 401
	KindForbidden                 // authenticated but not allowed → 403
	KindNotFound                  // → 404
	KindConflict                  // state conflict, e.g. concurrent update → 409
	KindUnprocessable             // valid request rejected by a business rule → 422
	KindLocked                    // resource temporarily locked → 423
)

// Error is a business error with a stable machine-readable code
// (e.g. "INSUFFICIENT_FUNDS") that clients can switch on.
// The JSON form is used to persist errors (e.g. idempotent replays), not as the
// HTTP body; httpx.ErrorBody is the wire format.
type Error struct {
	Kind    Kind           `json:"kind"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Is makes errors.Is match on Code, so a copy produced by WithDetails
// still matches the original sentinel.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// WithDetails returns a copy of e carrying extra details for the response body.
func (e *Error) WithDetails(details map[string]any) *Error {
	cp := *e
	cp.Details = details
	return &cp
}

// Invalid is a shortcut for request validation failures.
func Invalid(format string, args ...any) *Error {
	return New(KindInvalid, "INVALID_REQUEST", fmt.Sprintf(format, args...))
}

// As extracts the *Error from err's chain, if any.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Common errors shared by several modules.
var (
	ErrUnauthorized         = New(KindUnauthorized, "UNAUTHORIZED", "authentication required")
	ErrNotFound             = New(KindNotFound, "NOT_FOUND", "resource not found")
	ErrIdempotencyKeyReused = New(KindUnprocessable, "IDEMPOTENCY_KEY_REUSED", "idempotency key was used with a different request")
	ErrIdempotencyInFlight  = New(KindConflict, "IDEMPOTENCY_IN_PROGRESS", "a request with this idempotency key is still being processed")
)
