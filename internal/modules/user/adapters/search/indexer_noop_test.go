package search_test

import (
	"encoding/json"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/adapters/search"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
)

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
