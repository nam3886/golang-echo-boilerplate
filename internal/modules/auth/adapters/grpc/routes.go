package grpc

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/gnha/golang-echo-boilerplate/gen/proto/auth/v1/authv1connect"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes mounts the Connect RPC AuthService handler on Echo without auth middleware.
// Login is public. Logout validates the token manually inside the handler.
func RegisterRoutes(e *echo.Echo, handler *AuthServiceHandler) {
	path, h := authv1connect.NewAuthServiceHandler(handler,
		connect.WithInterceptors(
			validate.NewInterceptor(),
		),
	)

	// No auth middleware — Login must be publicly accessible.
	// Logout performs its own token validation inline.
	g := e.Group(path)
	g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
