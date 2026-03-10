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

func TestCreateUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// GetByEmail returns not found (email available)
	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "new@example.com").
		Return(nil, sharederr.ErrNotFound())

	// Create succeeds
	mockRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	user, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "new@example.com",
		Name:     "New User",
		Password: "secret123",
		Role:     "member",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Email() != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", user.Email())
	}
	if user.Name() != "New User" {
		t.Errorf("expected name New User, got %s", user.Name())
	}
}

func TestCreateUserHandler_EmailTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// GetByEmail returns existing user (email taken)
	existing := domain.Reconstitute("00000000-0000-0000-0000-000000000001", "taken@example.com", "Existing", "pwd", domain.RoleMember, time.Now(), time.Now(), nil)
	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "taken@example.com").
		Return(existing, nil)

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	_, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "taken@example.com",
		Name:     "Another User",
		Password: "secret123",
		Role:     "member",
	})
	if err == nil {
		t.Fatal("expected error for taken email")
	}
	if !errors.Is(err, domain.ErrEmailTaken()) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestCreateUserHandler_InvalidRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// Email check returns not found (role validation happens after)
	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "user@example.com").
		Return(nil, sharederr.ErrNotFound())

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	_, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "user@example.com",
		Name:     "User",
		Password: "secret123",
		Role:     "superadmin", // not a valid role
	})
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !errors.Is(err, domain.ErrInvalidRole()) {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestCreateUserHandler_HasherFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "user@example.com").
		Return(nil, sharederr.ErrNotFound())

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.FailHasher{}, bus)

	_, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "user@example.com",
		Name:     "User",
		Password: "secret123",
		Role:     "member",
	})
	if err == nil {
		t.Fatal("expected error from hasher failure")
	}
}

func TestCreateUserHandler_RepoCreateFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "user@example.com").
		Return(nil, sharederr.ErrNotFound())

	mockRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("db connection lost"))

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	_, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "user@example.com",
		Name:     "User",
		Password: "secret123",
		Role:     "member",
	})
	if err == nil {
		t.Fatal("expected error from repo failure")
	}
}

func TestCreateUserHandler_PublishesEventOnSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "user@example.com").
		Return(nil, sharederr.ErrNotFound())

	mockRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	recorder := &testutil.CapturingPublisher{}
	bus := events.NewEventBus(recorder)
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	user, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "user@example.com",
		Name:     "User",
		Password: "secret123",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if len(recorder.Messages) == 0 || recorder.Messages[0].Topic != domain.TopicUserCreated {
		got := ""
		if len(recorder.Messages) > 0 {
			got = recorder.Messages[0].Topic
		}
		t.Errorf("expected topic %s, got %s", domain.TopicUserCreated, got)
	}
}

// TestCreateUserHandler_EventPublishFailure_DoesNotFail verifies that a publish
// error is logged but does not cause the handler to return an error (fire-and-forget).
func TestCreateUserHandler_EventPublishFailure_DoesNotFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "user@example.com").
		Return(nil, sharederr.ErrNotFound())

	mockRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	bus := events.NewEventBus(&testutil.FailPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	user, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "user@example.com",
		Name:     "User",
		Password: "secret123",
		Role:     "member",
	})
	if err != nil {
		t.Fatalf("expected no error even when publish fails, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
}

// TestCreateUserHandler_GetByEmailDBError verifies that a DB error on the
// email-uniqueness check propagates as an error (not silenced).
func TestCreateUserHandler_GetByEmailDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "user@example.com").
		Return(nil, fmt.Errorf("connection refused"))

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)

	_, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "user@example.com",
		Name:     "User",
		Password: "secret123",
		Role:     "member",
	})
	if err == nil {
		t.Fatal("expected error from DB failure on GetByEmail")
	}
}
