package audit

import (
	sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
	"github.com/gnha/gnha-services/internal/shared/events"
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
		{Name: "audit.user_created", Topic: events.TopicUserCreated, HandlerFunc: h.HandleUserCreated},
		{Name: "audit.user_updated", Topic: events.TopicUserUpdated, HandlerFunc: h.HandleUserUpdated},
		{Name: "audit.user_deleted", Topic: events.TopicUserDeleted, HandlerFunc: h.HandleUserDeleted},
	}
}
