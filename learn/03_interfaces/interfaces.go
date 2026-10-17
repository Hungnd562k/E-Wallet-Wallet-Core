// Package interfaces: chapter 03 of docs/go-map. Functions, methods,
// interfaces, embedding and dependency injection without a container.
package interfaces

import (
	"context"
	"fmt"
	"time"
)

// Notifier is defined where it is USED (here, by TransferService), not where it
// is implemented. Any type with this method satisfies it — no "implements" keyword.
type Notifier interface {
	Notify(ctx context.Context, to, message string) error
}

// Clock is the same trick the real repo uses (internal/platform/clock).
type Clock interface {
	Now() time.Time
}

// SMS is one implementation. It never mentions Notifier.
type SMS struct{ Sent []string }

func (s *SMS) Notify(_ context.Context, to, msg string) error {
	s.Sent = append(s.Sent, to+": "+msg)
	return nil
}

// NotifierFunc adapts a plain function to Notifier, like http.HandlerFunc.
type NotifierFunc func(ctx context.Context, to, msg string) error

func (f NotifierFunc) Notify(ctx context.Context, to, msg string) error { return f(ctx, to, msg) }

// TransferService receives its dependencies through the constructor.
// "Accept interfaces, return structs": fields are interfaces, the constructor returns *TransferService.
type TransferService struct {
	notifier Notifier
	clock    Clock
}

func NewTransferService(n Notifier, c Clock) *TransferService {
	return &TransferService{notifier: n, clock: c}
}

// Transfer returns two values: the result and an error. This is how Go
// signals failure — there are no exceptions.
func (s *TransferService) Transfer(ctx context.Context, to string, amount int64) (string, error) {
	if amount <= 0 {
		return "", fmt.Errorf("amount must be positive, got %d", amount)
	}
	receipt := fmt.Sprintf("%s sent %d at %s", to, amount, s.clock.Now().Format(time.TimeOnly))
	if err := s.notifier.Notify(ctx, to, receipt); err != nil {
		return "", fmt.Errorf("notify: %w", err)
	}
	return receipt, nil
}

// Audited embeds a Notifier: its Notify method is promoted, and we decorate it.
// Embedding is composition, not inheritance.
type Audited struct {
	Notifier // embedded field: no name, the type name is the field name
	Log      []string
}

func (a *Audited) Notify(ctx context.Context, to, msg string) error {
	a.Log = append(a.Log, "notify "+to)
	return a.Notifier.Notify(ctx, to, msg) // call the embedded implementation
}

// Counter returns a closure that captures n: each call increments the same variable.
func Counter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

// Masker returns a function that masks phone numbers, e.g. "+84901234567" → "+8490****567".
// Exercise (docs/go-map/03-ham-method-interface.md): implement it and wrap a
// Notifier so every message it sends has phone numbers masked.
func Masker() func(string) string {
	return func(s string) string {
		// TODO(bài tập): hint — regexp.MustCompile(`\+84\d{9}`).ReplaceAllStringFunc
		return s
	}
}
