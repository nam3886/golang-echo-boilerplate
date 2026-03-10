package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/mocks"
	"go.uber.org/mock/gomock"
)

func TestGetUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	user := domain.Reconstitute("user-id-1", "test@example.com", "Test User", "hashed", domain.RoleMember, time.Now(), time.Now(), nil)
	mockRepo.EXPECT().
		GetByID(gomock.Any(), domain.UserID("user-id-1")).
		Return(user, nil)

	handler := NewGetUserHandler(mockRepo)
	got, err := handler.Handle(context.Background(), "user-id-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Email() != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", got.Email())
	}
}

func TestGetUserHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByID(gomock.Any(), domain.UserID("missing-id")).
		Return(nil, sharederr.ErrNotFound())

	handler := NewGetUserHandler(mockRepo)
	_, err := handler.Handle(context.Background(), "missing-id")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestGetUserHandler_EmptyID_ReturnsError(t *testing.T) {
	h := NewGetUserHandler(nil)
	_, err := h.Handle(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestGetUserHandler_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByID(gomock.Any(), domain.UserID("user-id-1")).
		Return(nil, fmt.Errorf("db connection error"))

	handler := NewGetUserHandler(mockRepo)
	_, err := handler.Handle(context.Background(), "user-id-1")
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}
