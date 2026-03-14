package grpc

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	authv1 "github.com/gnha/golang-echo-boilerplate/gen/proto/auth/v1"
	"github.com/gnha/golang-echo-boilerplate/gen/proto/auth/v1/authv1connect"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/auth/app"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/connectutil"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// AuthServiceHandler implements the Connect RPC AuthService.
type AuthServiceHandler struct {
	login  *app.LoginHandler
	logout *app.LogoutHandler
	cfg    *config.Config
}

// NewAuthServiceHandler constructs the handler.
func NewAuthServiceHandler(login *app.LoginHandler, logout *app.LogoutHandler, cfg *config.Config) *AuthServiceHandler {
	return &AuthServiceHandler{login: login, logout: logout, cfg: cfg}
}

// Verify interface compliance at compile time.
var _ authv1connect.AuthServiceHandler = (*AuthServiceHandler)(nil)

// Login authenticates a user and returns a token pair.
// This endpoint is public — no auth middleware is applied at the route level.
func (h *AuthServiceHandler) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	result, err := h.login.Handle(ctx, app.LoginCmd{
		Email:    req.Msg.Email,
		Password: req.Msg.Password,
	})
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}
	return connect.NewResponse(&authv1.LoginResponse{
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
	}), nil
}

// Logout revokes the caller's current access token.
// Auth is validated manually here since the entire service is mounted without middleware.
func (h *AuthServiceHandler) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	token := extractBearerToken(req.Header().Get("Authorization"))
	if token == "" {
		return nil, connectutil.DomainErrorToConnect(ctx, sharederr.ErrUnauthorized())
	}

	claims, err := auth.ValidateAccessToken(h.cfg, token)
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, sharederr.ErrUnauthorized())
	}

	if err := h.logout.Handle(ctx, claims); err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}
	return connect.NewResponse(&authv1.LogoutResponse{}), nil
}

func extractBearerToken(header string) string {
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return header[7:]
	}
	return ""
}
