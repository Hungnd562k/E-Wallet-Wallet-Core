// Package wallet manages wallets and their lifecycle. A wallet owns exactly one
// ledger account (WALLET:<id>); its balance lives in the ledger, not here.
package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet/store"
)

type Status = domain.Status

const (
	StatusInactive = domain.StatusInactive
	StatusActive   = domain.StatusActive
	StatusFrozen   = domain.StatusFrozen
)

var (
	ErrNotFound         = apperr.New(apperr.KindNotFound, "WALLET_NOT_FOUND", "wallet not found")
	ErrNotActive        = apperr.New(apperr.KindForbidden, "WALLET_NOT_ACTIVE", "wallet is not active")
	ErrInvalidStatus    = apperr.New(apperr.KindConflict, "WALLET_INVALID_TRANSITION", "wallet status change not allowed")
	ErrConcurrentUpdate = apperr.New(apperr.KindConflict, "CONCURRENT_UPDATE", "wallet was modified concurrently, retry")
)

// Wallet is the public view of a wallet.
type Wallet struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	LedgerAccountID uuid.UUID
	Currency        string
	Status          Status
	Version         int32
}

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
}

func NewService(pool *pgxpool.Pool, l *ledger.Service) *Service {
	return &Service{pool: pool, ledger: l}
}

func (s *Service) queries(ctx context.Context) *store.Queries {
	return store.New(db.Conn(ctx, s.pool))
}

// Create opens an INACTIVE wallet and its ledger account in one transaction.
// Activation happens later, after eKYC and a bank link (BR-05).
func (s *Service) Create(ctx context.Context, customerID uuid.UUID, currency string) (Wallet, error) {
	w := Wallet{ID: id.New(), CustomerID: customerID, Currency: currency, Status: StatusInactive}
	err := db.WithTx(ctx, s.pool, func(ctx context.Context) error {
		accountID, err := s.ledger.CreateWalletAccount(ctx, w.ID, currency)
		if err != nil {
			return err
		}
		w.LedgerAccountID = accountID

		q := s.queries(ctx)
		if err := q.InsertWallet(ctx, store.InsertWalletParams{
			ID:              w.ID,
			CustomerID:      customerID,
			LedgerAccountID: accountID,
			Currency:        currency,
			Status:          string(w.Status),
		}); err != nil {
			return fmt.Errorf("insert wallet: %w", err)
		}
		return s.recordHistory(ctx, q, w.ID, "", w.Status, "")
	})
	return w, err
}

// Activate moves an INACTIVE or FROZEN wallet to ACTIVE.
func (s *Service) Activate(ctx context.Context, walletID uuid.UUID, reason string) error {
	return s.transition(ctx, walletID, StatusActive, reason)
}

func (s *Service) transition(ctx context.Context, walletID uuid.UUID, to Status, reason string) error {
	return db.WithTx(ctx, s.pool, func(ctx context.Context) error {
		w, err := s.Get(ctx, walletID)
		if err != nil {
			return err
		}
		if !w.Status.CanTransition(to) {
			return ErrInvalidStatus.WithDetails(map[string]any{"from": w.Status, "to": to})
		}
		q := s.queries(ctx)
		n, err := q.UpdateStatus(ctx, store.UpdateStatusParams{
			ID:         walletID,
			ToStatus:   string(to),
			Reason:     nilIfEmpty(reason),
			FromStatus: string(w.Status),
			Version:    w.Version,
		})
		if err != nil {
			return fmt.Errorf("update wallet status: %w", err)
		}
		if n == 0 {
			return ErrConcurrentUpdate
		}
		return s.recordHistory(ctx, q, walletID, w.Status, to, reason)
	})
}

func (s *Service) Get(ctx context.Context, walletID uuid.UUID) (Wallet, error) {
	row, err := s.queries(ctx).GetWallet(ctx, walletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Wallet{}, ErrNotFound
	}
	if err != nil {
		return Wallet{}, fmt.Errorf("get wallet: %w", err)
	}
	return toWallet(row), nil
}

func (s *Service) GetByCustomer(ctx context.Context, customerID uuid.UUID) (Wallet, error) {
	row, err := s.queries(ctx).GetWalletByCustomer(ctx, customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Wallet{}, ErrNotFound
	}
	if err != nil {
		return Wallet{}, fmt.Errorf("get wallet by customer: %w", err)
	}
	return toWallet(row), nil
}

// LockActive row-locks the given wallets (ascending id order) for the rest of
// the caller's transaction and checks that every one can transact. A frozen
// wallet therefore cannot be frozen "between" the check and the posting.
func (s *Service) LockActive(ctx context.Context, walletIDs ...uuid.UUID) (map[uuid.UUID]Wallet, error) {
	if !db.InTx(ctx) {
		return nil, errors.New("wallet: LockActive must run inside db.WithTx")
	}
	rows, err := s.queries(ctx).LockWallets(ctx, walletIDs)
	if err != nil {
		return nil, fmt.Errorf("lock wallets: %w", err)
	}
	out := make(map[uuid.UUID]Wallet, len(rows))
	for _, row := range rows {
		w := toWallet(row)
		if !w.Status.CanTransact() {
			return nil, ErrNotActive.WithDetails(map[string]any{"wallet_id": id.Format(id.Wallet, w.ID), "status": w.Status})
		}
		out[w.ID] = w
	}
	for _, wid := range walletIDs {
		if _, ok := out[wid]; !ok {
			return nil, ErrNotFound
		}
	}
	return out, nil
}

// TouchActivity records money movement, used by the dormant-wallet job (BR-63).
func (s *Service) TouchActivity(ctx context.Context, at time.Time, walletIDs ...uuid.UUID) error {
	return s.queries(ctx).TouchActivity(ctx, store.TouchActivityParams{At: at, Ids: walletIDs})
}

func (s *Service) recordHistory(ctx context.Context, q *store.Queries, walletID uuid.UUID, from, to Status, reason string) error {
	err := q.InsertStatusHistory(ctx, store.InsertStatusHistoryParams{
		ID:         id.New(),
		WalletID:   walletID,
		FromStatus: nilIfEmpty(string(from)),
		ToStatus:   string(to),
		Reason:     nilIfEmpty(reason),
		ActorType:  "SYSTEM",
		RequestID:  observability.RequestID(ctx),
	})
	if err != nil {
		return fmt.Errorf("insert wallet status history: %w", err)
	}
	return nil
}

func toWallet(r store.WalletWallet) Wallet {
	return Wallet{
		ID:              r.ID,
		CustomerID:      r.CustomerID,
		LedgerAccountID: r.LedgerAccountID,
		Currency:        r.Currency,
		Status:          Status(r.Status),
		Version:         r.Version,
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
