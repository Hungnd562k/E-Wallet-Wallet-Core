// The _test suffix makes this an external ("black-box") test package: it sees
// bank exactly like any other importer would, so only exported names compile.
package bank_test

import (
	"errors"
	"testing"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/learn/04_packages/bank"
)

func TestOpen(t *testing.T) {
	acc, err := bank.Open("alice", bank.MinOpeningBalance)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Balance() != bank.MinOpeningBalance {
		t.Errorf("balance = %d", acc.Balance())
	}
	// acc.balance = 1 // ← does not compile: balance is unexported.

	if _, err := bank.Open("bob", 1); !errors.Is(err, bank.ErrTooLow()) {
		t.Errorf("want ErrTooLow, got %v", err)
	}
}
