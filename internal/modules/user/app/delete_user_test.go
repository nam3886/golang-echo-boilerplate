package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/mocks"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
	"go.uber.org/mock/gomock"
)

// deletedUserFixture returns a reconstituted user for SoftDelete mock returns.
func deletedUserFixture() *domain.User {
	now := time.Now()
	return domain.Reconstitute(
		"user-id-1", "user@example.com", "Test User", "hashed",
		domain.RoleMember, now, now, &now,
	)
}

func TestDeleteUserHandler_EmptyID_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewDeleteUserHandler(mockRepo, bus)
	err := h.Handle(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestDeleteUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("user-id-1")).
		Return(deletedUserFixture(), nil)

	recorder := &testutil.CapturingPublisher{}
	bus := events.NewEventBus(recorder)
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(memberCtx("user-id-1"), "user-id-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(recorder.Messages) == 0 || recorder.Messages[0].Topic != domain.TopicUserDeleted {
		got := ""
		if len(recorder.Messages) > 0 {
			got = recorder.Messages[0].Topic
		}
		t.Errorf("expected event topic %s, got %s", domain.TopicUserDeleted, got)
	}
}

func TestDeleteUserHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("missing-id")).
		Return(nil, domain.ErrUserNotFound())

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(memberCtx("missing-id"), "missing-id")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	if !errors.Is(err, domain.ErrUserNotFound()) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestDeleteUserHandler_AlreadyDeleted verifies that re-deleting an already-deleted
// user surfaces ErrNotFound (SoftDelete WHERE deleted_at IS NULL finds no row).
func TestDeleteUserHandler_AlreadyDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("already-deleted-id")).
		Return(nil, domain.ErrUserNotFound())

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(memberCtx("already-deleted-id"), "already-deleted-id")
	if err == nil {
		t.Fatal("expected error for already-deleted user")
	}
	if !errors.Is(err, domain.ErrUserNotFound()) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteUserHandler_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("user-id-1")).
		Return(nil, fmt.Errorf("db error"))

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(memberCtx("user-id-1"), "user-id-1")
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}

// TestDeleteUserHandler_Forbidden_NonOwner verifies that a caller who is neither
// the owner nor has user:delete permission receives ErrForbidden.
func TestDeleteUserHandler_Forbidden_NonOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	// caller is "other-user-id", target is "user-id-1"
	ctx := auth.WithUser(context.Background(), &auth.TokenClaims{
		UserID:      "other-user-id",
		Role:        "member",
		Permissions: []string{"user:read"},
	})
	err := handler.Handle(ctx, "user-id-1")
	if err == nil {
		t.Fatal("expected ErrForbidden for non-owner without user:delete")
	}
	if !errors.Is(err, sharederr.ErrForbidden()) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestDeleteUserHandler_EventPublishFailureDoesNotFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("user-id-1")).
		Return(deletedUserFixture(), nil)

	bus := events.NewEventBus(&testutil.FailPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	// Event publish failure is logged but must not propagate.
	err := handler.Handle(memberCtx("user-id-1"), "user-id-1")
	if err != nil {
		t.Fatalf("expected no error even when event publish fails, got %v", err)
	}
}
