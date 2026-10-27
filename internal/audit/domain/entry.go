package domain

import (
	"github.com/google/uuid"
)

type Entry struct {
	ActorType    string         `json:"actor_type"`
	ActorId      uuid.UUID      `json:"actor_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceId   string         `json:"resource_id"`
	OutCome      string         `json:"outcome"`
	Reason       string         `json:"reason"`
	RequestId    string         `json:"request_id"`
	TraceId      string         `json:"trace_id"`
	Ip           string         `json:"ip"`
	UserAgent    string         `json:"user_agent"`
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
	Payment_Transfer     Action = "Payment.Transfer"
	Wallet_Status_Change Action = "Wallet.Status.Change"
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
