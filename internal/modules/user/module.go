package user

import (
	"context"

	"github.com/gnha/gnha-services/internal/modules/user/adapters/grpc"
	"github.com/gnha/gnha-services/internal/modules/user/adapters/postgres"
	usersearch "github.com/gnha/gnha-services/internal/modules/user/adapters/search"
	"github.com/gnha/gnha-services/internal/modules/user/app"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/events"
	"go.uber.org/fx"
)

// Module provides the user module to the Fx container.
var Module = fx.Module("user",
	fx.Provide(
		fx.Annotate(
			postgres.NewPgUserRepository,
			fx.As(new(domain.UserRepository)),
		),
	),
	fx.Provide(app.NewCreateUserHandler),
	fx.Provide(app.NewGetUserHandler),
	fx.Provide(app.NewListUsersHandler),
	fx.Provide(app.NewUpdateUserHandler),
	fx.Provide(app.NewDeleteUserHandler),
	fx.Provide(grpc.NewUserServiceHandler),
	fx.Invoke(grpc.RegisterRoutes),
	// Search (optional — no-op when Elasticsearch is disabled)
	fx.Provide(usersearch.NewIndexer),
	fx.Provide(usersearch.NewRepository),
	fx.Provide(fx.Annotate(
		provideSearchHandlers,
		fx.ResultTags(`group:"event_handlers"`),
	)),
	fx.Invoke(registerSearchLifecycle),
)

func provideSearchHandlers(ix *usersearch.Indexer) []events.HandlerRegistration {
	if ix == nil {
		return nil
	}
	return []events.HandlerRegistration{
		{Name: "search.user_created", Topic: domain.TopicUserCreated, HandlerFunc: ix.HandleUserCreated},
		{Name: "search.user_updated", Topic: domain.TopicUserUpdated, HandlerFunc: ix.HandleUserUpdated},
		{Name: "search.user_deleted", Topic: domain.TopicUserDeleted, HandlerFunc: ix.HandleUserDeleted},
	}
}

// registerSearchLifecycle registers the search index creation as an Fx lifecycle
// hook so it runs with the startup context (timeout-aware) and errors surface
// through the normal Fx startup failure path.
func registerSearchLifecycle(lc fx.Lifecycle, repo *usersearch.Repository) {
	if repo == nil {
		return
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.EnsureIndex(ctx)
		},
	})
}
