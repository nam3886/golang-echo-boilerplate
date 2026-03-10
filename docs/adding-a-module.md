# Adding a Module

Step-by-step guide for adding a new domain module (e.g., `product`). All examples follow the actual `user` module patterns.

## Quick Start (Recommended)

Run the scaffold generator to create all module files:

```bash
task module:create name=product
```

For custom plural naming:

```bash
task module:create name=category plural=categories
```

This creates 27 files + runs code generation. Then:
1. Customize proto fields in `proto/{name}/v1/{name}.proto`
2. Customize DB columns in `db/migrations/{timestamp}_create_{plural}.sql`
3. Customize SQL queries in `db/queries/{name}.sql`
4. Run `task generate` after customizing proto/SQL
5. Update domain entity, handlers, and adapters to match new fields
6. (Auto-generated) Verify event contracts in `internal/shared/events/contracts/{name}_events.go`
7. (Auto-generated) Verify event re-exports in `internal/modules/{name}/domain/events.go`
8. (Auto-injected) Verify RBAC permissions in `rbac.go` and `rbac_interceptor.go`
9. Register module in `cmd/server/main.go` (auto-injected by scaffold)
10. Run `task migrate:up && task check`

## Module Structure Tiers

Modules follow one of two architectural patterns based on their purpose:

### Tier 1: Full Hexagonal (CRUD Modules)

Use for domain entities with their own business logic and client-facing APIs.

**Examples:** `user`, `product`, `category` — anything with Create/Read/Update/Delete operations exposed via gRPC/HTTP.

**Structure:**
```
internal/modules/product/
├── domain/              # Entity, repository interface, events
├── app/                 # CRUD handlers (Create, Get, List, Update, Delete)
├── adapters/
│   ├── postgres/        # Real database implementation
│   └── grpc/            # Connect RPC handler + routes
└── module.go            # fx Module with providers + invokers
```

**Key files:**
- `domain/{entity}.go` — Entity with validation logic
- `domain/repository.go` — Repository interface
- `domain/events.go` — Event topics + event types
- `app/{action}_{entity}.go` — Command/Query handlers
- `adapters/postgres/repository.go` — Database implementation
- `adapters/grpc/handler.go` — RPC handler

### Tier 2: Flat Event Subscribers

Use for infrastructure-adjacent modules that react to domain events (no direct user API).

**Examples:** `audit` (logs all mutations), `notification` (sends emails/notifications), `search` (Elasticsearch indexing).

**Structure:**
```
internal/modules/audit/
├── handler.go           # Event handler implementation
└── module.go            # fx Module with event handler registration
```

**No:** Proto definitions, migrations, SQL queries, gRPC handlers, or repository interfaces.

**Event Handler Registration:**
Each handler registers itself via the `event_handlers` fx group. The framework automatically creates a per-handler subscriber queue:

```go
// module.go
func provideHandlers(h *Handler) []events.HandlerRegistration {
    return []events.HandlerRegistration{
        {Name: "audit.user_created", Topic: contracts.TopicUserCreated, HandlerFunc: h.HandleUserCreated},
        {Name: "audit.user_updated", Topic: contracts.TopicUserUpdated, HandlerFunc: h.HandleUserUpdated},
    }
}
```

Each handler gets its own AMQP queue (e.g., `user.created_audit.user_created`) via `SubscriberFactory`,
ensuring it receives all published events instead of round-robin distribution.

## Manual Steps (Reference)

The sections below detail what the scaffold generates, for reference.

## 1. Create Proto Definition

```bash
mkdir -p proto/product/v1
```

Create `proto/product/v1/product.proto`:

```proto
syntax = "proto3";
package product.v1;
option go_package = "github.com/gnha/golang-echo-boilerplate/gen/proto/product/v1;productv1";

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

service ProductService {
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse);
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
  rpc UpdateProduct(UpdateProductRequest) returns (UpdateProductResponse);
  rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse);
}

message Product {
  string id = 1;
  string name = 2;
  google.protobuf.Timestamp created_at = 3;
  google.protobuf.Timestamp updated_at = 4;
}

message CreateProductRequest {
  string name = 1 [(buf.validate.field).string = {min_len: 1, max_len: 255}];
}
message CreateProductResponse { Product product = 1; }

message GetProductRequest {
  string id = 1 [(buf.validate.field).string.uuid = true];
}
message GetProductResponse { Product product = 1; }

message ListProductsRequest {
  int32 limit = 1 [(buf.validate.field).int32 = {gte: 1, lte: 100}];
  string cursor = 2;
}
message ListProductsResponse {
  repeated Product items = 1;
  string next_cursor = 2;
  bool has_more = 3;
}
```

## 2. Create Migration + SQL Queries

Create migration `db/migrations/000X_create_products.sql`:

```sql
-- +goose Up
CREATE TABLE products (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_products_active ON products (id) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_name ON products (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_cursor ON products (created_at DESC, id DESC) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS products;
```

Create `db/queries/product.sql`:

```sql
-- name: GetProductByID :one
SELECT id, name, created_at, updated_at, deleted_at FROM products WHERE id = $1 AND deleted_at IS NULL;

-- name: GetProductByIDForUpdate :one
SELECT id, name, created_at, updated_at, deleted_at FROM products WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListProducts :many
SELECT id, name, created_at, updated_at, deleted_at FROM products
WHERE deleted_at IS NULL
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at'), sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: CreateProduct :one
INSERT INTO products (id, name) VALUES ($1, $2)
RETURNING id, name, created_at, updated_at, deleted_at;

-- name: UpdateProduct :one
UPDATE products
SET name = COALESCE(sqlc.narg('name'), name), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, created_at, updated_at, deleted_at;

-- name: SoftDeleteProduct :one
UPDATE products SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, created_at, updated_at, deleted_at;
```

## 3. Generate Code

```bash
task generate
```

Runs `buf generate` (proto → `gen/proto/product/`) and `sqlc generate` (SQL → `gen/sqlc/`).

## 4. Create Module Structure

```bash
mkdir -p internal/modules/product/{domain,app,adapters/{postgres,grpc}}
```

### domain/product.go — Entity (unexported fields + getters)

```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

type ProductID string

type Product struct {
    id        ProductID
    name      string
    createdAt time.Time
    updatedAt time.Time
    deletedAt *time.Time
}

// NewProduct creates a validated Product entity.
func NewProduct(name string) (*Product, error) {
    if name == "" {
        return nil, ErrNameRequired()
    }
    now := time.Now()
    return &Product{
        id:        ProductID(uuid.NewString()),
        name:      name,
        createdAt: now,
        updatedAt: now,
    }, nil
}

// Reconstitute rebuilds a Product from persistence (no validation).
func Reconstitute(id ProductID, name string, createdAt, updatedAt time.Time, deletedAt *time.Time) *Product {
    return &Product{
        id: id, name: name,
        createdAt: createdAt, updatedAt: updatedAt, deletedAt: deletedAt,
    }
}

// Getters
func (p *Product) ID() ProductID       { return p.id }
func (p *Product) Name() string        { return p.name }
func (p *Product) CreatedAt() time.Time { return p.createdAt }
func (p *Product) UpdatedAt() time.Time { return p.updatedAt }

// ChangeName updates the product name.
func (p *Product) ChangeName(name string) error {
    if name == "" {
        return ErrNameRequired()
    }
    if name == p.name {
        return nil
    }
    p.name = name
    p.updatedAt = time.Now()
    return nil
}
```

### domain/errors.go — Domain Errors

```go
package domain

import sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"

// Constructor functions return fresh instances to prevent data races
// when errors are wrapped concurrently.

func ErrNameRequired() *sharederr.DomainError {
    return sharederr.New(sharederr.CodeInvalidArgument, "name is required")
}

func ErrProductNotFound() *sharederr.DomainError {
    return sharederr.New(sharederr.CodeNotFound, "product not found")
}
```

### domain/repository.go — Interface with mockgen directive

```go
package domain

import "context"

//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_product_repository.go -package=mocks

type ListResult struct {
    Products   []*Product
    NextCursor string
    HasMore    bool
}

type ProductRepository interface {
    GetByID(ctx context.Context, id ProductID) (*Product, error)
    List(ctx context.Context, limit int, cursor string) (ListResult, error)
    Create(ctx context.Context, p *Product) error
    Update(ctx context.Context, id ProductID, fn func(*Product) error) error
    SoftDelete(ctx context.Context, id ProductID) (*Product, error)
}
```

### adapters/postgres/repository.go — DB Adapter

```go
package postgres

import (
    "context"
    "errors"
    "fmt"

    sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
    "github.com/gnha/golang-echo-boilerplate/internal/modules/product/domain"
    sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type PgProductRepository struct {
    pool *pgxpool.Pool
}

func NewPgProductRepository(pool *pgxpool.Pool) *PgProductRepository {
    return &PgProductRepository{pool: pool}
}

func (r *PgProductRepository) GetByID(ctx context.Context, id domain.ProductID) (*domain.Product, error) {
    uid, err := parseProductID(id)
    if err != nil {
        return nil, err
    }
    q := sqlcgen.New(r.pool)
    row, err := q.GetProductByID(ctx, uid)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, sharederr.ErrNotFound()
        }
        return nil, fmt.Errorf("getting product by id: %w", err)
    }
    return toDomain(row), nil
}

func (r *PgProductRepository) Create(ctx context.Context, p *domain.Product) error {
    uid, err := parseProductID(p.ID())
    if err != nil {
        return err
    }
    q := sqlcgen.New(r.pool)
    row, err := q.CreateProduct(ctx, sqlcgen.CreateProductParams{
        ID:   uid,
        Name: p.Name(),
    })
    if err != nil {
        return fmt.Errorf("inserting product: %w", err)
    }
    // Overwrite entity with DB-authoritative timestamps (created_at, updated_at).
    *p = *toDomain(row)
    return nil
}
```

### adapters/grpc/handler.go — Connect RPC Handler

```go
package grpc

import (
    "context"
    "connectrpc.com/connect"
    productv1 "github.com/gnha/golang-echo-boilerplate/gen/proto/product/v1"
    "github.com/gnha/golang-echo-boilerplate/gen/proto/product/v1/productv1connect"
    "github.com/gnha/golang-echo-boilerplate/internal/modules/product/app"
)

type ProductServiceHandler struct {
    createProduct *app.CreateProductHandler
    // ... other handlers
}

func NewProductServiceHandler(createProduct *app.CreateProductHandler) *ProductServiceHandler {
    return &ProductServiceHandler{createProduct: createProduct}
}

var _ productv1connect.ProductServiceHandler = (*ProductServiceHandler)(nil)

func (h *ProductServiceHandler) CreateProduct(ctx context.Context, req *connect.Request[productv1.CreateProductRequest]) (*connect.Response[productv1.CreateProductResponse], error) {
    p, err := h.createProduct.Handle(ctx, app.CreateProductCmd{Name: req.Msg.Name})
    if err != nil {
        return nil, connectutil.DomainErrorToConnect(err)
    }
    return connect.NewResponse(&productv1.CreateProductResponse{
        Product: toProto(p),
    }), nil
}
```

### adapters/grpc/routes.go — Route Registration

```go
package grpc

import (
    "net/http"
    "connectrpc.com/connect"
    "connectrpc.com/validate"
    "github.com/gnha/golang-echo-boilerplate/gen/proto/product/v1/productv1connect"
    "github.com/gnha/golang-echo-boilerplate/internal/shared/config"
    appmw "github.com/gnha/golang-echo-boilerplate/internal/shared/middleware"
    "github.com/labstack/echo/v4"
    "github.com/redis/go-redis/v9"
)

func RegisterRoutes(e *echo.Echo, handler *ProductServiceHandler, cfg *config.Config, rdb *redis.Client) {
    path, h := productv1connect.NewProductServiceHandler(handler,
        connect.WithInterceptors(
            appmw.RBACInterceptor(),
            validate.NewInterceptor(),
        ),
    )
    // Mount Connect handler under auth. RBAC is enforced via RBACInterceptor per procedure.
    g := e.Group(path, appmw.Auth(cfg, rdb))
    g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
```

### RBAC Setup

To enable role-based access control for your new module:

1. **Define permission constants** in `internal/shared/middleware/rbac.go`:
   ```go
   PermProductRead   Permission = "product:read"
   PermProductWrite  Permission = "product:write"
   PermProductDelete Permission = "product:delete"
   ```

2. **Register all procedures** in `internal/shared/middleware/rbac_interceptor.go`:
   ```go
   // In procedurePermissions map:
   productv1connect.ProductServiceGetProductProcedure:    PermProductRead,
   productv1connect.ProductServiceListProductsProcedure:  PermProductRead,
   productv1connect.ProductServiceCreateProductProcedure: PermProductWrite,
   productv1connect.ProductServiceUpdateProductProcedure: PermProductWrite,
   productv1connect.ProductServiceDeleteProductProcedure: PermProductDelete,
   ```

   **Important:** ALL procedures must be listed (fail-closed). Read procedures are mapped
   even though the Echo group guard already checks them — the interceptor ensures
   unmapped procedures are denied by default.

3. The scaffold automatically:
   - Adds `RBACInterceptor()` to Connect handler setup (already in routes.go)
   - Injects procedure permission mappings in the scaffold comment `// ADD_PROCEDURE_PERMISSION_HERE`

### module.go — fx Module

```go
package product

import (
    "go.uber.org/fx"
    "github.com/gnha/golang-echo-boilerplate/internal/modules/product/adapters/grpc"
    "github.com/gnha/golang-echo-boilerplate/internal/modules/product/adapters/postgres"
    "github.com/gnha/golang-echo-boilerplate/internal/modules/product/app"
    "github.com/gnha/golang-echo-boilerplate/internal/modules/product/domain"
)

var Module = fx.Module("product",
    fx.Provide(
        fx.Annotate(
            postgres.NewPgProductRepository,
            fx.As(new(domain.ProductRepository)),
        ),
    ),
    fx.Provide(app.NewCreateProductHandler),
    fx.Provide(app.NewGetProductHandler),
    fx.Provide(app.NewListProductsHandler),
    fx.Provide(app.NewUpdateProductHandler),
    fx.Provide(app.NewDeleteProductHandler),
    fx.Provide(grpc.NewProductServiceHandler),
    fx.Invoke(grpc.RegisterRoutes),
)
```

## 5. Event Publishing (optional)

> **Note:** Canonical event types and topic constants are defined in
> `internal/shared/events/contracts/{name}_events.go`. Domain modules re-export them
> via type aliases in `domain/events.go` for internal convenience.
> External subscribers (audit, notification) import from `contracts/` directly.

### Create Contracts (internal/shared/events/contracts/)

Create `internal/shared/events/contracts/product_events.go`:

```go
package contracts

import "time"

// Event topics for the product module.
const (
    TopicProductCreated = "product.created"
    TopicProductUpdated = "product.updated"
    TopicProductDeleted = "product.deleted"
)

// ProductCreatedEvent is published when a product is created.
type ProductCreatedEvent struct {
    ProductID string    `json:"product_id"`
    ActorID   string    `json:"actor_id"`
    Name      string    `json:"name"`
    IPAddress string    `json:"ip_address,omitempty"`
    At        time.Time `json:"at"`
}

// ProductUpdatedEvent is published when a product is updated.
type ProductUpdatedEvent struct {
    ProductID string    `json:"product_id"`
    ActorID   string    `json:"actor_id"`
    Name      string    `json:"name"`
    IPAddress string    `json:"ip_address,omitempty"`
    At        time.Time `json:"at"`
}

// ProductDeletedEvent is published when a product is soft-deleted.
type ProductDeletedEvent struct {
    ProductID string    `json:"product_id"`
    ActorID   string    `json:"actor_id"`
    IPAddress string    `json:"ip_address,omitempty"`
    At        time.Time `json:"at"`
}
```

### Re-export in Domain (domain/events.go)

Create `internal/modules/{name}/domain/events.go`:

```go
package domain

import "github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"

// Re-export event topics from shared contracts for internal convenience.
const (
    TopicProductCreated = contracts.TopicProductCreated
    TopicProductUpdated = contracts.TopicProductUpdated
    TopicProductDeleted = contracts.TopicProductDeleted
)

// Re-export event types from shared contracts.
type ProductCreatedEvent = contracts.ProductCreatedEvent
type ProductUpdatedEvent = contracts.ProductUpdatedEvent
type ProductDeletedEvent = contracts.ProductDeletedEvent
```

### Publishing Events in App Handlers

In your app handler (e.g., `CreateProductHandler`), inject `events.EventPublisher` and publish after DB write:

```go
if err := h.bus.Publish(ctx, domain.TopicProductCreated, domain.ProductCreatedEvent{
    ProductID: string(p.ID()),
    ActorID:   actorID,
    Name:      p.Name(),
    IPAddress: netutil.GetClientIP(ctx),
    At:        time.Now(),
}); err != nil {
    slog.ErrorContext(ctx, "failed to publish event", "err", err)
}
```

## 6. Register in main.go

```go
import "github.com/gnha/golang-echo-boilerplate/internal/modules/product"

fx.New(
    shared.Module,
    // ... existing modules
    product.Module,
    fx.Invoke(startServer),
).Run()
```

## Adding a Field Checklist

When adding a new field to an existing entity (e.g., adding `description` to `Product`), update these 11 files in order:

1. **Proto Definition** (`proto/{name}/v1/{name}.proto`)
   - Add field to message type with field number and validation
   - Example: `string description = 3 [(buf.validate.field).string.min_len = 1];`

2. **Database Migration** (`db/migrations/{timestamp}_create_{plural}.sql`)
   - Add column with type, constraints, default value
   - Example: `description TEXT NOT NULL DEFAULT ''`
   - Run: `task generate:sqlc`

3. **SQL Queries** (`db/queries/{name}.sql`)
   - Add field to SELECT clauses in GetByID, List, Create, Update queries
   - Ensure INSERT/UPDATE includes the new field

4. **Code Generation**
   - Run: `task generate` (buf + sqlc)
   - Generates proto Go types in `gen/proto/{name}/v1/`
   - Generates sqlc types in `gen/sqlc/`

5. **Domain Entity** (`internal/modules/{name}/domain/{entity}.go`)
   - Add unexported field to struct: `description string`
   - Add getter: `func (p *Product) Description() string { return p.description }`
   - Update `NewProduct()` constructor with validation
   - Update `Reconstitute()` signature and implementation

6. **Domain Errors** (`internal/modules/{name}/domain/errors.go`)
   - Add validation error if needed: `func ErrDescriptionRequired() *sharederr.DomainError { ... }`

7. **Domain Events** (`internal/modules/{name}/domain/events.go`)
   - Add field to event types where relevant (UserCreatedEvent, UserUpdatedEvent, etc.)
   - Example: Add `Description string` to ProductCreatedEvent

8. **App Handlers** (`internal/modules/{name}/app/{action}_{entity}.go`)
   - Update CreateHandler to accept and validate new field
   - Update UpdateHandler to support field mutations
   - Publish updated events with new field values

9. **Postgres Adapter** (`internal/modules/{name}/adapters/postgres/repository.go`)
   - Update `Create()` to pass new field to sqlc
   - Update `Update()` mutation closure to handle field changes
   - Update `toDomain()` mapper to reconstruct field from DB row

10. **gRPC Mapper** (`internal/modules/{name}/adapters/grpc/mapper.go`)
    - Update `toProto()` to map domain field → proto message field
    - Update `toDomain()` to map proto message field → domain entity

11. **Tests** (`**/*_test.go`)
    - Add field to test fixtures: `testutil.DefaultProductFixture()`
    - Add unit tests for validation logic in `domain/{entity}_test.go`
    - Add integration tests for persistence in `adapters/postgres/repository_test.go`
    - Update existing test assertions if they check all fields

**Run after all changes:**
```bash
task generate         # Regenerate code from proto/SQL
task lint             # Check formatting
task test             # Run unit tests
task test:integration # Run integration tests with real DB
go build ./...        # Final compilation check
```

## 7. Verify

```bash
go build ./...          # compilation check
go vet ./...            # correctness check
task test               # unit tests
task test:integration   # integration tests (requires Docker)
```
