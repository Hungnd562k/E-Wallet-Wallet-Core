package outbox

import (
	"math/rand/v2"
	"time"
)

const (
	backoffBase = time.Second
	backoffMax  = 5 * time.Minute
)

// Backoff returns the delay before retry number attempt (1-based):
// exponential (1s, 2s, 4s, …) capped at 5 minutes, with ±20% jitter so that
// many failing events do not retry in lockstep.
func Backoff(attempt int32) time.Duration {
	return backoffWithJitter(attempt, rand.Float64())
}

// backoffWithJitter is the deterministic core of Backoff; r is in [0,1).
func backoffWithJitter(attempt int32, r float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := backoffMax
	if attempt <= 20 { // 2^19 s already exceeds the cap; avoid overflow
		if exp := backoffBase << (attempt - 1); exp < backoffMax {
			d = exp
		}
	}
	jitter := 0.8 + 0.4*r // [0.8, 1.2)
	return time.Duration(float64(d) * jitter)
}
