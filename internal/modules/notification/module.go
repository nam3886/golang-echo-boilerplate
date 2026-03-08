package notification

import (
	userdomain "github.com/gnha/gnha-services/internal/modules/user/domain"
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
		{Name: "notify.user_created", Topic: userdomain.TopicUserCreated, HandlerFunc: h.HandleUserCreated},
	}
}
