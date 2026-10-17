package concurrency

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSafeCounter(t *testing.T) {
	var c SafeCounter
	items := make([]int, 1000)
	FanOut(items, func(int) { c.Inc() })
	if c.Value() != 1000 {
		t.Errorf("got %d", c.Value())
	}
}

func TestPipeline(t *testing.T) {
	if got := Pipeline([]int64{100, 200, 300}); got != 600 {
		t.Errorf("got %d", got)
	}
}

func TestWorkerPoolKeepsOrder(t *testing.T) {
	got := WorkerPool([]int{1, 2, 3, 4, 5}, 3, func(x int) int { return x * x })
	want := []int{1, 4, 9, 16, 25}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}

func TestCallWithTimeout(t *testing.T) {
	fast := func(context.Context) (string, error) { return "ok", nil }
	slow := func(ctx context.Context) (string, error) {
		select {
		case <-time.After(time.Second):
			return "late", nil
		case <-ctx.Done(): // well-behaved code stops when the context is cancelled
			return "", ctx.Err()
		}
	}

	if v, err := CallWithTimeout(context.Background(), 100*time.Millisecond, fast); v != "ok" || err != nil {
		t.Errorf("fast: %q %v", v, err)
	}
	if _, err := CallWithTimeout(context.Background(), 10*time.Millisecond, slow); !errors.Is(err, ErrTimeout) {
		t.Errorf("slow: %v", err)
	}
}

func TestFetchAllCancelsOnError(t *testing.T) {
	boom := errors.New("boom")
	var started atomic.Int32
	_, err := FetchAll(context.Background(), []string{"a", "b", "c", "d", "e", "f"},
		func(ctx context.Context, id string) (int64, error) {
			started.Add(1)
			if id == "a" {
				return 0, boom
			}
			<-ctx.Done() // the others wait until the group cancels them
			return 0, ctx.Err()
		})
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
}

func TestRateLimiter(t *testing.T) {
	t.Skip("bài tập: implement RateLimiter then delete this line")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rl := NewRateLimiter(ctx, 2, time.Hour)
	first, second, third := rl.Allow(), rl.Allow(), rl.Allow()
	if !first || !second || third {
		t.Error("want exactly 2 allowed calls")
	}
}
