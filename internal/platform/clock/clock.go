// Package clock abstracts time so tests can control it.
// Production code depends on Clock, never on time.Now directly.
package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

// VNLocation is used for business dates and monthly limits (BR-27).
var VNLocation = mustLoad("Asia/Ho_Chi_Minh")

// Real is the system clock, in UTC and truncated to microseconds: Postgres
// timestamptz stores microseconds, so a value read back equals the one written.
type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// Fake is a manually advanced clock for tests. Safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// BusinessDate returns the calendar date of t in Vietnam time.
func BusinessDate(t time.Time) time.Time {
	y, m, d := t.In(VNLocation).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// Fall back to a fixed +07:00 offset when tzdata is unavailable (e.g. scratch images).
		return time.FixedZone(name, 7*60*60)
	}
	return loc
}
