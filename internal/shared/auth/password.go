package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16

	// minPasswordBytes is the minimum allowed password size in bytes.
	minPasswordBytes = 8

	// maxPasswordBytes is the maximum allowed password size in bytes.
	// Argon2id (like bcrypt) silently truncates beyond 72 bytes, so we reject early.
	maxPasswordBytes = 72
)

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}

type argon2Hasher struct{}

// NewPasswordHasher returns an argon2id password hasher.
func NewPasswordHasher() PasswordHasher {
	return &argon2Hasher{}
}

// Hash creates an argon2id hash of the password.
func (h *argon2Hasher) Hash(password string) (string, error) {
	if len([]byte(password)) < minPasswordBytes {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordBytes)
	}
	if len([]byte(password)) > maxPasswordBytes {
		return "", fmt.Errorf("password exceeds maximum length of %d bytes", maxPasswordBytes)
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

// Verify checks a password against an encoded argon2id hash.
func (h *argon2Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var memory uint32
	var time uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
	if err != nil {
		return false, fmt.Errorf("parsing hash params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decoding salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decoding hash: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}
