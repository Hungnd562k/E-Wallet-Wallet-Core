package integration

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox/outboxstore"
)

// enqueue writes one event of a test-unique type, so tests do not see each other's events.
func enqueue(t *testing.T, clk clock.Clock, eventType string) uuid.UUID {
	t.Helper()
	var eventID uuid.UUID
	err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
		var err error
		eventID, err = outbox.NewWriter(pool, clk).Enqueue(ctx, outbox.Event{
			AggregateType: "test", AggregateID: uuid.New(), Type: eventType, Payload: map[string]int{"n": 1},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return eventID
}

func eventStatus(t *testing.T, eventID uuid.UUID) (status string, attempts int) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT status, attempts FROM platform.outbox_events WHERE id = $1`, eventID).Scan(&status, &attempts)
	if err != nil {
		t.Fatal(err)
	}
	return status, attempts
}

// drain runs the dispatcher until it claims nothing more. Events of other
// tests may be processed too; that is harmless.
func drain(t *testing.T, d *outbox.Dispatcher) {
	t.Helper()
	for range 20 {
		n, err := d.RunOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
}

// claimAs leases due events to workerID exactly like RunOnce does, but never
// processes them: the state a worker leaves behind when it crashes right after
// claiming. It fails the test unless eventID was among the claimed events.
func claimAs(t *testing.T, clk clock.Clock, workerID string, lease time.Duration, eventID uuid.UUID) {
	t.Helper()
	now := clk.Now()
	events, err := outboxstore.New(pool).ClaimEvents(context.Background(), outboxstore.ClaimEventsParams{
		WorkerID:   workerID,
		LeaseUntil: now.Add(lease),
		Now:        now,
		Batch:      100,
	})
	if err != nil {
		t.Fatalf("claim as %s: %v", workerID, err)
	}
	for _, e := range events {
		if e.ID == eventID {
			return
		}
	}
	t.Fatalf("worker %s did not claim event %s", workerID, eventID)
}

func TestOutboxDeliversOnce(t *testing.T) {
	clk := clock.NewFake(time.Now())
	eventType := "test.delivers_once." + uuid.NewString()
	eventID := enqueue(t, clk, eventType)

	var calls atomic.Int32
	d := outbox.NewDispatcher(pool, clk, outbox.DispatcherConfig{WorkerID: "t", BatchSize: 10, MaxAttempts: 3})
	d.Register(eventType, "counter", func(context.Context, outbox.Message) error {
		calls.Add(1)
		return nil
	})

	drain(t, d)
	drain(t, d)

	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times, want 1", got)
	}
	if status, _ := eventStatus(t, eventID); status != "DONE" {
		t.Errorf("status = %s, want DONE", status)
	}
}

func TestOutboxRetriesThenDeadLetters(t *testing.T) {
	clk := clock.NewFake(time.Now())
	eventType := "test.dead_letter." + uuid.NewString()
	eventID := enqueue(t, clk, eventType)

	d := outbox.NewDispatcher(pool, clk, outbox.DispatcherConfig{WorkerID: "t", BatchSize: 10, MaxAttempts: 3})
	d.Register(eventType, "always-fails", func(context.Context, outbox.Message) error {
		return errors.New("boom")
	})

	drain(t, d)
	if status, attempts := eventStatus(t, eventID); status != "PENDING" || attempts != 1 {
		t.Fatalf("after 1st failure: %s/%d, want PENDING/1", status, attempts)
	}

	// Not due yet: the backoff delay has not elapsed on the fake clock.
	drain(t, d)
	if _, attempts := eventStatus(t, eventID); attempts != 1 {
		t.Fatalf("retried before backoff elapsed: attempts=%d", attempts)
	}

	for range 2 {
		clk.Advance(10 * time.Minute)
		drain(t, d)
	}
	if status, attempts := eventStatus(t, eventID); status != "DEAD" || attempts != 3 {
		t.Fatalf("final: %s/%d, want DEAD/3", status, attempts)
	}
}

func TestOutboxReclaimedAfterWorkerCrash(t *testing.T) {
	const lease = time.Minute
	clk := clock.NewFake(time.Now())
	eventType := "test.crash." + uuid.NewString()
	eventID := enqueue(t, clk, eventType)

	var calls atomic.Int32
	newWorker := func(id string) *outbox.Dispatcher {
		d := outbox.NewDispatcher(pool, clk, outbox.DispatcherConfig{
			WorkerID: id, BatchSize: 10, MaxAttempts: 3, Lease: time.Minute})
		d.Register(eventType, "counter", func(context.Context, outbox.Message) error {
			calls.Add(1)
			return nil
		})
		return d
	}

	// Worker A claims the event, then "crashes" before processing it.
	// Simulated by running only the claim query with worker_id "A"
	// (e.g. outboxstore.New(pool).ClaimEvents(...)), never calling the handler.
	claimAs(t, clk, "A", lease, eventID)

	b := newWorker("B")
	drain(t, b) // lease of A still valid
	if calls.Load() != 0 {
		t.Fatal("B took a leased event")
	}

	clk.Advance(time.Minute + time.Second) // lease expires
	drain(t, b)
	drain(t, b)

	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times, want 1", got)
	}
	if s, _ := eventStatus(t, eventID); s != "DONE" {
		t.Errorf("status = %s, want DONE", s)
	}
}
