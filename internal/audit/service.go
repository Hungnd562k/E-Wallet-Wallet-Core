// Package audit writes the append-only audit log. The app DB role has only
// INSERT/SELECT on this schema. Schema: audit. BR-64, 71.
package audit

import "github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/audit/domain"

// TODO(ST-3): Service.Record, which always sanitizes Entry.Metadata before INSERT.

// SanitizeMetadata returns a copy of m with sensitive values hidden (BR-71).
func SanitizeMetadata(m map[string]any) map[string]any { return domain.SanitizeMetadata(m) }

// MetadataFrom turns a struct into sanitized metadata using its json tags.
func MetadataFrom(v any) (map[string]any, error) { return domain.MetadataFrom(v) }
