package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"
	"github.com/redis/go-redis/v9"
)

// LogoutHandler handles token revocation.
// Required: rdb, bus
type LogoutHandler struct {
	rdb *redis.Client
	bus events.EventPublisher
}

// NewLogoutHandler constructs the handler.
// Panics if any required dependency is nil.
func NewLogoutHandler(rdb *redis.Client, bus events.EventPublisher) *LogoutHandler {
	if rdb == nil {
		panic("NewLogoutHandler: rdb must not be nil")
	}
	if bus == nil {
		panic("NewLogoutHandler: bus must not be nil")
	}
	return &LogoutHandler{rdb: rdb, bus: bus}
}

// Handle blacklists the caller's current access token.
func (h *LogoutHandler) Handle(ctx context.Context, claims *auth.TokenClaims) (err error) {
	ctx, span := otel.Tracer("auth").Start(ctx, "LogoutHandler.Handle")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if claims == nil {
		return sharederr.ErrUnauthorized()
	}

	expiry := claims.ExpiresAt.Time
	if err := auth.BlacklistToken(ctx, h.rdb, claims.ID, expiry); err != nil {
		return fmt.Errorf("blacklisting token: %w", err)
	}

	// Durable audit log before event publish — logged regardless of bus availability.
	slog.InfoContext(ctx, "logout success",
		"module", "auth", "operation", "LogoutHandler",
		"user_id", claims.UserID, "token_id", claims.ID)

	// Publish event after successful blacklist (fail-open).
	if pubErr := h.bus.Publish(ctx, contracts.TopicUserLoggedOut, contracts.UserLoggedOutEvent{
		Version:   1,
		UserID:    claims.UserID,
		TokenID:   claims.ID,
		IPAddress: netutil.GetClientIP(ctx),
		At:        time.Now(),
	}); pubErr != nil {
		slog.ErrorContext(ctx, "failed to publish user.logged_out event",
			"module", "auth", "operation", "LogoutHandler",
			"user_id", claims.UserID, "error_code", "event_publish_failed",
			"retryable", true, "err", pubErr)
	}

	return nil
}
