package observability

import "context"

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
	return c
}
