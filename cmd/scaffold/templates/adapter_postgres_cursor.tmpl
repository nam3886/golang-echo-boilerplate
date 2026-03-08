package postgres

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// cursorPayload is the keyset pagination cursor structure.
type cursorPayload struct {
	T time.Time `json:"t"`
	U uuid.UUID `json:"u"`
}

func encodeCursor(t time.Time, id uuid.UUID) (string, error) {
	data, err := json.Marshal(cursorPayload{T: t, U: id})
	if err != nil {
		return "", fmt.Errorf("encoding cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

func decodeCursor(cursor string) (*cursorPayload, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, err
	}
	var c cursorPayload
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
