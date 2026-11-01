package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

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
