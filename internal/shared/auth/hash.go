package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashJTI returns the first 8 hex characters of the SHA-256 hash of jti.
// Safe to include in logs — identifies the token for correlation without exposing the raw ID.
func HashJTI(jti string) string {
	h := sha256.Sum256([]byte(jti))
	return hex.EncodeToString(h[:4]) // 4 bytes → 8 hex chars
}
