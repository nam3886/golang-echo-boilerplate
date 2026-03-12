package grpc

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/gnha/golang-echo-boilerplate/gen/proto/user/v1/userv1connect"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	appmw "github.com/gnha/golang-echo-boilerplate/internal/shared/middleware"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// userProcedurePerms maps each UserService procedure to its required permission.
// Defined here (in the user module's grpc adapter) so rbac_interceptor.go has no
// cross-module import while still enforcing per-procedure RBAC.
var userProcedurePerms = map[string]appmw.Permission{
	userv1connect.UserServiceGetUserProcedure:    appmw.PermUserRead,
	userv1connect.UserServiceListUsersProcedure:  appmw.PermUserRead,
	userv1connect.UserServiceCreateUserProcedure: appmw.PermUserWrite,
	userv1connect.UserServiceUpdateUserProcedure: appmw.PermUserWrite,
	userv1connect.UserServiceDeleteUserProcedure: appmw.PermUserDelete,
}

// RegisterRoutes mounts the Connect RPC UserService handler on Echo with auth.
func RegisterRoutes(e *echo.Echo, handler *UserServiceHandler, cfg *config.Config, rdb *redis.Client) {
	path, h := userv1connect.NewUserServiceHandler(handler,
		connect.WithInterceptors(
			appmw.RBACInterceptor(userProcedurePerms),
			validate.NewInterceptor(),
		),
	)

	// Mount Connect handler under auth. All permission checks are handled
	// by RBACInterceptor per procedure (fail-closed).
	g := e.Group(path, appmw.Auth(cfg, rdb))
	g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
