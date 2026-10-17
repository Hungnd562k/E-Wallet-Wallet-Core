// Package integration tests the application against a real Postgres started
// with testcontainers. Run with `make test-integration` (needs Docker);
// `go test -short` skips these tests.
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/migrations"
)

// pool is shared by all tests in this package. Tests isolate themselves by
// creating their own customers and wallets with fresh ids.
var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	// testing.Short() needs flags parsed, which m.Run would do later.
	if testingShort() {
		fmt.Println("skipping integration tests in -short mode")
		os.Exit(0)
	}
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("wallet"),
		tcpostgres.WithUsername("wallet"),
		tcpostgres.WithPassword("wallet"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		return 1
	}
	if err := migrations.Up(url); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if pool, err = db.NewPool(ctx, url); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pool.Close()

	return m.Run()
}

func testingShort() bool {
	for _, a := range os.Args[1:] {
		if a == "-test.short" || a == "-test.short=true" {
			return true
		}
	}
	return false
}
