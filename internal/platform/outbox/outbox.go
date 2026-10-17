// Package outbox implements the transactional outbox (architecture §5.1, §6).
//
// Writer.Enqueue inserts an event in the caller's transaction, so the event
// exists if and only if the business change committed. Dispatcher (in the
// worker) delivers events at-least-once; consumers are made idempotent with
// platform.processed_events.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox/outboxstore"
)

// Event is a domain event to publish. Type is versioned, e.g. "payment.succeeded.v1".
type Event struct {
	AggregateType string
	AggregateID   uuid.UUID
	Type          string
	Payload       any // marshalled to JSON
}

// Message is an event as delivered to a consumer.
type Message struct {
	ID          uuid.UUID
	Type        string
	AggregateID uuid.UUID
	Payload     json.RawMessage
	Headers     map[string]string
	Attempts    int32
}

const headerRequestID = "request_id"

type Writer struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewWriter(pool *pgxpool.Pool, clk clock.Clock) *Writer {
	return &Writer{pool: pool, clock: clk}
}

// Enqueue stores e in the caller's transaction and returns the event id.
func (w *Writer) Enqueue(ctx context.Context, e Event) (uuid.UUID, error) {
	if !db.InTx(ctx) {
		return uuid.Nil, errors.New("outbox: Enqueue must run inside db.WithTx")
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal %s payload: %w", e.Type, err)
	}
	headers, err := json.Marshal(map[string]string{headerRequestID: observability.RequestID(ctx)})
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal headers: %w", err)
	}

	eventID := id.New()
	err = outboxstore.New(db.Conn(ctx, w.pool)).InsertEvent(ctx, outboxstore.InsertEventParams{
		ID:            eventID,
		AggregateType: e.AggregateType,
		AggregateID:   e.AggregateID,
		EventType:     e.Type,
		Payload:       payload,
		Headers:       headers,
		AvailableAt:   w.clock.Now(),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert outbox event: %w", err)
	}
	return eventID, nil
}
