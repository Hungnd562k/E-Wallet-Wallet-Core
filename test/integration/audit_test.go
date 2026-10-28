package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/domain"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/db"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
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

// auditCtx is what the HTTP middlewares put in the context of a real request.
func auditCtx(requestID string) context.Context {
	ctx := observability.WithRequestID(context.Background(), requestID)
	ctx = observability.WithTraceID(ctx, "4bf92f3577b34da6a3ce929d0e0e4736")
	return observability.WithClient(ctx, observability.Client{IP: "203.0.113.7", UserAgent: "EWalletApp/1.0"})
}

type auditRow struct {
	actorID               *uuid.UUID
	reason, ip, userAgent *string
	requestID, traceID    string
	metadata              map[string]any
}

// auditRowByRequestID reads back the single audit row written for requestID.
func auditRowByRequestID(t *testing.T, requestID string) (auditRow, int) {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT actor_id, reason, ip, user_agent, request_id, trace_id, metadata
		   FROM audit.audit_logs WHERE request_id = $1`, requestID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var r auditRow
	n := 0
	for rows.Next() {
		n++
		if err := rows.Scan(&r.actorID, &r.reason, &r.ip, &r.userAgent, &r.requestID, &r.traceID, &r.metadata); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return r, n
}

func TestInsertValidAuditRecord(t *testing.T) {
	e := newEnv(t)
	requestID := "req_" + id.New().String()
	actor := id.New()

	err := db.WithTx(auditCtx(requestID), pool, func(ctx context.Context) error {
		return e.modules.Audit.Record(ctx, audit.Entry{
			ActorType:    string(audit.ActorStaff),
			ActorId:      actor,
			Action:       string(audit.ActionWalletStatusChange),
			ResourceType: "wallet",
			ResourceId:   "wal_test",
			OutCome:      string(audit.OutcomeSuccess),
			Reason:       "This is the reason",
			Metadata: map[string]any{
				"amount":   1_000_000,
				"pin":      "123456",
				"password": "hungndtest123!@#",
				"phone":    "0869856211",
				"note":     "CCCD 001099012345",
			},
		})
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	row, n := auditRowByRequestID(t, requestID)
	if n != 1 {
		t.Fatalf("want 1 audit row, got %d", n)
	}
	if row.actorID == nil || *row.actorID != actor {
		t.Errorf("actor_id = %v, want %v", row.actorID, actor)
	}
	if row.traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id = %q", row.traceID)
	}
	if row.ip == nil || *row.ip != "203.0.113.7" || row.userAgent == nil || *row.userAgent != "EWalletApp/1.0" {
		t.Errorf("ip/user_agent = %v / %v", row.ip, row.userAgent)
	}
	if row.metadata["pin"] != "[REDACTED]" || row.metadata["password"] != "[REDACTED]" {
		t.Errorf("secrets not redacted: %v", row.metadata)
	}
	if row.metadata["note"] != "CCCD ********2345" || row.metadata["phone"] != "0869856211" {
		t.Errorf("metadata = %v", row.metadata)
	}
	// JSON numbers come back as float64 when scanned into map[string]any.
	if row.metadata["amount"] != float64(1_000_000) {
		t.Errorf("amount = %#v", row.metadata["amount"])
	}
}

func TestAuditSystemActorStoresNulls(t *testing.T) {
	e := newEnv(t)
	requestID := "req_" + id.New().String()
	// A worker job: no HTTP request, so no client info.
	ctx := observability.WithRequestID(context.Background(), requestID)

	err := db.WithTx(ctx, pool, func(ctx context.Context) error {
		return e.modules.Audit.Record(ctx, audit.Entry{
			ActorType:    string(audit.ActorSystem),
			Action:       "ledger.drift.check",
			ResourceType: "ledger",
			ResourceId:   "all",
			OutCome:      string(audit.OutcomeSuccess),
		})
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	row, n := auditRowByRequestID(t, requestID)
	if n != 1 {
		t.Fatalf("want 1 audit row, got %d", n)
	}
	if row.actorID != nil || row.reason != nil || row.ip != nil || row.userAgent != nil {
		t.Errorf("want NULL actor_id/reason/ip/user_agent, got %v %v %v %v", row.actorID, row.reason, row.ip, row.userAgent)
	}
	if len(row.metadata) != 0 {
		t.Errorf("metadata = %v, want {}", row.metadata)
	}
}

func TestAuditRollsBackWithTransaction(t *testing.T) {
	e := newEnv(t)
	requestID := "req_" + id.New().String()
	errBusiness := errors.New("business rule failed after audit")

	err := db.WithTx(auditCtx(requestID), pool, func(ctx context.Context) error {
		if err := e.modules.Audit.Record(ctx, audit.Entry{
			ActorType:    string(audit.ActorCustomer),
			ActorId:      id.New(),
			Action:       string(audit.ActionPaymentTransfer),
			ResourceType: "payment",
			ResourceId:   "txn_test",
			OutCome:      string(audit.OutcomeSuccess),
		}); err != nil {
			return err
		}
		return errBusiness
	})
	if !errors.Is(err, errBusiness) {
		t.Fatalf("err = %v", err)
	}
	if _, n := auditRowByRequestID(t, requestID); n != 0 {
		t.Errorf("audit row survived a rolled-back transaction")
	}
}
