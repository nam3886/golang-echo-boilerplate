package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/gnha/gnha-services/internal/shared/mocks"
	"go.uber.org/mock/gomock"
)

func TestUpdateUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	user := domain.Reconstitute("user-id-1", "u@example.com", "Old Name", "hash", domain.RoleMember, time.Now(), time.Now(), nil)

	// Update calls the fn with the fetched user, fn mutates it in place.
	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("user-id-1"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			return fn(user)
		})

	newName := "New Name"
	bus := &stubEventPublisher{}
	handler := NewUpdateUserHandler(mockRepo, bus)

	got, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "user-id-1",
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Name() != "New Name" {
		t.Errorf("expected name New Name, got %s", got.Name())
	}
	if bus.topic != "user.updated" {
		t.Errorf("expected event topic user.updated, got %s", bus.topic)
	}
}

func TestUpdateUserHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("missing-id"), gomock.Any()).
		Return(domainerr.ErrNotFound())

	newName := "New Name"
	bus := &stubEventPublisher{}
	handler := NewUpdateUserHandler(mockRepo, bus)

	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "missing-id",
		Name: &newName,
	})
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	var domErr *domainerr.DomainError
	if !errors.As(err, &domErr) || domErr.Code != domainerr.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}
}

func TestUpdateUserHandler_InvalidRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	user := domain.Reconstitute("user-id-1", "u@example.com", "Name", "hash", domain.RoleMember, time.Now(), time.Now(), nil)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("user-id-1"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			return fn(user)
		})

	invalidRole := "superadmin"
	bus := &stubEventPublisher{}
	handler := NewUpdateUserHandler(mockRepo, bus)

	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "user-id-1",
		Role: &invalidRole,
	})
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
}

func TestUpdateUserHandler_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("user-id-1"), gomock.Any()).
		Return(fmt.Errorf("db error"))

	newName := "New Name"
	bus := &stubEventPublisher{}
	handler := NewUpdateUserHandler(mockRepo, bus)

	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "user-id-1",
		Name: &newName,
	})
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}

func TestUpdateUserHandler_EventPublishFailureDoesNotFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	user := domain.Reconstitute("user-id-1", "u@example.com", "Name", "hash", domain.RoleMember, time.Now(), time.Now(), nil)
	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("user-id-1"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			return fn(user)
		})

	newName := "New Name"
	bus := &failEventPublisher{}
	handler := NewUpdateUserHandler(mockRepo, bus)

	// Event publish failure is logged but does not propagate to caller.
	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "user-id-1",
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("expected no error even when event publish fails, got %v", err)
	}
}

// stubEventPublisher captures the last published topic.
type stubEventPublisher struct {
	topic string
}

func (s *stubEventPublisher) Publish(_ context.Context, topic string, _ any) error {
	s.topic = topic
	return nil
}

// failEventPublisher always fails on Publish.
type failEventPublisher struct{}

func (f *failEventPublisher) Publish(_ context.Context, _ string, _ any) error {
	return fmt.Errorf("event bus unavailable")
}
