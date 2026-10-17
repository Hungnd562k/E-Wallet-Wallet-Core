// Command worker runs wallet-worker: the outbox dispatcher today, and later the
// bank-operation runner, merchant callbacks and scheduled jobs (architecture §6).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/app"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/config"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(observability.NewLogger(os.Stdout, cfg.LogLevel).With("worker_id", cfg.WorkerID))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	clk := clock.Real{}
	dispatcher := app.NewDispatcher(pool, clk, app.NewModules(pool, clk), outbox.DispatcherConfig{
		WorkerID:     cfg.WorkerID,
		BatchSize:    cfg.OutboxBatchSize,
		PollInterval: cfg.OutboxPollInterval,
		MaxAttempts:  int32(cfg.OutboxMaxAttempts),
	})

	// Each long-running component is one goroutine in the group. When ctx is
	// cancelled they all return and Wait unblocks. Add the bank runner and the
	// scheduler here as further g.Go calls.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return dispatcher.Run(ctx) })

	slog.Info("wallet-worker started", "env", cfg.Env)
	err = g.Wait()
	slog.Info("wallet-worker stopped")
	return err
}
