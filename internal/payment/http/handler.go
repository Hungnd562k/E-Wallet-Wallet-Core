// Package paymenthttp is the HTTP transport of the payment module: decode,
// authenticate, call payment.Service, encode. No business rules here.
//
// The directory is named "http" (architecture §4.2) but the package is
// "paymenthttp" so it does not shadow net/http in this file.
package paymenthttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/payment"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/auth"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/httpx"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
)

const headerIdempotencyKey = "Idempotency-Key"

type Handler struct {
	svc *payment.Service
}

func NewHandler(svc *payment.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the customer-facing payment endpoints. The router must already
// apply auth.Middleware.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/v1/transfers", h.createTransfer)
	r.Get("/v1/payments/{payment_id}", h.getPayment)
}

// transferRequest mirrors TransferRequest in api/openapi.yaml.
type transferRequest struct {
	Recipient struct {
		WalletID string `json:"wallet_id"`
		Phone    string `json:"phone"`
	} `json:"recipient"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	PIN         string `json:"pin"` // TODO(authz): verified by authz once implemented; ignored for now
}

func (h *Handler) createTransfer(w http.ResponseWriter, r *http.Request) {
	customer, ok := auth.CustomerFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.ErrUnauthorized)
		return
	}

	body, err := httpx.ReadBody(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req transferRequest
	if err := httpx.Unmarshal(body, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Recipient.Phone != "" {
		// TODO(identity): resolve recipient by phone once the identity module exists.
		httpx.WriteError(w, r, apperr.New(apperr.KindUnprocessable, "NOT_SUPPORTED_YET", "recipient.phone is not supported yet, use recipient.wallet_id"))
		return
	}
	recipient, err := id.Parse(id.Wallet, req.Recipient.WalletID)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid("recipient.wallet_id is invalid"))
		return
	}

	p, err := h.svc.Transfer(r.Context(), payment.TransferCommand{
		CustomerID:        customer.ID,
		IdempotencyKey:    r.Header.Get(headerIdempotencyKey),
		RecipientWalletID: recipient,
		Amount:            req.Amount,
		Currency:          req.Currency,
		Description:       req.Description,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, p)
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	customer, ok := auth.CustomerFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.ErrUnauthorized)
		return
	}
	pid, err := id.Parse(id.Payment, chi.URLParam(r, "payment_id"))
	if err != nil {
		httpx.WriteError(w, r, payment.ErrNotFound)
		return
	}
	p, err := h.svc.Get(r.Context(), customer.ID, pid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}
