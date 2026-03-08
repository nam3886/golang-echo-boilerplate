package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/gnha/gnha-services/internal/shared/events"
	"github.com/gnha/gnha-services/internal/shared/mocks"
	"github.com/gnha/gnha-services/internal/shared/testutil"
	"go.uber.org/mock/gomock"
)

func TestDeleteUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("user-id-1")).
		Return(nil)

	recorder := &testutil.CapturingPublisher{}
	bus := events.NewEventBus(recorder)
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(context.Background(), "user-id-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if recorder.Topic != domain.TopicUserDeleted {
		t.Errorf("expected event topic %s, got %s", domain.TopicUserDeleted, recorder.Topic)
	}
}

func TestDeleteUserHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("missing-id")).
		Return(domainerr.ErrNotFound())

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(context.Background(), "missing-id")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	var domErr *domainerr.DomainError
	if !errors.As(err, &domErr) || domErr.Code != domainerr.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}
}

func TestDeleteUserHandler_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("user-id-1")).
		Return(fmt.Errorf("db error"))

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	err := handler.Handle(context.Background(), "user-id-1")
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}

func TestDeleteUserHandler_EventPublishFailureDoesNotFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	mockRepo.EXPECT().
		SoftDelete(gomock.Any(), domain.UserID("user-id-1")).
		Return(nil)

	bus := events.NewEventBus(&testutil.FailPublisher{})
	handler := NewDeleteUserHandler(mockRepo, bus)

	// Event publish failure is logged but must not propagate.
	err := handler.Handle(context.Background(), "user-id-1")
	if err != nil {
		t.Fatalf("expected no error even when event publish fails, got %v", err)
	}
}
