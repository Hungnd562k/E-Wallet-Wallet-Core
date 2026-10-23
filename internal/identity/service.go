// Package identity handles phone registration with OTP, login, JWT (Ed25519)
// and refresh-token rotation, devices, PIN (argon2id) and data-processing
// consents. Schema: identity. BR-01…03, 38, 39, 70.
package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity/store"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/date"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO: implement. See ../E-Wallet/docs/architecture.md §4.1 and erd.md for the schema.

var (
	ErrNotFound           = apperr.New(apperr.KindNotFound, "CUSTOMER_NOT_FOUND", "Could not found customer's information")
	ErrNotActive          = apperr.New(apperr.KindInvalid, "UNVERIFIED_CUSTOMER", "Customer has not been verified yet")
	ErrInvalidPhoneNumber = apperr.New(apperr.KindInvalid, "INVALID_PHONE_NUMBER", "Invalid phone number")
)

// CustomerCommand is the INPUT command of create or update customer
type CustomerCommand struct {
	Id       uuid.UUID
	Phone    string
	FullName string
	Dob      time.Time
}

// Customer is the OUTPUT of query customer
type Customer struct {
	Id       uuid.UUID
	Phone    string
	FullName string
	Dob      time.Time
}

type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, clock: clk}
}

// queries returns sqlc queries bound to the transaction in ctx, or to the pool.
func (s *Service) queries(ctx context.Context) *store.Queries {
	return store.New(db.Conn(ctx, s.pool))
}

// TODO: need test.
// Get Customer by phone number
func (s *Service) GetCustomerByPhone(ctx context.Context, phone string) (Customer, error) {
	c, err := s.queries(ctx).GetCustomerByPhone(ctx, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	}
	if c.OnboardingStatus != string(domain.OnboardingRegistered) {
		return Customer{}, ErrNotActive
	}

	return toCustomer(c), nil
}

// TODO: need test.
// Create new customer
func (s *Service) Create(ctx context.Context, customerCommand CustomerCommand) error {
	phone := domain.NormalizePhone(customerCommand.Phone)
	if phone == "" {
		return ErrInvalidPhoneNumber
	}
	now := s.clock.Now()
	err := s.queries(ctx).InsertCustomer(ctx, store.InsertCustomerParams{
		ID:               id.New(),
		Phone:            customerCommand.Phone,
		PhoneVerifiedAt:  now,
		OnboardingStatus: "",
		FullName:         &customerCommand.FullName,
		DateOfBirth:      date.TimeToPgTypeDate(customerCommand.Dob),
		CreatedAt:        now,
		UpdatedAt:        now,
		Version:          1,
	})
	if err != nil {
		return fmt.Errorf("InsertCustomer failed: %s", err)
	}

	return nil
}

func toCustomer(r store.IdentityCustomer) Customer {
	return Customer{
		Id:       r.ID,
		Phone:    r.Phone,
		FullName: *r.FullName,
		Dob:      r.DateOfBirth.Time,
	}
}
