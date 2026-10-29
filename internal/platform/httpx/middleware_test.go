package httpx

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
)

func TestParseTraceparent(t *testing.T) {
	tests := []struct {
		name, header, want string
		ok                 bool
	}{
		{"valid", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "4bf92f3577b34da6a3ce929d0e0e4736", true},
		{"empty", "", "", false},
		{"all-zero trace id", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", "", false},
		{"uppercase hex", "00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01", "", false},
		{"forbidden version ff", "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "", false},
		{"short trace id", "00-4bf92f35-00f067aa0ba902b7-01", "", false},
		{"garbage", "hello", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTraceparent(tt.header)
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseTraceparent(%q) = %q, %v; want %q, %v", tt.header, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// capture runs h behind mw and returns the request context the handler saw.
func capture(t *testing.T, mw func(http.Handler) http.Handler, r *http.Request) *http.Request {
	t.Helper()
	var seen *http.Request
	mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r })).
		ServeHTTP(httptest.NewRecorder(), r)
	return seen
}

func TestRequestIDKeepsClientTraceID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(HeaderRequestID, "req_client")
	r.Header.Set(HeaderTraceparent, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	ctx := capture(t, RequestID, r).Context()
	if got := observability.RequestID(ctx); got != "req_client" {
		t.Errorf("request_id = %q", got)
	}
	if got := observability.TraceID(ctx); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id = %q", got)
	}
}

func TestRequestIDGeneratesTraceID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	ctx := capture(t, RequestID, r).Context()
	if got := observability.TraceID(ctx); len(got) != 32 || !isLowerHex(got) {
		t.Errorf("generated trace_id = %q, want 32 lowercase hex chars", got)
	}
}

func TestClientInfo(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:51234"
	r.Header.Set("User-Agent", "EWalletApp/1.0 "+strings.Repeat("x", 1000))
	r.Header.Set("X-Forwarded-For", "1.2.3.4") // must be ignored: any client can set it

	c := observability.ClientFrom(capture(t, ClientInfo, r).Context())
	if c.IP != "203.0.113.7" {
		t.Errorf("ip = %q, want the TCP peer without port", c.IP)
	}
	if len(c.UserAgent) != maxUserAgentLen || !strings.HasPrefix(c.UserAgent, "EWalletApp/1.0") {
		t.Errorf("user agent not truncated to %d: len %d", maxUserAgentLen, len(c.UserAgent))
	}
}

// AIR-1: the user agent and request id go into audit rows (Postgres text), so
// they must stay valid UTF-8 whatever bytes the client sends.
func TestClientInfoKeepsUserAgentValidUTF8(t *testing.T) {
	tests := map[string]string{
		// 511 ASCII bytes + "ệ" (3 bytes): cutting at byte 512 splits the rune.
		"multi-byte rune across the cap": strings.Repeat("a", maxUserAgentLen-1) + "ệ",
		// Go's server accepts obs-text (bytes >= 0x80) in header values.
		"invalid byte": "EWalletApp/1.0 \xff",
	}
	for name, ua := range tests {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("User-Agent", ua)

			got := observability.ClientFrom(capture(t, ClientInfo, r).Context()).UserAgent
			if !utf8.ValidString(got) {
				t.Errorf("user agent is not valid UTF-8: %q", got)
			}
			if len(got) > maxUserAgentLen {
				t.Errorf("user agent len %d > %d", len(got), maxUserAgentLen)
			}
		})
	}
}

func TestRequestIDKeepsValidUTF8(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(HeaderRequestID, "req_\xff")

	if got := observability.RequestID(capture(t, RequestID, r).Context()); !utf8.ValidString(got) {
		t.Errorf("request_id is not valid UTF-8: %q", got)
	}
}

// TestServerPassesInvalidUTF8Header shows the bytes really reach the handler
// from a raw TCP client, so the cases above are not only a test artefact.
func TestServerPassesInvalidUTF8Header(t *testing.T) {
	seen := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.UserAgent()
	}))
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nUser-Agent: bad\xff\r\nConnection: close\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	if ua := <-seen; utf8.ValidString(ua) {
		t.Fatalf("expected the server to pass invalid UTF-8 through, got %q", ua)
	}
}
