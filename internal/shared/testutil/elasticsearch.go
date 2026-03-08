package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gnha/gnha-services/internal/shared/search"
	tc "github.com/testcontainers/testcontainers-go"
	esMod "github.com/testcontainers/testcontainers-go/modules/elasticsearch"
)

// NewTestElasticsearch returns a search.Client for integration tests.
// If ELASTICSEARCH_URL is set (e.g. CI service containers), it connects directly.
// Otherwise, it starts a local testcontainer.
func NewTestElasticsearch(t *testing.T) *search.Client {
	t.Helper()
	ctx := context.Background()

	if esURL := os.Getenv("ELASTICSEARCH_URL"); esURL != "" {
		es, err := elasticsearch.NewClient(elasticsearch.Config{
			Addresses: []string{esURL},
		})
		if err != nil {
			t.Fatalf("connecting to ELASTICSEARCH_URL: %v", err)
		}
		return &search.Client{ES: es, IndexPrefix: "test"}
	}

	container, err := esMod.Run(ctx, "elasticsearch:8.17.0",
		tc.WithEnv(map[string]string{
			"discovery.type":          "single-node",
			"xpack.security.enabled":  "false",
			"ES_JAVA_OPTS":            "-Xms256m -Xmx256m",
		}),
	)
	if err != nil {
		t.Fatalf("starting elasticsearch container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	settings := container.Settings
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{settings.Address},
	})
	if err != nil {
		t.Fatalf("creating elasticsearch client: %v", err)
	}

	return &search.Client{ES: es, IndexPrefix: "test"}
}
