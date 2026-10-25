package domain

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"debug", "0869856211", "+84869856211"},
		{"local format", "0901234567", "+84901234567"},
		{"with spaces", "0901 234 567", "+84901234567"},
		{"international", "+84 901-234-567", "+84901234567"},
		{"too short", "090123", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // subtests run concurrently; each has its own tt (Go ≥ 1.22)
			if got := NormalizePhone(tt.in); got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
