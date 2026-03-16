// Package contracts defines shared event types and topic constants.
// Modules subscribe to these contracts instead of importing other modules'
// domain packages, preserving the "no cross-module imports" rule.
package contracts

import "time"

// UserEventSchemaVersion is the current event schema version for user domain events.
// Increment when making breaking changes; deploy subscribers before publishers.
const UserEventSchemaVersion = "v1"

// User event topics.
const (
	TopicUserCreated = "user.created"
	TopicUserUpdated = "user.updated"
	TopicUserDeleted = "user.deleted"
)

// UserCreatedEvent is published when a user is created.
type UserCreatedEvent struct {
	// EventID is a UUID v4 uniquely identifying this event for deduplication.
	// Subscribers MUST use this field to detect and skip replays.
	EventID string `json:"event_id"`
	// Version is the schema version (e.g. "v1"). Breaking changes require a new version.
	Version   string    `json:"version"`
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
	// EventID is a UUID v4 uniquely identifying this event for deduplication.
	EventID string `json:"event_id"`
	// Version is the schema version.
	Version       string    `json:"version"`
	UserID        string    `json:"user_id"`
	ActorID       string    `json:"actor_id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	ChangedFields []string  `json:"changed_fields,omitempty"`
	IPAddress     string    `json:"ip_address,omitempty"`
	At            time.Time `json:"at"`
}

// UserDeletedEvent is published when a user is soft-deleted.
type UserDeletedEvent struct {
	// EventID is a UUID v4 uniquely identifying this event for deduplication.
	EventID string `json:"event_id"`
	// Version is the schema version.
	Version   string    `json:"version"`
	UserID    string    `json:"user_id"`
	ActorID   string    `json:"actor_id"`
	IPAddress string    `json:"ip_address,omitempty"`
	At        time.Time `json:"at"`
}

// Auth event topics.
const (
	TopicUserLoggedIn  = "user.logged_in"
	TopicUserLoggedOut = "user.logged_out"
)

// UserLoggedInEvent is published when a user successfully authenticates.
type UserLoggedInEvent struct {
	// EventID is a UUID v4 uniquely identifying this event for deduplication.
	EventID string `json:"event_id"`
	// Version is the schema version.
	Version   string    `json:"version"`
	UserID    string    `json:"user_id"`
	IPAddress string    `json:"ip_address,omitempty"`
	At        time.Time `json:"at"`
}

// UserLoggedOutEvent is published when a user's token is revoked.
type UserLoggedOutEvent struct {
	// EventID is a UUID v4 uniquely identifying this event for deduplication.
	EventID string `json:"event_id"`
	// Version is the schema version.
	Version   string    `json:"version"`
	UserID    string    `json:"user_id"`
	TokenID   string    `json:"token_id"`
	IPAddress string    `json:"ip_address,omitempty"`
	At        time.Time `json:"at"`
}
