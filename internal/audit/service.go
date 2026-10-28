// Package audit writes the append-only audit log. The app DB role has only
// INSERT/SELECT on this schema. Schema: audit. BR-64, 71.
package audit

import (
	"context"
	"encoding/json"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/store"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO(ST-3): Service.Record, which always sanitizes Entry.Metadata before INSERT.

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

func (s *Service) Record(ctx context.Context, entry domain.Entry) error {
	if !db.InTx(ctx) {
		return domain.ErrNotInTx
	}

	err := db.WithTx(ctx, s.pool, func(ctx context.Context) error {
		metaData, err := json.Marshal(entry.Metadata)
		if err != nil {
			return err
		}

		err = entry.Validate()
		if err != nil {
			return err
		}

		redactedMsg, err := domain.MetadataFrom(entry)
		if err != nil {
			return err
		}

		data, err := json.Marshal(redactedMsg)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}

		s.queries(ctx).InsertAuditLogs(ctx, store.InsertAuditLogsParams{
			ID:           id.New(),
			OccurredAt:   s.clock.Now(),
			ActorType:    entry.ActorType,
			ActorID:      &entry.ActorId,
			Action:       entry.Action,
			ResourceType: entry.ResourceType,
			ResourceID:   entry.ResourceId,
			Outcome:      entry.OutCome,
			Reason:       &entry.Reason,
			RequestID:    observability.RequestID(ctx),
			TraceID:      entry.TraceId,
			Ip:           &entry.Ip,
			UserAgent:    &entry.UserAgent,
			Metadata:     metaData,
		})
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
