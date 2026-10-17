// Package domain holds the pure accounting rules of the ledger: no database,
// no I/O, so it is unit-testable without any setup (erd §3).
package domain

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
)

// Direction is the side of an entry: debit or credit.
type Direction string

const (
	Debit  Direction = "D"
	Credit Direction = "C"
)

// Kind is the business event a ledger transaction records (erd §3.2).
type Kind string

const (
	KindTopup       Kind = "TOPUP"
	KindTransfer    Kind = "TRANSFER"
	KindHold        Kind = "HOLD"
	KindHoldCapture Kind = "HOLD_CAPTURE"
	KindHoldRelease Kind = "HOLD_RELEASE"
	KindQRPayment   Kind = "QR_PAYMENT"
	KindRefund      Kind = "REFUND"
	KindFee         Kind = "FEE"
	KindAdjustment  Kind = "ADJUSTMENT"
	KindReversal    Kind = "REVERSAL"
)

// System account codes created by migration 0002.
const (
	CodeSafeguardBank = "SAFEGUARD_BANK"
	CodeWalletHold    = "WALLET_HOLD"
	CodeFeeRevenue    = "FEE_REVENUE"
	CodeReconSuspense = "RECON_SUSPENSE"
)

// WalletAccountCode is the code of a wallet's available-balance account.
func WalletAccountCode(walletID uuid.UUID) string { return "WALLET:" + walletID.String() }

// Line is one side of a posting: move Amount on AccountID in Direction.
type Line struct {
	AccountID uuid.UUID
	Direction Direction
	Amount    int64
}

// Posting is a balanced set of lines recorded as one ledger transaction.
type Posting struct {
	Kind        Kind
	PaymentID   *uuid.UUID
	Description string
	Currency    string
	Lines       []Line
}

var (
	ErrUnbalanced        = apperr.New(apperr.KindInternal, "LEDGER_UNBALANCED", "posting debits and credits differ")
	ErrInsufficientFunds = apperr.New(apperr.KindUnprocessable, "INSUFFICIENT_FUNDS", "insufficient available balance")
)

// Validate checks the structural rules every posting must satisfy before it
// touches the database. The DB triggers check the same invariant again (ADR-08).
func (p Posting) Validate() error {
	if len(p.Lines) < 2 {
		return fmt.Errorf("%w: need at least 2 lines, got %d", ErrUnbalanced, len(p.Lines))
	}
	if p.Currency == "" {
		return fmt.Errorf("posting currency is required")
	}
	var debit, credit int64
	for i, l := range p.Lines {
		if l.Amount <= 0 {
			return fmt.Errorf("line %d: amount must be positive, got %d", i, l.Amount)
		}
		switch l.Direction {
		case Debit:
			debit += l.Amount
		case Credit:
			credit += l.Amount
		default:
			return fmt.Errorf("line %d: invalid direction %q", i, l.Direction)
		}
	}
	if debit != credit {
		return fmt.Errorf("%w: D=%d C=%d", ErrUnbalanced, debit, credit)
	}
	return nil
}

// AccountIDs returns the distinct account ids referenced by the posting.
func (p Posting) AccountIDs() []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(p.Lines))
	ids := make([]uuid.UUID, 0, len(p.Lines))
	for _, l := range p.Lines {
		if _, ok := seen[l.AccountID]; !ok {
			seen[l.AccountID] = struct{}{}
			ids = append(ids, l.AccountID)
		}
	}
	return ids
}

// Apply returns the balance after applying l to an account with the given
// normal side. Balances are stored positive on the normal side (erd §3.1):
// an entry on the normal side increases the balance, the other side decreases it.
func Apply(normalSide Direction, balance int64, l Line) int64 {
	if l.Direction == normalSide {
		return balance + l.Amount
	}
	return balance - l.Amount
}
