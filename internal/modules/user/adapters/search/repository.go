package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	sharedsearch "github.com/gnha/golang-echo-boilerplate/internal/shared/search"
)

// Result holds search results with pagination metadata.
type Result struct {
	UserIDs []string
	Total   int64
}

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
func (r *Repository) Search(ctx context.Context, query string, limit, offset int) (*Result, error) {
	if r == nil {
		return &Result{}, nil
	}

	q := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":     query,
				"fields":    []string{"name", "email"},
				"fuzziness": "AUTO",
			},
		},
		"from": offset,
		"size": limit,
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

	return &Result{
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
		slog.InfoContext(ctx, "search: index already exists", "index", r.indexName)
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
		return fmt.Errorf("search: create index returned %s", res.Status())
	}

	slog.InfoContext(ctx, "search: created index", "index", r.indexName)
	return nil
}
