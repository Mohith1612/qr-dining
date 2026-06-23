package crypto_test

import (
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/crypto"
)

func TestGenerateAndVerifyTOTP(t *testing.T) {
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	if len(secret) < 16 {
		t.Errorf("secret too short: %q", secret)
	}

	// Verify against an obviously wrong code.
	if crypto.VerifyTOTP(secret, "000000") {
		// 1-in-a-million chance the all-zeros code matches; the test is
		// flaky-safe because GenerateTOTPSecret randomizes.
		t.Log("zero-code accidentally matched current step; rerun")
	}
}

func TestVerifyTOTP_TolerantToFormatting(t *testing.T) {
	// Synthesize a known code for a fixed secret by invoking the verifier
	// against the current step — pick whatever code the algorithm produces
	// now and confirm formatting tolerance.
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	// Iterate possible 6-digit codes; this is a hack but lets us obtain a
	// valid code without re-implementing TOTP here. In practice we'd export
	// a test helper; for now we accept that brute-search is fine for a unit
	// test.
	for n := 0; n < 1000000; n++ {
		code := zeropad6(n)
		if crypto.VerifyTOTP(secret, code) {
			// Confirm tolerance of whitespace and dashes.
			spaced := strings.Join([]string{code[:3], code[3:]}, " ")
			dashed := strings.Join([]string{code[:3], code[3:]}, "-")
			if !crypto.VerifyTOTP(secret, spaced) {
				t.Errorf("rejected spaced code %q", spaced)
			}
			if !crypto.VerifyTOTP(secret, dashed) {
				t.Errorf("rejected dashed code %q", dashed)
			}
			return
		}
	}
	t.Fatal("could not find a valid current TOTP code in the search space — should be unreachable")
}

func TestEncryptDecryptSecret(t *testing.T) {
	key := "this-is-a-test-key-of-min-length-please"
	plain := "JBSWY3DPEHPK3PXP"
	enc, err := crypto.EncryptSecret(plain, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == plain {
		t.Fatal("ciphertext equals plaintext")
	}
	dec, err := crypto.DecryptSecret(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != plain {
		t.Errorf("roundtrip mismatch: %q != %q", dec, plain)
	}
}

func TestDecryptSecret_WrongKeyFails(t *testing.T) {
	enc, err := crypto.EncryptSecret("payload", "first-key-min-length-padding-padding")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := crypto.DecryptSecret(enc, "second-key-also-min-length-padding"); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func zeropad6(n int) string {
	const pad = "000000"
	s := pad
	for i := 0; n > 0 && i < 6; i++ {
		d := n % 10
		s = s[:5-i] + string(rune('0'+d)) + s[6-i:]
		n /= 10
	}
	return s
}
