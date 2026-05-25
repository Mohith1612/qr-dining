package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateToken returns a 64-character hex-encoded random token (32 bytes).
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
