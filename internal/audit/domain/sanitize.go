package domain

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"
)

const redactedValue = "[REDACTED]"

// SanitizeMetadata returns a copy of m that is safe to store in audit_logs.metadata
// (BR-71): values under sensitive keys (pin, otp, token, cccd…) become [REDACTED]
// at any depth, and 12-digit CCCD numbers inside strings are masked.
// m itself is not modified, so the caller's data stays intact.
func SanitizeMetadata(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if observability.IsSensitiveKey(k) {
			out[k] = redactedValue
			continue
		}
		out[k] = sanitizeValue(v)
	}
	return out
}

func sanitizeValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		return SanitizeMetadata(v)
	case []map[string]any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = SanitizeMetadata(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = sanitizeValue(item)
		}
		return out
	case string:
		return observability.MaskCCCD(v)
	default:
		// Numbers, bools, json.Number…: an amount like 100000000000 is not a CCCD.
		return v
	}
}

// MetadataFrom converts a struct (or any JSON-encodable value) into a metadata map
// using its json tags, then sanitizes it. Numbers are kept as json.Number so
// int64 amounts do not lose precision through float64.
func MetadataFrom(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("audit metadata: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("audit metadata must be a JSON object: %w", err)
	}
	return SanitizeMetadata(m), nil
}
