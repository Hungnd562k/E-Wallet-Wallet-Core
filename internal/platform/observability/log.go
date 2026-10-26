// Package observability configures structured logging with redaction of
// sensitive keys (architecture §8.2). Tracing and metrics come later.
package observability

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
)

type ctxKey int

const requestIDKey ctxKey = iota

// sensitiveKeys are attribute keys whose values must never reach the logs.
// Keys are split on "_", "-" and "." and matched word by word, case-insensitively,
// so "refresh_token" and "X-Authorization" are caught but "shipping" is not.
var sensitiveKeys = []string{"cccd", "pin", "otp", "token", "authorization", "password", "secret", "image"}

const redacted = "[REDACTED]"

// NewLogger returns a JSON logger that redacts sensitive attributes and adds
// request_id from the context to every record.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redact,
	})
	return slog.New(contextHandler{h})
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if isSensitive(a.Key) {
		return slog.String(a.Key, redacted)
	}
	return a
}

// Not sure what I'm doing
func IsSensitiveKey(key string) bool {
	words := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for _, w := range words {
		if slices.Contains(sensitiveKeys, w) {
			return true
		}
	}
	return false
}

func isSensitive(key string) bool {
	words := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for _, w := range words {
		if slices.Contains(sensitiveKeys, w) {
			return true
		}
	}
	return false
}

// WithRequestID stores the request id for logging and for propagation into outbox events.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// contextHandler decorates records with values carried in the context.
// Use the *Context logging methods (InfoContext, ErrorContext…) to benefit from it.
type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
