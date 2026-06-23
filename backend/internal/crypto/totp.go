package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP implementation per RFC 6238 with SHA-1, 30-second step, 6-digit codes —
// the parameters required for compatibility with Google Authenticator, Authy,
// and 1Password. The library is deliberately tiny and dependency-free so the
// surface for supply-chain compromise on a credential primitive is zero.

const (
	totpStep         = 30 * time.Second
	totpDigits       = 6
	totpDriftSteps   = 1 // accept previous and next step to tolerate clock skew
	totpSecretBytes  = 20
)

// GenerateTOTPSecret returns a base32-encoded secret suitable for storing and
// for displaying in an authenticator app provisioning URI.
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, totpSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// BuildOTPAuthURI returns an otpauth:// URI consumable by every common
// authenticator app. `issuer` and `accountName` are user-visible labels.
func BuildOTPAuthURI(issuer, accountName, secret string) string {
	label := url.PathEscape(issuer + ":" + accountName)
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprintf("%d", totpDigits))
	v.Set("period", fmt.Sprintf("%d", int(totpStep.Seconds())))
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// VerifyTOTP returns true if the supplied code matches the current TOTP value
// for `secret`, allowing ±totpDriftSteps of clock skew. Whitespace and dashes
// in the user-typed code are tolerated. Comparison is constant-time.
func VerifyTOTP(secret, code string) bool {
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
	if len(cleaned) != totpDigits {
		return false
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	now := time.Now().Unix() / int64(totpStep.Seconds())
	for offset := -int64(totpDriftSteps); offset <= int64(totpDriftSteps); offset++ {
		expected := totpAt(key, now+offset)
		if hmac.Equal([]byte(expected), []byte(cleaned)) {
			return true
		}
	}
	return false
}

func totpAt(key []byte, counter int64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, value%mod)
}
