// Package types: chapter 02 of docs/go-map. Value types, pointers, slices,
// maps, enums and generics.
package types

import "fmt"

// VND is a defined type: same representation as int64 but a distinct type,
// so you cannot accidentally add a VND to a plain int64 without converting.
type VND int64

func (v VND) String() string { return fmt.Sprintf("%d VND", int64(v)) } // implements fmt.Stringer

// Status is an "enum": Go has none, so use a defined type plus constants.
type Status int

const (
	Active Status = iota // iota counts 0, 1, 2… inside a const block
	Suspended
	Frozen
	Closed
)

func (s Status) String() string {
	switch s {
	case Active:
		return "ACTIVE"
	case Suspended:
		return "SUSPENDED"
	case Frozen:
		return "FROZEN"
	case Closed:
		return "CLOSED"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// Wallet is a struct: a value type, copied on assignment (like a C# struct, not a class).
type Wallet struct {
	ID      string
	Balance VND
	Status  Status
	Tags    []string // a slice is a small header pointing to a shared array
}

// DepositByValue receives a copy: the caller's wallet is unchanged.
func DepositByValue(w Wallet, amount VND) { w.Balance += amount }

// DepositByPointer receives the address: the caller's wallet changes.
func DepositByPointer(w *Wallet, amount VND) { w.Balance += amount } // w.Balance auto-dereferences

// Freeze has a pointer receiver because it mutates the wallet.
func (w *Wallet) Freeze() { w.Status = Frozen }

// IsActive has a value receiver: it only reads.
func (w Wallet) IsActive() bool { return w.Status == Active }

// Number is a constraint: any type whose underlying type is int64 or int.
type Number interface {
	~int64 | ~int
}

// Sum is generic: it works for []VND, []int64, []int…
func Sum[T Number](xs []T) T {
	var total T // zero value of T
	for _, x := range xs {
		total += x
	}
	return total
}

// GroupByStatus shows map[K]V. Reading a missing key returns the zero value;
// appending to a nil slice allocates, so no initialisation is needed.
func GroupByStatus(ws []Wallet) map[Status][]string {
	out := make(map[Status][]string)
	for _, w := range ws {
		out[w.Status] = append(out[w.Status], w.ID)
	}
	return out
}

// Filter returns the wallets with at least min balance.
// Exercise (docs/go-map/02-kieu-du-lieu.md): implement it generically as
// Filter[T any](xs []T, keep func(T) bool) []T and use it here.
func Filter(ws []Wallet, min VND) []Wallet {
	// TODO(bài tập)
	// Ví dụ:
	// make([]Wallet, 2) -> mảng có sẵn 2 phần tử, len 2, cap 2, sẵn 2 phần tử default value
	// make([]Wallet, 0) -> len 0, cap 0
	// make([]Wallet, 0, 2) -> len 0, cap 2, ko có sẵn 2 phần tử default value
	out := make([]Wallet, 0, len(ws)+1)
	for _, w := range ws {
		if w.Balance < min {
			continue
		}
		out = append(out, w)
	}
	return out
}

// Filter but for generic type
func FilterGeneric[T any](xs []T, keep func(T) bool) []T {
	out := make([]T, 0, len(xs)-1)
	for _, v := range xs {
		if !keep(v) {
			continue
		}
		out = append(out, v)
	}
	return out
}
