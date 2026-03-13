package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	userv1 "github.com/gnha/golang-echo-boilerplate/gen/proto/user/v1"
	grpcadapter "github.com/gnha/golang-echo-boilerplate/internal/modules/user/adapters/grpc"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/app"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/mocks"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
	"go.uber.org/mock/gomock"
)

func makeUser(id, email, name string, role domain.Role) *domain.User {
	return domain.Reconstitute(
		domain.UserID(id), email, name, "hashed_pwd", role,
		time.Now(), time.Now(), nil,
	)
}

func buildTestHandlers(t *testing.T, ctrl *gomock.Controller) (*mocks.MockUserRepository, *grpcadapter.UserServiceHandler) {
	t.Helper()
	mockRepo := mocks.NewMockUserRepository(ctrl)
	bus := events.NewEventBus(&testutil.NoopPublisher{})

	create := app.NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)
	get := app.NewGetUserHandler(mockRepo)
	list := app.NewListUsersHandler(mockRepo)
	update := app.NewUpdateUserHandler(mockRepo, bus)
	del := app.NewDeleteUserHandler(mockRepo, bus)

	h := grpcadapter.NewUserServiceHandler(create, get, list, update, del)
	return mockRepo, h
}

func TestHandler_CreateUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo, h := buildTestHandlers(t, ctrl)

	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "new@example.com").
		Return(nil, sharederr.ErrNotFound())
	mockRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	resp, err := h.CreateUser(context.Background(), connect.NewRequest(&userv1.CreateUserRequest{
		Email:    "new@example.com",
		Name:     "New User",
		Password: "secret123",
		Role:     "member",
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Msg.User.Email != "new@example.com" {
		t.Errorf("expected email=new@example.com, got %s", resp.Msg.User.Email)
	}
	if resp.Msg.User.Name != "New User" {
		t.Errorf("expected name=New User, got %s", resp.Msg.User.Name)
	}
}

func TestHandler_GetUser_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo, h := buildTestHandlers(t, ctrl)

	mockRepo.EXPECT().
		GetByID(gomock.Any(), domain.UserID("missing-id")).
		Return(nil, sharederr.ErrNotFound())

	_, err := h.GetUser(context.Background(), connect.NewRequest(&userv1.GetUserRequest{
		Id: "missing-id",
	}))
	if err == nil {
		t.Fatal("expected connect error, got nil")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", ce.Code())
	}
}

func TestHandler_ListUsers_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo, h := buildTestHandlers(t, ctrl)

	u1 := makeUser("id-1", "a@example.com", "Alice", domain.RoleMember)
	u2 := makeUser("id-2", "b@example.com", "Bob", domain.RoleViewer)

	mockRepo.EXPECT().
		List(gomock.Any(), 1, 2).
		Return(domain.ListResult{
			Users: []*domain.User{u1, u2},
			Total: 5,
		}, nil)

	resp, err := h.ListUsers(context.Background(), connect.NewRequest(&userv1.ListUsersRequest{
		Page:     1,
		PageSize: 2,
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Msg.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Msg.Items))
	}
	if resp.Msg.Total != 5 {
		t.Errorf("expected total=5, got %d", resp.Msg.Total)
	}
	if resp.Msg.Page != 1 {
		t.Errorf("expected page=1, got %d", resp.Msg.Page)
	}
	if resp.Msg.TotalPages != 3 {
		t.Errorf("expected totalPages=3, got %d", resp.Msg.TotalPages)
	}
}

func TestHandler_UpdateUser_PartialFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo, h := buildTestHandlers(t, ctrl)

	existing := makeUser("id-1", "user@example.com", "Original", domain.RoleMember)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("id-1"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			return fn(existing)
		})

	newName := "Updated Name"
	resp, err := h.UpdateUser(context.Background(), connect.NewRequest(&userv1.UpdateUserRequest{
		Id:   "id-1",
		Name: &newName,
		// Email and Role intentionally omitted (nil)
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Msg.User.Name != "Updated Name" {
		t.Errorf("expected name=Updated Name, got %s", resp.Msg.User.Name)
	}
	// Email unchanged since it was not set in request
	if resp.Msg.User.Email != "user@example.com" {
		t.Errorf("expected email unchanged=user@example.com, got %s", resp.Msg.User.Email)
	}
}

func TestHandler_DeleteUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo, h := buildTestHandlers(t, ctrl)

	now := time.Now()
	deleted := domain.Reconstitute(
		domain.UserID("id-del"), "gone@example.com", "Gone User", "hashed_pwd", domain.RoleMember,
		now, now, &now, // deletedAt set so IsDeleted() == true
	)
	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("id-del")).
		Return(deleted, nil)

	resp, err := h.DeleteUser(context.Background(), connect.NewRequest(&userv1.DeleteUserRequest{
		Id: "id-del",
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestHandler_DeleteUser_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo, h := buildTestHandlers(t, ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("no-such-id")).
		Return(nil, sharederr.ErrNotFound())

	_, err := h.DeleteUser(context.Background(), connect.NewRequest(&userv1.DeleteUserRequest{
		Id: "no-such-id",
	}))
	if err == nil {
		t.Fatal("expected connect error, got nil")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", ce.Code())
	}
}
