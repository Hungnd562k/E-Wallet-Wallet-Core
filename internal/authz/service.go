// Package authz decides the authentication level a transaction needs (PIN, or
// OTP + face) and manages one-time step-up challenges bound to a transaction
// intent. Schema: authz. BR-35…40.
package authz

import (
	"context"
	"regexp"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/authz/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity/store"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
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

type PIN string

var (
	ErrInvalidPin = apperr.New(apperr.KindForbidden, "INVALID_PIN", "Invalid PIN")
	ErrWrongPin   = apperr.New(apperr.KindInvalid, "WRONG_PIN", "Wrong PIN")
)

func NewService(pool *pgxpool.Pool, clk clock.Clock, i *identity.Service) *Service {
	return &Service{pool: pool, clock: clk, i: i}
}

// Create PIN :D
func (s *Service) Create_PIN(ctx context.Context, customerId uuid.UUID, pin PIN) error {
	if !pin.isValidPin() {
		return ErrInvalidPin
	}
	pinHash, err := domain.Hash_PIN(string(pin))
	if err != nil {
		return err
	}
	now := s.clock.Now()
	err = s.i.CreateCredentials(ctx, store.IdentityCredential{
		CustomerID:     customerId,
		PinHash:        &pinHash,
		PinFailedCount: 0,
		PinLockedUntil: nil,
		PinUpdatedAt:   &now,
		PinHardLocked:  false,
		Version:        1,
	})

	if err != nil {
		return err
	}

	return nil
}

// Update PIN :D
func (s *Service) Update_PIN(ctx context.Context, customerId uuid.UUID, currPIN PIN, newPIN PIN) error {
	return nil
}

// Authenticate PIN :D
func (s *Service) Authz_PIN(ctx context.Context, customerId uuid.UUID, pin PIN) error {
	c, err := s.i.GetCredentialByCusId(ctx, customerId)
	if err != nil {
		return err
	}

	err = domain.Verify(string(pin), *c.PinHash)
	// CATCH UP: làm tiếp bắt đầu từ đây, đang dở dang đoạn viết sql update credential khi nhập sai pin, nếu nhập đúng pin thì phải reset
	if err != nil {
		// if errors.Is(err, domain.ErrInvalidPin) {

		// }
		return err
	}
	return nil
}

// PIN regex format string
var pinRegex = regexp.MustCompile(`^[0-9]{6}$`)

// PIN phải là chuỗi dài 6 ký tự, chỉ bao gồm số
func (pin PIN) isValidPin() bool {
	return pinRegex.MatchString(string(pin))
}
