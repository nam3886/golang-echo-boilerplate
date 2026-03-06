# Phase 5: Example Module (User)

**Priority:** P0 | **Effort:** L (4-8h) | **Status:** completed
**Depends on:** Phase 4
**Completed:** 2026-03-04

## Context

- [Architecture Patterns](../reports/researcher-260304-1437-golang-architecture-patterns.md) — Hexagonal, repository per aggregate, closure-based transactions

## Overview

Implement a complete "user" module demonstrating the full hexagonal architecture: domain entities, application services (command/query handlers), adapters (PostgreSQL repository, Connect RPC handler, Echo HTTP handler), and Fx module wiring. This serves as the canonical example for creating new modules.

## Architecture

```
internal/modules/user/
  domain/
    user.go            # Entity + value objects + constructor with validation
    repository.go      # Repository interface (port)
    errors.go          # Module-specific domain errors
  app/
    create_user.go     # CreateUser command handler
    get_user.go        # GetUser query handler
    list_users.go      # ListUsers query handler
    update_user.go     # UpdateUser command handler
    delete_user.go     # DeleteUser (soft) command handler
  adapters/
    postgres/
      repository.go    # Repository implementation using sqlc
    grpc/
      handler.go       # Connect RPC UserService handler
    http/
      handler.go       # Echo handlers (health, non-RPC routes)
  module.go            # Fx module definition
```

## Implementation Steps

### 1. Domain entity with encapsulated validation
```go
// internal/modules/user/domain/user.go
type UserID string
type Role string
const (
    RoleAdmin  Role = "admin"
    RoleMember Role = "member"
    RoleViewer Role = "viewer"
)

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

// Constructor validates invariants
func NewUser(email, name, hashedPassword string, role Role) (*User, error) {
    if email == "" { return nil, ErrEmailRequired }
    if name == "" { return nil, ErrNameRequired }
    if !role.IsValid() { return nil, ErrInvalidRole }
    return &User{
        id: UserID(uuid.NewString()),
        email: email, name: name, password: hashedPassword, role: role,
        createdAt: time.Now(), updatedAt: time.Now(),
    }, nil
}

// Getters (no setters — mutation through explicit methods)
func (u *User) ID() UserID       { return u.id }
func (u *User) Email() string    { return u.email }
// ...

// Business logic methods
func (u *User) ChangeName(name string) error {
    if name == "" { return ErrNameRequired }
    u.name = name
    u.updatedAt = time.Now()
    return nil
}

func (u *User) ChangeRole(role Role) error {
    if !role.IsValid() { return ErrInvalidRole }
    u.role = role
    u.updatedAt = time.Now()
    return nil
}
```

### 2. Repository interface (port)
```go
// internal/modules/user/domain/repository.go
type UserRepository interface {
    GetByID(ctx context.Context, id UserID) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    List(ctx context.Context, limit int, cursor string) ([]*User, string, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, id UserID, fn func(*User) error) error
    SoftDelete(ctx context.Context, id UserID) error
}
```

### 3. Application service — command handler
```go
// internal/modules/user/app/create_user.go
type CreateUserCmd struct {
    Email    string
    Name     string
    Password string
    Role     string
}

type CreateUserHandler struct {
    repo   domain.UserRepository
    hasher auth.PasswordHasher
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) (*domain.User, error) {
    // Check email uniqueness
    existing, err := h.repo.GetByEmail(ctx, cmd.Email)
    if err != nil && !errors.Is(err, shared.ErrNotFound) {
        return nil, err
    }
    if existing != nil {
        return nil, shared.ErrAlreadyExists
    }

    hashedPwd, err := h.hasher.Hash(cmd.Password)
    if err != nil { return nil, fmt.Errorf("hashing password: %w", err) }

    user, err := domain.NewUser(cmd.Email, cmd.Name, hashedPwd, domain.Role(cmd.Role))
    if err != nil { return nil, err }

    if err := h.repo.Create(ctx, user); err != nil {
        return nil, fmt.Errorf("creating user: %w", err)
    }
    return user, nil
}
```

### 4. Repository implementation (adapter) — closure-based UoW
```go
// internal/modules/user/adapters/postgres/repository.go
type PgUserRepository struct {
    pool *pgxpool.Pool
}

func (r *PgUserRepository) Update(ctx context.Context, id domain.UserID, fn func(*domain.User) error) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil { return err }
    defer tx.Rollback(ctx)

    queries := sqlcgen.New(tx)
    row, err := queries.GetUserByID(ctx, uuid.MustParse(string(id)))
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) { return shared.ErrNotFound }
        return err
    }

    user := toDomain(row)
    if err := fn(user); err != nil { return err }

    _, err = queries.UpdateUser(ctx, toUpdateParams(user))
    if err != nil { return err }

    return tx.Commit(ctx)
}

// toDomain: sqlcgen.User → domain.User
// toUpdateParams: domain.User → sqlcgen.UpdateUserParams
```

### 5. Connect RPC handler (adapter)
```go
// internal/modules/user/adapters/grpc/handler.go
type UserServiceHandler struct {
    createUser *app.CreateUserHandler
    getUser    *app.GetUserHandler
    listUsers  *app.ListUsersHandler
    updateUser *app.UpdateUserHandler
    deleteUser *app.DeleteUserHandler
}

func (h *UserServiceHandler) CreateUser(
    ctx context.Context,
    req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.User], error) {
    // protovalidate already validated by interceptor
    user, err := h.createUser.Handle(ctx, app.CreateUserCmd{
        Email:    req.Msg.Email,
        Name:     req.Msg.Name,
        Password: req.Msg.Password,
        Role:     req.Msg.Role,
    })
    if err != nil { return nil, domainErrorToConnect(err) }

    return connect.NewResponse(toProto(user)), nil
}

// domainErrorToConnect: DomainError → connect.Error
```

### 6. Mount Connect handler in Echo
```go
// internal/modules/user/adapters/grpc/handler.go
func RegisterRoutes(e *echo.Echo, handler *UserServiceHandler, authMw echo.MiddlewareFunc) {
    path, h := userv1connect.NewUserServiceHandler(handler,
        connect.WithInterceptors(
            otelconnect.NewInterceptor(),
            validateInterceptor(),   // protovalidate
            authConnectInterceptor(), // auth for gRPC
        ),
    )
    // Mount Connect handler under Echo with auth middleware
    group := e.Group("", authMw)
    group.Any(path+"*", echo.WrapHandler(h))
}
```

### 7. Fx module wiring
```go
// internal/modules/user/module.go
var Module = fx.Module("user",
    fx.Provide(
        fx.Annotate(
            postgres.NewPgUserRepository,
            fx.As(new(domain.UserRepository)),
        ),
    ),
    fx.Provide(app.NewCreateUserHandler),
    fx.Provide(app.NewGetUserHandler),
    fx.Provide(app.NewListUsersHandler),
    fx.Provide(app.NewUpdateUserHandler),
    fx.Provide(app.NewDeleteUserHandler),
    fx.Provide(grpc.NewUserServiceHandler),
    fx.Invoke(grpc.RegisterRoutes),
)
```

### 8. Register in main.go
```go
// cmd/server/main.go
fx.New(
    shared.Module,
    user.Module,    // ← add
    fx.Invoke(startServer),
).Run()
```

## Todo

- [x] Domain entity (User) with encapsulated fields + constructor validation
- [x] Domain errors (ErrEmailRequired, ErrInvalidRole, etc.)
- [x] Repository interface in domain/
- [x] CreateUser command handler
- [x] GetUser query handler
- [x] ListUsers query handler (cursor pagination)
- [x] UpdateUser command handler (closure-based UoW)
- [x] DeleteUser command handler (soft delete)
- [x] PostgreSQL repository implementation (sqlc + pgx)
- [x] Domain↔DB type mappers (toDomain, toUpdateParams, toProto)
- [x] Connect RPC handler implementing UserService
- [x] DomainError → Connect Error mapper
- [x] protovalidate interceptor
- [x] Mount Connect handler in Echo
- [x] Fx module wiring
- [x] Register module in main.go
- [x] `task generate` + `go build ./...` passes
- [x] Manual test: create user → get user → list users → update → delete

## Success Criteria

- Full CRUD via Connect RPC (both gRPC and JSON)
- `curl -X POST http://localhost:8080/user.v1.UserService/CreateUser -H "Content-Type: application/json" -d '{"email":"test@test.com","name":"Test","password":"12345678","role":"member"}'` → 200
- Domain validation errors → 400 with structured error body
- Cursor pagination works for list endpoint
- Soft delete: deleted user not returned in list/get
- Closure-based transaction works for update

## Next Steps

→ Phase 6: Events & CQRS (Watermill, RabbitMQ, audit, notifications, cron)
