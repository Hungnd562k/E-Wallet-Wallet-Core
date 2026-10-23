// Command devseed creates demo data for local development: two active wallets
// for fixed customer ids, each topped up with 1,000,000 VND through a real
// TOPUP ledger posting. Safe to run repeatedly.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/app"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/date"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet"
)

const initialBalance = 10_000_000 // VND

var customers = []struct {
	name  string
	id    uuid.UUID
	phone string
	Dob   date.Date
}{
	{
		"Nguyễn Đức Hưng",
		uuid.MustParse("0199a000-0000-7000-8000-00000000000a"),
		"0869856211",
		date.Date{Year: 2000, Month: 6, Day: 5},
	},
	{
		"Nguyễn Phúc Huy",
		uuid.MustParse("0199a000-0000-7000-8000-00000000000b"),
		"0987635422",
		date.Date{Year: 2000, Month: 6, Day: 5},
	},
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "devseed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	pool, err := db.NewPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	m := app.NewModules(pool, clock.Real{})

	for _, c := range customers {
		err := ensureIdentityCustomer(ctx, m, c.phone)
		if err != nil {
			return fmt.Errorf("%s: %w", c.phone, err)
		}
		w, err := ensureWallet(ctx, pool, m, c.id)
		if err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		balance, err := m.Ledger.Balance(ctx, w.LedgerAccountID)
		if err != nil {
			return err
		}
		fmt.Printf("%-6s customer_id=%s wallet_id=%s balance=%d\n",
			c.name, c.id, id.Format(id.Wallet, w.ID), balance)
	}
	return nil
}

func ensureWallet(ctx context.Context, pool *pgxpool.Pool, m *app.Modules, customerID uuid.UUID) (wallet.Wallet, error) {
	w, err := m.Wallet.GetByCustomer(ctx, customerID)
	if err == nil {
		return w, nil // already seeded
	}
	if !errors.Is(err, wallet.ErrNotFound) {
		return wallet.Wallet{}, err
	}

	if w, err = m.Wallet.Create(ctx, customerID, "VND"); err != nil {
		return wallet.Wallet{}, err
	}
	if err := m.Wallet.Activate(ctx, w.ID, "devseed"); err != nil {
		return wallet.Wallet{}, err
	}

	// Simulate a confirmed bank top-up (erd §3.2 row 1): D SAFEGUARD_BANK / C WALLET.
	err = db.WithTx(ctx, pool, func(ctx context.Context) error {
		safeguard, err := m.Ledger.AccountIDByCode(ctx, ledger.CodeSafeguardBank)
		if err != nil {
			return err
		}
		_, err = m.Ledger.Post(ctx, ledger.Posting{
			Kind:        ledger.KindTopup,
			Description: "devseed initial balance",
			Currency:    "VND",
			Lines: []ledger.Line{
				{AccountID: safeguard, Direction: ledger.Debit, Amount: initialBalance},
				{AccountID: w.LedgerAccountID, Direction: ledger.Credit, Amount: initialBalance},
			},
		})
		return err
	})
	return w, err
}

// Seed identity.customers + eKYC
func ensureIdentityCustomer(ctx context.Context, m *app.Modules, phone string) error {
	c, err := m.Identity.GetCustomerByPhone(ctx, phone)
	if err == nil {
		return nil // already seeded
	}
	// Create
	err = m.Identity.Create(ctx, identity.CustomerCommand{
		Id:       c.Id,
		Phone:    c.Phone,
		FullName: c.FullName,
		Dob:      c.Dob,
	})

	if err != nil {
		return err
	}

	return nil
}
