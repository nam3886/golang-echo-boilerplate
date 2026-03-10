package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenClaims holds JWT claims for access tokens.
type TokenClaims struct {
	jwt.RegisteredClaims
	UserID      string   `json:"uid"`
	Role        string   `json:"role"`
	Permissions []string `json:"perms,omitempty"`
}

// jwtAudience is the intended audience for access tokens issued by this service.
const jwtAudience = "golang-echo-boilerplate"

// GenerateAccessToken creates a signed JWT access token.
func GenerateAccessToken(cfg *config.Config, userID, role string, permissions []string) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.AppName,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{jwtAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.JWTAccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
		UserID:      userID,
		Role:        role,
		Permissions: permissions,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateAccessToken parses and validates a JWT access token.
func ValidateAccessToken(cfg *config.Config, tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	},
		jwt.WithIssuer(cfg.AppName),
		jwt.WithAudience(jwtAudience),
	)
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
