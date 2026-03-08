package search_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/gnha-services/internal/modules/user/adapters/search"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/testutil"
	"github.com/google/uuid"
)

func TestIndexer_HandleUserCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := testutil.NewTestElasticsearch(t)
	repo := search.NewRepository(client)
	if err := repo.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("ensure index: %v", err)
	}

	ix := search.NewIndexer(client)

	ev := domain.UserCreatedEvent{
		UserID: uuid.New().String(),
		Email:  "alice@example.com",
		Name:   "Alice Smith",
		Role:   "member",
		At:     time.Now(),
	}
	payload, _ := json.Marshal(ev)
	msg := message.NewMessage(uuid.New().String(), payload)

	if err := ix.HandleUserCreated(msg); err != nil {
		t.Fatalf("HandleUserCreated: %v", err)
	}

	// ES needs a refresh to make the doc searchable
	_, _ = client.ES.Indices.Refresh(client.ES.Indices.Refresh.WithIndex(client.IndexName(search.UsersIndexSuffix)))

	result, err := repo.Search(context.Background(), "alice", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.UserIDs) != 1 || result.UserIDs[0] != ev.UserID {
		t.Errorf("expected user %s in results, got %v", ev.UserID, result.UserIDs)
	}
}

func TestIndexer_HandleUserUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := testutil.NewTestElasticsearch(t)
	repo := search.NewRepository(client)
	if err := repo.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("ensure index: %v", err)
	}

	ix := search.NewIndexer(client)
	userID := uuid.New().String()

	// Create first
	createEv := domain.UserCreatedEvent{
		UserID: userID, Email: "bob@example.com", Name: "Bob Jones", Role: "member", At: time.Now(),
	}
	payload, _ := json.Marshal(createEv)
	if err := ix.HandleUserCreated(message.NewMessage(uuid.New().String(), payload)); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update name
	updateEv := domain.UserUpdatedEvent{
		UserID: userID, Name: "Robert Jones", Email: "bob@example.com", Role: "admin", At: time.Now(),
	}
	payload, _ = json.Marshal(updateEv)
	if err := ix.HandleUserUpdated(message.NewMessage(uuid.New().String(), payload)); err != nil {
		t.Fatalf("update: %v", err)
	}

	_, _ = client.ES.Indices.Refresh(client.ES.Indices.Refresh.WithIndex(client.IndexName(search.UsersIndexSuffix)))

	result, err := repo.Search(context.Background(), "Robert", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.UserIDs) != 1 || result.UserIDs[0] != userID {
		t.Errorf("expected user %s after update, got %v", userID, result.UserIDs)
	}
}

func TestIndexer_HandleUserDeleted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := testutil.NewTestElasticsearch(t)
	repo := search.NewRepository(client)
	if err := repo.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("ensure index: %v", err)
	}

	ix := search.NewIndexer(client)
	userID := uuid.New().String()

	// Create
	createEv := domain.UserCreatedEvent{
		UserID: userID, Email: "charlie@example.com", Name: "Charlie Brown", Role: "member", At: time.Now(),
	}
	payload, _ := json.Marshal(createEv)
	if err := ix.HandleUserCreated(message.NewMessage(uuid.New().String(), payload)); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Delete
	deleteEv := domain.UserDeletedEvent{UserID: userID, At: time.Now()}
	payload, _ = json.Marshal(deleteEv)
	if err := ix.HandleUserDeleted(message.NewMessage(uuid.New().String(), payload)); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, _ = client.ES.Indices.Refresh(client.ES.Indices.Refresh.WithIndex(client.IndexName(search.UsersIndexSuffix)))

	result, err := repo.Search(context.Background(), "charlie", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.UserIDs) != 0 {
		t.Errorf("expected no results after delete, got %v", result.UserIDs)
	}
}

func TestIndexer_NilReceiver_Noop(t *testing.T) {
	var ix *search.Indexer

	payload, _ := json.Marshal(domain.UserCreatedEvent{UserID: "test"})
	msg := message.NewMessage("1", payload)

	if err := ix.HandleUserCreated(msg); err != nil {
		t.Errorf("nil HandleUserCreated should be noop, got %v", err)
	}
	if err := ix.HandleUserUpdated(msg); err != nil {
		t.Errorf("nil HandleUserUpdated should be noop, got %v", err)
	}
	if err := ix.HandleUserDeleted(msg); err != nil {
		t.Errorf("nil HandleUserDeleted should be noop, got %v", err)
	}
}
