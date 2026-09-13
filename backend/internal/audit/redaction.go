package audit

import (
	"encoding/json"
	"strings"
)

var sensitiveKeys = map[string]bool{
	"pin":         true,
	"pin_hash":    true,
	"current_pin": true,
	"new_pin":     true,
	"token":       true,
	"token_hash":  true,
	// The guest session credential. `token` above does not cover it: the lookup
	// is an exact match on the whole key, and no entry in sensitiveSubstrings
	// matches "session_token" either. This is the field behind the three
	// SESSION_CREATED / sessions.active / force-close leaks.
	"session_token": true,
	// Plural. The singular "recovery_code" below never matched the column, the
	// json tag, or the Go field — all three are recovery_codes.
	"recovery_codes": true,
	// sha256 of the ephemeral MFA challenge. The "mfa" substring rule does not
	// reach it: the name says nothing about MFA.
	"challenge_hash":      true,
	"password":            true,
	"password_hash":       true,
	"card_number":         true,
	"cvv":                 true,
	"cvc":                 true,
	"expiry":              true,
	"pan":                 true,
	"card_pan":            true,
	"signature":           true,
	"webhook_signature":   true,
	"payment_signature":   true,
	"x_payment_signature": true,
	"csrf_token":          true,
	"refresh_token":       true,
	"access_token":        true,
	"client_secret":       true,
	"private_key":         true,
	"api_key":             true,
	"otp":                 true,
	"otp_code":            true,
	"mfa_code":            true,
	"recovery_code":       true,
}

func isSensitive(key string) bool {
	k := strings.ToLower(key)
	if matchesSensitive(k) {
		return true
	}
	// A struct with no json tags serializes under its Go field names, so the
	// same credential arrives as "RecoveryCodes" rather than "recovery_codes"
	// and every rule above misses it: the map lookup compares whole keys, and
	// the substrings all contain underscores. repository.PlatformMFA is exactly
	// that shape — no tags, holding the bcrypt recovery-code hashes.
	//
	// Derived from the ORIGINAL key, not from k: lowercasing first would erase
	// the camelCase boundaries this depends on.
	//
	// Purely additive. This runs only after the check above fails, so nothing
	// that was redacted stops being redacted; and a key already in snake_case
	// normalizes to itself, so it cannot change classification either.
	if snake := snakeCaseKey(key); snake != k {
		return matchesSensitive(snake)
	}
	return false
}

// matchesSensitive applies the two rules to an already-lowercased key.
func matchesSensitive(k string) bool {
	if sensitiveKeys[k] {
		return true
	}
	// Substring matches catch derived names like "stripe_signature",
	// "csrf_token_v2", "mfa_secret" without enumerating each variant.
	for _, needle := range sensitiveSubstrings {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}

// snakeCaseKey lowercases a key and inserts underscores at camelCase
// boundaries: RecoveryCodes -> recovery_codes, OTPCode -> otp_code. Keys that
// are already snake_case are returned lowercased and otherwise unchanged.
func snakeCaseKey(key string) string {
	var out strings.Builder
	runes := []rune(key)
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && i > 0 {
			prevLower := runes[i-1] < 'A' || runes[i-1] > 'Z'
			// Break entering an uppercase run (recoveryCodes), and before the
			// last capital of a run that starts a word (OTPCode -> otp_code).
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if (prevLower || nextLower) && out.Len() > 0 && !strings.HasSuffix(out.String(), "_") {
				out.WriteByte('_')
			}
		}
		if isUpper {
			out.WriteRune(r - 'A' + 'a')
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

var sensitiveSubstrings = []string{
	"secret",
	"bcrypt",
	"signature",
	"refresh_token",
	"access_token",
	"private_key",
	"api_key",
	"_pin",
	"pin_",
	"mfa",
	"2fa",
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
