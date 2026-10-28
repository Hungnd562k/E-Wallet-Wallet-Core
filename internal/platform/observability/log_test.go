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

func TestMaskCCCD(t *testing.T) {
	tests := []struct{ in, want string }{
		{"001099012345", "********2345"},
		{"cccd: 001099012345.", "cccd: ********2345."},
		{"a 001099012345 b 079200000001", "a ********2345 b ********0001"},
		{"0869856211", "0869856211"},       // phone, 10 digits
		{"0010990123456", "0010990123456"}, // 13 digits is not a CCCD
		{"so tien 1000000", "so tien 1000000"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := MaskCCCD(tt.in); got != tt.want {
			t.Errorf("MaskCCCD(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoggerMasksCCCDInFreeText(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, slog.LevelInfo)

	log.Info("kyc submitted", "note", "khach gui CCCD 001099012345", "amount", int64(100000000000))

	out := buf.String()
	if bytes.Contains(buf.Bytes(), []byte("001099012345")) {
		t.Errorf("CCCD leaked into log: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("********2345")) {
		t.Errorf("masked CCCD missing: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("100000000000")) {
		t.Errorf("numeric amount should not be masked: %s", out)
	}
}

func TestLoggerAddsTraceID(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, slog.LevelInfo)
	ctx := WithTraceID(WithRequestID(context.Background(), "req_1"), "4bf92f3577b34da6a3ce929d0e0e4736")

	log.InfoContext(ctx, "hello")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" || rec["request_id"] != "req_1" {
		t.Errorf("record = %v, want request_id and trace_id", rec)
	}
}
