package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/authz"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/google/uuid"
)

func prepareNewCustomer(ctx context.Context, e *env, customerId uuid.UUID) error {
	phone := newPhone()
	err := e.modules.Identity.Create(ctx, identity.CustomerCommand{Id: customerId,
		Phone:            phone,
		FullName:         "John Doe",
		Dob:              time.Now(),
		OnboardingStatus: identity.OnboardingPending,
	})
	if err != nil {
		return err
	}

	return nil
}

func TestCreatePIN(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	customerId := id.New()
	prepareNewCustomer(ctx, e, customerId)
	err := e.modules.Authz.Create_PIN(ctx, customerId, "123456")
	if err != nil {
		t.Errorf("Expect success result, got: %v", err)
	}
}

func TestCreateInvalidPIN(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	customerId := id.New()
	prepareNewCustomer(ctx, e, customerId)
	err := e.modules.Authz.Create_PIN(ctx, customerId, "123456sfzzz")
	if err == nil {
		t.Errorf("Expect fail result")
	} else {
		fmt.Println("Expected error: ", err)
	}
}

func TestAuthz_PIN(t *testing.T) {
	ctx := context.Background()
	customerId := id.New()
	e := newEnv(t)
	prepareNewCustomer(ctx, e, customerId)
	err := e.modules.Authz.Create_PIN(ctx, customerId, "123456")

	if err != nil {
		t.Errorf("Error creating new credentials")
	}

	tests := []struct {
		id            int
		name          string
		correctPIN    string
		wrongPIN      string
		numberOfRetry int8
	}{
		{1, "Nhập đúng luôn lần đầu", "123456", "123456", 1},
		{2, "Nhập sai 5 lần liên tiếp => Hard lock", "123456", "543676", 5},
		{3, "Nhập sai 2 lần liên tiếp, sau đó nhập đúng => Reset bộ đếm", "123456", "543676", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0
			for i < int(tt.numberOfRetry) {
				err := e.modules.Authz.Authz_PIN(ctx, customerId, authz.PIN(tt.wrongPIN))
				t.Errorf("Result: %v", err)
				i++
			}
		})
	}
}
