package errs

import (
	"errors"
	"strings"
	"testing"
)

func TestWithdrawErrors(t *testing.T) {
	tests := []struct {
		name       string
		wallet     Wallet
		amount     int64
		wantStatus int
	}{
		{"ok", Wallet{Balance: 100}, 50, 200},
		{"insufficient", Wallet{Balance: 10}, 50, 422},
		{"frozen", Wallet{Balance: 100, Frozen: true}, 1, 403},
		{"over limit", Wallet{Balance: 1000, Limit: 100}, 500, 422},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTTPStatus(tt.wallet.Withdraw(tt.amount)); got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestWrappingKeepsContextAndIdentity(t *testing.T) {
	err := (&Wallet{Balance: 10}).Withdraw(50)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatal("sentinel lost")
	}
	if !strings.Contains(err.Error(), "withdraw 50 from balance 10") {
		t.Errorf("context lost: %q", err)
	}
}

func TestErrorsAsExtractsData(t *testing.T) {
	err := (&Wallet{Balance: 1000, Limit: 100}).Withdraw(500)
	var le *LimitError
	if !errors.As(err, &le) || le.Limit != 100 {
		t.Fatalf("got %v", err)
	}
}

func TestValidateAll(t *testing.T) {
	if err := ValidateAll([]int64{1, 2}); err != nil {
		t.Errorf("want nil, got %v", err)
	}
	err := ValidateAll([]int64{0, 5, -1})
	if err == nil || strings.Count(err.Error(), "must be positive") != 2 {
		t.Errorf("want 2 joined errors, got %v", err)
	}
}

func TestPanicRecover(t *testing.T) {
	err := SafeCall(func() { MustPositive(-1) })
	if err == nil || !strings.Contains(err.Error(), "recovered") {
		t.Errorf("got %v", err)
	}
}

func TestTransferAll(t *testing.T) {
	t.Skip("bài tập: implement TransferAll then delete this line")
	w := &Wallet{Balance: 100}
	err := TransferAll(w, []int64{30, 500, 20, 0})
	if !errors.Is(err, ErrInsufficientFunds) || w.Balance != 50 {
		t.Errorf("err=%v balance=%d", err, w.Balance)
	}
}
