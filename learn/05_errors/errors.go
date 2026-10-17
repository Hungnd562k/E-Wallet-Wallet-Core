// Package errs: chapter 05 of docs/go-map. Errors are values returned like any
// other value. Wrap with %w to add context, inspect with errors.Is / errors.As.
package errs

import (
	"errors"
	"fmt"
)

// Sentinel errors: package-level values compared with errors.Is.
var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrWalletFrozen      = errors.New("wallet frozen")
)

// LimitError is a custom error type carrying data; inspect it with errors.As.
type LimitError struct {
	Limit, Attempted int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("limit %d exceeded by attempt of %d", e.Limit, e.Attempted)
}

type Wallet struct {
	Balance int64
	Frozen  bool
	Limit   int64
}

// Withdraw returns nil on success. Note the order of checks and the early returns.
func (w *Wallet) Withdraw(amount int64) error {
	if w.Frozen {
		return ErrWalletFrozen
	}
	if w.Limit > 0 && amount > w.Limit {
		return &LimitError{Limit: w.Limit, Attempted: amount}
	}
	if amount > w.Balance {
		// %w wraps: the message gains context, errors.Is still finds the sentinel.
		return fmt.Errorf("withdraw %d from balance %d: %w", amount, w.Balance, ErrInsufficientFunds)
	}
	w.Balance -= amount
	return nil
}

// HTTPStatus maps errors to status codes the way internal/platform/httpx does.
func HTTPStatus(err error) int {
	var limitErr *LimitError
	switch {
	case err == nil:
		return 200
	case errors.Is(err, ErrInsufficientFunds):
		return 422
	case errors.Is(err, ErrWalletFrozen):
		return 403
	case errors.As(err, &limitErr): // As finds the *LimitError in the chain and assigns it
		return 422
	default:
		return 500
	}
}

// ValidateAll collects every problem instead of stopping at the first one.
func ValidateAll(amounts []int64) error {
	var errs []error
	for i, a := range amounts {
		if a <= 0 {
			errs = append(errs, fmt.Errorf("amounts[%d]: must be positive", i))
		}
	}
	return errors.Join(errs...) // nil when errs is empty
}

// MustPositive panics: use panic only for programmer bugs or impossible states,
// never for business errors. Must… is the naming convention.
func MustPositive(a int64) int64 {
	if a <= 0 {
		panic(fmt.Sprintf("MustPositive: %d", a))
	}
	return a
}

// SafeCall converts a panic into an error with recover; recover only works inside a deferred function.
func SafeCall(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	f()
	return nil
}

// TransferAll withdraws each amount and returns all failures joined, wrapping
// each with its index. Exercise (docs/go-map/05-error-handling.md).
func TransferAll(w *Wallet, amounts []int64) error {
	// TODO(bài tập)
	return nil
}
