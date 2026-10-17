// Package notification sends push/SMS notifications (mock providers for now).
//
// It consumes domain events from the outbox and never calls other modules:
// it declares the payload shape it needs itself (consumer-side contract),
// so it does not import the payment module.
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/outbox"
)

// ConsumerName identifies this module in platform.processed_events.
const ConsumerName = "notification"

type Service struct{}

func NewService() *Service { return &Service{} }

// paymentSucceeded is the subset of payment.succeeded.v1 this module reads.
type paymentSucceeded struct {
	PaymentID      uuid.UUID `json:"payment_id"`
	Amount         int64     `json:"amount"`
	Currency       string    `json:"currency"`
	SourceWalletID uuid.UUID `json:"source_wallet_id"`
	DestWalletID   uuid.UUID `json:"dest_wallet_id"`
}

// HandlePaymentSucceeded is an outbox.Handler for payment.succeeded.v1.
// TODO: render templates and call the push/SMS mock provider; persist notification.notifications.
func (s *Service) HandlePaymentSucceeded(ctx context.Context, msg outbox.Message) error {
	var e paymentSucceeded
	if err := json.Unmarshal(msg.Payload, &e); err != nil {
		return fmt.Errorf("decode %s: %w", msg.Type, err)
	}
	slog.InfoContext(ctx, "notify payment succeeded (mock)",
		"payment_id", e.PaymentID, "amount", e.Amount, "currency", e.Currency,
		"from_wallet", e.SourceWalletID, "to_wallet", e.DestWalletID)
	return nil
}
