package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
)

func TestNotInTransaction(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	entry := domain.Entry{
		ActorType:    "actor_type",
		ActorId:      id.New(),
		Action:       "test",
		ResourceType: "resource_type",
		ResourceId:   "resource_id",
		OutCome:      "must_fail",
		Reason:       "because_not_in_transaction",
		RequestId:    "request_id",
		TraceId:      "trace_id",
		Ip:           "ip",
		UserAgent:    "user_agent",
		Metadata:     nil,
	}

	err := e.modules.Audit.Record(ctx, entry)
	if err != domain.ErrNotInTx {
		t.Errorf("Expect errNotInTx error code, got %v", err)
	}
}

func TestInsertInValidAuditRecord(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	tests := map[string]struct {
		entry       domain.Entry
		expectedErr domain.EntryError
	}{
		string(domain.UnAllowedEmptyAction): {
			entry: domain.Entry{
				ActorType:    string(domain.Customer),
				ActorId:      id.New(),
				Action:       "", // empty action
				ResourceType: "resource_type",
				ResourceId:   "resource_id",
				OutCome:      string(domain.Denied),
				Reason:       "because_not_in_transaction",
				RequestId:    "request_id",
				TraceId:      "trace_id",
				Ip:           "ip",
				UserAgent:    "user_agent",
				Metadata:     nil,
			},
			expectedErr: domain.UnAllowedEmptyAction,
		},
		string(domain.UnAllowedEmtpyResouceType): {
			entry: domain.Entry{
				ActorType:    string(domain.Customer),
				ActorId:      id.New(),
				Action:       "action",
				ResourceType: "", // empty resource type
				ResourceId:   "resource_id",
				OutCome:      string(domain.Denied),
				Reason:       "because_not_in_transaction",
				RequestId:    "request_id",
				TraceId:      "trace_id",
				Ip:           "ip",
				UserAgent:    "user_agent",
				Metadata:     nil,
			},
			expectedErr: domain.UnAllowedEmtpyResouceType,
		},
		string(domain.UnAllowedEmptyResourceId): {
			entry: domain.Entry{
				ActorType:    "actor_type",
				ActorId:      id.New(),
				Action:       "action",
				ResourceType: "resource_type",
				ResourceId:   "", // empty resource_id
				OutCome:      "must_fail",
				Reason:       "because_not_in_transaction",
				RequestId:    "request_id",
				TraceId:      "trace_id",
				Ip:           "ip",
				UserAgent:    "user_agent",
				Metadata:     nil,
			},
			expectedErr: domain.UnAllowedEmptyResourceId,
		},
		string(domain.InvalidActorType): {
			entry: domain.Entry{
				ActorType:    "invalid_actor_type", // invalid actor type
				ActorId:      id.New(),
				Action:       "action",
				ResourceType: "resource_type",
				ResourceId:   "resource_type",
				OutCome:      "must_fail",
				Reason:       "because_not_in_transaction",
				RequestId:    "request_id",
				TraceId:      "trace_id",
				Ip:           "ip",
				UserAgent:    "user_agent",
				Metadata:     nil,
			},
			expectedErr: domain.InvalidActorType,
		},
		string(domain.InvalidOutcome): {
			entry: domain.Entry{
				ActorType:    string(domain.Customer),
				ActorId:      id.New(),
				Action:       "action",
				ResourceType: "resource_type",
				ResourceId:   "resource_type",
				OutCome:      "invalid_outcome",
				Reason:       "because_not_in_transaction",
				RequestId:    "request_id",
				TraceId:      "trace_id",
				Ip:           "ip",
				UserAgent:    "user_agent",
				Metadata:     nil,
			},
			expectedErr: domain.InvalidOutcome,
		},
		string(domain.StaffRequireReason): {
			entry: domain.Entry{
				ActorType:    string(domain.Staff),
				ActorId:      id.New(),
				Action:       "action",
				ResourceType: "resource_type",
				ResourceId:   "resource_id",
				OutCome:      string(domain.Success),
				Reason:       "", // empty reason for staff
				RequestId:    "request_id",
				TraceId:      "trace_id",
				Ip:           "ip",
				UserAgent:    "user_agent",
				Metadata:     nil,
			},
			expectedErr: domain.StaffRequireReason,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := db.WithTx(ctx, pool, func(ctx context.Context) error {
				return e.modules.Audit.Record(ctx, test.entry)
			})

			if !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error %v, got %v", test.expectedErr, err)
			}
		})
	}
}

func TestInsertValidAuditRecord(t *testing.T) {

}
