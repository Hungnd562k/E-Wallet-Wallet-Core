package interfaces

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fixedClock is a hand-written fake. Go projects rarely need a mocking library.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

var noon = fixedClock{time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}

// Compile-time checks that the types satisfy the interface.
var (
	_ Notifier = (*SMS)(nil)
	_ Notifier = NotifierFunc(nil)
	_ Notifier = (*Audited)(nil)
)

func TestTransferNotifies(t *testing.T) {
	sms := &SMS{}
	svc := NewTransferService(sms, noon)

	receipt, err := svc.Transfer(context.Background(), "bob", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(receipt, "12:00:00") || len(sms.Sent) != 1 {
		t.Errorf("receipt=%q sent=%v", receipt, sms.Sent)
	}
}

func TestTransferWrapsNotifierError(t *testing.T) {
	boom := errors.New("sms gateway down")
	failing := NotifierFunc(func(context.Context, string, string) error { return boom })

	_, err := NewTransferService(failing, noon).Transfer(context.Background(), "bob", 500)
	if !errors.Is(err, boom) {
		t.Fatalf("want wrapped %v, got %v", boom, err)
	}
}

func TestEmbeddingDecorates(t *testing.T) {
	sms := &SMS{}
	audited := &Audited{Notifier: sms}
	_, _ = NewTransferService(audited, noon).Transfer(context.Background(), "bob", 1)
	if len(audited.Log) != 1 || len(sms.Sent) != 1 {
		t.Errorf("log=%v sent=%v", audited.Log, sms.Sent)
	}
}

func TestCounterClosure(t *testing.T) {
	next := Counter()
	next()
	next()
	if got := next(); got != 3 {
		t.Errorf("got %d", got)
	}
	if other := Counter(); other() != 1 {
		t.Error("each Counter has its own n")
	}
}

func TestMasker(t *testing.T) {
	// t.Skip("bài tập: implement Masker then delete this line")
	if got := Masker()("call +84901234567 now"); got != "call +8490****567 now" {
		t.Errorf("got %q", got)
	}
}
