package domain

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to Status
		want     bool
	}{
		{StatusInactive, StatusActive, true},
		{StatusActive, StatusFrozen, true},
		{StatusFrozen, StatusActive, true},
		{StatusClosed, StatusActive, false}, // closed is terminal
		{StatusInactive, StatusFrozen, false},
		{StatusFrozen, StatusClosed, false}, // must unfreeze first
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := tt.from.CanTransition(tt.to); got != tt.want {
				t.Errorf("CanTransition = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOnlyActiveCanTransact(t *testing.T) {
	for _, s := range []Status{StatusInactive, StatusActive, StatusSuspended, StatusFrozen, StatusClosed} {
		if got, want := s.CanTransact(), s == StatusActive; got != want {
			t.Errorf("%s.CanTransact() = %v, want %v", s, got, want)
		}
	}
}
