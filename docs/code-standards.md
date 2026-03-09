# Code Standards

This document outlines Go coding standards and patterns used across the GNHA Services codebase.

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

All errors use the `DomainError` pattern from `internal/shared/errors` (actually `internal/shared/errors/domainerr`).
Module-specific errors are **constructor functions**, not package-level vars:

```go
// Define module-specific errors as constructor functions
// (errors.go in domain/)
func ErrUserNotFound() *sharederr.DomainError {
    return sharederr.New(sharederr.CodeNotFound, "user not found")
}

func ErrEmailTaken() *sharederr.DomainError {
    return sharederr.New(sharederr.CodeAlreadyExists, "email already taken")
}
```

> **Why constructor functions?** Package-level `var` errors are shared mutable pointers.
> If two goroutines wrap the same error concurrently (`fmt.Errorf("ctx: %w", ErrNotFound)`),
> they race on the error's internal state. Constructor functions return fresh instances
> on every call, eliminating the race.

#### DomainError.Is() — Category Matching

`DomainError.Is()` matches by **error category (ErrorCode)**, not by specific error identity.
This means `errors.Is(ErrUserNotFound(), ErrOrderNotFound())` returns `true` because both have
`CodeNotFound`. This is intentional — HTTP status mapping relies on the code, not the message.
For identity-specific matching, use `errors.As` and check the `Message` field.

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
    if email == "" {
        return nil, ErrInvalidEmail()
    }
    if !role.IsValid() {
        return nil, ErrInvalidRole()
    }
    // ...
}

// Reconstitute rebuilds from persistence (no validation)
func Reconstitute(id UserID, email, name, password string, role Role,
    createdAt, updatedAt time.Time, deletedAt *time.Time) *User {
    return &User{id: id, email: email, ...}
}
```

### Repository Interface

Repositories abstract persistence:

```go
type ListResult struct {
    Users      []*User
    NextCursor string
    HasMore    bool
}

type UserRepository interface {
    GetByID(ctx context.Context, id UserID) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    List(ctx context.Context, limit int, cursor string) (ListResult, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, id UserID, fn func(*User) error) error
    SoftDelete(ctx context.Context, id UserID) (*User, error)
}
```

**Key Patterns:**

- **Get methods** return `ErrNotFound` when resource doesn't exist
- **List** returns `(users, nextCursor, hasMore, error)` with cursor-based pagination; implementation probes with `limit+1` internally to detect page boundaries
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
    // Extract ActorID from auth context for audit trail
    actorID := auth.ActorIDFromContext(ctx)
    if err := h.bus.Publish(ctx, domain.TopicUserCreated, domain.UserCreatedEvent{
        UserID:    string(user.ID()),
        ActorID:   actorID,
        Email:     user.Email(),
        Name:      user.Name(),
        Role:      string(user.Role()),
        IPAddress: netutil.GetClientIP(ctx),
        At:        time.Now(),
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
    user, err := h.repo.GetByID(ctx, domain.UserID(id))
    if err != nil {
        return nil, err
    }
    return user, nil
}
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
    err := h.repo.Update(ctx, domain.UserID(cmd.ID), func(user *domain.User) error {
        if cmd.Name != nil {
            if err := user.ChangeName(*cmd.Name); err != nil {
                return err
            }
        }
        updated = user
        return nil
    })
    if err != nil {
        return nil, err
    }

    // Publish event with ActorID from context
    var actorID string
    if actor := auth.UserFromContext(ctx); actor != nil {
        actorID = actor.UserID
    }
    if err := h.bus.Publish(ctx, domain.TopicUserUpdated, domain.UserUpdatedEvent{
        UserID:    cmd.ID,
        ActorID:   actorID,
        Name:      updated.Name(),
        Email:     updated.Email(),
        Role:      string(updated.Role()),
        IPAddress: netutil.GetClientIP(ctx),
        At:        time.Now(),
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish user.updated event",
            "user_id", cmd.ID, "err", err)
    }

    return updated, nil
}

// DeleteUserHandler soft-deletes a user
type DeleteUserHandler struct {
    repo domain.UserRepository
    bus  events.EventPublisher
}

func (h *DeleteUserHandler) Handle(ctx context.Context, id string) error {
    user, err := h.repo.SoftDelete(ctx, domain.UserID(id))
    if err != nil {
        return err
    }

    // Publish event with ActorID from context
    var actorID string
    if actor := auth.UserFromContext(ctx); actor != nil {
        actorID = actor.UserID
    }
    if err := h.bus.Publish(ctx, domain.TopicUserDeleted, domain.UserDeletedEvent{
        UserID:    id,
        ActorID:   actorID,
        IPAddress: netutil.GetClientIP(ctx),
        At:        *user.DeletedAt(), // DB-authoritative deletion timestamp
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish user.deleted event",
            "user_id", id, "err", err)
    }

    return nil
}
```

**Event Publishing Pattern:**
- Extract ActorID from `auth.UserFromContext(ctx)` to track who initiated the mutation
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
    _, err = q.CreateUser(ctx, sqlcgen.CreateUserParams{
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
    return nil
}
```

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
        return nil, domainErrorToConnect(err)
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

func domainErrorToConnect(err error) error {
    var de *errors.DomainError
    if errors.As(err, &de) {
        return connect.NewError(codeToConnectCode(de.Code), de)
    }
    return connect.NewError(connect.CodeInternal, err)
}
```

## Event System

### Event Structure

All domain events follow a consistent structure with ActorID for audit trails:

```go
// UserCreatedEvent is published when a user is created
type UserCreatedEvent struct {
    UserID    string    `json:"user_id"`   // Resource ID
    ActorID   string    `json:"actor_id"`  // User who initiated action
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    Role      string    `json:"role"`
    IPAddress string    `json:"ip_address,omitempty"` // Client IP for audit trail
    At        time.Time `json:"at"`        // Event timestamp
}

// UserUpdatedEvent is published when a user is updated
type UserUpdatedEvent struct {
    UserID    string    `json:"user_id"`
    ActorID   string    `json:"actor_id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Role      string    `json:"role"`
    IPAddress string    `json:"ip_address,omitempty"`
    At        time.Time `json:"at"`
}

// UserDeletedEvent is published when a user is soft-deleted
type UserDeletedEvent struct {
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
const (
    TopicUserCreated = "user.created"
    TopicUserUpdated = "user.updated"
    TopicUserDeleted = "user.deleted"
)
```

### Cross-Module Event Consumption

Subscriber modules import event types from `internal/shared/events/contracts/`,
not from other modules' `domain/` packages. This preserves the no-cross-module-imports rule.
If the subscriber only needs a few fields, prefer a local struct (as `audit` does with `auditPayload`).

```go
// notification/subscriber.go — importing from shared contracts (correct)
import "github.com/gnha/gnha-services/internal/shared/events/contracts"

var event contracts.UserCreatedEvent
json.Unmarshal(msg.Payload, &event)

// audit/subscriber.go — local struct (also acceptable, lower coupling)
type auditPayload struct {
    UserID    string `json:"user_id"`
    ActorID   string `json:"actor_id"`
    IPAddress string `json:"ip_address,omitempty"`
}
```

## Pagination

### Cursor-Based Pagination Pattern

List endpoints use cursor-based pagination for efficient large-dataset traversal:

```go
// Repository signature
List(ctx context.Context, limit int, cursor string) (ListResult, error)
// Returns: (ListResult{Users, NextCursor, HasMore}, error)
```

**Implementation Details:**
- Internally fetches `limit+1` records to detect whether more pages exist
- Avoids extra COUNT queries or offset calculations
- Returns `nextCursor` only if `hasMore` is true
- Cursor is an opaque base64-encoded string containing timestamp+UUID for keyset pagination

**Usage Pattern:**
1. Client requests: `List(limit: 20, cursor: "")`
2. Repository returns: `([20 users], "cursor-abc...", true, nil)`
3. Client requests next page: `List(limit: 20, cursor: "cursor-abc...")`
4. When `hasMore == false`, pagination is complete

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

    handler := app.NewCreateUserHandler(repo, &testutil.StubHasher{}, &testutil.NoopPublisher{})

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
    "github.com/gnha/gnha-services/internal/shared/testutil"
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

## Code Quality Guidelines

- **Keep files under 200 lines**: Split large files by responsibility
- **Single Responsibility**: One concept per file/function
- **No cyclic imports**: Use dependency injection (fx)
- **Explicit dependencies**: Pass everything as parameters, no global state
- **Defer cleanup**: Use `defer` for resource cleanup
- **Context propagation**: Always pass `context.Context` as first parameter
- **No hardcoded values**: Use configuration and constants
- **Comments for "why"**: Explain reasoning, not what code does
