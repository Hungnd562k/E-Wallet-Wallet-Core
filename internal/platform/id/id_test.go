package id

import (
	"strings"
	"testing"
)

func TestFormatParseRoundTrip(t *testing.T) {
	u := New()
	s := Format(Payment, u)
	if !strings.HasPrefix(s, "txn_") || strings.Contains(s, "-") {
		t.Fatalf("unexpected format %q", s)
	}
	got, err := Parse(Payment, s)
	if err != nil || got != u {
		t.Fatalf("Parse(%q) = %v, %v; want %v", s, got, err, u)
	}
	if got, err := Parse(Payment, u.String()); err != nil || got != u {
		t.Fatalf("plain UUID should parse, got %v, %v", got, err)
	}
	if _, err := Parse(Payment, "txn_nothex"); err == nil {
		t.Fatal("want error for garbage")
	}
}

func TestNewIsTimeOrdered(t *testing.T) {
	a, b := New(), New()
	if a.String() >= b.String() {
		t.Fatalf("UUIDv7 should sort by creation: %s >= %s", a, b)
	}
}
