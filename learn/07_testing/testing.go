// Package testingdemo: chapter 07 of docs/go-map. The code under test is
// small; the point is the test file next to it.
package testingdemo

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
)

// NormalizePhone turns "0901 234 567" or "+84 901-234-567" into "+84901234567".
// It returns "" when the input is not a Vietnamese mobile number.
func NormalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "84") && len(d) == 11:
		return "+" + d
	case strings.HasPrefix(d, "0") && len(d) == 10:
		return "+84" + d[1:]
	}
	return ""
}

// BalanceHandler is a tiny HTTP handler to show httptest.
func BalanceHandler(lookup func(walletID string) (int64, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bal, ok := lookup(r.URL.Query().Get("wallet_id"))
		if !ok {
			http.Error(w, `{"error":"WALLET_NOT_FOUND"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"balance": bal})
	}
}
