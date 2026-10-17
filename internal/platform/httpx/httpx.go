// Package httpx holds HTTP helpers shared by every module's transport layer:
// JSON encoding, the uniform error body, and common middleware.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
)

// ErrorBody is the uniform error format (architecture §7.3).
type ErrorBody struct {
	Error     string         `json:"error"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteRawJSON writes an already-encoded body, e.g. a response replayed from the idempotency store.
func WriteRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// ErrorResponse converts err to a status code and body. Business errors keep
// their code and message; anything else becomes an opaque 500 so internals never leak.
func ErrorResponse(r *http.Request, err error) (int, ErrorBody) {
	body := ErrorBody{RequestID: observability.RequestID(r.Context())}
	if e, ok := apperr.As(err); ok {
		body.Error, body.Message, body.Details = e.Code, e.Message, e.Details
		return StatusOf(e.Kind), body
	}
	body.Error, body.Message = "INTERNAL_ERROR", "internal error"
	return http.StatusInternalServerError, body
}

// WriteError logs unexpected errors and writes the uniform error body.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status, body := ErrorResponse(r, err)
	if status >= 500 {
		slog.ErrorContext(r.Context(), "request failed", "err", err, "path", r.URL.Path)
	}
	WriteJSON(w, status, body)
}

func StatusOf(k apperr.Kind) int {
	switch k {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnprocessable:
		return http.StatusUnprocessableEntity
	case apperr.KindLocked:
		return http.StatusLocked
	default:
		return http.StatusInternalServerError
	}
}

const maxBodyBytes = 1 << 20 // 1 MiB

// ReadBody reads the (size-limited) body so it can be both hashed and decoded.
func ReadBody(r *http.Request) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(b) > maxBodyBytes {
		return nil, apperr.Invalid("request body too large")
	}
	return b, nil
}

// Unmarshal strictly decodes an already-read body.
func Unmarshal(body []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var syn *json.SyntaxError
		if errors.As(err, &syn) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return apperr.Invalid("malformed JSON body")
		}
		return apperr.Invalid("invalid JSON body: %v", err)
	}
	return nil
}
