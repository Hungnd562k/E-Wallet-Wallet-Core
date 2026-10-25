package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// actor_type, actor_id, action, resource_type, resource_id, outcome, reason, request_id, trace_id, ip, user_agent, metadata

// -- auto-generated definition
// create table audit_logs
// (
//     id            uuid                                         not null
//         primary key,
//     occurred_at   timestamp with time zone default now()       not null,
//     actor_type    text                                         not null
//         constraint audit_logs_actor_type_check
//             check (actor_type = ANY (ARRAY ['CUSTOMER'::text, 'STAFF'::text, 'MERCHANT'::text, 'SYSTEM'::text])),
//     actor_id      uuid,
//     action        text                                         not null,
//     resource_type text                                         not null,
//     resource_id   text                                         not null,
//     outcome       text                                         not null
//         constraint audit_logs_outcome_check
//             check (outcome = ANY (ARRAY ['SUCCESS'::text, 'DENIED'::text, 'ERROR'::text])),
//     reason        text,
//     request_id    text                                         not null,
//     trace_id      text                                         not null,
//     ip            text,
//     user_agent    text,
//     metadata      jsonb                    default '{}'::jsonb not null
//         constraint audit_logs_metadata_check
//             check (jsonb_typeof(metadata) = 'object'::text),
//     constraint audit_logs_staff_reason
//         check ((actor_type <> 'STAFF'::text) OR (reason IS NOT NULL))
// );

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
		return fmt.Errorf("action could not be empty")
	}

	// Resource's values could not be empty
	if entry.ResourceType == "" {
		return fmt.Errorf("resource_type could not be empty")
	}

	if entry.ResourceId == "" {
		return fmt.Errorf("resource_id could not be empty")
	}

	actorType := ActorType(entry.ActorType)
	if !actorType.isValidActorType() {
		return fmt.Errorf("actor_type is invalid")
	}

	outcome := Outcome(entry.OutCome)
	if !outcome.isValidOutcome() {
		return fmt.Errorf("outcome is invalid")
	}

	if actorType == Staff && entry.Reason == "" {
		return fmt.Errorf("require reason for staff")
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
