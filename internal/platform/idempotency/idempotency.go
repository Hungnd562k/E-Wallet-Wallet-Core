// Package idempotency makes create-requests safe to retry (planning §8, erd §4.14).
//
// Usage inside the caller's db.WithTx:
//
//	stored, err := store.Begin(ctx, req) // claims the key or returns the saved response
//	if stored != nil { replay it }
//	... do the work ...
//	store.Complete(ctx, req, resp)
//
// Concurrency: Begin's INSERT ... ON CONFLICT DO NOTHING blocks while another
// open transaction holds the same key, then sees its committed outcome. Two
// parallel requests with one key therefore never both execute.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/clock"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/idempotency/idemstore"
)

const (
	ScopeCustomer = "CUSTOMER"
	ScopeMerchant = "MERCHANT"
	ScopeStaff    = "STAFF"

	retention = 7 * 24 * time.Hour
	inFlight  = 30 * time.Second
)

var ErrMissingKey = apperr.Invalid("Idempotency-Key header is required")

// Request identifies one logical operation.
type Request struct {
	ScopeType string
	ScopeID   uuid.UUID
	Key       string
	Endpoint  string
	Hash      []byte // fingerprint of the payload; see Hash
}

// Response is what gets replayed for a repeated request.
type Response struct {
	Status     int
	Body       json.RawMessage
	ResourceID *uuid.UUID
}

type Store struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewStore(pool *pgxpool.Pool, clk clock.Clock) *Store {
	return &Store{pool: pool, clock: clk}
}

// Hash fingerprints a payload. Callers pass a struct of the semantically
// relevant fields, so formatting differences in the raw body do not matter.
func Hash(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("hash payload: %w", err)
	}
	sum := sha256.Sum256(b)
	return sum[:], nil
}

// Begin claims req.Key for this request. It returns a non-nil Response when the
// key was already completed with the same payload, ErrIdempotencyKeyReused when
// the payload differs, and ErrIdempotencyInFlight when another multi-step
// request is still holding it.
func (s *Store) Begin(ctx context.Context, req Request) (*Response, error) {
	if req.Key == "" {
		return nil, ErrMissingKey
	}
	if len(req.Key) > 128 {
		return nil, apperr.Invalid("Idempotency-Key must be at most 128 characters")
	}
	if !db.InTx(ctx) {
		return nil, errors.New("idempotency: Begin must run inside db.WithTx")
	}

	q := idemstore.New(db.Conn(ctx, s.pool))
	now := s.clock.Now()
	lockedUntil := now.Add(inFlight)
	n, err := q.InsertKey(ctx, idemstore.InsertKeyParams{
		ScopeType:   req.ScopeType,
		ScopeID:     req.ScopeID,
		Key:         req.Key,
		RequestHash: req.Hash,
		Endpoint:    req.Endpoint,
		LockedUntil: &lockedUntil,
		ExpiresAt:   now.Add(retention),
	})
	if err != nil {
		return nil, fmt.Errorf("insert idempotency key: %w", err)
	}
	if n == 1 {
		return nil, nil // claimed: caller does the work
	}

	row, err := q.GetKeyForUpdate(ctx, idemstore.GetKeyForUpdateParams{
		ScopeType: req.ScopeType, ScopeID: req.ScopeID, Key: req.Key,
	})
	if err != nil {
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}
	if row.Endpoint != req.Endpoint || !bytes.Equal(row.RequestHash, req.Hash) {
		return nil, apperr.ErrIdempotencyKeyReused
	}
	if row.Status != "COMPLETED" {
		return nil, apperr.ErrIdempotencyInFlight
	}
	resp := &Response{Body: row.ResponseBody, ResourceID: row.ResourceID}
	if row.ResponseStatus != nil {
		resp.Status = int(*row.ResponseStatus)
	}
	return resp, nil
}

// Complete stores the response of a request claimed by Begin, in the same transaction.
func (s *Store) Complete(ctx context.Context, req Request, resp Response) error {
	status := int32(resp.Status)
	err := idemstore.New(db.Conn(ctx, s.pool)).CompleteKey(ctx, idemstore.CompleteKeyParams{
		ScopeType:      req.ScopeType,
		ScopeID:        req.ScopeID,
		Key:            req.Key,
		ResponseStatus: &status,
		ResponseBody:   resp.Body,
		ResourceID:     resp.ResourceID,
	})
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}

// SaveFailure stores a business-error response after the main transaction has
// rolled back, so retries get the same error instead of re-executing (erd §4.7).
// It is a single statement on the pool, outside any transaction.
func (s *Store) SaveFailure(ctx context.Context, req Request, resp Response) error {
	status := int32(resp.Status)
	err := idemstore.New(s.pool).UpsertCompletedKey(ctx, idemstore.UpsertCompletedKeyParams{
		ScopeType:      req.ScopeType,
		ScopeID:        req.ScopeID,
		Key:            req.Key,
		RequestHash:    req.Hash,
		Endpoint:       req.Endpoint,
		ResponseStatus: &status,
		ResponseBody:   resp.Body,
		ExpiresAt:      s.clock.Now().Add(retention),
	})
	if err != nil {
		return fmt.Errorf("save idempotency failure: %w", err)
	}
	return nil
}
