package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/mocks"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
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

// TestUpdateUserHandler_NoFieldsProvided verifies the fast-path: when no fields
// are provided, GetByID is called instead of Update and the user is returned unchanged.
func TestUpdateUserHandler_NoFieldsProvided(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByID(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001")).
		Return(makeTestUser(), nil)

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewUpdateUserHandler(mockRepo, bus)

	user, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID: "00000000-0000-0000-0000-000000000001",
		// Name, Role, Email all nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Name() != "Original Name" {
		t.Errorf("expected unchanged name %q, got %q", "Original Name", user.Name())
	}
}

// TestUpdateUserHandler_SameValues_NoEvent verifies that when all provided field
// values match the current state, no event is published and the user is returned.
func TestUpdateUserHandler_SameValues_NoEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// Simulate real repo behaviour: fn returns ErrNoChange → repo commits read-tx and returns nil.
	mockRepo.EXPECT().
		Update(gomock.Any(), domain.UserID("00000000-0000-0000-0000-000000000001"), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ domain.UserID, fn func(*domain.User) error) error {
			u := makeTestUser()
			if err := fn(u); err != nil {
				if errors.Is(err, sharederr.ErrNoChange()) {
					return nil // mirrors real repo: commit read-tx, no SQL UPDATE
				}
				return err
			}
			return nil
		})

	recorder := &testutil.CapturingPublisher{}
	bus := events.NewEventBus(recorder)
	handler := NewUpdateUserHandler(mockRepo, bus)

	user, err := handler.Handle(context.Background(), UpdateUserCmd{
		ID:   "00000000-0000-0000-0000-000000000001",
		Name: testutil.Ptr("Original Name"), // same as fixture — no mutation
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if len(recorder.Messages) > 0 {
		t.Error("expected no event for same-value update")
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
