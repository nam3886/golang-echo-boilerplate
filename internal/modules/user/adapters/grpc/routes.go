package grpc

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/gnha/gnha-services/gen/proto/user/v1/userv1connect"
	"github.com/gnha/gnha-services/internal/shared/config"
	appmw "github.com/gnha/gnha-services/internal/shared/middleware"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RegisterRoutes mounts the Connect RPC UserService handler on Echo with auth.
func RegisterRoutes(e *echo.Echo, handler *UserServiceHandler, cfg *config.Config, rdb *redis.Client) {
	path, h := userv1connect.NewUserServiceHandler(handler,
		connect.WithInterceptors(
			appmw.RBACInterceptor(),
			validate.NewInterceptor(),
		),
	)

	// Mount Connect handler under auth + base RBAC (user:read).
	// Write/delete permissions enforced by RBACInterceptor per procedure.
	g := e.Group(path, appmw.Auth(cfg, rdb), appmw.RequirePermission(appmw.PermUserRead))
	g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
