// Command api runs wallet-api, the REST API for mobile, back-office and merchants.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/app"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/auth"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/config"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
)

const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(observability.NewLogger(os.Stdout, cfg.LogLevel))

	// ctx is cancelled on Ctrl+C or SIGTERM (docker stop), which starts graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Only local/test environments exist, so the dev header authenticator is used.
	// Replace with the identity module's JWT authenticator when it lands.
	slog.Warn("using DevHeaderAuth: requests are authenticated by the " + auth.HeaderDevCustomerID + " header")
	modules := app.NewModules(pool, clock.Real{})
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.NewAPIHandler(pool, modules, auth.DevHeaderAuth{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("wallet-api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down wallet-api")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
