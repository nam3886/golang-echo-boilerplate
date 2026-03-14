package auth

import (
	"github.com/gnha/golang-echo-boilerplate/internal/modules/auth/adapters/grpc"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/auth/app"
	sharedauth "github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"go.uber.org/fx"
)

// Module provides the auth module to the Fx container.
var Module = fx.Module("auth",
	fx.Provide(
		fx.Annotate(
			sharedauth.NewRedisBlacklister,
			fx.As(new(sharedauth.Blacklister)),
		),
	),
	fx.Provide(app.NewLoginHandler),
	fx.Provide(app.NewLogoutHandler),
	fx.Provide(grpc.NewAuthServiceHandler),
	fx.Invoke(grpc.RegisterRoutes),
)
