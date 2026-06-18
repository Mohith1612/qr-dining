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
