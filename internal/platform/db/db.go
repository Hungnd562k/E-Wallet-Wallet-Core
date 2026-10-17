// Package db owns the Postgres pool and the transaction-in-context pattern
// that lets several modules write inside one ACID transaction (architecture §5.1).
//
// Repositories never hold a *pgxpool.Pool directly. They call db.Conn(ctx, pool),
// which returns the pgx.Tx stored in ctx by WithTx, or the pool when there is none.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is satisfied by *pgxpool.Pool, *pgx.Conn and pgx.Tx, and is the same
// interface sqlc-generated code accepts.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

type txKey struct{}

// WithTx runs fn inside a transaction. The transaction is stored in the ctx
// passed to fn so that every repository reached from fn joins it.
// If a transaction is already in ctx, fn simply joins it (no nesting).
// Returning an error from fn, or panicking, rolls everything back.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// Conn returns the transaction stored in ctx, or fallback when there is none.
func Conn(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return fallback
}

// InTx reports whether ctx carries a transaction. Modules whose methods must
// only run inside one (e.g. ledger.Post) use it as a guard.
func InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(pgx.Tx)
	return ok
}

// Postgres error codes we react to. See https://www.postgresql.org/docs/16/errcodes-appendix.html
const (
	CodeUniqueViolation = "23505"
	CodeCheckViolation  = "23514"
	CodeRaiseException  = "P0001"
)

// IsCode reports whether err (or anything it wraps) is a Postgres error with the given SQLSTATE.
func IsCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

// IsConstraint reports whether err is a violation of the named constraint.
func IsConstraint(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == name
}
