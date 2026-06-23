package audit

import (
	"encoding/json"
	"testing"
)

func TestRedact_SensitiveKeys(t *testing.T) {
	cases := []string{"pin", "pin_hash", "password", "password_hash", "token", "card_number", "cvv", "cvc", "expiry"}
	for _, key := range cases {
		raw, _ := json.Marshal(map[string]any{key: "sensitive-value"})
		got, _ := Redact(raw, nil)

		var result map[string]any
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("key %q: unmarshal: %v", key, err)
		}
		if result[key] != "[REDACTED]" {
			t.Errorf("key %q: got %v, want [REDACTED]", key, result[key])
		}
	}
}

func TestRedact_SecretContains(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"api_secret": "sk-123", "bcrypt_hash": "hash"})
	got, _ := Redact(raw, nil)

	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"api_secret", "bcrypt_hash"} {
		if result[k] != "[REDACTED]" {
			t.Errorf("key %q: got %v, want [REDACTED]", k, result[k])
		}
	}
}

func TestRedact_NestedSensitiveKey(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"user": map[string]any{
			"name":     "alice",
			"password": "hunter2",
		},
	})
	got, _ := Redact(raw, nil)

	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	user, _ := result["user"].(map[string]any)
	if user == nil {
		t.Fatal("user key missing")
	}
	if user["password"] != "[REDACTED]" {
		t.Errorf("nested password: got %v, want [REDACTED]", user["password"])
	}
	if user["name"] != "alice" {
		t.Errorf("name should be preserved, got %v", user["name"])
	}
}

func TestRedact_NonSensitiveKey(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"name": "alice", "email": "alice@example.com"})
	got, _ := Redact(raw, nil)

	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["name"] != "alice" {
		t.Errorf("name: got %v, want alice", result["name"])
	}
	if result["email"] != "alice@example.com" {
		t.Errorf("email: got %v, want alice@example.com", result["email"])
	}
}

func TestRedact_NilInput(t *testing.T) {
	before, after := Redact(nil, nil)
	if before != nil || after != nil {
		t.Errorf("nil input should return nil, got %v, %v", before, after)
	}
}

func TestRedact_BothBeforeAndAfter(t *testing.T) {
	before, _ := json.Marshal(map[string]any{"pin": "1234", "role": "staff"})
	after, _ := json.Marshal(map[string]any{"pin": "5678", "role": "manager"})
	gotBefore, gotAfter := Redact(before, after)

	var rb, ra map[string]any
	json.Unmarshal(gotBefore, &rb)
	json.Unmarshal(gotAfter, &ra)

	if rb["pin"] != "[REDACTED]" {
		t.Errorf("before.pin: got %v, want [REDACTED]", rb["pin"])
	}
	if ra["pin"] != "[REDACTED]" {
		t.Errorf("after.pin: got %v, want [REDACTED]", ra["pin"])
	}
	if rb["role"] != "staff" {
		t.Errorf("before.role: got %v, want staff", rb["role"])
	}
	if ra["role"] != "manager" {
		t.Errorf("after.role: got %v, want manager", ra["role"])
	}
}

func TestRedact_ArrayOfObjects(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"attempts": []any{
			map[string]any{"token": "tok_123", "status": "failed"},
			map[string]any{"card_number": "4242424242424242", "cvv": "123", "amount": 100},
		},
	})
	got, _ := Redact(raw, nil)

	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	attempts, _ := result["attempts"].([]any)
	if len(attempts) != 2 {
		t.Fatalf("attempts length = %d, want 2", len(attempts))
	}
	first := attempts[0].(map[string]any)
	second := attempts[1].(map[string]any)
	if first["token"] != "[REDACTED]" {
		t.Errorf("token: got %v, want [REDACTED]", first["token"])
	}
	if first["status"] != "failed" {
		t.Errorf("status: got %v, want failed", first["status"])
	}
	if second["card_number"] != "[REDACTED]" || second["cvv"] != "[REDACTED]" {
		t.Errorf("payment fields not redacted: %+v", second)
	}
	if second["amount"] != float64(100) {
		t.Errorf("amount should be preserved, got %v", second["amount"])
	}
}

func TestRedact_PhaseAExpandedKeys(t *testing.T) {
	keys := []string{
		"signature",
		"webhook_signature",
		"payment_signature",
		"x_payment_signature",
		"csrf_token",
		"refresh_token",
		"access_token",
		"client_secret",
		"private_key",
		"api_key",
		"otp",
		"mfa_code",
		"recovery_code",
		"pan",
		"card_pan",
	}
	for _, key := range keys {
		raw, _ := json.Marshal(map[string]any{key: "sensitive-value"})
		got, _ := Redact(raw, nil)
		var result map[string]any
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("key %q: unmarshal: %v", key, err)
		}
		if result[key] != "[REDACTED]" {
			t.Errorf("key %q: got %v, want [REDACTED]", key, result[key])
		}
	}
}

func TestRedact_PhaseASubstrings(t *testing.T) {
	// Derived names should match via substring rules added in Phase A.
	raw, _ := json.Marshal(map[string]any{
		"stripe_signature":  "sig_123",
		"X-Razorpay-Signature": "sig_456",
		"new_pin_hash":      "hash",
		"mfa_secret":        "totp",
		"client_2fa_seed":   "abc",
		"benign_field":      "ok",
	})
	got, _ := Redact(raw, nil)
	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"stripe_signature", "X-Razorpay-Signature", "new_pin_hash", "mfa_secret", "client_2fa_seed"} {
		if result[k] != "[REDACTED]" {
			t.Errorf("key %q: got %v, want [REDACTED]", k, result[k])
		}
	}
	if result["benign_field"] != "ok" {
		t.Errorf("benign_field should be preserved, got %v", result["benign_field"])
	}
}

func TestRedact_NestedArrays(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"events": []any{
			[]any{
				map[string]any{
					"password": "hunter2",
					"payload": []any{
						map[string]any{"api_secret": "sk_123", "name": "safe"},
					},
				},
			},
			"scalar",
		},
	})
	got, _ := Redact(raw, nil)

	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	events := result["events"].([]any)
	nested := events[0].([]any)[0].(map[string]any)
	if nested["password"] != "[REDACTED]" {
		t.Errorf("password: got %v, want [REDACTED]", nested["password"])
	}
	payload := nested["payload"].([]any)[0].(map[string]any)
	if payload["api_secret"] != "[REDACTED]" {
		t.Errorf("api_secret: got %v, want [REDACTED]", payload["api_secret"])
	}
	if payload["name"] != "safe" {
		t.Errorf("name should be preserved, got %v", payload["name"])
	}
	if events[1] != "scalar" {
		t.Errorf("scalar array value should be preserved, got %v", events[1])
	}
}
