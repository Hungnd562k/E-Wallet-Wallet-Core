package domain

import (
	"github.com/google/uuid"
)

// Entry is what a caller knows about an audited action. request_id, trace_id,
// ip and user_agent are not here: Record reads them from the context, so a
// caller can neither forget nor fake them.
type Entry struct {
	ActorType    string         `json:"actor_type"`
	ActorId      uuid.UUID      `json:"actor_id"` // uuid.Nil is stored as NULL (SYSTEM, unauthenticated)
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceId   string         `json:"resource_id"`
	OutCome      string         `json:"outcome"`
	Reason       string         `json:"reason"` // "" is stored as NULL; required for STAFF
	Metadata     map[string]any `json:"metadata"`
}

type EntryError string

func (e EntryError) Error() string {
	return string(e)
}

const (
	ErrNotInTx                EntryError = "audit: Record must run inside db.WithTx"
	UnAllowedEmptyAction      EntryError = "action could not be empty"
	UnAllowedEmtpyResouceType EntryError = "resource_type could not be empty"
	UnAllowedEmptyResourceId  EntryError = "resource_id could not be empty"
	InvalidActorType          EntryError = "actor_type is invalid"
	InvalidOutcome            EntryError = "outcome is invalid"
	StaffRequireReason        EntryError = "require reason for staff"
)

type Action string

const (
	// Lowercase and dotted, like the examples in erd §4.12 (wallet.freeze, kyc.image.view).
	Payment_Transfer     Action = "payment.transfer"
	Wallet_Status_Change Action = "wallet.status.change"
)

type ActorType string

const (
	Customer ActorType = "CUSTOMER"
	Staff    ActorType = "STAFF"
	Merchant ActorType = "MERCHANT"
	System   ActorType = "SYSTEM"
)

type Outcome string

const (
	Success Outcome = "SUCCESS"
	Denied  Outcome = "DENIED"
	Error   Outcome = "ERROR"
)

// TODO: this function must return a map[string]error.
// Must const-ize returned errors.
func (entry Entry) Validate() error {
	if entry.Action == "" {
		return UnAllowedEmptyAction
	}

	// Resource's values could not be empty
	if entry.ResourceType == "" {
		return UnAllowedEmtpyResouceType
	}

	if entry.ResourceId == "" {
		return UnAllowedEmptyResourceId
	}

	actorType := ActorType(entry.ActorType)
	if !actorType.isValidActorType() {
		return InvalidActorType
	}

	outcome := Outcome(entry.OutCome)
	if !outcome.isValidOutcome() {
		return InvalidOutcome
	}

	if actorType == Staff && entry.Reason == "" {
		return StaffRequireReason
	}

	return nil
}

// Return false if actor_type not in ...
func (a ActorType) isValidActorType() bool {
	switch a {
	case Customer, Staff, Merchant, System:
		return true
	default:
		return false
	}
}

// Return false if outcome not in ...
func (o Outcome) isValidOutcome() bool {
	switch o {
	case Success, Denied, Error:
		return true
	default:
		return false
	}
}
