// Package authz decides the authentication level a transaction needs (PIN, or
// OTP + face) and manages one-time step-up challenges bound to a transaction
// intent. Schema: authz. BR-35…40.
package authz

import (
	"context"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO: implement. See ../E-Wallet/docs/architecture.md §4.1 and erd.md for the schema.

type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
	i     *identity.Service
}

func NewService(pool *pgxpool.Pool, clk clock.Clock, i *identity.Service) *Service {
	return &Service{pool: pool, clock: clk}
}

// Create PIN :D
func (s *Service) Create_PIN(ctx context.Context, customerId uuid.UUID, pin string) error {
	return nil
}

// Update PIN :D
func (s *Service) Update_PIN(ctx context.Context, customerId uuid.UUID, currPIN string, newPIN string) error {
	return nil
}

// Authenticate PIN :D
func (s *Service) Authz_PIN(ctx context.Context, customerId uuid.UUID, pin string) error {
	return nil
}
