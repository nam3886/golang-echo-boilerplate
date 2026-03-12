// Package search provides an Elasticsearch client and helpers.
package search

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/retry"
)

// Client wraps the official Elasticsearch client with an index prefix.
type Client struct {
	ES          *elasticsearch.Client
	IndexPrefix string
}

// NewClient creates an Elasticsearch client. Returns (nil, nil) when
// ElasticsearchURL is empty, disabling the search subsystem.
// All consumers MUST nil-check the returned *Client before use.
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.ElasticsearchURL == "" {
		slog.Info("elasticsearch disabled (ELASTICSEARCH_URL empty)")
		return nil, nil
	}

	esCfg := elasticsearch.Config{
		Addresses: []string{cfg.ElasticsearchURL},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := retry.Connect(ctx, "elasticsearch", 10, func() (*Client, error) {
		es, err := elasticsearch.NewClient(esCfg)
		if err != nil {
			return nil, err
		}
		res, err := es.Ping()
		if err != nil {
			return nil, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.IsError() {
			return nil, fmt.Errorf("ping returned %s", res.Status())
		}
		return &Client{ES: es, IndexPrefix: cfg.ElasticsearchIndexPrefix}, nil
	})
	if err != nil {
		return nil, err
	}
	slog.Info("elasticsearch connected", "url", cfg.ElasticsearchURL)
	return client, nil
}

// IndexName returns the fully qualified index name with prefix.
func (c *Client) IndexName(suffix string) string {
	return c.IndexPrefix + "_" + suffix
}

// HealthCheck pings Elasticsearch and returns an error if unreachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	res, err := c.ES.Ping(c.ES.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("elasticsearch ping: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("elasticsearch unhealthy: %s", res.Status())
	}
	return nil
}
