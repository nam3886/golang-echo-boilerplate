package domain

import "github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"

// Re-export event topics and types from shared contracts so user module code
// can import from its own domain package. This is the canonical import path
// within the user module. Other modules (audit, notification) MUST import
// from contracts/ directly — never from another module's domain/.
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

// EventSchemaVersion is the current event schema version for user domain events.
const EventSchemaVersion = contracts.UserEventSchemaVersion
