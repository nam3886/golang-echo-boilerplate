package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

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

// dummyArgon2Hash is a pre-computed argon2id hash used to equalize response time
// when an unknown email is provided. Without this, the unknown-email path returns
// instantly while the wrong-password path runs argon2id, leaking email registration status.
const dummyArgon2Hash = "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// LoginCmd holds input for user login.
type LoginCmd struct {
	Email    string
	Password string
}

// LoginResult holds the tokens issued on successful login.
type LoginResult struct {
	AccessToken string
	ExpiresIn   int64 // seconds until access token expiry
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
			// Run dummy verification to equalize timing with the wrong-password path,
			// preventing email enumeration via response time side-channel.
			_, _ = h.hasher.Verify(cmd.Password, dummyArgon2Hash)
			slog.WarnContext(ctx, "login failed: unknown email",
				"module", "auth", "operation", "LoginHandler",
				"error_code", "invalid_credentials", "ip", netutil.GetClientIP(ctx),
				"retryable", false)
			h.publishLoginFailed(ctx, cmd.Email, "unknown_email")
			return LoginResult{}, ErrInvalidCredentials()
		}
		return LoginResult{}, fmt.Errorf("credential lookup: %w", err)
	}

	match, err := h.hasher.Verify(cmd.Password, hashedPwd)
	if err != nil || !match {
		slog.WarnContext(ctx, "login failed: wrong password",
			"module", "auth", "operation", "LoginHandler",
			"error_code", "invalid_credentials", "ip", netutil.GetClientIP(ctx),
			"retryable", false)
		h.publishLoginFailed(ctx, cmd.Email, "wrong_password")
		return LoginResult{}, ErrInvalidCredentials()
	}

	permissions := auth.PermissionsForRole(role)

	accessToken, err := auth.GenerateAccessToken(h.cfg, userID, role, permissions)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generating access token: %w", err)
	}

	// Durable audit log before event publish — logged regardless of bus availability.
	slog.InfoContext(ctx, "login success",
		"module", "auth", "operation", "LoginHandler",
		"user_id", userID, "ip", netutil.GetClientIP(ctx))

	// Publish event after successful authentication (fail-open).
	if pubErr := h.bus.Publish(ctx, contracts.TopicUserLoggedIn, contracts.UserLoggedInEvent{
		EventID:   uuid.NewString(),
		Version:   contracts.UserEventSchemaVersion,
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
		AccessToken: accessToken,
		ExpiresIn:   int64(h.cfg.JWTAccessTTL.Seconds()),
	}, nil
}

// publishLoginFailed publishes a login_failed event for audit persistence (fail-open).
func (h *LoginHandler) publishLoginFailed(ctx context.Context, email, reason string) {
	if pubErr := h.bus.Publish(ctx, contracts.TopicUserLoginFailed, contracts.UserLoginFailedEvent{
		EventID:   uuid.NewString(),
		Version:   contracts.UserEventSchemaVersion,
		Email:     email,
		Reason:    reason,
		IPAddress: netutil.GetClientIP(ctx),
		At:        time.Now(),
	}); pubErr != nil {
		slog.ErrorContext(ctx, "failed to publish user.login_failed event",
			"module", "auth", "operation", "LoginHandler",
			"error_code", "event_publish_failed",
			"retryable", true, "err", pubErr)
	}
}
