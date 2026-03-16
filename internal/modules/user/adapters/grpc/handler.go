package grpc

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	userv1 "github.com/gnha/golang-echo-boilerplate/gen/proto/user/v1"
	"github.com/gnha/golang-echo-boilerplate/gen/proto/user/v1/userv1connect"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/app"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/connectutil"
)

// UserServiceHandler implements the Connect RPC UserService.
type UserServiceHandler struct {
	createUser *app.CreateUserHandler
	getUser    *app.GetUserHandler
	listUsers  *app.ListUsersHandler
	updateUser *app.UpdateUserHandler
	deleteUser *app.DeleteUserHandler
}

// NewUserServiceHandler constructs the handler.
// Panics if any required dependency is nil.
func NewUserServiceHandler(
	createUser *app.CreateUserHandler,
	getUser *app.GetUserHandler,
	listUsers *app.ListUsersHandler,
	updateUser *app.UpdateUserHandler,
	deleteUser *app.DeleteUserHandler,
) *UserServiceHandler {
	if createUser == nil {
		panic("NewUserServiceHandler: createUser must not be nil")
	}
	if getUser == nil {
		panic("NewUserServiceHandler: getUser must not be nil")
	}
	if listUsers == nil {
		panic("NewUserServiceHandler: listUsers must not be nil")
	}
	if updateUser == nil {
		panic("NewUserServiceHandler: updateUser must not be nil")
	}
	if deleteUser == nil {
		panic("NewUserServiceHandler: deleteUser must not be nil")
	}
	return &UserServiceHandler{
		createUser: createUser,
		getUser:    getUser,
		listUsers:  listUsers,
		updateUser: updateUser,
		deleteUser: deleteUser,
	}
}

// Verify interface compliance.
var _ userv1connect.UserServiceHandler = (*UserServiceHandler)(nil)

func (h *UserServiceHandler) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
	slog.DebugContext(ctx, "grpc: CreateUser called", "module", "user", "operation", "CreateUser")
	user, err := h.createUser.Handle(ctx, app.CreateUserCmd{
		Email:    req.Msg.Email,
		Name:     req.Msg.Name,
		Password: req.Msg.Password,
		Role:     req.Msg.Role,
	})
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}
	return connect.NewResponse(&userv1.CreateUserResponse{User: toProto(user)}), nil
}

func (h *UserServiceHandler) GetUser(ctx context.Context, req *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
	slog.DebugContext(ctx, "grpc: GetUser called", "module", "user", "operation", "GetUser")
	user, err := h.getUser.Handle(ctx, req.Msg.Id)
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}
	return connect.NewResponse(&userv1.GetUserResponse{User: toProto(user)}), nil
}

func (h *UserServiceHandler) ListUsers(ctx context.Context, req *connect.Request[userv1.ListUsersRequest]) (*connect.Response[userv1.ListUsersResponse], error) {
	slog.DebugContext(ctx, "grpc: ListUsers called", "module", "user", "operation", "ListUsers")
	result, err := h.listUsers.Handle(ctx, int(req.Msg.Page), int(req.Msg.PageSize))
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}

	items := make([]*userv1.User, 0, len(result.Users))
	for _, u := range result.Users {
		items = append(items, toProto(u))
	}

	totalPages := result.TotalPages(result.PageSize)

	return connect.NewResponse(&userv1.ListUsersResponse{
		Items:      items,
		Total:      int64(result.Total),
		Page:       req.Msg.Page,
		PageSize:   int32(result.PageSize),
		TotalPages: int64(totalPages),
	}), nil
}

func (h *UserServiceHandler) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
	slog.DebugContext(ctx, "grpc: UpdateUser called", "module", "user", "operation", "UpdateUser")
	cmd := app.UpdateUserCmd{ID: req.Msg.Id}
	if req.Msg.Name != nil {
		cmd.Name = req.Msg.Name
	}
	if req.Msg.Role != nil {
		cmd.Role = req.Msg.Role
	}
	if req.Msg.Email != nil {
		cmd.Email = req.Msg.Email
	}

	user, err := h.updateUser.Handle(ctx, cmd)
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}
	return connect.NewResponse(&userv1.UpdateUserResponse{User: toProto(user)}), nil
}

func (h *UserServiceHandler) DeleteUser(ctx context.Context, req *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error) {
	slog.DebugContext(ctx, "grpc: DeleteUser called", "module", "user", "operation", "DeleteUser")
	if err := h.deleteUser.Handle(ctx, req.Msg.Id); err != nil {
		return nil, connectutil.DomainErrorToConnect(ctx, err)
	}
	return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
}
