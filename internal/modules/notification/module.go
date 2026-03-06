package notification

import (
	"github.com/gnha/gnha-services/internal/shared/events"
	"go.uber.org/fx"
)

// Module provides the notification module to the Fx container.
var Module = fx.Module("notification",
	fx.Provide(fx.Annotate(
		NewSMTPSender,
		fx.As(new(Sender)),
	)),
	fx.Provide(NewHandler),
	fx.Provide(fx.Annotate(
		provideHandlers,
		fx.ResultTags(`group:"event_handlers"`),
	)),
)

func provideHandlers(h *Handler) []events.HandlerRegistration {
	return []events.HandlerRegistration{
		{Name: "notify.user_created", Topic: events.TopicUserCreated, HandlerFunc: h.HandleUserCreated},
	}
}
