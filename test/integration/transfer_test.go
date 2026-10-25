package integration

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestTransferSucceeds(t *testing.T) {
	e := newEnv(t)
	a, b := e.newWallet(t, 1_000_000), e.newWallet(t, 0)

	r := e.transfer(t, a, b.ID, 200_000, "k1")

	if r.Status != http.StatusCreated || r.Body["status"] != "SUCCEEDED" {
		t.Fatalf("got %d %v", r.Status, r.Body)
	}
	if got := e.balance(t, a); got != 800_000 {
		t.Errorf("sender balance = %d, want 800000", got)
	}
	if got := e.balance(t, b); got != 200_000 {
		t.Errorf("recipient balance = %d, want 200000", got)
	}
	if n := count(t, `SELECT count(*) FROM platform.outbox_events
		WHERE event_type = 'payment.succeeded.v1' AND payload->>'source_wallet_id' = $1`, a.ID.String()); n != 1 {
		t.Errorf("outbox events = %d, want 1", n)
	}
	if n := count(t, `SELECT count(*) FROM payment.wallet_activities WHERE wallet_id IN ($1, $2)`, a.ID, b.ID); n != 2 {
		t.Errorf("wallet activities = %d, want 2", n)
	}
}

func TestTransferIdempotency(t *testing.T) {
	e := newEnv(t)
	a, b := e.newWallet(t, 1_000_000), e.newWallet(t, 0)

	first := e.transfer(t, a, b.ID, 100_000, "same-key")
	replay := e.transfer(t, a, b.ID, 100_000, "same-key")

	if first.Status != http.StatusCreated || replay.Status != http.StatusCreated {
		t.Fatalf("statuses %d, %d", first.Status, replay.Status)
	}
	if first.Body["payment_id"] != replay.Body["payment_id"] {
		t.Errorf("replay returned a different payment: %v vs %v", first.Body["payment_id"], replay.Body["payment_id"])
	}
	if got := e.balance(t, a); got != 900_000 {
		t.Errorf("money moved twice: sender balance = %d", got)
	}

	t.Run("same key, different payload", func(t *testing.T) {
		r := e.transfer(t, a, b.ID, 1, "same-key")
		if r.Status != http.StatusUnprocessableEntity || r.Body["error"] != "IDEMPOTENCY_KEY_REUSED" {
			t.Fatalf("got %d %v", r.Status, r.Body)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		r := e.transfer(t, a, b.ID, 1, "")
		if r.Status != http.StatusBadRequest {
			t.Fatalf("got %d %v", r.Status, r.Body)
		}
	})
}

func TestTransferInsufficientFunds(t *testing.T) {
	e := newEnv(t)
	a, b := e.newWallet(t, 50_000), e.newWallet(t, 0)

	for i := range 2 { // second call is a replay of the stored failure
		r := e.transfer(t, a, b.ID, 50_001, "too-much")
		if r.Status != http.StatusUnprocessableEntity || r.Body["error"] != "INSUFFICIENT_FUNDS" {
			t.Fatalf("call %d: got %d %v", i, r.Status, r.Body)
		}
	}
	if got := e.balance(t, a); got != 50_000 {
		t.Errorf("balance changed to %d", got)
	}
	if n := count(t, `SELECT count(*) FROM payment.payments WHERE source_wallet_id = $1`, a.ID); n != 0 {
		t.Errorf("a failed transfer must not store a payment, found %d", n)
	}
}

func TestTransferRejectsInvalidInput(t *testing.T) {
	e := newEnv(t)
	a := e.newWallet(t, 1000)

	tests := []struct {
		name     string
		to       uuid.UUID
		amount   int64
		wantCode string
	}{
		{"to self", a.ID, 10, "SAME_WALLET"},
		{"unknown recipient", uuid.New(), 10, "WALLET_NOT_FOUND"},
		{"zero amount", uuid.New(), 0, "INVALID_REQUEST"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := e.transfer(t, a, tt.to, tt.amount, fmt.Sprintf("invalid-%d", i))
			if r.Body["error"] != tt.wantCode {
				t.Fatalf("got %d %v, want %s", r.Status, r.Body, tt.wantCode)
			}
		})
	}
}

// TestConcurrentCrossTransfers fires transfers A→B and B→A in parallel. Locking
// in id order must prevent deadlocks, no balance may go negative, and the total
// must be conserved. Run with -race.
func TestConcurrentCrossTransfers(t *testing.T) {
	e := newEnv(t)
	a, b := e.newWallet(t, 100_000), e.newWallet(t, 100_000)

	const n = 50
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		statuses = map[int]int{}
	)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			from, to := a, b
			if i%2 == 1 {
				from, to = b, a
			}
			r, err := e.tryTransfer(from, to.ID, 7_000, fmt.Sprintf("cross-%d", i))
			if err != nil {
				t.Error(err) // t.Error is goroutine-safe; t.Fatal is not
				return
			}
			mu.Lock()
			statuses[r.Status]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	for status := range statuses {
		if status != http.StatusCreated && status != http.StatusUnprocessableEntity {
			t.Errorf("unexpected status %d (x%d)", status, statuses[status])
		}
	}
	balA, balB := e.balance(t, a), e.balance(t, b)
	if balA < 0 || balB < 0 {
		t.Errorf("negative balance: A=%d B=%d", balA, balB)
	}
	if balA+balB != 200_000 {
		t.Errorf("money created or destroyed: A+B = %d", balA+balB)
	}
	assertLedgerConsistent(t)
}

// TestConcurrentSameKey sends one logical request many times at once: exactly
// one must execute, every caller must see the same payment.
func TestConcurrentSameKey(t *testing.T) {
	e := newEnv(t)
	a, b := e.newWallet(t, 1_000_000), e.newWallet(t, 0)

	const n = 10
	ids := make([]any, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := e.tryTransfer(a, b.ID, 10_000, "burst")
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = r.Body["payment_id"] // each goroutine writes its own index: no race
		}()
	}
	wg.Wait()

	for i, pid := range ids {
		if pid == nil || pid != ids[0] {
			t.Fatalf("response %d has payment_id %v, want %v", i, pid, ids[0])
		}
	}
	if got := e.balance(t, b); got != 10_000 {
		t.Errorf("recipient balance = %d, want exactly one transfer (10000)", got)
	}
}
