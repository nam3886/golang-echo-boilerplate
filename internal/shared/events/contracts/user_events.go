// Package contracts defines shared event types and topic constants.
// Modules subscribe to these contracts instead of importing other modules'
// domain packages, preserving the "no cross-module imports" rule.
package contracts

import "time"

// User event topics.
const (
	TopicUserCreated = "user.created"
	TopicUserUpdated = "user.updated"
	TopicUserDeleted = "user.deleted"
)

// UserCreatedEvent is published when a user is created.
type UserCreatedEvent struct {
	UserID    string    `json:"user_id"`
	ActorID   string    `json:"actor_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	IPAddress string    `json:"ip_address,omitempty"`
	At        time.Time `json:"at"`
}

// UserUpdatedEvent is published when a user is updated.
type UserUpdatedEvent struct {
	UserID    string    `json:"user_id"`
	ActorID   string    `json:"actor_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IPAddress string    `json:"ip_address,omitempty"`
	At        time.Time `json:"at"`
}

// UserDeletedEvent is published when a user is soft-deleted.
type UserDeletedEvent struct {
	UserID    string    `json:"user_id"`
	ActorID   string    `json:"actor_id"`
	IPAddress string    `json:"ip_address,omitempty"`
	At        time.Time `json:"at"`
}
