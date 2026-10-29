package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
)

const (
	HeaderRequestID   = "X-Request-Id"
	HeaderTraceparent = "traceparent"
)

// maxUserAgentLen caps what a client can make us store in every audit row.
const maxUserAgentLen = 512

// RequestID accepts the client's X-Request-Id (if sane) or generates one, stores it
// in the context and echoes it back. It does the same for the W3C trace id taken
// from the traceparent header, so logs and audit rows always carry both.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !validRequestID(id) {
			id = newRequestID()
		}
		traceID, ok := parseTraceparent(r.Header.Get(HeaderTraceparent))
		if !ok {
			traceID = newTraceID()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := observability.WithRequestID(r.Context(), id)
		ctx = observability.WithTraceID(ctx, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClientInfo stores the caller's IP and user agent in the context for audit rows.
// The IP is the TCP peer address; X-Forwarded-For is not trusted because any
// client can set it. Behind a reverse proxy, put a trusted real-IP middleware first.
func ClientInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		ua := r.UserAgent()
		if len(ua) > maxUserAgentLen {
			ua = ua[:maxUserAgentLen]
		}
		ctx := observability.WithClient(r.Context(), observability.Client{IP: ip, UserAgent: ua})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// parseTraceparent returns the trace id of a W3C traceparent header
// ("00-<32 hex trace id>-<16 hex parent id>-<2 hex flags>").
func parseTraceparent(h string) (string, bool) {
	parts := strings.Split(h, "-")
	if len(parts) != 4 || len(parts[0]) != 2 || parts[0] == "ff" ||
		len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return "", false
	}
	traceID := parts[1]
	if !isLowerHex(traceID) || traceID == strings.Repeat("0", 32) {
		return "", false
	}
	return traceID, true
}

func isLowerHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Recover turns a panic into a 500 with the uniform body instead of killing the connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				WriteError(w, r, fmt.Errorf("panic: %v", p))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// AccessLog logs one line per request. It never logs bodies.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.InfoContext(r.Context(), "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// validRequestID reports whether a client's X-Request-Id is safe to echo back and
// store: 1–128 characters from [A-Za-z0-9._-] (architecture §10, AIR-1).
func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "req_" + hex.EncodeToString(b[:])
}

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
