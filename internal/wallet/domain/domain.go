// Package domain holds the wallet lifecycle rules (erd §6, BR-05/07/61–63).
package domain

// Status is the lifecycle state of a wallet.
type Status string

const (
	StatusInactive  Status = "INACTIVE"
	StatusActive    Status = "ACTIVE"
	StatusSuspended Status = "SUSPENDED"
	StatusFrozen    Status = "FROZEN"
	StatusClosed    Status = "CLOSED"
)

// transitions lists the allowed status changes. A map of sets reads like the table in erd §6.
var transitions = map[Status][]Status{
	StatusInactive:  {StatusActive},
	StatusActive:    {StatusSuspended, StatusFrozen, StatusClosed},
	StatusSuspended: {StatusActive, StatusFrozen, StatusClosed},
	StatusFrozen:    {StatusActive},
}

// CanTransition reports whether a wallet may move from s to to.
func (s Status) CanTransition(to Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}

// CanTransact reports whether money may move in or out of a wallet in status s.
func (s Status) CanTransact() bool { return s == StatusActive }
