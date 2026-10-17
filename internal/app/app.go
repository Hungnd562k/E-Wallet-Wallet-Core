// Package app is the composition root: it constructs every module and wires
// them together (dependency injection by hand, no container). cmd/* binaries
// and integration tests both build the application through here.
//
// Only this package and cmd/* may import module sub-packages such as
// payment/http; modules talk to each other through their service.go.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/notification"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/payment"
	paymenthttp "github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/payment/http"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/auth"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/httpx"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/idempotency"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet"
)

// Modules holds the constructed module services.
type Modules struct {
	Ledger       *ledger.Service
	Wallet       *wallet.Service
	Payment      *payment.Service
	Notification *notification.Service
}

func NewModules(pool *pgxpool.Pool, clk clock.Clock) *Modules {
	l := ledger.NewService(pool, clk)
	w := wallet.NewService(pool, l)
	return &Modules{
		Ledger:       l,
		Wallet:       w,
		Payment:      payment.NewService(pool, clk, l, w, idempotency.NewStore(pool, clk), outbox.NewWriter(pool, clk)),
		Notification: notification.NewService(),
	}
}

// NewAPIHandler builds the HTTP handler of wallet-api.
func NewAPIHandler(pool *pgxpool.Pool, m *Modules, authn auth.Authenticator) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.Recover, httpx.AccessLog)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			slog.WarnContext(ctx, "readiness check failed", "err", err)
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "db": "down"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authn))
		paymenthttp.NewHandler(m.Payment).Routes(r)
	})
	return r
}

// NewDispatcher builds the outbox dispatcher of wallet-worker with every consumer registered.
func NewDispatcher(pool *pgxpool.Pool, clk clock.Clock, m *Modules, cfg outbox.DispatcherConfig) *outbox.Dispatcher {
	d := outbox.NewDispatcher(pool, clk, cfg)
	d.Register(payment.EventPaymentSucceeded, notification.ConsumerName, m.Notification.HandlePaymentSucceeded)
	return d
}
