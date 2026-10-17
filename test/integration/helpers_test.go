package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/app"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/auth"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet"
)

// env is a running API plus direct access to the modules for setup and assertions.
type env struct {
	modules *app.Modules
	server  *httptest.Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	m := app.NewModules(pool, clock.Real{})
	srv := httptest.NewServer(app.NewAPIHandler(pool, m, auth.DevHeaderAuth{}))
	t.Cleanup(srv.Close)
	return &env{modules: m, server: srv}
}

// newWallet creates an active wallet for a fresh customer, funded with balance VND.
func (e *env) newWallet(t *testing.T, balance int64) wallet.Wallet {
	t.Helper()
	ctx := context.Background()
	w, err := e.modules.Wallet.Create(ctx, uuid.New(), "VND")
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	if err := e.modules.Wallet.Activate(ctx, w.ID, "test"); err != nil {
		t.Fatalf("activate wallet: %v", err)
	}
	if balance > 0 {
		err = db.WithTx(ctx, pool, func(ctx context.Context) error {
			safeguard, err := e.modules.Ledger.AccountIDByCode(ctx, ledger.CodeSafeguardBank)
			if err != nil {
				return err
			}
			_, err = e.modules.Ledger.Post(ctx, ledger.Posting{
				Kind: ledger.KindTopup, Currency: "VND",
				Lines: []ledger.Line{
					{AccountID: safeguard, Direction: ledger.Debit, Amount: balance},
					{AccountID: w.LedgerAccountID, Direction: ledger.Credit, Amount: balance},
				},
			})
			return err
		})
		if err != nil {
			t.Fatalf("fund wallet: %v", err)
		}
	}
	return w
}

// balance returns the stored balance and fails if it differs from Σentries (drift).
func (e *env) balance(t *testing.T, w wallet.Wallet) int64 {
	t.Helper()
	ctx := context.Background()
	stored, err := e.modules.Ledger.Balance(ctx, w.LedgerAccountID)
	if err != nil {
		t.Fatal(err)
	}
	recomputed, err := e.modules.Ledger.RecomputedBalance(ctx, w.LedgerAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if stored != recomputed {
		t.Fatalf("ledger drift on %s: stored=%d entries=%d", w.ID, stored, recomputed)
	}
	return stored
}

type response struct {
	Status int
	Body   map[string]any
}

// transfer calls POST /v1/transfers and fails the test on transport errors.
// Must be called from the test goroutine; use tryTransfer inside goroutines.
func (e *env) transfer(t *testing.T, from wallet.Wallet, to uuid.UUID, amount int64, key string) response {
	t.Helper()
	r, err := e.tryTransfer(from, to, amount, key)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// tryTransfer is safe to call from any goroutine: it returns errors instead of
// calling t.Fatal, which may only be called from the goroutine running the test.
func (e *env) tryTransfer(from wallet.Wallet, to uuid.UUID, amount int64, key string) (response, error) {
	body := fmt.Sprintf(`{"recipient":{"wallet_id":%q},"amount":%d,"currency":"VND","pin":"123456"}`,
		id.Format(id.Wallet, to), amount)
	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/v1/transfers", bytes.NewBufferString(body))
	if err != nil {
		return response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.HeaderDevCustomerID, from.CustomerID.String())
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return response{}, err
	}
	r := response{Status: resp.StatusCode}
	if err := json.Unmarshal(raw, &r.Body); err != nil {
		return response{}, fmt.Errorf("decode body %q: %w", raw, err)
	}
	return r, nil
}

func count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
