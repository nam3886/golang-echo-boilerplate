package auth

import (
	"github.com/gnha/golang-echo-boilerplate/internal/modules/auth/adapters/grpc"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/auth/app"
	"go.uber.org/fx"
)

// Module provides the auth module to the Fx container.
var Module = fx.Module("auth",
	fx.Provide(app.NewLoginHandler),
	fx.Provide(app.NewLogoutHandler),
	fx.Provide(grpc.NewAuthServiceHandler),
	fx.Invoke(grpc.RegisterRoutes),
)
