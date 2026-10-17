// Package auth authenticates incoming requests and exposes the caller in the context.
//
// The identity module will provide a JWT (Ed25519) Authenticator (architecture §8.1).
// Until then only DevHeaderAuth exists, which trusts a header and must never be
// enabled outside APP_ENV=local.
package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/httpx"
)

// Customer is the authenticated end user of a request.
type Customer struct {
	ID       uuid.UUID
	DeviceID string
}

// Authenticator extracts the caller from a request.
type Authenticator interface {
	Authenticate(r *http.Request) (Customer, error)
}

type ctxKey struct{}

// Middleware rejects unauthenticated requests with 401 and stores the Customer in the context.
func Middleware(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := a.Authenticate(r)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, c)))
		})
	}
}

// CustomerFrom returns the authenticated customer. ok is false outside Middleware.
func CustomerFrom(ctx context.Context) (Customer, bool) {
	c, ok := ctx.Value(ctxKey{}).(Customer)
	return c, ok
}

// HeaderDevCustomerID carries the customer id when DevHeaderAuth is used.
const HeaderDevCustomerID = "X-Dev-Customer-Id"

// DevHeaderAuth trusts the X-Dev-Customer-Id header. Local development only.
type DevHeaderAuth struct{}

func (DevHeaderAuth) Authenticate(r *http.Request) (Customer, error) {
	id, err := uuid.Parse(r.Header.Get(HeaderDevCustomerID))
	if err != nil {
		return Customer{}, apperr.ErrUnauthorized
	}
	return Customer{ID: id, DeviceID: r.Header.Get("X-Device-Id")}, nil
}
