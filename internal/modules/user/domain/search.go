package domain

import "context"

// UserSearchResult holds results from a full-text user search.
type UserSearchResult struct {
	UserIDs []string
	Total   int64
}

// UserSearch is the interface for full-text user search.
// Implemented by adapters/search.Repository; nil when Elasticsearch is disabled.
//
// Contracts:
//   - Search: retryable on transient ES errors; returns empty result (not nil) for no matches
//   - EnsureIndex: idempotent; safe to call on every startup (no-op if index exists)
type UserSearch interface {
	Search(ctx context.Context, query string, limit, offset int) (*UserSearchResult, error)
	EnsureIndex(ctx context.Context) error
}
