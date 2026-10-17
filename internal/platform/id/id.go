// Package id generates UUIDv7 identifiers and formats them as prefixed public IDs
// (architecture §9): the DB stores a bare UUID, the API shows "txn_…", "wal_…".
package id

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Prefix is the public-ID prefix of a resource type.
type Prefix string

const (
	Payment  Prefix = "txn"
	Wallet   Prefix = "wal"
	Customer Prefix = "cus"
)

// New returns a new time-ordered UUIDv7. It panics only if the system's
// random source fails, which is unrecoverable anyway.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Format renders u as "<prefix>_<uuid without dashes>".
func Format(p Prefix, u uuid.UUID) string {
	return string(p) + "_" + strings.ReplaceAll(u.String(), "-", "")
}

// Parse accepts either "<prefix>_<hex>" or a plain UUID string.
func Parse(p Prefix, s string) (uuid.UUID, error) {
	if rest, ok := strings.CutPrefix(s, string(p)+"_"); ok {
		s = rest
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s id %q: %w", p, s, err)
	}
	return u, nil
}
