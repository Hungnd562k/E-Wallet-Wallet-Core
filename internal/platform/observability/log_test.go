package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLoggerRedactsAndAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, slog.LevelInfo)
	ctx := WithRequestID(context.Background(), "req_1")

	log.InfoContext(ctx, "hello",
		"pin", "123456",
		"refresh_token", "abc",
		"X-Authorization", "Bearer x",
		"shipping", "kept", // contains "pin" as a substring but is not a sensitive word
		slog.Group("user", "cccd", "0123456789"),
	)

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"pin", "refresh_token", "X-Authorization"} {
		if rec[k] != redacted {
			t.Errorf("%s = %v, want redacted", k, rec[k])
		}
	}
	if rec["shipping"] != "kept" {
		t.Errorf("shipping should not be redacted, got %v", rec["shipping"])
	}
	if user := rec["user"].(map[string]any); user["cccd"] != redacted {
		t.Errorf("nested cccd not redacted: %v", user["cccd"])
	}
	if rec["request_id"] != "req_1" {
		t.Errorf("request_id = %v", rec["request_id"])
	}
}
