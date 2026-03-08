package domain

import "github.com/gnha/gnha-services/internal/shared/events/contracts"

// Re-export event topics from shared contracts so existing user module code
// compiles unchanged. External modules (audit, notification) should import
// from contracts directly to avoid cross-module coupling.
const (
	TopicUserCreated = contracts.TopicUserCreated
	TopicUserUpdated = contracts.TopicUserUpdated
	TopicUserDeleted = contracts.TopicUserDeleted
)

// UserCreatedEvent is re-exported from shared contracts.
type UserCreatedEvent = contracts.UserCreatedEvent

// UserUpdatedEvent is re-exported from shared contracts.
type UserUpdatedEvent = contracts.UserUpdatedEvent

// UserDeletedEvent is re-exported from shared contracts.
type UserDeletedEvent = contracts.UserDeletedEvent
