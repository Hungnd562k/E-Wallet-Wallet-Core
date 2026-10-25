package integration

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/identity"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/google/uuid"
)

func newPhone() string {
	return fmt.Sprintf("09%08d", rand.IntN(100_000_000))
}

// Create valid customer
func TestCreateCustomer(t *testing.T) {
	e := newEnv(t)
	phone := newPhone()
	ctx := context.Background()
	err := e.modules.Identity.Create(ctx, identity.CustomerCommand{Id: id.New(),
		Phone:            phone,
		FullName:         "John Doe",
		Dob:              time.Now(),
		OnboardingStatus: identity.OnboardingPending,
	})

	if err != nil {
		t.Fatalf("Create failed %v", err)
	}

	c, err := e.modules.Identity.GetCustomerByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("Got %v", err)
	}

	if c.Id == uuid.Nil {
		t.Errorf("Id was not generated")
	}

	if c.FullName == "" {
		t.Errorf("Fullname was not generated")
	}

	if c.OnboardingStatus != identity.OnboardingPending {
		t.Errorf("Invalid Onboarding status")
	}
}

func TestCreateInvalidPhone(t *testing.T) {
	e := newEnv(t)
	phone := "gsdfg"
	ctx := context.Background()
	err := e.modules.Identity.Create(ctx, identity.CustomerCommand{Id: uuid.Nil,
		Phone:            phone,
		FullName:         "John Doe",
		Dob:              time.Now(),
		OnboardingStatus: "",
	})

	if err != identity.ErrInvalidPhoneNumber {
		t.Errorf("Expect ErrInvalidPhoneNumber error!")
	}
}

func TestGetCustomerFromPhoneNotReal(t *testing.T) {
	e := newEnv(t)
	phone := "fascasdcfasd"
	ctx := context.Background()
	_, err := e.modules.Identity.GetCustomerByPhone(ctx, phone)

	if err != identity.ErrInvalidPhoneNumber {
		t.Errorf("Expect ErrInvalidPhoneNumber error!")
	}
}

// Phone not found
func TestPhoneNotFound(t *testing.T) {
	e := newEnv(t)
	phone := newPhone()
	ctx := context.Background()
	err := e.modules.Identity.Create(ctx, identity.CustomerCommand{Id: id.New(),
		Phone:            phone,
		FullName:         "John Doe",
		Dob:              time.Now(),
		OnboardingStatus: identity.OnboardingPending,
	})

	if err != nil {
		t.Fatalf("Create failed %v", err)
	}

	_, err = e.modules.Identity.GetCustomerByPhone(ctx, "23452345")
	if err != identity.ErrInvalidPhoneNumber {
		t.Errorf("Expect ErrInvalidPhoneNumber error!")
	}
}
