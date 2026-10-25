package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidEntry(t *testing.T) {
	var entry = Entry{
		ActorType:    string(Customer),
		ActorId:      uuid.Nil,
		Action:       "test action",
		ResourceType: "test resource_type",
		ResourceId:   "test resource_id",
		OutCome:      string(Success),
		Reason:       "",
		RequestId:    "",
		TraceId:      "",
		Ip:           "",
		UserAgent:    "",
		Metadata:     nil,
	}

	err := entry.Validate()
	if err != nil {
		t.Errorf("Expect valid entry, got %v", err)
	}
}

func TestStaffNotComeWithReason(t *testing.T) {
	var entry = Entry{
		ActorType:    string(Staff),
		ActorId:      uuid.Nil,
		Action:       "test action",
		ResourceType: "test resource_type",
		ResourceId:   "test resource_id",
		OutCome:      string(Success),
		Reason:       "",
		RequestId:    "",
		TraceId:      "",
		Ip:           "",
		UserAgent:    "",
		Metadata:     nil,
	}

	err := entry.Validate()
	if err == nil {
		t.Errorf("Expect reason could not be empty for Staff actor_type")
	}
}
