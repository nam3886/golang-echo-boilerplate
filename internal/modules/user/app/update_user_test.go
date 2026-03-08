package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/events"
	"github.com/gnha/gnha-services/internal/shared/mocks"
	"github.com/gnha/gnha-services/internal/shared/testutil"
	"go.uber.org/mock/gomock"
)

func makeTestUser() *domain.User {
	return domain.Reconstitute(
		"00000000-0000-0000-0000-000000000001",
		"user@example.com",
		"Original Name",
		"hashed_pwd",
		domain.RoleMember,
		time.Now(),
		time.Now(),
		nil,
	)
}

func TestUpdateUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			return fn(u)
		})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	user, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: testutil.Ptr("Updated Name"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Name() != "Updated Name" {
		t.Errorf("expected name Updated Name, got %s", user.Name())
	}
}

func TestUpdateUserHandler_RepoFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("db error"))

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: testutil.Ptr("New Name"),
	})
	if err == nil {
		t.Fatal("expected error from repo failure")
	}
}

// TestUpdateUserHandler_RoleOnlyUpdate verifies that setting only Role (no Name)
// applies the role change without affecting the name.
func TestUpdateUserHandler_RoleOnlyUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			return fn(u)
		})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	user, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "00000000-0000-0000-0000-000000000001",
		Role: testutil.Ptr("admin"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Role() != domain.RoleAdmin {
		t.Errorf("expected role admin, got %s", user.Role())
	}
	// Name must remain unchanged
	if user.Name() != "Original Name" {
		t.Errorf("expected name Original Name, got %s", user.Name())
	}
}

// TestUpdateUserHandler_EmptyName_ReturnsError verifies that passing an empty
// string as Name is rejected by the domain and surfaces as an error.
func TestUpdateUserHandler_EmptyName_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			return fn(u)
		})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: testutil.Ptr(""), // empty string must be rejected
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !errors.Is(err, domain.ErrNameRequired()) {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestUpdateUserHandler_EmailChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			return fn(u)
		})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	user, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:    "00000000-0000-0000-0000-000000000001",
		Email: testutil.Ptr("new@example.com"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Email() != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", user.Email())
	}
}

func TestUpdateUserHandler_InvalidEmail_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			return fn(u)
		})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	_, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:    "00000000-0000-0000-0000-000000000001",
		Email: testutil.Ptr("not-an-email"),
	})
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
	if !errors.Is(err, domain.ErrInvalidEmail()) {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}
}

// TestUpdateUserHandler_EventPublishFailure_DoesNotFail verifies that a publish
// error is logged but does not cause the handler to return an error.
func TestUpdateUserHandler_EventPublishFailure_DoesNotFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			return fn(u)
		})

	bus := events.NewEventBus(&testutil.FailPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	user, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: testutil.Ptr("New Name"),
	})
	if err != nil {
		t.Fatalf("expected no error even when publish fails, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
}
