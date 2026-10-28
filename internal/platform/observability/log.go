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
// request_id and trace_id from the context to every record.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redact,
	})
	return slog.New(contextHandler{h})
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if IsSensitiveKey(a.Key) {
		return slog.String(a.Key, redacted)
	}
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, MaskCCCD(a.Value.String()))
	}
	return a
}

// IsSensitiveKey reports whether values under key must be hidden. It is shared
// with the audit module so logs and audit metadata redact the same keys.
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

// cccdLen is the length of a Vietnamese citizen id (CCCD) number.
const cccdLen = 12

// MaskCCCD hides every run of exactly 12 digits in s except its last 4,
// so a CCCD number slipped into free text ("note", "description") is not leaked
// (BR-71). Shorter or longer digit runs (phone numbers, amounts) are kept.
func MaskCCCD(s string) string {
	var b strings.Builder
	masked := false
	for i := 0; i < len(s); {
		if !isDigit(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && isDigit(s[j]) {
			j++
		}
		if j-i == cccdLen {
			b.WriteString(strings.Repeat("*", cccdLen-4))
			b.WriteString(s[j-4 : j])
			masked = true
		} else {
			b.WriteString(s[i:j])
		}
		i = j
	}
	if !masked {
		return s
	}
	return b.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

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
	if id := TraceID(ctx); id != "" {
		r.AddAttrs(slog.String("trace_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
