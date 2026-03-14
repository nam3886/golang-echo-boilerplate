# Code Standards

This document outlines Go coding standards and patterns used across the Golang Echo Boilerplate codebase.

## Project Structure

The codebase follows a **Hexagonal Architecture** pattern organized as a modular monolith:

```
.
├── cmd/server/            # Application entry point
├── internal/
│   ├── shared/            # Cross-cutting infrastructure
│   └── modules/           # Domain modules (user, audit, notification)
├── proto/                 # Protocol Buffer definitions
├── db/                    # Database migrations & queries
└── gen/                   # Generated code (from buf & sqlc)
```

## File Naming Conventions

- **Files**: Use `snake_case` for all file names (per Go convention)
- **Packages**: Use lowercase package names matching directory names
- **Source files**: Group by responsibility (e.g., `create_user.go`, `user.go`, `errors.go`)

Note: Go convention requires lowercase filenames with underscores. While the general project style guide
may reference kebab-case for other file types, Go source files must use snake_case per the Go standard library conventions.

### Module File Organization

Each module follows this structure:

```
internal/modules/{module}/
├── domain/
│   ├── {entity}.go        # Domain entity with encapsulated fields
│   ├── repository.go      # Repository interface
│   └── errors.go          # Module-specific domain errors
├── app/
│   ├── create_{entity}.go # Command handlers
│   ├── get_{entity}.go    # Query handlers
│   └── ...
├── adapters/
│   ├── postgres/          # PostgreSQL repository implementation
│   │   └── repository.go
│   └── grpc/              # Connect RPC handler
│       ├── handler.go
│       ├── routes.go
│       └── mapper.go
└── module.go              # fx Module definition
```

## Naming Conventions

### Types & Interfaces

```go
// Entities: PascalCase, encapsulated fields
type User struct {
    id        UserID
    email     string
    // ...
}

// Typed identifiers for domain concepts
type UserID string

// Enums: PascalCase constants
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleMember Role = "member"
    RoleViewer Role = "viewer"
)

// Interfaces: PascalCase ending with "er" or "able"
type UserRepository interface { }
type PasswordHasher interface { }
```

### Functions & Methods

```go
// Package-level functions: PascalCase
func NewUser(email, name, hashedPassword string, role Role) (*User, error) { }

// Getters: No "Get" prefix
func (u *User) ID() UserID { return u.id }
func (u *User) Email() string { return u.email }

// Setters/Mutations: Verb-first
func (u *User) ChangeName(name string) error { }
func (u *User) ChangeRole(role Role) error { }

// Predicates: "Is" prefix
func (r Role) IsValid() bool { }
```

### Command & Query Objects

```go
// Commands: Suffix with "Cmd"
type CreateUserCmd struct {
    Email    string
    Name     string
    Password string
    Role     string
}

// Queries: Suffix with "Query" (if needed)
type GetUserQuery struct {
    ID string
}

// Handlers: "{Action}{Entity}Handler"
type CreateUserHandler struct { }
type GetUserHandler struct { }
```

## Error Handling

### Domain Errors

All errors use the `DomainError` pattern from `internal/shared/errors` (imported as `sharederr`).
Module-specific errors are **constructor functions**, not package-level vars:

```go
// Define module-specific errors as constructor functions
// (errors.go in domain/)
func ErrUserNotFound() *sharederr.DomainError {
    return sharederr.New(sharederr.CodeNotFound, "user.not_found", "user not found")
}

func ErrEmailTaken() *sharederr.DomainError {
    return sharederr.New(sharederr.CodeAlreadyExists, "user.email_taken", "email already taken")
}
```

> **Why constructor functions?** Package-level `var` errors are shared mutable pointers.
> If two goroutines wrap the same error concurrently (`fmt.Errorf("ctx: %w", ErrNotFound)`),
> they race on the error's internal state. Constructor functions return fresh instances
> on every call, eliminating the race.

#### DomainError.Is() — Matching Rules

`DomainError.Is()` uses a two-tier matching strategy:
- **When both errors have a Key:** matches by Code + Key (precise). So
  `errors.Is(ErrUserNotFound(), ErrOrderNotFound())` returns `false` because keys differ
  (`user.not_found` vs `order.not_found`).
- **When either error has no Key:** matches by Code alone (category). This enables
  `errors.Is(err, sharederr.ErrNotFound())` for HTTP status mapping.

Module-specific errors with different keys do NOT match each other.

### Error Codes

Use standard error codes that map to HTTP status codes:

```go
const (
    CodeInvalidArgument      ErrorCode = "INVALID_ARGUMENT"        // 400
    CodeUnauthenticated      ErrorCode = "UNAUTHENTICATED"         // 401
    CodePermissionDenied     ErrorCode = "PERMISSION_DENIED"       // 403
    CodeNotFound             ErrorCode = "NOT_FOUND"               // 404
    CodeAlreadyExists        ErrorCode = "ALREADY_EXISTS"          // 409
    CodeFailedPrecondition   ErrorCode = "FAILED_PRECONDITION"     // 412
    CodeResourceExhausted    ErrorCode = "RESOURCE_EXHAUSTED"      // 429
    CodeInternal             ErrorCode = "INTERNAL"                // 500
    CodeUnavailable          ErrorCode = "UNAVAILABLE"             // 503
)
```

### Error Wrapping

```go
// Wrap infrastructure errors with context
return nil, fmt.Errorf("checking email: %w", err)

// Return domain errors directly
return nil, domain.ErrEmailTaken()

// Check wrapped errors
if errors.Is(err, sharederr.ErrNotFound()) { }
```

### Structured Error Logging

All error logs MUST include structured metadata for observability and debugging. Use `slog.ErrorContext` with:

1. **Module name** (`"module"` key) — which service or module
2. **Operation** (`"operation"` key) — what was being attempted
3. **Error code** (`"error_code"` key) — machine-readable error classification
4. **Retryable flag** (`"retryable"` key) — `true` for transient failures (network, timeout), `false` for permanent ones (schema mismatch, invalid data)
5. **Error object** (`"err"` key) — the underlying error

Example:
```go
// Transient error (retry on failure)
slog.ErrorContext(ctx, "failed to send email",
    "module", "notification",
    "user_id", event.UserID,
    "error_code", "smtp_transient",
    "retryable", true,
    "err", err)

// Permanent error (ack on failure, no retry)
slog.ErrorContext(ctx, "failed to unmarshal event",
    "module", "audit",
    "msg_id", msg.UUID,
    "error_code", "unmarshal_failed",
    "retryable", false,
    "err", err)

// Warning for recoverable issues
slog.WarnContext(ctx, "dedup check failed, proceeding",
    "module", "notification",
    "msg_id", msg.UUID,
    "err", err)
```

Use `slog.WarnContext` for recoverable issues that don't block operations (e.g., optional cache checks). Never use `log.Printf` or bare `fmt.Println` in app handlers or subscriber code.

### Domain vs App-Layer Errors

- **Domain errors** (`domain/errors.go`): Business rule violations. Use named constructor
  functions like `ErrInvalidEmail()`, `ErrNameRequired()`. These represent domain invariant
  failures.
- **App-layer errors** (inline in handlers): Input/plumbing validation. Use
  `sharederr.New(sharederr.CodeInvalidArgument, "user.id_required", "user ID is required")` for app-level
  checks that don't belong to the domain model (e.g., empty ID, missing required command fields).

Both produce `DomainError` and map to HTTP status codes identically. The distinction is
organizational: domain errors live with the entity, app errors live with the handler.

## Domain Layer (domain/)

### Entity Definition

Entities encapsulate fields and include business logic:

```go
type User struct {
    id        UserID
    email     string
    name      string
    password  string // hashed
    role      Role
    createdAt time.Time
    updatedAt time.Time
    deletedAt *time.Time
}

// Constructor validates inputs and enforces invariants
func NewUser(email, name, hashedPassword string, role Role) (*User, error) {
    addr, err := mail.ParseAddress(email)
    if err != nil {
        return nil, ErrInvalidEmail()
    }
    email = addr.Address
    if name == "" {
        return nil, ErrNameRequired()
    }
    if !role.IsValid() {
        return nil, ErrInvalidRole()
    }
    if hashedPassword == "" {
        return nil, ErrPasswordRequired()
    }
    // ...
}

// Reconstitute rebuilds from persistence (no validation).
// Parameter order: identity → content → metadata → lifecycle
func Reconstitute(id UserID, email, name, password string, role Role,
    createdAt, updatedAt time.Time, deletedAt *time.Time) *User {
    return &User{id: id, email: email, ...}
}
```

### Repository Interface

Repositories abstract persistence:

```go
type ListResult struct {
    Users []*User
    Total int
}

type UserRepository interface {
    GetByID(ctx context.Context, id UserID) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    List(ctx context.Context, page, pageSize int) (ListResult, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, id UserID, fn func(*User) error) error
    SoftDelete(ctx context.Context, id UserID) (*User, error)
}
```

**Key Patterns:**

- **Get methods** return `ErrNotFound` when resource doesn't exist
- **List** returns `ListResult{Users, Total}` with offset-based pagination; accepts `page` and `pageSize`, executes LIMIT/OFFSET + COUNT query
- **Create** may catch database constraint errors (e.g., Postgres 23505 for uniqueness) and map to domain errors
- **Update** accepts a closure to apply mutations within a transaction; publishes events after successful persistence
- **SoftDelete** marks record as deleted, returns the deleted entity for event publishing; returns `ErrNotFound` if the user doesn't exist

## Application Layer (app/)

### Command Handlers

Handle side-effects with validation, business logic, and events:

```go
type CreateUserHandler struct {
    repo   domain.UserRepository
    hasher auth.PasswordHasher
    bus    events.EventPublisher
}

func NewCreateUserHandler(
    repo domain.UserRepository,
    hasher auth.PasswordHasher,
    bus events.EventPublisher,
) *CreateUserHandler {
    return &CreateUserHandler{repo: repo, hasher: hasher, bus: bus}
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) (*domain.User, error) {
    // Validation
    existing, err := h.repo.GetByEmail(ctx, cmd.Email)
    if err != nil && !errors.Is(err, sharederr.ErrNotFound()) {
        return nil, fmt.Errorf("checking email: %w", err)
    }
    if existing != nil {
        return nil, domain.ErrEmailTaken()
    }

    // Business logic
    hashedPwd, err := h.hasher.Hash(cmd.Password)
    if err != nil {
        return nil, fmt.Errorf("hashing password: %w", err)
    }

    user, err := domain.NewUser(cmd.Email, cmd.Name, hashedPwd, domain.Role(cmd.Role))
    if err != nil {
        return nil, err
    }

    // Persistence
    if err := h.repo.Create(ctx, user); err != nil {
        return nil, fmt.Errorf("creating user: %w", err)
    }

    // Events (after successful persistence)
    if err := h.bus.Publish(ctx, domain.TopicUserCreated, domain.UserCreatedEvent{
        Version:   1,
        UserID:    string(user.ID()),
        ActorID:   auth.ActorIDFromContext(ctx),
        Email:     user.Email(),
        Name:      user.Name(),
        Role:      string(user.Role()),
        IPAddress: netutil.GetClientIP(ctx),
        At:        user.CreatedAt(),
    }); err != nil {
        // Log event publishing failures but don't fail the handler
        slog.ErrorContext(ctx, "failed to publish user.created event",
            "user_id", string(user.ID()), "err", err)
    }

    return user, nil
}
```

### Query Handlers

Handle read operations without side effects:

```go
type GetUserHandler struct {
    repo domain.UserRepository
}

func (h *GetUserHandler) Handle(ctx context.Context, id string) (*domain.User, error) {
    if id == "" {
        return nil, sharederr.New(sharederr.CodeInvalidArgument, "user.id_required", "user ID is required")
    }
    user, err := h.repo.GetByID(ctx, domain.UserID(id))
    if err != nil {
        return nil, fmt.Errorf("getting user %s: %w", id, err)
    }
    return user, nil
}
```

### Update Pattern with ErrNoChange

When a handler attempts to mutate an entity, track changed fields in a slice. If no mutations occur, return `sharederr.ErrNoChange()` inside the repo's update closure. The repo catches `ErrNoChange`, commits a read-only transaction, and returns `nil`. The handler checks `len(changedFields) == 0` and skips event publishing:

```go
var changedFields []string
err := h.repo.Update(ctx, id, func(user *domain.User) error {
    if cmd.Name != nil && *cmd.Name != user.Name() {
        if err := user.ChangeName(*cmd.Name); err != nil {
            return err
        }
        changedFields = append(changedFields, "name")
    }
    if len(changedFields) == 0 {
        return sharederr.ErrNoChange()
    }
    return nil
})
if err != nil {
    return nil, err
}
if len(changedFields) == 0 {
    return user, nil  // No event published
}
// Publish event here (include changedFields in event payload)
```

### Update & Delete Handlers

Update and Delete handlers follow the same event publishing pattern as Create:

```go
// UpdateUserHandler applies partial mutations within a transaction
type UpdateUserHandler struct {
    repo domain.UserRepository
    bus  events.EventPublisher
}

func (h *UpdateUserHandler) Handle(ctx context.Context, cmd UpdateUserCmd) (*domain.User, error) {
    var updated *domain.User
    var changedFields []string
    err := h.repo.Update(ctx, domain.UserID(cmd.ID), func(user *domain.User) error {
        if cmd.Name != nil && *cmd.Name != user.Name() {
            if err := user.ChangeName(*cmd.Name); err != nil {
                return err
            }
            changedFields = append(changedFields, "name")
        }
        updated = user
        if len(changedFields) == 0 {
            return sharederr.ErrNoChange()
        }
        return nil
    })
    // err==nil && len(changedFields)==0: repo committed read-only tx (ErrNoChange), no SQL UPDATE issued.
    if err != nil {
        return nil, err
    }

    if len(changedFields) == 0 {
        return updated, nil
    }

    // Publish event with ActorID from context
    if err := h.bus.Publish(ctx, domain.TopicUserUpdated, domain.UserUpdatedEvent{
        Version:       1,
        UserID:        string(updated.ID()),
        ActorID:       auth.ActorIDFromContext(ctx),
        Name:          updated.Name(),
        Email:         updated.Email(),
        Role:          string(updated.Role()),
        ChangedFields: changedFields,
        IPAddress:     netutil.GetClientIP(ctx),
        At:            updated.UpdatedAt(),
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish user.updated event",
            "user_id", string(updated.ID()), "err", err)
    }

    return updated, nil
}

// DeleteUserHandler soft-deletes a user
type DeleteUserHandler struct {
    repo domain.UserRepository
    bus  events.EventPublisher
}

func (h *DeleteUserHandler) Handle(ctx context.Context, id string) error {
    if id == "" {
        return sharederr.New(sharederr.CodeInvalidArgument, "user.id_required", "user ID is required")
    }
    user, err := h.repo.SoftDelete(ctx, domain.UserID(id))
    if err != nil {
        return fmt.Errorf("deleting user %s: %w", id, err)
    }

    // Use DB-authoritative deletion timestamp; fall back to UpdatedAt if nil (defensive).
    deletedAt := user.UpdatedAt()
    if user.IsDeleted() {
        deletedAt = *user.DeletedAt()
    }

    if err := h.bus.Publish(ctx, domain.TopicUserDeleted, domain.UserDeletedEvent{
        Version:   1,
        UserID:    id,
        ActorID:   auth.ActorIDFromContext(ctx),
        IPAddress: netutil.GetClientIP(ctx),
        At:        deletedAt,
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish user.deleted event",
            "user_id", id, "err", err)
    }

    return nil
}
```

**Event Publishing Pattern:**
- Extract ActorID via `auth.ActorIDFromContext(ctx)` to track who initiated the mutation
- Publish events *after* successful persistence
- Log but don't fail the handler if event publishing fails (graceful degradation)
- Each mutation publishes a distinct event type (UserCreatedEvent, UserUpdatedEvent, UserDeletedEvent)

## Adapter Layer (adapters/)

### PostgreSQL Repository

Implement domain interfaces using sqlc-generated code:

```go
// Per-method sqlcgen.New() enables passing tx instead of pool for transactions.
type PgUserRepository struct {
    pool *pgxpool.Pool
}

func NewPgUserRepository(pool *pgxpool.Pool) *PgUserRepository {
    return &PgUserRepository{pool: pool}
}

func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
    uid, err := parseUserID(user.ID())
    if err != nil {
        return err
    }
    q := sqlcgen.New(r.pool)
    row, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
        ID:       uid,
        Email:    user.Email(),
        Name:     user.Name(),
        Password: user.Password(),
        Role:     string(user.Role()),
    })
    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            return domain.ErrEmailTaken()
        }
        return fmt.Errorf("inserting user: %w", err)
    }
    // Overwrite entity with DB-authoritative timestamps.
    *user = *toDomainFromCreateRow(row, user.Password())
    return nil
}
```

**Constraint error handling** — always check both error code AND constraint name for precise mapping:

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    if pgErr.ConstraintName == "idx_users_email_active" {
        return domain.ErrEmailTaken()
    }
    return sharederr.New(sharederr.CodeAlreadyExists, "entity.duplicate", "duplicate entry")
}
```

### Per-Query Mapper Pattern

sqlc generates a unique Go struct per query (e.g., `GetUserByIDRow`, `ListUsersRow`, `CreateUserRow`) because each query may SELECT different columns. Each requires its own mapper:

```go
func toDomain(row sqlcgen.User) *domain.User           // full entity (e.g., GetByEmail with password)
func toDomainFromGetRow(row sqlcgen.GetUserByIDRow)     // read-only, excludes sensitive fields
func toDomainFromListRow(row sqlcgen.ListUsersRow)      // list queries
func toDomainFromCreateRow(row sqlcgen.CreateUserRow)   // CREATE ... RETURNING
func toDomainFromUpdateRow(row sqlcgen.UpdateUserRow)   // UPDATE ... RETURNING
func toDomainFromSoftDeleteRow(row sqlcgen.SoftDeleteUserRow) // soft-delete
```

The scaffold generates a single `toDomain()` as a starting point. Add per-query variants as you customize your SQL queries to return different column sets.

### Connect RPC Handler

Implement protobuf service using Connect RPC:

```go
type UserServiceHandler struct {
    createUser *app.CreateUserHandler
    getUser    *app.GetUserHandler
    // ...
}

func (h *UserServiceHandler) CreateUser(
    ctx context.Context,
    req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.CreateUserResponse], error) {
    user, err := h.createUser.Handle(ctx, app.CreateUserCmd{
        Email:    req.Msg.Email,
        Name:     req.Msg.Name,
        Password: req.Msg.Password,
        Role:     req.Msg.Role,
    })
    if err != nil {
        return nil, connectutil.DomainErrorToConnect(ctx, err)
    }
    return connect.NewResponse(&userv1.CreateUserResponse{
        User: toProto(user),
    }), nil
}

// Verify interface compliance
var _ userv1connect.UserServiceHandler = (*UserServiceHandler)(nil)
```

### Mappers

Convert between domain and proto types:

```go
func toProto(user *domain.User) *userv1.User {
    return &userv1.User{
        Id:        string(user.ID()),
        Email:     user.Email(),
        Name:      user.Name(),
        Role:      string(user.Role()),
        CreatedAt: timestamppb.New(user.CreatedAt()),
    }
}

// Domain-to-Connect error mapping is centralized in connectutil.DomainErrorToConnect().
// See internal/shared/connectutil/errors.go for implementation.
```

## Event System

### Event Structure

All domain events follow a consistent structure with ActorID for audit trails:

```go
// UserCreatedEvent is published when a user is created
type UserCreatedEvent struct {
    Version   int       `json:"version"`              // Schema version, currently 1
    UserID    string    `json:"user_id"`              // Resource ID
    ActorID   string    `json:"actor_id"`             // User who initiated action
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    Role      string    `json:"role"`
    IPAddress string    `json:"ip_address,omitempty"` // Client IP for audit trail
    At        time.Time `json:"at"`                   // Event timestamp
}

// UserUpdatedEvent is published when a user is updated
type UserUpdatedEvent struct {
    Version       int      `json:"version"`
    UserID        string   `json:"user_id"`
    ActorID       string   `json:"actor_id"`
    Name          string   `json:"name"`
    Email         string   `json:"email"`
    Role          string   `json:"role"`
    ChangedFields []string `json:"changed_fields,omitempty"` // Fields that changed
    IPAddress     string   `json:"ip_address,omitempty"`
    At            time.Time `json:"at"`
}

// UserDeletedEvent is published when a user is soft-deleted
type UserDeletedEvent struct {
    Version   int       `json:"version"`
    UserID    string    `json:"user_id"`
    ActorID   string    `json:"actor_id"`
    IPAddress string    `json:"ip_address,omitempty"`
    At        time.Time `json:"at"`
}
```

### Topic Constants

Event contract types (structs + topic constants) are defined centrally in
`internal/shared/events/contracts/`. Each module re-exports them via type
aliases in `domain/events.go` for internal convenience. External subscribers
(audit, notification) import from `contracts/` directly.

```go
// internal/modules/user/domain/events.go
import "github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"

const (
    TopicUserCreated = contracts.TopicUserCreated
    TopicUserUpdated = contracts.TopicUserUpdated
    TopicUserDeleted = contracts.TopicUserDeleted
)
```

### Cross-Module Event Consumption

Subscriber modules import event types from `internal/shared/events/contracts/`,
not from other modules' `domain/` packages. This preserves the no-cross-module-imports rule.
All subscribers (audit, notification) use the shared contract types.

```go
// Both audit and notification subscribers use shared contracts:
import "github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"

var event contracts.UserCreatedEvent
json.Unmarshal(msg.Payload, &event)
// Access: event.UserID, event.ActorID, event.IPAddress, event.Email, etc.
```

## Pagination

### Offset-Based Pagination Pattern

List endpoints use offset-based pagination with total count:

```go
// Repository signature
List(ctx context.Context, page, pageSize int) (ListResult, error)
// Returns: (ListResult{Users []*User, Total int}, error)
```

**Implementation Details:**
- SQL: `LIMIT $1 OFFSET $2` with a separate `COUNT(*)` query
- `page` is 1-indexed; `pageSize` range 1–100
- Caller derives `total_pages` as `ceil(Total / pageSize)`

**Usage Pattern:**
1. Client requests: `List(page: 1, pageSize: 20)`
2. Repository returns: `ListResult{Users: [...], Total: 85}`
3. Client computes `total_pages = ceil(85/20) = 5`
4. Client requests next page: `List(page: 2, pageSize: 20)`

## Testing Conventions

### Test File Naming

```
{source_file}_test.go
```

### Mocking with mockgen

Use `mockgen` to generate mocks from repository interfaces:

```go
// domain/repository.go
package domain

import "context"

//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_user_repository.go -package=mocks

type UserRepository interface {
    GetByID(ctx context.Context, id UserID) (*User, error)
    Create(ctx context.Context, user *User) error
    // ...
}
```

Run `task generate:mocks` to generate all mocks via `go generate ./...`

### Test Structure

```go
func TestCreateUserHandler_Handle_Success(t *testing.T) {
    // Arrange
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    repo := mocks.NewMockUserRepository(ctrl)
    repo.EXPECT().
        GetByEmail(gomock.Any(), "user@example.com").
        Return(nil, sharederr.ErrNotFound()).
        Times(1)
    repo.EXPECT().
        Create(gomock.Any(), gomock.Any()).
        Return(nil).
        Times(1)

    bus := events.NewEventBus(&testutil.NoopPublisher{})
    handler := app.NewCreateUserHandler(repo, &testutil.StubHasher{}, bus)

    // Act
    user, err := handler.Handle(context.Background(), app.CreateUserCmd{
        Email:    "user@example.com",
        Name:     "John Doe",
        Password: "secure123",
        Role:     "member",
    })

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if user == nil {
        t.Fatal("expected user, got nil")
    }
    if got := user.Email(); got != "user@example.com" {
        t.Errorf("email = %q, want %q", got, "user@example.com")
    }
}
```

**Mock Generation Flags:**
- `-source=repository.go` - Interface file to generate mocks from
- `-destination=../../../shared/mocks/mock_*.go` - Output location relative to source
- `-package=mocks` - Package name for generated mocks

### Integration Testing

Use `testcontainers` for real infrastructure:

```go
import (
    "github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
)

func TestUserIntegration(t *testing.T) {
    pool := testutil.NewTestPostgres(t) // auto-cleanup via t.Cleanup
    testutil.RunMigrations(t, pool)

    repo := postgres.NewPgUserRepository(pool)
    // Real database tests
}
```

## Module Registration (fx)

Each module must define an fx.Module:

```go
// internal/modules/user/module.go
package user

import (
    "go.uber.org/fx"
    // ...
)

var Module = fx.Module("user",
    fx.Provide(
        fx.Annotate(
            postgres.NewPgUserRepository,
            fx.As(new(domain.UserRepository)),
        ),
    ),
    fx.Provide(app.NewCreateUserHandler),
    fx.Provide(app.NewGetUserHandler),
    // ...
    fx.Provide(grpc.NewUserServiceHandler),
    fx.Invoke(grpc.RegisterRoutes),
)
```

## Configuration Validation

Environment variable validation happens at startup in `config.Load()`. Validations are **fail-fast and fatal in production**:

```go
// Validation examples:
if cfg.IsProduction() && slices.Contains(cfg.CORSOrigins, "*") {
    return nil, fmt.Errorf("CORS_ORIGINS=* is not allowed in production")
}

if cfg.IsProduction() && e.IPExtractor == nil {
    log.Fatal("FATAL: rate limiter uses default IPExtractor in production; " +
        "set e.IPExtractor = echo.ExtractIPFromXFFHeader() for accurate client IP")
}
```

**Key validations:**
- `APP_ENV` must be one of: development, staging, production
- `JWT_SECRET` minimum 32 characters
- All URLs (DATABASE_URL, REDIS_URL, RABBITMQ_URL) must be valid
- `DB_MIN_CONNS` cannot exceed `DB_MAX_CONNS`
- `CORS_ORIGINS=*` rejected in production (use explicit origins)
- IP extractor must be explicitly configured in production (prevents rate-limit bypass via spoofed X-Forwarded-For)
- `OTEL_SAMPLING_RATIO` must be between 0 and 1
- `RATE_LIMIT_RPM` must be > 0

**Philosophy:** Production environments must be explicitly secure — defaults are safe for development but insufficient for production. Configuration mismatches (e.g., wildcard CORS, missing IP extractor) are fatal startup errors, not warnings.

## Code Quality Guidelines

- **Keep files under 200 lines**: Split large files by responsibility
- **Single Responsibility**: One concept per file/function
- **No cyclic imports**: Use dependency injection (fx)
- **Explicit dependencies**: Pass everything as parameters, no global state
- **Defer cleanup**: Use `defer` for resource cleanup
- **Context propagation**: Always pass `context.Context` as first parameter
- **No hardcoded values**: Use configuration and constants
- **Comments for "why"**: Explain reasoning, not what code does
