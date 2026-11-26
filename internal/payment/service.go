// Package payment orchestrates money movements: it validates, enforces
// idempotency, calls ledger/wallet (and later limit/authz/banklink) inside one
// transaction, owns the payment state machine and the history read-model.
package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/payment/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/payment/store"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/httpx"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/idempotency"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet"
)

type (
	Type   = domain.Type
	Status = domain.Status
)

const (
	EventPaymentSucceeded = "payment.succeeded.v1"
	endpointTransfer      = "POST /v1/transfers"
)

var ErrNotFound = apperr.New(apperr.KindNotFound, "PAYMENT_NOT_FOUND", "payment not found")

// Payment is the public view of a payment. Its JSON form is the API response
// and is also what the idempotency store replays.
type Payment struct {
	ID             string     `json:"payment_id"`
	Type           Type       `json:"type"`
	Status         Status     `json:"status"`
	Amount         int64      `json:"amount"`
	Fee            int64      `json:"fee"`
	Currency       string     `json:"currency"`
	SourceWalletID string     `json:"source_wallet_id,omitempty"`
	DestWalletID   string     `json:"dest_wallet_id,omitempty"`
	Description    *string    `json:"description"`
	FailureCode    *string    `json:"failure_code"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

// SucceededEvent is the payload of payment.succeeded.v1.
type SucceededEvent struct {
	PaymentID      uuid.UUID `json:"payment_id"`
	Type           Type      `json:"type"`
	Amount         int64     `json:"amount"`
	Currency       string    `json:"currency"`
	SourceWalletID uuid.UUID `json:"source_wallet_id"`
	DestWalletID   uuid.UUID `json:"dest_wallet_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type AuditMetadata struct {
	Amount       int64
	Currency     string
	SourceWallet uuid.UUID
	DestWallet   uuid.UUID
}

// TransferCommand is the input of Transfer. The caller (HTTP layer) has
// already authenticated CustomerID.
type TransferCommand struct {
	CustomerID        uuid.UUID
	IdempotencyKey    string
	RecipientWalletID uuid.UUID
	Amount            int64
	Currency          string
	Description       string
}

type Service struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	ledger  *ledger.Service
	wallets *wallet.Service
	idem    *idempotency.Store
	outbox  *outbox.Writer
	audit   *audit.Service
}

func NewService(pool *pgxpool.Pool, clk clock.Clock, l *ledger.Service, w *wallet.Service,
	idem *idempotency.Store, ob *outbox.Writer, a *audit.Service) *Service {
	return &Service{pool: pool, clock: clk, ledger: l, wallets: w, idem: idem, outbox: ob, audit: a}
}

func (s *Service) queries(ctx context.Context) *store.Queries {
	return store.New(db.Conn(ctx, s.pool))
}

// Transfer moves money between two wallets synchronously, in one transaction
// (architecture §5.2). Retrying with the same IdempotencyKey returns the
// original outcome, success or business error, without executing again.
func (s *Service) Transfer(ctx context.Context, cmd TransferCommand) (Payment, error) {
	hash, err := idempotency.Hash(struct {
		To          uuid.UUID
		Amount      int64
		Currency    string
		Description string
	}{cmd.RecipientWalletID, cmd.Amount, cmd.Currency, cmd.Description})
	if err != nil {
		return Payment{}, err
	}
	idemReq := idempotency.Request{
		ScopeType: idempotency.ScopeCustomer,
		ScopeID:   cmd.CustomerID,
		Key:       cmd.IdempotencyKey,
		Endpoint:  endpointTransfer,
		Hash:      hash,
	}

	var (
		result   Payment
		replayed bool
	)
	err = db.WithTx(ctx, s.pool, func(ctx context.Context) error {
		stored, err := s.idem.Begin(ctx, idemReq)
		if err != nil {
			return err
		}
		if stored != nil {
			replayed = true
			result, err = replay(stored)
			return err
		}

		result, err = s.transferInTx(ctx, cmd)
		if err != nil {
			return err
		}
		body, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal payment: %w", err)
		}
		pid, _ := id.Parse(id.Payment, result.ID)
		return s.idem.Complete(ctx, idemReq, idempotency.Response{Status: http.StatusCreated, Body: body, ResourceID: &pid})
	})

	// A business error rolled the transaction back. Remember it under the key so a
	// retry gets the same answer. Infrastructure errors are not saved: retrying may succeed.
	if e, ok := apperr.As(err); ok && !replayed && e.Kind != apperr.KindInternal && !isIdempotencyError(err) {
		body, mErr := json.Marshal(e)
		if mErr == nil {
			if sErr := s.idem.SaveFailure(ctx, idemReq, idempotency.Response{Status: httpx.StatusOf(e.Kind), Body: body}); sErr != nil {
				return Payment{}, errors.Join(err, sErr)
			}
		}
	}
	return result, err
}

func (s *Service) transferInTx(ctx context.Context, cmd TransferCommand) (Payment, error) {
	source, err := s.wallets.GetByCustomer(ctx, cmd.CustomerID)
	if err != nil {
		return Payment{}, err
	}
	t := domain.Transfer{
		SourceWalletID: source.ID,
		DestWalletID:   cmd.RecipientWalletID,
		Amount:         cmd.Amount,
		Currency:       cmd.Currency,
		Description:    cmd.Description,
	}
	if err := t.Validate(); err != nil {
		return Payment{}, err
	}

	// TODO(authz, BR-35/36): require PIN or step-up challenge before moving money.
	// TODO(limit, BR-25..29): limit.CheckAndConsume in this same transaction.

	wallets, err := s.wallets.LockActive(ctx, t.SourceWalletID, t.DestWalletID)
	if err != nil {
		return Payment{}, err
	}
	src, dst := wallets[t.SourceWalletID], wallets[t.DestWalletID]

	paymentID := id.New()
	ledgerTxnID, err := s.ledger.Post(ctx, ledger.Posting{
		Kind:        ledger.KindTransfer,
		PaymentID:   &paymentID,
		Description: t.Description,
		Currency:    t.Currency,
		Lines: []ledger.Line{
			{AccountID: src.LedgerAccountID, Direction: ledger.Debit, Amount: t.Amount},
			{AccountID: dst.LedgerAccountID, Direction: ledger.Credit, Amount: t.Amount},
		},
	})
	if err != nil {
		return Payment{}, err
	}

	now := s.clock.Now()
	requestID := observability.RequestID(ctx)
	desc := nilIfEmpty(t.Description)
	q := s.queries(ctx)

	if err := q.InsertPayment(ctx, store.InsertPaymentParams{
		ID:             paymentID,
		Type:           string(domain.TypeTransfer),
		Status:         string(domain.StatusSucceeded),
		CustomerID:     cmd.CustomerID,
		SourceWalletID: &src.ID,
		DestWalletID:   &dst.ID,
		Amount:         t.Amount,
		Currency:       t.Currency,
		Description:    desc,
		LedgerTxnID:    &ledgerTxnID,
		IdempotencyKey: cmd.IdempotencyKey,
		RequestID:      requestID,
		CompletedAt:    &now,
		CreatedAt:      now,
	}); err != nil {
		return Payment{}, fmt.Errorf("insert payment: %w", err)
	}

	if err := q.InsertStatusHistory(ctx, store.InsertStatusHistoryParams{
		ID:        id.New(),
		PaymentID: paymentID,
		ToStatus:  string(domain.StatusSucceeded),
		ActorType: "CUSTOMER",
		ActorID:   &cmd.CustomerID,
		RequestID: requestID,
	}); err != nil {
		return Payment{}, fmt.Errorf("insert payment history: %w", err)
	}

	// Read-model: one row per side so each wallet's history is a single index scan.
	for _, a := range []struct {
		walletID  uuid.UUID
		direction string
	}{{src.ID, "OUT"}, {dst.ID, "IN"}} {
		if err := q.InsertActivity(ctx, store.InsertActivityParams{
			ID:          id.New(),
			WalletID:    a.walletID,
			PaymentID:   paymentID,
			Direction:   a.direction,
			Type:        string(domain.TypeTransfer),
			Amount:      t.Amount,
			Status:      string(domain.StatusSucceeded),
			Description: desc,
			OccurredAt:  now,
			// TODO(identity): counterparty_display (masked name/phone) once customers exist.
		}); err != nil {
			return Payment{}, fmt.Errorf("insert wallet activity: %w", err)
		}
	}

	if err := s.wallets.TouchActivity(ctx, now, src.ID, dst.ID); err != nil {
		return Payment{}, fmt.Errorf("touch wallet activity: %w", err)
	}

	// TODO: SCRUM-78: ST-4 Gắn audit vào chuyển tiền và đổi trạng thái ví + cập nhật archtest
	err = s.audit.Record(ctx, audit.Entry{
		ActorType:    string(audit.ActorCustomer),
		ActorId:      cmd.RecipientWalletID,
		Action:       string(audit.ActionPaymentTransfer),
		ResourceType: "",
		ResourceId:   "",
		OutCome:      string(audit.OutcomeSuccess),
		Reason:       cmd.Description,
		Metadata: map[string]any{
			"amount":        t.Amount,
			"currency":      t.Currency,
			"source_wallet": t.SourceWalletID.String(),
			"dest_wallet":   t.DestWalletID.String(),
		},
	})

	if _, err := s.outbox.Enqueue(ctx, outbox.Event{
		AggregateType: "payment",
		AggregateID:   paymentID,
		Type:          EventPaymentSucceeded,
		Payload: SucceededEvent{
			PaymentID: paymentID, Type: domain.TypeTransfer, Amount: t.Amount, Currency: t.Currency,
			SourceWalletID: src.ID, DestWalletID: dst.ID, OccurredAt: now,
		},
	}); err != nil {
		return Payment{}, err
	}

	return Payment{
		ID:             id.Format(id.Payment, paymentID),
		Type:           domain.TypeTransfer,
		Status:         domain.StatusSucceeded,
		Amount:         t.Amount,
		Currency:       t.Currency,
		SourceWalletID: id.Format(id.Wallet, src.ID),
		DestWalletID:   id.Format(id.Wallet, dst.ID),
		Description:    desc,
		CreatedAt:      now,
		CompletedAt:    &now,
	}, nil
}

// Get returns a payment visible to customerID (the initiator).
func (s *Service) Get(ctx context.Context, customerID, paymentID uuid.UUID) (Payment, error) {
	row, err := s.queries(ctx).GetPayment(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.CustomerID != customerID) {
		return Payment{}, ErrNotFound // same error for "not yours": do not leak existence
	}
	if err != nil {
		return Payment{}, fmt.Errorf("get payment: %w", err)
	}
	return toPayment(row), nil
}

// replay rebuilds the original outcome from a stored idempotent response.
func replay(r *idempotency.Response) (Payment, error) {
	if r.Status >= 200 && r.Status < 300 {
		var p Payment
		if err := json.Unmarshal(r.Body, &p); err != nil {
			return Payment{}, fmt.Errorf("decode stored payment: %w", err)
		}
		return p, nil
	}
	var e apperr.Error
	if err := json.Unmarshal(r.Body, &e); err != nil {
		return Payment{}, fmt.Errorf("decode stored error: %w", err)
	}
	return Payment{}, &e
}

func isIdempotencyError(err error) bool {
	return errors.Is(err, apperr.ErrIdempotencyKeyReused) ||
		errors.Is(err, apperr.ErrIdempotencyInFlight) ||
		errors.Is(err, idempotency.ErrMissingKey)
}

func toPayment(r store.PaymentPayment) Payment {
	p := Payment{
		ID:          id.Format(id.Payment, r.ID),
		Type:        Type(r.Type),
		Status:      Status(r.Status),
		Amount:      r.Amount,
		Fee:         r.Fee,
		Currency:    r.Currency,
		Description: r.Description,
		FailureCode: r.FailureCode,
		CreatedAt:   r.CreatedAt.UTC(), // pgx returns timestamptz in the local zone
	}
	if r.CompletedAt != nil {
		t := r.CompletedAt.UTC()
		p.CompletedAt = &t
	}
	if r.SourceWalletID != nil {
		p.SourceWalletID = id.Format(id.Wallet, *r.SourceWalletID)
	}
	if r.DestWalletID != nil {
		p.DestWalletID = id.Format(id.Wallet, *r.DestWalletID)
	}
	return p
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
