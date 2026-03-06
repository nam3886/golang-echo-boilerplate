package app

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/gnha/gnha-services/internal/shared/events"
	"github.com/gnha/gnha-services/internal/shared/mocks"
	"go.uber.org/mock/gomock"
)

// stubHasher returns the password as-is (no real hashing in tests).
type stubHasher struct{}

func (s *stubHasher) Hash(password string) (string, error) { return "hashed_" + password, nil }
func (s *stubHasher) Verify(_, _ string) (bool, error)     { return true, nil }

func TestCreateUserHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockUserRepository(ctrl)

	// GetByEmail returns not found (email available)
	mockRepo.EXPECT().
		GetByEmail(gomock.Any(), "new@example.com").
		Return(nil, sharederr.ErrNotFound)

	// Create succeeds
	mockRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	bus := events.NewEventBus(&noopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &stubHasher{}, bus)

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

	bus := events.NewEventBus(&noopPublisher{})
	handler := NewCreateUserHandler(mockRepo, &stubHasher{}, bus)

	_, err := handler.Handle(context.Background(), CreateUserCmd{
		Email:    "taken@example.com",
		Name:     "Another User",
		Password: "secret123",
		Role:     "member",
	})
	if err == nil {
		t.Fatal("expected error for taken email")
	}
	if err != domain.ErrEmailTaken {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

// noopPublisher is a Watermill publisher that discards all messages.
type noopPublisher struct{}

func (p *noopPublisher) Publish(topic string, messages ...*message.Message) error { return nil }
func (p *noopPublisher) Close() error                                             { return nil }
