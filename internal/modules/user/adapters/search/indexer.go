// Package search handles Elasticsearch indexing for user documents.
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	sharedsearch "github.com/gnha/gnha-services/internal/shared/search"
)

// Indexer handles indexing user documents into Elasticsearch.
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
func (ix *Indexer) HandleUserCreated(msg *message.Message) error {
	if ix == nil {
		return nil
	}

	var ev domain.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("search: failed to unmarshal user.created event", "err", err)
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
		slog.Error("search: failed to index user", "user_id", ev.UserID, "err", err)
		return fmt.Errorf("search: index user: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		slog.Error("search: ES index error", "user_id", ev.UserID, "status", res.Status())
		return fmt.Errorf("search: index user returned %s", res.Status())
	}

	slog.Debug("search: indexed user", "user_id", ev.UserID)
	return nil
}

// HandleUserUpdated performs a partial update of name/email/role/updated_at.
func (ix *Indexer) HandleUserUpdated(msg *message.Message) error {
	if ix == nil {
		return nil
	}

	var ev domain.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("search: failed to unmarshal user.updated event", "err", err)
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
		slog.Error("search: failed to update user", "user_id", ev.UserID, "err", err)
		return fmt.Errorf("search: update user: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		slog.Error("search: ES update error", "user_id", ev.UserID, "status", res.Status())
		return fmt.Errorf("search: update user returned %s", res.Status())
	}

	slog.Debug("search: updated user", "user_id", ev.UserID)
	return nil
}

// HandleUserDeleted removes a user document from the index.
func (ix *Indexer) HandleUserDeleted(msg *message.Message) error {
	if ix == nil {
		return nil
	}

	var ev domain.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("search: failed to unmarshal user.deleted event", "err", err)
		return nil
	}

	res, err := ix.client.ES.Delete(
		ix.indexName,
		ev.UserID,
		ix.client.ES.Delete.WithContext(msg.Context()),
	)
	if err != nil {
		slog.Error("search: failed to delete user", "user_id", ev.UserID, "err", err)
		return fmt.Errorf("search: delete user: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() && res.StatusCode != 404 {
		slog.Error("search: ES delete error", "user_id", ev.UserID, "status", res.Status())
		return fmt.Errorf("search: delete user returned %s", res.Status())
	}

	slog.Debug("search: deleted user", "user_id", ev.UserID)
	return nil
}
