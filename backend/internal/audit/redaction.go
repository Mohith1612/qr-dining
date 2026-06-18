package audit

import (
	"encoding/json"
	"strings"
)

var sensitiveKeys = map[string]bool{
	"pin":           true,
	"pin_hash":      true,
	"current_pin":   true,
	"new_pin":       true,
	"token":         true,
	"token_hash":    true,
	"password":      true,
	"password_hash": true,
	"card_number":   true,
	"cvv":           true,
	"cvc":           true,
	"expiry":        true,
}

func isSensitive(key string) bool {
	k := strings.ToLower(key)
	if sensitiveKeys[k] {
		return true
	}
	return strings.Contains(k, "secret") || strings.Contains(k, "bcrypt")
}

// Redact removes sensitive fields from before/after JSON blobs before audit insertion.
// It walks object keys and arrays recursively. Non-object or nil input is returned unchanged.
// The function is idempotent.
func Redact(before, after json.RawMessage) (json.RawMessage, json.RawMessage) {
	return redactJSON(before), redactJSON(after)
}

func redactJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	redacted, changed := redactValue(value)
	if !changed {
		return raw
	}
	out, err := json.Marshal(redacted)
	if err != nil {
		return raw
	}
	return out
}

func redactValue(v any) (any, bool) {
	switch typed := v.(type) {
	case map[string]any:
		redactMap(typed)
		return typed, true
	case []any:
		for i, item := range typed {
			redacted, changed := redactValue(item)
			if changed {
				typed[i] = redacted
			}
		}
		return typed, true
	default:
		return v, false
	}
}

func redactMap(m map[string]any) {
	for k, v := range m {
		if isSensitive(k) {
			m[k] = "[REDACTED]"
			continue
		}
		if redacted, changed := redactValue(v); changed {
			m[k] = redacted
		}
	}
}
