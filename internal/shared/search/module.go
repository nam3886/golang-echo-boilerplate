package search

import (
	"go.uber.org/fx"
)

// Module provides the optional Elasticsearch client to the Fx container.
// Note: go-elasticsearch client has no Close() method; no shutdown hook needed.
var Module = fx.Module("search",
	fx.Provide(NewClient),
)
