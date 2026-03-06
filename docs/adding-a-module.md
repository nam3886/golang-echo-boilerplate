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

This creates 19 files + runs code generation. Then:
1. Customize proto fields in `proto/{name}/v1/{name}.proto`
2. Customize DB columns in `db/migrations/{timestamp}_create_{plural}.sql`
3. Customize SQL queries in `db/queries/{name}.sql`
4. Run `task generate` after customizing proto/SQL
5. Update domain entity, handlers, and adapters to match new fields
6. Add event topics to `internal/shared/events/topics.go`
7. Register module in `cmd/server/main.go`
8. Run `task migrate:up && task check`

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
option go_package = "github.com/gnha/gnha-services/gen/proto/product/v1;productv1";

import "buf/validate/validate.proto";

service ProductService {
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse);
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
}

message Product {
  string id = 1;
  string name = 2;
  string created_at = 3;
  string updated_at = 4;
}

message CreateProductRequest {
  string name = 1 [(buf.validate.field).string.min_len = 1];
}
message CreateProductResponse { Product product = 1; }

message GetProductRequest {
  string id = 1 [(buf.validate.field).string.uuid = true];
}
message GetProductResponse { Product product = 1; }

message ListProductsRequest {
  int32 limit = 1;
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
  id         UUID PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS products;
```

Create `db/queries/product.sql`:

```sql
-- name: GetProductByID :one
SELECT * FROM products WHERE id = $1 AND deleted_at IS NULL;

-- name: ListProducts :many
SELECT * FROM products
WHERE deleted_at IS NULL
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at'), sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: CreateProduct :one
INSERT INTO products (id, name) VALUES ($1, $2) RETURNING *;

-- name: UpdateProduct :one
UPDATE products
SET name = COALESCE(sqlc.narg('name'), name), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProduct :execrows
UPDATE products SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;
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
        return nil, ErrNameRequired
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
        return ErrNameRequired
    }
    p.name = name
    p.updatedAt = time.Now()
    return nil
}
```

### domain/errors.go — Domain Errors

```go
package domain

import sharederr "github.com/gnha/gnha-services/internal/shared/errors"

var (
    ErrNameRequired    = sharederr.New(sharederr.CodeInvalidArgument, "name is required")
    ErrProductNotFound = sharederr.New(sharederr.CodeNotFound, "product not found")
)
```

### domain/repository.go — Interface with mockgen directive

```go
package domain

import "context"

//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_product_repository.go -package=mocks

type ProductRepository interface {
    GetByID(ctx context.Context, id ProductID) (*Product, error)
    List(ctx context.Context, limit int, cursor string) ([]*Product, string, bool, error)
    Create(ctx context.Context, p *Product) error
    Update(ctx context.Context, id ProductID, fn func(*Product) error) error
    SoftDelete(ctx context.Context, id ProductID) error
}
```

### adapters/postgres/repository.go — DB Adapter

```go
package postgres

import (
    "context"
    "errors"
    "fmt"

    sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
    "github.com/gnha/gnha-services/internal/modules/product/domain"
    sharederr "github.com/gnha/gnha-services/internal/shared/errors"
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
    uid, err := uuid.Parse(string(id))
    if err != nil {
        return nil, sharederr.New(sharederr.CodeInvalidArgument, "invalid product ID")
    }
    q := sqlcgen.New(r.pool)
    row, err := q.GetProductByID(ctx, uid)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, sharederr.ErrNotFound
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
    _, err = q.CreateProduct(ctx, sqlcgen.CreateProductParams{
        ID:   uid,
        Name: p.Name(),
    })
    if err != nil {
        return fmt.Errorf("inserting product: %w", err)
    }
    return nil
}
```

### adapters/grpc/handler.go — Connect RPC Handler

```go
package grpc

import (
    "context"
    "connectrpc.com/connect"
    productv1 "github.com/gnha/gnha-services/gen/proto/product/v1"
    "github.com/gnha/gnha-services/gen/proto/product/v1/productv1connect"
    "github.com/gnha/gnha-services/internal/modules/product/app"
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
        return nil, domainErrorToConnect(err)
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
    "github.com/gnha/gnha-services/gen/proto/product/v1/productv1connect"
    "github.com/gnha/gnha-services/internal/shared/config"
    appmw "github.com/gnha/gnha-services/internal/shared/middleware"
    "github.com/labstack/echo/v4"
    "github.com/redis/go-redis/v9"
)

func RegisterRoutes(e *echo.Echo, handler *ProductServiceHandler, cfg *config.Config, rdb *redis.Client) {
    path, h := productv1connect.NewProductServiceHandler(handler,
        connect.WithInterceptors(validate.NewInterceptor()),
    )
    g := e.Group(path, appmw.Auth(cfg, rdb))
    g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
```

### module.go — fx Module

```go
package product

import (
    "go.uber.org/fx"
    "github.com/gnha/gnha-services/internal/modules/product/adapters/grpc"
    "github.com/gnha/gnha-services/internal/modules/product/adapters/postgres"
    "github.com/gnha/gnha-services/internal/modules/product/app"
    "github.com/gnha/gnha-services/internal/modules/product/domain"
)

var Module = fx.Module("product",
    fx.Provide(
        fx.Annotate(
            postgres.NewPgProductRepository,
            fx.As(new(domain.ProductRepository)),
        ),
    ),
    fx.Provide(app.NewCreateProductHandler),
    fx.Provide(grpc.NewProductServiceHandler),
    fx.Invoke(grpc.RegisterRoutes),
)
```

## 5. Event Publishing (optional)

Define topics in `internal/shared/events/topics.go`:

```go
const TopicProductCreated = "product.created"

type ProductCreatedEvent struct {
    ProductID string    `json:"product_id"`
    ActorID   string    `json:"actor_id"`
    Name      string    `json:"name"`
    At        time.Time `json:"at"`
}
```

Publish in your app handler after DB write:

```go
if err := h.bus.Publish(ctx, events.TopicProductCreated, events.ProductCreatedEvent{
    ProductID: string(p.ID()),
    Name:      p.Name(),
    At:        time.Now(),
}); err != nil {
    slog.ErrorContext(ctx, "failed to publish event", "err", err)
}
```

## 6. Register in main.go

```go
import "github.com/gnha/gnha-services/internal/modules/product"

fx.New(
    shared.Module,
    // ... existing modules
    product.Module,
    fx.Invoke(startServer),
).Run()
```

## 7. Verify

```bash
go build ./...          # compilation check
go vet ./...            # correctness check
task test               # unit tests
task test:integration   # integration tests (requires Docker)
```
