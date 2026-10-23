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

type OnboardingStatus = domain.ONBOARDING

const (
	OnboardingRegistered = domain.OnboardingRegistered
	OnboardingPending    = domain.OnboardingPending
	OnboardingVerified   = domain.OnboardingVerified
)

// CustomerCommand is the INPUT command of create or update customer.
// A zero Id gets a new UUIDv7; an empty OnboardingStatus means REGISTERED.
type CustomerCommand struct {
	Id               uuid.UUID
	Phone            string
	FullName         string
	Dob              time.Time
	OnboardingStatus OnboardingStatus
}

// Customer is the OUTPUT of query customer
type Customer struct {
	Id               uuid.UUID
	Phone            string
	FullName         string
	Dob              time.Time
	OnboardingStatus OnboardingStatus
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
// GetCustomerByPhone returns the customer with this phone, whatever its
// onboarding status; callers decide which statuses they accept.
func (s *Service) GetCustomerByPhone(ctx context.Context, phone string) (Customer, error) {
	normalized := domain.NormalizePhone(phone)
	if normalized == "" {
		return Customer{}, ErrInvalidPhoneNumber
	}
	c, err := s.queries(ctx).GetCustomerByPhone(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	}
	if err != nil {
		return Customer{}, fmt.Errorf("get customer by phone: %w", err)
	}
	return toCustomer(c), nil
}

// TODO: need test.
// Create inserts a new customer. It joins the transaction in ctx, if any.
func (s *Service) Create(ctx context.Context, customerCommand CustomerCommand) error {
	phone := domain.NormalizePhone(customerCommand.Phone)
	if phone == "" {
		return ErrInvalidPhoneNumber
	}
	customerID := customerCommand.Id
	if customerID == uuid.Nil {
		customerID = id.New()
	}
	status := customerCommand.OnboardingStatus
	if status == "" {
		status = OnboardingRegistered
	}
	now := s.clock.Now()
	err := s.queries(ctx).InsertCustomer(ctx, store.InsertCustomerParams{
		ID:               customerID,
		Phone:            phone,
		PhoneVerifiedAt:  now,
		OnboardingStatus: string(status),
		FullName:         nilIfEmpty(customerCommand.FullName),
		DateOfBirth:      date.TimeToPgTypeDate(customerCommand.Dob),
		CreatedAt:        now,
		UpdatedAt:        now,
		Version:          1,
	})
	if err != nil {
		return fmt.Errorf("insert customer: %w", err)
	}

	return nil
}

func toCustomer(r store.IdentityCustomer) Customer {
	c := Customer{
		Id:               r.ID,
		Phone:            r.Phone,
		OnboardingStatus: OnboardingStatus(r.OnboardingStatus),
	}
	if r.FullName != nil {
		c.FullName = *r.FullName
	}
	if r.DateOfBirth.Valid {
		c.Dob = r.DateOfBirth.Time
	}
	return c
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
