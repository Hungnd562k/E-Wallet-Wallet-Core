package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsMatchesWrappedAndDetailedCopies(t *testing.T) {
	sentinel := New(KindUnprocessable, "INSUFFICIENT_FUNDS", "no money")

	wrapped := fmt.Errorf("transfer: %w", sentinel)
	if !errors.Is(wrapped, sentinel) {
		t.Error("wrapped error should match its sentinel")
	}

	detailed := sentinel.WithDetails(map[string]any{"wallet": "w1"})
	if !errors.Is(detailed, sentinel) {
		t.Error("WithDetails copy should still match the sentinel")
	}
	if sentinel.Details != nil {
		t.Error("WithDetails must not mutate the shared sentinel")
	}

	if errors.Is(wrapped, ErrNotFound) {
		t.Error("different codes must not match")
	}

	e, ok := As(wrapped)
	if !ok || e.Code != "INSUFFICIENT_FUNDS" {
		t.Errorf("As = %v, %v", e, ok)
	}
}
