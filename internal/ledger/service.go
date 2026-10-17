// Package ledger is the double-entry ledger and the only module allowed to
// change balances (architecture §4.1).
//
// This file is the module's public API. Other modules import only this package;
// ledger/domain and ledger/store are implementation details.
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger/store"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
)

// Re-exported domain types so callers never import ledger/domain.
type (
	Posting   = domain.Posting
	Line      = domain.Line
	Kind      = domain.Kind
	Direction = domain.Direction
)

const (
	Debit  = domain.Debit
	Credit = domain.Credit

	KindTopup    = domain.KindTopup
	KindTransfer = domain.KindTransfer

	CodeSafeguardBank = domain.CodeSafeguardBank
)

var (
	ErrInsufficientFunds = domain.ErrInsufficientFunds
	ErrAccountNotFound   = apperr.New(apperr.KindNotFound, "LEDGER_ACCOUNT_NOT_FOUND", "ledger account not found")
	errNotInTx           = errors.New("ledger: Post must run inside db.WithTx")
)

type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, clock: clk}
}

// queries returns sqlc queries bound to the transaction in ctx, or to the pool.
func (s *Service) queries(ctx context.Context) *store.Queries {
	return store.New(db.Conn(ctx, s.pool))
}

// CreateWalletAccount opens the WALLET:<walletID> liability account.
func (s *Service) CreateWalletAccount(ctx context.Context, walletID uuid.UUID, currency string) (uuid.UUID, error) {
	accountID := id.New()
	err := s.queries(ctx).InsertAccount(ctx, store.InsertAccountParams{
		ID:         accountID,
		Code:       domain.WalletAccountCode(walletID),
		Type:       "LIABILITY",
		NormalSide: string(domain.Credit),
		OwnerType:  "WALLET",
		OwnerID:    &walletID,
		Currency:   currency,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert wallet account: %w", err)
	}
	return accountID, nil
}

// AccountIDByCode resolves a system account such as SAFEGUARD_BANK.
func (s *Service) AccountIDByCode(ctx context.Context, code string) (uuid.UUID, error) {
	acc, err := s.queries(ctx).GetAccountByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrAccountNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get account %s: %w", code, err)
	}
	return acc.ID, nil
}

// Balance returns the stored balance of an account (positive on its normal side).
func (s *Service) Balance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	acc, err := s.queries(ctx).GetAccount(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrAccountNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get account: %w", err)
	}
	return acc.Balance, nil
}

// RecomputedBalance sums the account's entries. It must always equal Balance;
// a difference is ledger drift (architecture §9).
func (s *Service) RecomputedBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	return s.queries(ctx).SumEntriesByAccount(ctx, accountID)
}

// Post records a balanced posting. It must be called inside db.WithTx so the
// entries commit or roll back together with the caller's other writes.
//
// Steps: validate → lock accounts in id order → insert transaction and entries
// → reject negative final balances → update balances.
func (s *Service) Post(ctx context.Context, p Posting) (uuid.UUID, error) {
	if !db.InTx(ctx) {
		return uuid.Nil, errNotInTx
	}
	if err := p.Validate(); err != nil {
		return uuid.Nil, err
	}

	q := s.queries(ctx)
	ids := p.AccountIDs()
	rows, err := q.LockAccounts(ctx, ids)
	if err != nil {
		return uuid.Nil, fmt.Errorf("lock accounts: %w", err)
	}
	if len(rows) != len(ids) {
		return uuid.Nil, ErrAccountNotFound
	}

	accounts := make(map[uuid.UUID]*store.LedgerAccount, len(rows))
	for i := range rows {
		acc := &rows[i]
		if acc.Status != "OPEN" {
			return uuid.Nil, fmt.Errorf("account %s is %s", acc.Code, acc.Status)
		}
		if acc.Currency != p.Currency {
			return uuid.Nil, fmt.Errorf("account %s currency %s, posting %s", acc.Code, acc.Currency, p.Currency)
		}
		accounts[acc.ID] = acc
	}

	txnID := id.New()
	now := s.clock.Now()
	if err := q.InsertTransaction(ctx, store.InsertTransactionParams{
		ID:            txnID,
		Kind:          string(p.Kind),
		PaymentID:     p.PaymentID,
		Description:   p.Description,
		EffectiveDate: clock.BusinessDate(now),
	}); err != nil {
		return uuid.Nil, fmt.Errorf("insert transaction: %w", err)
	}

	for _, l := range p.Lines {
		acc := accounts[l.AccountID]
		acc.Balance = domain.Apply(domain.Direction(acc.NormalSide), acc.Balance, l)
		if err := q.InsertEntry(ctx, store.InsertEntryParams{
			TransactionID: txnID,
			AccountID:     acc.ID,
			Direction:     string(l.Direction),
			Amount:        l.Amount,
			Currency:      p.Currency,
			BalanceAfter:  acc.Balance,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("insert entry: %w", err)
		}
	}

	// Only the final balance matters: a posting may debit and credit the same account.
	// The accounts_non_negative CHECK would reject this too; checking here gives a
	// clean business error instead of a constraint violation.
	for i := range rows {
		acc := &rows[i] // same pointer as in the accounts map, so it sees the new balance
		if acc.Balance < 0 && !acc.AllowNegative {
			return uuid.Nil, ErrInsufficientFunds
		}
		if err := q.UpdateBalance(ctx, store.UpdateBalanceParams{ID: acc.ID, Balance: acc.Balance}); err != nil {
			return uuid.Nil, fmt.Errorf("update balance %s: %w", acc.Code, err)
		}
	}
	return txnID, nil
}
