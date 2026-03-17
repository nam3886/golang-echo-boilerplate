// Package search handles Elasticsearch indexing for user documents.
//
// Failure mode: Elasticsearch unavailability.
// All indexer methods log errors and return nil (fire-and-forget).
// Search results degrade but CRUD operations continue unaffected.
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharedsearch "github.com/gnha/golang-echo-boilerplate/internal/shared/search"
)

// Indexer handles indexing user documents into Elasticsearch.
// Required: client (nil client -> nil Indexer returned by constructor)
type Indexer struct {
	client    *sharedsearch.Client
	indexName string
}

// NewIndexer creates a user search indexer. Returns nil if client is nil.
func NewIndexer(client *sharedsearch.Client) *Indexer {
	if client == nil {
		return nil
	}
	return &Indexer{
		client:    client,
		indexName: client.IndexName(UsersIndexSuffix),
	}
}

// HandleUserCreated indexes a full user document on creation.
// Idempotent: ES Index with explicit document ID is an upsert.
func (ix *Indexer) HandleUserCreated(msg *message.Message) error {
	if ix == nil {
		return nil
	}

	var ev domain.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "search: failed to unmarshal user.created event",
			"module", "search", "operation", "HandleUserCreated",
			"error_code", "unmarshal_failed", "retryable", false, "err", err)
		return nil // ack — schema mismatch won't be fixed by retrying
	}
	doc := UserDocument{
		ID:        ev.UserID,
		Email:     ev.Email,
		Name:      ev.Name,
		Role:      ev.Role,
		CreatedAt: ev.At,
		UpdatedAt: ev.At,
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("search: marshal user document: %w", err)
	}

	res, err := ix.client.ES.Index(
		ix.indexName,
		bytes.NewReader(body),
		ix.client.ES.Index.WithDocumentID(ev.UserID),
		ix.client.ES.Index.WithContext(msg.Context()),
	)
	if err != nil {
		slog.ErrorContext(msg.Context(), "search: failed to index user",
			"module", "search", "operation", "HandleUserCreated",
			"error_code", "es_transport_error", "retryable", true,
			"user_id", ev.UserID, "err", err)
		return fmt.Errorf("search: index user: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		slog.ErrorContext(msg.Context(), "search: ES index error",
			"module", "search", "operation", "HandleUserCreated",
			"error_code", "es_status_error", "retryable", false,
			"user_id", ev.UserID, "status", res.Status())
		return fmt.Errorf("search: index user returned %s", res.Status())
	}

	slog.DebugContext(msg.Context(), "search: indexed user", "user_id", ev.UserID)
	return nil
}

// HandleUserUpdated performs a partial update of name/email/role/updated_at.
// Idempotent: ES Update with same payload produces the same document state.
func (ix *Indexer) HandleUserUpdated(msg *message.Message) error {
	if ix == nil {
		return nil
	}

	var ev domain.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "search: failed to unmarshal user.updated event",
			"module", "search", "operation", "HandleUserUpdated",
			"error_code", "unmarshal_failed", "retryable", false, "err", err)
		return nil
	}
	doc := map[string]any{
		"doc": map[string]any{
			"name":       ev.Name,
			"email":      ev.Email,
			"role":       ev.Role,
			"updated_at": ev.At,
		},
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("search: marshal partial update: %w", err)
	}

	res, err := ix.client.ES.Update(
		ix.indexName,
		ev.UserID,
		bytes.NewReader(body),
		ix.client.ES.Update.WithContext(msg.Context()),
	)
	if err != nil {
		slog.ErrorContext(msg.Context(), "search: failed to update user",
			"module", "search", "operation", "HandleUserUpdated",
			"error_code", "es_transport_error", "retryable", true,
			"user_id", ev.UserID, "err", err)
		return fmt.Errorf("search: update user: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		slog.ErrorContext(msg.Context(), "search: ES update error",
			"module", "search", "operation", "HandleUserUpdated",
			"error_code", "es_status_error", "retryable", false,
			"user_id", ev.UserID, "status", res.Status())
		return fmt.Errorf("search: update user returned %s", res.Status())
	}

	slog.DebugContext(msg.Context(), "search: updated user", "user_id", ev.UserID)
	return nil
}

// HandleUserDeleted removes a user document from the index.
// Idempotent: ES Delete returns 404 for already-deleted docs (tolerated).
func (ix *Indexer) HandleUserDeleted(msg *message.Message) error {
	if ix == nil {
		return nil
	}

	var ev domain.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "search: failed to unmarshal user.deleted event",
			"module", "search", "operation", "HandleUserDeleted",
			"error_code", "unmarshal_failed", "retryable", false, "err", err)
		return nil
	}
	res, err := ix.client.ES.Delete(
		ix.indexName,
		ev.UserID,
		ix.client.ES.Delete.WithContext(msg.Context()),
	)
	if err != nil {
		slog.ErrorContext(msg.Context(), "search: failed to delete user",
			"module", "search", "operation", "HandleUserDeleted",
			"error_code", "es_transport_error", "retryable", true,
			"user_id", ev.UserID, "err", err)
		return fmt.Errorf("search: delete user: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() && res.StatusCode != 404 {
		slog.ErrorContext(msg.Context(), "search: ES delete error",
			"module", "search", "operation", "HandleUserDeleted",
			"error_code", "es_status_error", "retryable", false,
			"user_id", ev.UserID, "status", res.Status())
		return fmt.Errorf("search: delete user returned %s", res.Status())
	}

	slog.DebugContext(msg.Context(), "search: deleted user", "user_id", ev.UserID)
	return nil
}
