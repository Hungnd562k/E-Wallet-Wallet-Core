// Package bank: chapter 04 of docs/go-map. Visibility is decided by the first
// letter: Upper = exported (public), lower = unexported (package-private).
// There is no "protected" and no "internal" keyword; the internal/ directory
// rule plays that role between packages.
package bank

import "errors"

// MinOpeningBalance is exported: other packages can read bank.MinOpeningBalance.
const MinOpeningBalance = 50_000

var errTooLow = errors.New("opening balance too low") // unexported: only this package sees it

// Account is exported but its balance field is not, so other packages can only
// change it through methods. This is how Go does encapsulation.
type Account struct {
	Owner   string // exported field
	balance int64  // unexported field
}

// Open is the constructor convention: a plain function named New…/Open… that validates.
func Open(owner string, initial int64) (*Account, error) {
	if initial < MinOpeningBalance {
		return nil, errTooLow
	}
	return &Account{Owner: owner, balance: initial}, nil
}

func (a *Account) Balance() int64 { return a.balance }

// ErrTooLow lets callers check the error without exposing the variable itself for reassignment.
func ErrTooLow() error { return errTooLow }
