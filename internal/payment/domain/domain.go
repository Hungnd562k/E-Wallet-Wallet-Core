// Package domain holds payment types, the status state machine (erd §6)
// and command validation. Pure functions only.
package domain

import (
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
)

type Type string

const (
	TypeTopup     Type = "TOPUP"
	TypeTransfer  Type = "TRANSFER"
	TypeWithdraw  Type = "WITHDRAW"
	TypeInterbank Type = "INTERBANK"
	TypeQRPayment Type = "QR_PAYMENT"
	TypeRefund    Type = "REFUND"
)

type Status string

const (
	StatusCreated             Status = "CREATED"
	StatusPending             Status = "PENDING"
	StatusSucceeded           Status = "SUCCEEDED"
	StatusFailed              Status = "FAILED"
	StatusNeedsReconciliation Status = "NEEDS_RECONCILIATION"
	StatusReversed            Status = "REVERSED"
)

var transitions = map[Status][]Status{
	StatusCreated:             {StatusPending, StatusSucceeded},
	StatusPending:             {StatusSucceeded, StatusFailed, StatusNeedsReconciliation},
	StatusNeedsReconciliation: {StatusSucceeded, StatusFailed},
	StatusSucceeded:           {StatusReversed},
}

func (s Status) CanTransition(to Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}

// IsFinal reports whether no further transition is expected without manual action.
func (s Status) IsFinal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusReversed
}

const (
	CurrencyVND       = "VND"
	MaxDescriptionLen = 140 // characters, per OpenAPI TransferRequest
)

var (
	ErrSameWallet          = apperr.New(apperr.KindUnprocessable, "SAME_WALLET", "cannot transfer to the same wallet")
	ErrUnsupportedCurrency = apperr.New(apperr.KindUnprocessable, "UNSUPPORTED_CURRENCY", "only VND is supported")
)

// Transfer is a validated wallet-to-wallet transfer request.
type Transfer struct {
	SourceWalletID uuid.UUID
	DestWalletID   uuid.UUID
	Amount         int64
	Currency       string
	Description    string
}

// Validate applies the rules that need no database access.
func (t Transfer) Validate() error {
	switch {
	case t.Amount <= 0:
		return apperr.Invalid("amount must be a positive integer (VND)")
	case t.Currency != CurrencyVND:
		return ErrUnsupportedCurrency
	case utf8.RuneCountInString(t.Description) > MaxDescriptionLen:
		return apperr.Invalid("description must be at most %d characters", MaxDescriptionLen)
	case t.DestWalletID == uuid.Nil:
		return apperr.Invalid("recipient.wallet_id is required")
	case t.SourceWalletID == t.DestWalletID:
		return ErrSameWallet
	}
	return nil
}
