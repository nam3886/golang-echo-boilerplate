package grpc

import (
	"context"

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
func NewUserServiceHandler(
	createUser *app.CreateUserHandler,
	getUser *app.GetUserHandler,
	listUsers *app.ListUsersHandler,
	updateUser *app.UpdateUserHandler,
	deleteUser *app.DeleteUserHandler,
) *UserServiceHandler {
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
	user, err := h.createUser.Handle(ctx, app.CreateUserCmd{
		Email:    req.Msg.Email,
		Name:     req.Msg.Name,
		Password: req.Msg.Password,
		Role:     req.Msg.Role,
	})
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(err)
	}
	return connect.NewResponse(&userv1.CreateUserResponse{User: toProto(user)}), nil
}

func (h *UserServiceHandler) GetUser(ctx context.Context, req *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
	user, err := h.getUser.Handle(ctx, req.Msg.Id)
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(err)
	}
	return connect.NewResponse(&userv1.GetUserResponse{User: toProto(user)}), nil
}

func (h *UserServiceHandler) ListUsers(ctx context.Context, req *connect.Request[userv1.ListUsersRequest]) (*connect.Response[userv1.ListUsersResponse], error) {
	result, err := h.listUsers.Handle(ctx, int(req.Msg.Page), int(req.Msg.PageSize))
	if err != nil {
		return nil, connectutil.DomainErrorToConnect(err)
	}

	items := make([]*userv1.User, 0, len(result.Users))
	for _, u := range result.Users {
		items = append(items, toProto(u))
	}

	pageSize := int(req.Msg.PageSize)
	if pageSize <= 0 {
		pageSize = 20 // matches app layer default
	}
	totalPages := result.TotalPages(pageSize)

	return connect.NewResponse(&userv1.ListUsersResponse{
		Items:      items,
		Total:      int64(result.Total),
		Page:       req.Msg.Page,
		PageSize:   req.Msg.PageSize,
		TotalPages: int64(totalPages),
	}), nil
}

func (h *UserServiceHandler) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
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
		return nil, connectutil.DomainErrorToConnect(err)
	}
	return connect.NewResponse(&userv1.UpdateUserResponse{User: toProto(user)}), nil
}

func (h *UserServiceHandler) DeleteUser(ctx context.Context, req *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error) {
	if err := h.deleteUser.Handle(ctx, req.Msg.Id); err != nil {
		return nil, connectutil.DomainErrorToConnect(err)
	}
	return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
}
