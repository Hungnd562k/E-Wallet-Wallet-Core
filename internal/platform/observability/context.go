package observability

import (
	"context"
	"strings"
	"unicode/utf8"
)

type traceIDKey struct{}

type clientKey struct{}

// WithTraceID stores the W3C trace id (32 hex chars) of the current request.
// Logs and audit rows carry it next to request_id (architecture §10).
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}

// TraceID returns the trace id stored by WithTraceID, or "" when there is none.
func TraceID(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}

// Client describes who sent the request, as seen by the server.
type Client struct {
	IP        string
	UserAgent string
}

// WithClient stores the caller's IP and user agent for audit logging.
func WithClient(ctx context.Context, c Client) context.Context {
	return context.WithValue(ctx, clientKey{}, c)
}

// ClientFrom returns the Client stored by WithClient; fields are empty outside
// an HTTP request (worker, jobs).
func ClientFrom(ctx context.Context) Client {
	c, _ := ctx.Value(clientKey{}).(Client)
	c.UserAgent = toValidLengthHeader(c.UserAgent)
	return c
}

// Độ dài tối đa của User-Agent
const USER_AGENT_MAX_LENGTH = 512 // bytes

// Nếu User-Agent có length by bytes vượt 512 bytes, return 1 chuỗi được cắt bớt về độ dài cho phép.
// Phải cắt theo ký tự thay vì byte.
func toValidLengthHeader(s string) string {
	s = ReplaceInvalidUTF8(s)

	if len(s) <= USER_AGENT_MAX_LENGTH {
		return s
	}

	s = s[:USER_AGENT_MAX_LENGTH]

	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}

	return s
}

// Replace ký tự không phải UTF-8 hợp lệ
func ReplaceInvalidUTF8(s string) string {
	var b strings.Builder

	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)

		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
			s = s[1:]
			continue
		}

		b.WriteString(s[:size])
		s = s[size:]
	}

	return b.String()
}
