package search

import "time"

// UsersIndexSuffix is the ES index suffix for users.
const UsersIndexSuffix = "users"

// UsersMapping is the strict ES mapping for the users index.
const UsersMapping = `{
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "id":         { "type": "keyword" },
      "email":      { "type": "keyword", "copy_to": "searchable" },
      "name":       { "type": "text", "copy_to": "searchable" },
      "role":       { "type": "keyword" },
      "created_at": { "type": "date" },
      "updated_at": { "type": "date" },
      "searchable": { "type": "text" }
    }
  }
}`

// UserDocument represents a user document in Elasticsearch.
type UserDocument struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
