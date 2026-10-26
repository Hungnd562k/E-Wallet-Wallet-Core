package audit

import (
	"testing"
)

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

func TestSantizeMessage(t *testing.T) {
	// var r = transferRequest{
	// 	Recipient: struct {
	// 		WalletID string `json:"wallet_id"`
	// 		Phone    string `json:"phone"`
	// 	}{
	// 		WalletID: "0199a000-0000-7000-8000-00000000000a",
	// 		Phone:    "0869856211",
	// 	},
	// 	Amount:      10_000_000,
	// 	Currency:    "VND",
	// 	Description: "Test chuyen tien",
	// 	PIN:         "123456",
	// }

	// rAfterSanitize := Sanitize(r)
	// t.Errorf("Got: %+v", &rAfterSanitize)
}
