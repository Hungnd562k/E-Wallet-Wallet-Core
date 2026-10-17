// Package concurrency: chapter 06 of docs/go-map. Goroutines, channels,
// select, sync, context and errgroup. Run its tests with -race.
package concurrency

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// SafeCounter guards shared state with a mutex. Without it, concurrent
// increments race (go test -race reports it).
type SafeCounter struct {
	mu sync.Mutex // zero value is an unlocked mutex
	n  int
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock() // defer runs when the function returns, even on panic
	c.n++
}

func (c *SafeCounter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// FanOut runs f for every item in its own goroutine and waits for all of them.
// `go` starts a goroutine (much cheaper than a thread; closer to Task.Run).
func FanOut(items []int, f func(int)) {
	var wg sync.WaitGroup
	for _, it := range items {
		wg.Add(1)
		go func() { // since Go 1.22 each iteration has its own `it`
			defer wg.Done()
			f(it)
		}()
	}
	wg.Wait()
}

// Pipeline: a producer sends on a channel, a consumer ranges over it until it is closed.
// "Do not communicate by sharing memory; share memory by communicating."
func Pipeline(amounts []int64) int64 {
	ch := make(chan int64) // unbuffered: each send waits for a receive

	go func() {
		defer close(ch) // only the sender closes
		for _, a := range amounts {
			ch <- a
		}
	}()

	var total int64
	for a := range ch { // ends when ch is closed
		total += a
	}
	return total
}

// WorkerPool processes jobs with n workers and returns results in input order.
func WorkerPool(jobs []int, n int, work func(int) int) []int {
	type job struct{ idx, val int }
	in := make(chan job)
	results := make([]int, len(jobs)) // each worker writes a distinct index: no lock needed

	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				results[j.idx] = work(j.val)
			}
		}()
	}
	for i, v := range jobs {
		in <- job{i, v}
	}
	close(in)
	wg.Wait()
	return results
}

var ErrTimeout = errors.New("bank call timed out")

// CallWithTimeout waits for slow() or gives up after d, whichever comes first.
// context.Context is Go's CancellationToken, passed explicitly as the first argument.
func CallWithTimeout(ctx context.Context, d time.Duration, slow func(context.Context) (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel() // always release the timer

	type result struct {
		val string
		err error
	}
	done := make(chan result, 1) // buffered so the goroutine never blocks forever if we time out
	go func() {
		v, err := slow(ctx)
		done <- result{v, err}
	}()

	select { // waits on several channel operations at once
	case r := <-done:
		return r.val, r.err
	case <-ctx.Done():
		return "", ErrTimeout
	}
}

// FetchAll calls fetch for every id in parallel (at most 3 at a time). The
// first error cancels the shared context so the others can stop early.
// This is the pattern cmd/worker uses for its long-running components.
func FetchAll(ctx context.Context, ids []string, fetch func(context.Context, string) (int64, error)) ([]int64, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(3)
	out := make([]int64, len(ids))
	for i, id := range ids {
		g.Go(func() error {
			v, err := fetch(ctx, id)
			if err != nil {
				return err
			}
			out[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// RateLimiter allows at most n calls per interval.
// Exercise (docs/go-map/06-concurrency.md): implement with a buffered channel
// as a token bucket refilled by a time.Ticker goroutine that stops on ctx.Done().
type RateLimiter struct{}

func NewRateLimiter(ctx context.Context, n int, interval time.Duration) *RateLimiter {
	// TODO(bài tập)
	return &RateLimiter{}
}

// Allow reports whether a call may proceed now (non-blocking).
func (r *RateLimiter) Allow() bool {
	// TODO(bài tập)
	return true
}
