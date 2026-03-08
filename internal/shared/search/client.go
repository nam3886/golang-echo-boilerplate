// Package search provides an Elasticsearch client and helpers.
package search

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gnha/gnha-services/internal/shared/config"
)

// Client wraps the official Elasticsearch client with an index prefix.
type Client struct {
	ES          *elasticsearch.Client
	IndexPrefix string
}

// NewClient creates an Elasticsearch client. Returns nil when ElasticsearchURL
// is empty, making the entire search subsystem a no-op.
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.ElasticsearchURL == "" {
		slog.Info("elasticsearch disabled (ELASTICSEARCH_URL empty)")
		return nil, nil
	}

	var (
		es  *elasticsearch.Client
		err error
	)

	esCfg := elasticsearch.Config{
		Addresses: []string{cfg.ElasticsearchURL},
	}

	for i := range 10 {
		es, err = elasticsearch.NewClient(esCfg)
		if err == nil {
			res, pingErr := es.Ping()
			if pingErr == nil && !res.IsError() {
				_ = res.Body.Close()
				slog.Info("elasticsearch connected", "url", cfg.ElasticsearchURL)
				return &Client{
					ES:          es,
					IndexPrefix: cfg.ElasticsearchIndexPrefix,
				}, nil
			}
			if res != nil {
				_ = res.Body.Close()
			}
			if pingErr != nil {
				err = pingErr
			} else {
				err = fmt.Errorf("elasticsearch ping returned status %s", res.Status())
			}
		}
		slog.Warn("elasticsearch not ready, retrying", "attempt", i+1, "err", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("elasticsearch connection failed after 10 retries: %w", err)
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
