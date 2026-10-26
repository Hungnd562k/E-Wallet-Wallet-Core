package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizeMetadata(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "top-level sensitive keys",
			in:   map[string]any{"pin": "123456", "otp": "999999", "refresh_token": "abc"},
			want: map[string]any{"pin": redactedValue, "otp": redactedValue, "refresh_token": redactedValue},
		},
		{
			name: "cccd nested two levels",
			in:   map[string]any{"customer": map[string]any{"kyc": map[string]any{"cccd_number": "001099012345"}}},
			want: map[string]any{"customer": map[string]any{"kyc": map[string]any{"cccd_number": redactedValue}}},
		},
		{
			name: "maps inside a slice",
			in:   map[string]any{"steps": []any{map[string]any{"otp": "111111"}, "ok"}},
			want: map[string]any{"steps": []any{map[string]any{"otp": redactedValue}, "ok"}},
		},
		{
			name: "typed slice of maps",
			in:   map[string]any{"steps": []map[string]any{{"pin": "1"}}},
			want: map[string]any{"steps": []any{map[string]any{"pin": redactedValue}}},
		},
		{
			name: "cccd hidden in free text",
			in:   map[string]any{"description": "chuyen tien cho 001099012345"},
			want: map[string]any{"description": "chuyen tien cho ********2345"},
		},
		{
			name: "not redacted by mistake",
			in: map[string]any{
				"shipping": "kept",              // contains "pin" only as a substring
				"amount":   int64(100000000000), // 12 digits, but a number, not a CCCD
				"phone":    "0869856211",        // 10 digits
			},
			want: map[string]any{"shipping": "kept", "amount": int64(100000000000), "phone": "0869856211"},
		},
		{name: "nil stays nil", in: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeMetadata(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSanitizeMetadataDoesNotModifyInput(t *testing.T) {
	in := map[string]any{"pin": "123456", "nested": map[string]any{"otp": "1"}}
	SanitizeMetadata(in)
	if in["pin"] != "123456" || in["nested"].(map[string]any)["otp"] != "1" {
		t.Errorf("input was modified: %#v", in)
	}
}

// A struct cannot be passed to SanitizeMetadata directly: Go has no implicit
// struct→map conversion. MetadataFrom goes through JSON, so json tags become keys.
func TestMetadataFromStruct(t *testing.T) {
	type recipient struct {
		WalletID string `json:"wallet_id"`
		Phone    string `json:"phone"`
	}
	req := struct {
		Recipient   recipient `json:"recipient"`
		Amount      int64     `json:"amount"`
		Currency    string    `json:"currency"`
		Description string    `json:"description"`
		PIN         string    `json:"pin"`
	}{
		Recipient:   recipient{WalletID: "0199a000-0000-7000-8000-00000000000a", Phone: "0869856211"},
		Amount:      100_000_000_000,
		Currency:    "VND",
		Description: "CCCD 001099012345",
		PIN:         "123456",
	}

	got, err := MetadataFrom(req)
	if err != nil {
		t.Fatal(err)
	}
	if got["pin"] != redactedValue {
		t.Errorf("pin = %v, want redacted", got["pin"])
	}
	if got["amount"] != json.Number("100000000000") {
		t.Errorf("amount = %#v, want json.Number kept exact", got["amount"])
	}
	if got["description"] != "CCCD ********2345" {
		t.Errorf("description = %v", got["description"])
	}
	if r := got["recipient"].(map[string]any); r["phone"] != "0869856211" {
		t.Errorf("phone = %v, should be kept", r["phone"])
	}

	b, _ := json.Marshal(got)
	for _, secret := range []string{"123456", "001099012345"} {
		if strings.Contains(string(b), secret) {
			t.Errorf("metadata still contains %q: %s", secret, b)
		}
	}
}

func TestMetadataFromRejectsNonObject(t *testing.T) {
	if _, err := MetadataFrom([]int{1, 2}); err == nil {
		t.Error("want error for a JSON array")
	}
}
