package syntax

import (
	"fmt"
	"testing"
)

func TestFormatVND(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 ₫"},
		{999, "999 ₫"},
		{1000, "1.000 ₫"},
		{1234567, "1.234.567 ₫"},
		{-50000, "-50.000 ₫"},
		{-1234567, "-1.234.567 ₫"},
	}
	for _, tt := range tests {
		if got := FormatVND(tt.in); got != tt.want {
			t.Errorf("FormatVND(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFeeTier(t *testing.T) {
	if FeeTier(500_000) != "free" || FeeTier(5_000_000) != "standard" || FeeTier(0) != "invalid" {
		t.Error("unexpected tier")
	}
}

func TestSumPositive(t *testing.T) {
	if got := SumPositive(100, -5, 0, 50); got != 150 {
		t.Errorf("got %d", got)
	}
	if got := SumPositive(); got != 0 { // zero args is fine for variadics
		t.Errorf("got %d", got)
	}
}

func TestParseVND(t *testing.T) {
	// t.Skip("bài tập: implement ParseVND then delete this line")
	for _, in := range []int64{0, 1000, 1234567, -1234567} {
		got, err := ParseVND(FormatVND(in))
		if err != nil || got != in {
			t.Errorf("ParseVND(FormatVND(%d)) = %d, %v", in, got, err)
		}
	}
}

// Example functions are compiled, run by `go test`, and their output is
// checked against the "Output:" comment. They also show up in `go doc`.
func ExampleFormatVND() {
	fmt.Println(FormatVND(200000))
	// Output: 200.000 ₫
}
