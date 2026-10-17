package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
)

func TestTransferValidate(t *testing.T) {
	src, dst := uuid.New(), uuid.New()
	valid := Transfer{SourceWalletID: src, DestWalletID: dst, Amount: 1000, Currency: CurrencyVND}

	tests := []struct {
		name     string
		mutate   func(*Transfer)
		wantCode string // "" = valid
	}{
		{"valid", func(*Transfer) {}, ""},
		{"zero amount", func(t *Transfer) { t.Amount = 0 }, "INVALID_REQUEST"},
		{"negative amount", func(t *Transfer) { t.Amount = -5 }, "INVALID_REQUEST"},
		{"usd", func(t *Transfer) { t.Currency = "USD" }, ErrUnsupportedCurrency.Code},
		{"same wallet", func(t *Transfer) { t.DestWalletID = src }, ErrSameWallet.Code},
		{"description 140 runes ok", func(t *Transfer) { t.Description = strings.Repeat("ă", 140) }, ""},
		{"description 141 runes", func(t *Transfer) { t.Description = strings.Repeat("ă", 141) }, "INVALID_REQUEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := valid // struct copy: each case mutates its own value
			tt.mutate(&tr)
			err := tr.Validate()

			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			var appErr *apperr.Error
			if !errors.As(err, &appErr) || appErr.Code != tt.wantCode {
				t.Fatalf("want code %s, got %v", tt.wantCode, err)
			}
		})
	}
}

func TestStatusTransitions(t *testing.T) {
	if !StatusPending.CanTransition(StatusSucceeded) {
		t.Error("PENDING -> SUCCEEDED must be allowed")
	}
	if StatusFailed.CanTransition(StatusSucceeded) {
		t.Error("FAILED is final")
	}
	if StatusSucceeded.CanTransition(StatusPending) {
		t.Error("SUCCEEDED cannot go back to PENDING")
	}
}
