package outbox

import (
	"testing"
	"time"
)

func TestBackoffWithJitter(t *testing.T) {
	tests := []struct {
		attempt int32
		r       float64
		want    time.Duration
	}{
		{1, 0.5, time.Second}, // middle of jitter range = base
		{2, 0.5, 2 * time.Second},
		{4, 0.5, 8 * time.Second},
		{1, 0, 800 * time.Millisecond}, // -20%
		{30, 0.5, 5 * time.Minute},     // capped
		{0, 0.5, time.Second},          // treated as attempt 1
	}
	for _, tt := range tests {
		if got := backoffWithJitter(tt.attempt, tt.r); got != tt.want {
			t.Errorf("backoffWithJitter(%d, %v) = %v, want %v", tt.attempt, tt.r, got, tt.want)
		}
	}
}

func TestBackoffStaysInRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		d := Backoff(3) // 4s ± 20%
		if d < 3200*time.Millisecond || d >= 4800*time.Millisecond {
			t.Fatalf("Backoff(3) = %v out of range", d)
		}
	}
}
