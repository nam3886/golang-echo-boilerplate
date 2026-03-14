package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharedsearch "github.com/gnha/golang-echo-boilerplate/internal/shared/search"
)

// Compile-time check: Repository must satisfy domain.UserSearch.
var _ domain.UserSearch = (*Repository)(nil)

// Repository provides search operations against the users index.
type Repository struct {
	client    *sharedsearch.Client
	indexName string
}

// NewRepository creates a user search repository. Returns nil if client is nil.
func NewRepository(client *sharedsearch.Client) *Repository {
	if client == nil {
		return nil
	}
	return &Repository{
		client:    client,
		indexName: client.IndexName(UsersIndexSuffix),
	}
}

// Search performs a multi_match query on name+email with fuzziness.
func (r *Repository) Search(ctx context.Context, query string, limit, offset int) (*domain.UserSearchResult, error) {
	if r == nil {
		return &domain.UserSearchResult{}, nil
	}

	q := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":     query,
				"fields":    []string{"searchable"}, // mapping copies name+email into this field
				"fuzziness": "AUTO",
			},
		},
		"from":    offset,
		"size":    limit,
		"_source": false,
	}

	body, err := json.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("search: marshal query: %w", err)
	}

	res, err := r.client.ES.Search(
		r.client.ES.Search.WithContext(ctx),
		r.client.ES.Search.WithIndex(r.indexName),
		r.client.ES.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("search: execute query: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("search: query returned %s", res.Status())
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("search: decode response: %w", err)
	}

	ids := make([]string, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		ids[i] = hit.ID
	}

	return &domain.UserSearchResult{
		UserIDs: ids,
		Total:   result.Hits.Total.Value,
	}, nil
}

// EnsureIndex creates the users index if it does not already exist.
func (r *Repository) EnsureIndex(ctx context.Context) error {
	if r == nil {
		return nil
	}

	res, err := r.client.ES.Indices.Exists(
		[]string{r.indexName},
		r.client.ES.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("search: check index existence: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if !res.IsError() {
		slog.InfoContext(ctx, "search: index already exists",
			"module", "search", "operation", "EnsureIndex", "index", r.indexName)
		return nil
	}

	res, err = r.client.ES.Indices.Create(
		r.indexName,
		r.client.ES.Indices.Create.WithBody(strings.NewReader(UsersMapping)),
		r.client.ES.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("search: create index: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		// 400 with resource_already_exists_exception means a concurrent startup beat us — treat as success.
		// Parse the error type to avoid silently swallowing unrelated 400 errors.
		if res.StatusCode == 400 {
			var errResp struct {
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}
			_ = json.NewDecoder(res.Body).Decode(&errResp)
			if errResp.Error.Type == "resource_already_exists_exception" {
				slog.InfoContext(ctx, "search: index already exists (concurrent creation)",
					"module", "search", "operation", "EnsureIndex", "index", r.indexName)
				return nil
			}
		}
		return fmt.Errorf("search: create index returned %s", res.Status())
	}

	slog.InfoContext(ctx, "search: created index",
		"module", "search", "operation", "EnsureIndex", "index", r.indexName)
	return nil
}
