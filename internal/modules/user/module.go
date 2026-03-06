package user

import (
	"github.com/gnha/gnha-services/internal/modules/user/adapters/grpc"
	"github.com/gnha/gnha-services/internal/modules/user/adapters/postgres"
	"github.com/gnha/gnha-services/internal/modules/user/app"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
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
)
