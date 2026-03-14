package audit

import (
	sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// Module provides the audit module to the Fx container.
var Module = fx.Module("audit",
	fx.Provide(fx.Private, func(pool *pgxpool.Pool) *sqlcgen.Queries {
		return sqlcgen.New(pool)
	}),
	fx.Provide(NewHandler),
	fx.Provide(fx.Annotate(
		provideHandlers,
		fx.ResultTags(`group:"event_handlers"`),
	)),
)

func provideHandlers(h *Handler) []events.HandlerRegistration {
	return []events.HandlerRegistration{
		{Name: "audit.user_created", Topic: contracts.TopicUserCreated, HandlerFunc: h.HandleUserCreated},
		{Name: "audit.user_updated", Topic: contracts.TopicUserUpdated, HandlerFunc: h.HandleUserUpdated},
		{Name: "audit.user_deleted", Topic: contracts.TopicUserDeleted, HandlerFunc: h.HandleUserDeleted},
		{Name: "audit.user_logged_in", Topic: contracts.TopicUserLoggedIn, HandlerFunc: h.HandleUserLoggedIn},
		{Name: "audit.user_logged_out", Topic: contracts.TopicUserLoggedOut, HandlerFunc: h.HandleUserLoggedOut},
	}
}
