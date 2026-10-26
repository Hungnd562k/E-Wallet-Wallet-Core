// Package audit writes the append-only audit log. The app DB role has only
// INSERT/SELECT on this schema. Schema: audit. BR-64, 71.
package audit

import "github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/observability"

const redactedValue = "[REDACTED]"

func Sanitize(m map[string]any) map[string]any {
	for key, value := range m {
		if observability.IsSensitiveKey(key) {
			m[key] = redactedValue
			continue
		}

		m[key] = sanitizeValue(value)
	}

	return m
}

func sanitizeValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return Sanitize(v)
	case []any:
		for i, item := range v {
			v[i] = sanitizeValue(item)
		}
		return v
	default:
		return value
	}
}
