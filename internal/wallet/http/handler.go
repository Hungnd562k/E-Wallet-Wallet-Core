package wallethttp

import (
	"net/http"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/ledger"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/httpx"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/id"
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/wallet"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	walletSvc *wallet.Service
	ledgerSvc *ledger.Service
}

func NewHandler(walletSvc *wallet.Service, ledgerSvc *ledger.Service) *Handler {
	return &Handler{
		walletSvc: walletSvc,
		ledgerSvc: ledgerSvc,
	}
}

// Routes mounts the customer-facing payment endpoints. The router must already
// apply auth.Middleware.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/v1/wallet", h.InquiryBalanceByWalletID)
}

// Truy van so du tai khoan KH, Balance, Hold, Balance + Hold
func (h *Handler) InquiryBalanceByWalletID(w http.ResponseWriter, r *http.Request) {
	tmp := r.Header.Get("X-Dev-Customer-Id")
	walletId, err := id.Parse(id.Wallet, tmp)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	walletBalance, err := h.walletSvc.GetDetailByCustomer(r.Context(), walletId)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	balance, err := h.ledgerSvc.Balance(r.Context(), walletBalance.LedgerAccountID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	held := int64(0)

	walletBalance.AvailableBalance = balance
	walletBalance.HeldBalance = held
	walletBalance.TotalBalance = balance + held

	httpx.WriteJSON(w, http.StatusOK, walletBalance)
}
