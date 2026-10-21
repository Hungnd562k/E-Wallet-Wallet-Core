package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
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

func TestLedgerTransactionsAreAppendOnly(t *testing.T) {
	e := newEnv(t)
	e.newWallet(t, 1000) // guarantees at least one transaction exists

	_, err := pool.Exec(context.Background(), `UPDATE ledger.transactions SET description = 'tampered'`)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE on ledger.transactions must fail, got %v", err)
	}
	_, err = pool.Exec(context.Background(), `DELETE FROM ledger.transactions`)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE on ledger.transactions must fail, got %v", err)
	}
}

func TestLedgerRejectsEntryCurrencyMismatch(t *testing.T) {
	e := newEnv(t)
	w := e.newWallet(t, 0) // a VND account
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txnID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO ledger.transactions (id, kind, effective_date) VALUES ($1, 'ADJUSTMENT', current_date)`, txnID); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO ledger.entries (transaction_id, account_id, direction, amount, currency, balance_after)
		VALUES ($1, $2, 'C', 100, 'USD', 100)`, txnID, w.LedgerAccountID)
	if err == nil || !strings.Contains(err.Error(), "currency mismatch") {
		t.Fatalf("entry in another currency than its account must fail, got %v", err)
	}
}

func TestLedgerPostRequiresTx(t *testing.T) {
	e := newEnv(t)
	w := e.newWallet(t, 0)
	safeguard := e.safeguardID(t)

	_, err := e.modules.Ledger.Post(context.Background(), topup(safeguard, w.LedgerAccountID, 100, "VND"))
	if err == nil {
		t.Fatal("Post outside db.WithTx must fail")
	}
}

func TestLedgerPostRejectsCurrencyMismatch(t *testing.T) {
	e := newEnv(t)
	w := e.newWallet(t, 0)
	safeguard := e.safeguardID(t)

	err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
		_, err := e.modules.Ledger.Post(ctx, topup(safeguard, w.LedgerAccountID, 100, "USD"))
		return err
	})
	if !errors.Is(err, ledger.ErrCurrencyMismatch) {
		t.Fatalf("want ErrCurrencyMismatch, got %v", err)
	}
}

func TestLedgerPostRejectsClosedAccount(t *testing.T) {
	e := newEnv(t)
	w := e.newWallet(t, 0)
	safeguard := e.safeguardID(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `UPDATE ledger.accounts SET status = 'CLOSED' WHERE id = $1`, w.LedgerAccountID); err != nil {
		t.Fatal(err)
	}
	err := db.WithTx(ctx, pool, func(ctx context.Context) error {
		_, err := e.modules.Ledger.Post(ctx, topup(safeguard, w.LedgerAccountID, 100, "VND"))
		return err
	})
	if !errors.Is(err, ledger.ErrAccountClosed) {
		t.Fatalf("want ErrAccountClosed, got %v", err)
	}
}

func TestLedgerPostRejectsInsufficientFunds(t *testing.T) {
	e := newEnv(t)
	a, b := e.newWallet(t, 100), e.newWallet(t, 0)

	err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
		_, err := e.modules.Ledger.Post(ctx, ledger.Posting{
			Kind: ledger.KindTransfer, Currency: "VND",
			Lines: []ledger.Line{
				{AccountID: a.LedgerAccountID, Direction: ledger.Debit, Amount: 101},
				{AccountID: b.LedgerAccountID, Direction: ledger.Credit, Amount: 101},
			},
		})
		return err
	})
	if !errors.Is(err, ledger.ErrInsufficientFunds) {
		t.Fatalf("want ErrInsufficientFunds, got %v", err)
	}
	// The rolled-back posting must leave both balances untouched.
	if got := e.balance(t, a); got != 100 {
		t.Errorf("A balance = %d, want 100", got)
	}
	if got := e.balance(t, b); got != 0 {
		t.Errorf("B balance = %d, want 0", got)
	}
}

// assertLedgerConsistent runs the integrity checks of erd §7 over the whole
// ledger. They must hold after any sequence of committed postings.
func assertLedgerConsistent(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Drift: stored balance differs from the balance recomputed from entries.
	var drifted int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM (
		  SELECT a.id
		    FROM ledger.accounts a
		    LEFT JOIN ledger.entries e ON e.account_id = a.id
		   GROUP BY a.id
		  HAVING a.balance <> COALESCE(SUM(CASE WHEN e.direction = a.normal_side THEN e.amount ELSE -e.amount END), 0)
		) d`).Scan(&drifted)
	if err != nil {
		t.Fatal(err)
	}
	if drifted != 0 {
		t.Errorf("%d ledger accounts drifted from their entries", drifted)
	}

	// Accounting equation: ASSET = LIABILITY + REVENUE.
	var gap int64
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(balance) FILTER (WHERE type = 'ASSET'), 0)
		     - COALESCE(SUM(balance) FILTER (WHERE type IN ('LIABILITY', 'REVENUE')), 0)
		  FROM ledger.accounts`).Scan(&gap)
	if err != nil {
		t.Fatal(err)
	}
	if gap != 0 {
		t.Errorf("accounting equation broken: ASSET - (LIABILITY + REVENUE) = %d", gap)
	}

	// The balance_after of each account's latest entry is its stored balance.
	var stale int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		  FROM ledger.accounts a
		  JOIN LATERAL (
		    SELECT balance_after FROM ledger.entries e
		     WHERE e.account_id = a.id
		     ORDER BY e.seq DESC
		     LIMIT 1
		  ) last ON true
		 WHERE last.balance_after <> a.balance`).Scan(&stale)
	if err != nil {
		t.Fatal(err)
	}
	if stale != 0 {
		t.Errorf("%d ledger accounts whose latest balance_after differs from balance", stale)
	}
}

func (e *env) safeguardID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := e.modules.Ledger.AccountIDByCode(context.Background(), ledger.CodeSafeguardBank)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// topup is the posting of a confirmed bank top-up (erd §3.2 row 1).
func topup(safeguard, walletAccount uuid.UUID, amount int64, currency string) ledger.Posting {
	return ledger.Posting{
		Kind: ledger.KindTopup, Currency: currency,
		Lines: []ledger.Line{
			{AccountID: safeguard, Direction: ledger.Debit, Amount: amount},
			{AccountID: walletAccount, Direction: ledger.Credit, Amount: amount},
		},
	}
}
