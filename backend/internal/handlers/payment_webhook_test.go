package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/config"
)

func TestVerifyGenericWebhook(t *testing.T) {
	raw := []byte(`{"id":"evt_1"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(raw)
	sig := hex.EncodeToString(mac.Sum(nil))

	cfg := config.PaymentConfig{
		WebhookSecrets:            map[string]string{"generic": "secret"},
		WebhookTimestampTolerance: time.Minute,
	}
	if err := verifyGenericWebhook("generic", raw, ts, sig, cfg); err != nil {
		t.Fatalf("verifyGenericWebhook: %v", err)
	}
	if err := verifyGenericWebhook("generic", raw, ts, "bad", cfg); err == nil {
		t.Fatal("expected invalid signature to fail")
	}
}
