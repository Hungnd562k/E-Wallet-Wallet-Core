package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The ledger invariants are enforced by the database itself (ADR-08), so even
// buggy application code cannot corrupt them. These tests bypass the app.

func TestLedgerEntriesAreAppendOnly(t *testing.T) {
	e := newEnv(t)
	e.newWallet(t, 1000) // guarantees at least one entry exists

	_, err := pool.Exec(context.Background(), `UPDATE ledger.entries SET amount = amount + 1`)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE on ledger.entries must fail, got %v", err)
	}
	_, err = pool.Exec(context.Background(), `DELETE FROM ledger.entries`)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE on ledger.entries must fail, got %v", err)
	}
}

func TestLedgerRejectsUnbalancedTransactionAtCommit(t *testing.T) {
	e := newEnv(t)
	w := e.newWallet(t, 0)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a failed commit

	txnID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO ledger.transactions (id, kind, effective_date) VALUES ($1, 'ADJUSTMENT', current_date)`, txnID); err != nil {
		t.Fatal(err)
	}
	// A single credit with no matching debit: inserting succeeds because the
	// balance check is DEFERRED, but COMMIT must fail.
	if _, err := tx.Exec(ctx, `INSERT INTO ledger.entries (transaction_id, account_id, direction, amount, currency, balance_after)
		VALUES ($1, $2, 'C', 100, 'VND', 100)`, txnID, w.LedgerAccountID); err != nil {
		t.Fatal(err)
	}
	err = tx.Commit(ctx)
	if err == nil || !strings.Contains(err.Error(), "unbalanced") {
		t.Fatalf("commit of unbalanced transaction must fail, got %v", err)
	}
}

func TestLedgerBalanceCannotGoNegative(t *testing.T) {
	e := newEnv(t)
	w := e.newWallet(t, 0)
	_, err := pool.Exec(context.Background(), `UPDATE ledger.accounts SET balance = -1 WHERE id = $1`, w.LedgerAccountID)
	if err == nil || !strings.Contains(err.Error(), "accounts_non_negative") {
		t.Fatalf("negative balance must violate the CHECK, got %v", err)
	}
}
