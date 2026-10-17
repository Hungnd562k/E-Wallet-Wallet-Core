package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox/outboxstore"
)

// Handler processes one message. It runs inside a transaction (see Dispatcher),
// so DB writes it makes commit together with the processed_events marker.
// Returning an error schedules a retry.
type Handler func(ctx context.Context, msg Message) error

type consumer struct {
	name    string
	handler Handler
}

type DispatcherConfig struct {
	WorkerID     string
	BatchSize    int
	PollInterval time.Duration
	MaxAttempts  int32
	Lease        time.Duration // how long a claimed event stays invisible to other workers
	Concurrency  int           // messages processed in parallel per batch
}

// Dispatcher polls platform.outbox_events and delivers each event to the
// consumers registered for its type.
type Dispatcher struct {
	pool      *pgxpool.Pool
	clock     clock.Clock
	cfg       DispatcherConfig
	consumers map[string][]consumer // event type → consumers
}

func NewDispatcher(pool *pgxpool.Pool, clk clock.Clock, cfg DispatcherConfig) *Dispatcher {
	if cfg.Lease == 0 {
		cfg.Lease = time.Minute
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 4
	}
	return &Dispatcher{pool: pool, clock: clk, cfg: cfg, consumers: map[string][]consumer{}}
}

// Register subscribes a named consumer to an event type. The name is the
// idempotency scope in processed_events, so never rename a consumer casually.
// Call Register before Run; it is not safe for concurrent use.
func (d *Dispatcher) Register(eventType, consumerName string, h Handler) {
	d.consumers[eventType] = append(d.consumers[eventType], consumer{name: consumerName, handler: h})
}

// Run polls until ctx is cancelled. On cancellation it stops claiming new
// events and returns after the current batch finishes; anything unfinished is
// picked up by another worker when its lease expires.
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		n, err := d.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "outbox poll failed", "err", err)
		}
		if n > 0 && err == nil {
			continue // there may be more due events; poll again immediately
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// RunOnce claims and processes a single batch, returning how many events it claimed.
// Exposed for tests, which drive the dispatcher step by step.
func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	now := d.clock.Now()
	events, err := outboxstore.New(d.pool).ClaimEvents(ctx, outboxstore.ClaimEventsParams{
		WorkerID:   d.cfg.WorkerID,
		LeaseUntil: now.Add(d.cfg.Lease),
		Now:        now,
		Batch:      int32(d.cfg.BatchSize),
	})
	if err != nil {
		return 0, fmt.Errorf("claim events: %w", err)
	}

	// Finish in-flight work even if shutdown starts: detach from ctx's cancellation.
	workCtx := context.WithoutCancel(ctx)
	g, gctx := errgroup.WithContext(workCtx)
	g.SetLimit(d.cfg.Concurrency)
	for _, e := range events {
		g.Go(func() error {
			d.process(gctx, e)
			return nil // failures are recorded per event, never abort the batch
		})
	}
	return len(events), g.Wait()
}

func (d *Dispatcher) process(ctx context.Context, e outboxstore.PlatformOutboxEvent) {
	msg := Message{ID: e.ID, Type: e.EventType, AggregateID: e.AggregateID, Payload: e.Payload, Attempts: e.Attempts}
	_ = json.Unmarshal(e.Headers, &msg.Headers)
	ctx = observability.WithRequestID(ctx, msg.Headers[headerRequestID])
	log := slog.With("event_id", e.ID, "event_type", e.EventType, "attempt", e.Attempts+1)

	err := d.deliver(ctx, msg)
	q := outboxstore.New(d.pool)
	switch {
	case err == nil:
		if err := q.MarkDone(ctx, outboxstore.MarkDoneParams{ID: e.ID, ProcessedAt: d.clock.Now()}); err != nil {
			log.ErrorContext(ctx, "mark outbox event done", "err", err)
		}
	case e.Attempts+1 >= d.cfg.MaxAttempts:
		log.ErrorContext(ctx, "outbox event dead-lettered", "err", err)
		if err := q.MarkDead(ctx, outboxstore.MarkDeadParams{ID: e.ID, LastError: err.Error()}); err != nil {
			log.ErrorContext(ctx, "mark outbox event dead", "err", err)
		}
	default:
		delay := Backoff(e.Attempts + 1)
		log.WarnContext(ctx, "outbox event failed, will retry", "err", err, "retry_in", delay)
		if err := q.MarkRetry(ctx, outboxstore.MarkRetryParams{
			ID: e.ID, AvailableAt: d.clock.Now().Add(delay), LastError: err.Error(),
		}); err != nil {
			log.ErrorContext(ctx, "mark outbox event retry", "err", err)
		}
	}
}

// deliver runs every consumer of msg. Each consumer gets its own transaction
// containing its processed_events marker, so a consumer that already succeeded
// is skipped when the event is retried because a sibling consumer failed.
func (d *Dispatcher) deliver(ctx context.Context, msg Message) error {
	for _, c := range d.consumers[msg.Type] {
		err := db.WithTx(ctx, d.pool, func(ctx context.Context) error {
			n, err := outboxstore.New(db.Conn(ctx, d.pool)).InsertProcessed(ctx, outboxstore.InsertProcessedParams{
				Consumer: c.name, EventID: msg.ID,
			})
			if err != nil {
				return fmt.Errorf("mark processed: %w", err)
			}
			if n == 0 {
				return nil // already processed by this consumer
			}
			return c.handler(ctx, msg)
		})
		if err != nil {
			return fmt.Errorf("consumer %s: %w", c.name, err)
		}
	}
	return nil
}
