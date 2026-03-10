package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/mocks"
	"go.uber.org/mock/gomock"
)

func TestListUsersHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	users := []*domain.User{
		domain.Reconstitute("id-1", "a@example.com", "A", "hash", domain.RoleMember, time.Now(), time.Now(), nil),
		domain.Reconstitute("id-2", "b@example.com", "B", "hash", domain.RoleAdmin, time.Now(), time.Now(), nil),
	}
	mockRepo.EXPECT().
		List(gomock.Any(), 1, 20).
		Return(domain.ListResult{Users: users, Total: 5}, nil)

	handler := NewListUsersHandler(mockRepo)
	result, err := handler.Handle(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(result.Users))
	}
	if result.Total != 5 {
		t.Errorf("expected total=5, got %d", result.Total)
	}
}

func TestListUsersHandler_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		List(gomock.Any(), 1, 20).
		Return(domain.ListResult{Users: []*domain.User{}, Total: 0}, nil)

	handler := NewListUsersHandler(mockRepo)
	result, err := handler.Handle(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Users) != 0 {
		t.Errorf("expected 0 users, got %d", len(result.Users))
	}
	if result.Total != 0 {
		t.Errorf("expected total=0, got %d", result.Total)
	}
}

func TestListUsersHandler_DefaultPageSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// pageSize 0 should default to 20
	mockRepo.EXPECT().
		List(gomock.Any(), 1, 20).
		Return(domain.ListResult{}, nil)

	handler := NewListUsersHandler(mockRepo)
	_, err := handler.Handle(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestListUsersHandler_PageSizeCappedAt100(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// pageSize > 100 should be capped at 100
	mockRepo.EXPECT().
		List(gomock.Any(), 1, 100).
		Return(domain.ListResult{}, nil)

	handler := NewListUsersHandler(mockRepo)
	_, err := handler.Handle(context.Background(), 1, 200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestListUsersHandler_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		List(gomock.Any(), 1, 20).
		Return(domain.ListResult{}, fmt.Errorf("db error"))

	handler := NewListUsersHandler(mockRepo)
	_, err := handler.Handle(context.Background(), 1, 20)
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}
