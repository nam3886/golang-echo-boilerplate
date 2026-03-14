package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/adapters"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"
)

// ErrInvalidCredentials is returned when email or password is incorrect.
func ErrInvalidCredentials() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeUnauthenticated, "invalid_credentials", "invalid email or password")
}

// LoginCmd holds input for user login.
type LoginCmd struct {
	Email    string
	Password string
}

// LoginResult holds the tokens issued on successful login.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // seconds until access token expiry
}

// LoginHandler handles user authentication.
// Required: lookup, hasher, cfg, bus
type LoginHandler struct {
	lookup auth.CredentialLookup
	hasher auth.PasswordHasher
	cfg    *config.Config
	bus    events.EventPublisher
}

// NewLoginHandler constructs the handler.
// Panics if any required dependency is nil.
func NewLoginHandler(lookup auth.CredentialLookup, hasher auth.PasswordHasher, cfg *config.Config, bus events.EventPublisher) *LoginHandler {
	if lookup == nil {
		panic("NewLoginHandler: lookup must not be nil")
	}
	if hasher == nil {
		panic("NewLoginHandler: hasher must not be nil")
	}
	if cfg == nil {
		panic("NewLoginHandler: cfg must not be nil")
	}
	if bus == nil {
		panic("NewLoginHandler: bus must not be nil")
	}
	return &LoginHandler{lookup: lookup, hasher: hasher, cfg: cfg, bus: bus}
}

// Handle authenticates the user and returns token pair on success.
func (h *LoginHandler) Handle(ctx context.Context, cmd LoginCmd) (_ LoginResult, err error) {
	ctx, span := otel.Tracer("auth").Start(ctx, "LoginHandler.Handle")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	userID, hashedPwd, role, err := h.lookup.GetByEmail(ctx, cmd.Email)
	if err != nil {
		if errors.Is(err, sharederr.ErrNotFound()) {
			return LoginResult{}, ErrInvalidCredentials()
		}
		return LoginResult{}, fmt.Errorf("credential lookup: %w", err)
	}

	match, err := h.hasher.Verify(cmd.Password, hashedPwd)
	if err != nil || !match {
		return LoginResult{}, ErrInvalidCredentials()
	}

	permissions := adapters.PermissionsForRole(role)

	accessToken, err := auth.GenerateAccessToken(h.cfg, userID, role, permissions)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating refresh token: %w", err)
	}

	// Publish event after successful authentication (fail-open).
	if pubErr := h.bus.Publish(ctx, contracts.TopicUserLoggedIn, contracts.UserLoggedInEvent{
		Version:   1,
		UserID:    userID,
		IPAddress: netutil.GetClientIP(ctx),
		At:        time.Now(),
	}); pubErr != nil {
		slog.ErrorContext(ctx, "failed to publish user.logged_in event",
			"module", "auth", "operation", "LoginHandler",
			"user_id", userID, "error_code", "event_publish_failed",
			"retryable", true, "err", pubErr)
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(h.cfg.JWTAccessTTL.Seconds()),
	}, nil
}
