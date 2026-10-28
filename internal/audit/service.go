// Package audit writes the append-only audit log. The app DB role has only
// INSERT/SELECT on this schema. Schema: audit. BR-64, 71.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/store"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
)

type (
	Entry     = domain.Entry
	Action    = domain.Action
	ActorType = domain.ActorType
	Outcome   = domain.Outcome
)

const (
	ActionPaymentTransfer    = domain.Payment_Transfer
	ActionWalletStatusChange = domain.Wallet_Status_Change

	ActorCustomer = domain.Customer
	ActorStaff    = domain.Staff
	ActorMerchant = domain.Merchant
	ActorSystem   = domain.System

	OutcomeSuccess = domain.Success
	OutcomeDenied  = domain.Denied
	OutcomeError   = domain.Error
)

var ErrNotInTx = domain.ErrNotInTx

type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, clock: clk}
}

func (s *Service) queries(ctx context.Context) *store.Queries {
	return store.New(db.Conn(ctx, s.pool))
}

// SanitizeMetadata returns a copy of m with sensitive values hidden (BR-71).
func SanitizeMetadata(m map[string]any) map[string]any { return domain.SanitizeMetadata(m) }

// MetadataFrom turns a struct into sanitized metadata using its json tags.
func MetadataFrom(v any) (map[string]any, error) { return domain.MetadataFrom(v) }

// Record appends e to audit.audit_logs inside the caller's transaction, so the
// audit row commits or rolls back together with the change it describes.
// request_id, trace_id, ip and user_agent come from ctx; metadata is always
// sanitized here, so callers cannot leak a PIN or CCCD by forgetting to.
func (s *Service) Record(ctx context.Context, e Entry) error {
	if !db.InTx(ctx) {
		return domain.ErrNotInTx
	}
	if err := e.Validate(); err != nil {
		return fmt.Errorf("audit %s: %w", e.Action, err)
	}

	metadata := domain.SanitizeMetadata(e.Metadata)
	if metadata == nil {
		metadata = map[string]any{} // column is NOT NULL and must be a JSON object
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("audit %s: marshal metadata: %w", e.Action, err)
	}

	client := observability.ClientFrom(ctx)
	auditID := id.New()
	err = s.queries(ctx).InsertAuditLogs(ctx, store.InsertAuditLogsParams{
		ID:           auditID,
		OccurredAt:   s.clock.Now(),
		ActorType:    e.ActorType,
		ActorID:      nilIfZero(e.ActorId),
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceId,
		Outcome:      e.OutCome,
		Reason:       nilIfEmpty(e.Reason),
		RequestID:    observability.RequestID(ctx),
		TraceID:      observability.TraceID(ctx),
		Ip:           nilIfEmpty(client.IP),
		UserAgent:    nilIfEmpty(client.UserAgent),
		Metadata:     payload,
	})
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	// request_id and trace_id are added by the logger from ctx. Metadata is left
	// out on purpose: the audit row is the record, the log line only points to it.
	// The surrounding transaction may still roll back after this line.
	slog.InfoContext(ctx, "audit recorded",
		"audit_id", auditID,
		"action", e.Action,
		"actor_type", e.ActorType,
		"resource_type", e.ResourceType,
		"resource_id", e.ResourceId,
		"outcome", e.OutCome,
	)
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nilIfZero(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}
