# Phase 3: Code Gen Pipeline

**Priority:** P0 | **Effort:** M (2-4h) | **Status:** completed
**Depends on:** Phase 2
**Completed:** 2026-03-04

## Context

- [Brainstorm Report](../reports/brainstorm-260304-1636-golang-api-boilerplate.md) — Protobuf-first, buf, sqlc, code gen flow

## Overview

Set up buf CLI for protobuf code generation (Connect RPC + OpenAPI + validation), sqlc for SQL→Go, and Taskfile tasks to orchestrate the full pipeline.

## Files to Create

```
buf.yaml                              # buf module config
buf.gen.yaml                          # buf code generation config
buf.lock                              # buf dependency lock
sqlc.yaml                             # sqlc config
proto/user/v1/user.proto              # Example proto (user module)
db/migrations/00001_initial_schema.sql # Initial migration (users + audit_logs)
db/queries/user.sql                   # Example sqlc queries
```

## Implementation Steps

### 1. buf.yaml — module config
```yaml
version: v2
modules:
  - path: proto
deps:
  - buf.build/googleapis/googleapis
  - buf.build/bufbuild/protovalidate
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

### 2. buf.gen.yaml — code generation
```yaml
version: v2
plugins:
  # Go protobuf types
  - remote: buf.build/protocolbuffers/go
    out: gen/proto
    opt: paths=source_relative

  # Connect RPC Go handlers
  - remote: buf.build/connectrpc/go
    out: gen/proto
    opt: paths=source_relative

  # Protovalidate Go
  - remote: buf.build/bufbuild/protovalidate-go
    out: gen/proto
    opt: paths=source_relative

  # OpenAPI v2 spec
  - remote: buf.build/grpc-ecosystem/openapiv2
    out: gen/openapi

  # TypeScript client (connect-es)
  - remote: buf.build/connectrpc/es
    out: gen/ts
```

### 3. Example proto — user service
```protobuf
// proto/user/v1/user.proto
syntax = "proto3";
package user.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

service UserService {
  rpc GetUser(GetUserRequest) returns (User);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc CreateUser(CreateUserRequest) returns (User);
  rpc UpdateUser(UpdateUserRequest) returns (User);
  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
}

message User {
  string id = 1;
  string email = 2;
  string name = 3;
  string role = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}

message CreateUserRequest {
  string email = 1 [(buf.validate.field).string.email = true];
  string name = 2 [(buf.validate.field).string = {min_len: 1, max_len: 255}];
  string password = 3 [(buf.validate.field).string.min_len = 8];
  string role = 4 [(buf.validate.field).string = {in: ["admin", "member", "viewer"]}];
}

message GetUserRequest {
  string id = 1 [(buf.validate.field).string.uuid = true];
}

message ListUsersRequest {
  int32 limit = 1 [(buf.validate.field).int32 = {gte: 1, lte: 100}];
  string cursor = 2;
}

message ListUsersResponse {
  repeated User items = 1;
  string next_cursor = 2;
  bool has_more = 3;
}

message UpdateUserRequest {
  string id = 1 [(buf.validate.field).string.uuid = true];
  optional string name = 2;
  optional string role = 3;
}

message DeleteUserRequest {
  string id = 1 [(buf.validate.field).string.uuid = true];
}

message DeleteUserResponse {}
```

### 4. sqlc.yaml
```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries/"
    schema: "db/migrations/"
    gen:
      go:
        package: "sqlcgen"
        out: "gen/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "jsonb"
            go_type: "json.RawMessage"
            nullable: true
```

### 5. Initial migration
```sql
-- db/migrations/00001_initial_schema.sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      VARCHAR(255) NOT NULL UNIQUE,
    name       VARCHAR(255) NOT NULL,
    password   TEXT NOT NULL,
    role       VARCHAR(50) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_active ON users (id) WHERE deleted_at IS NULL;

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID NOT NULL,
    action      VARCHAR(20) NOT NULL,
    actor_id    UUID NOT NULL,
    changes     JSONB,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_logs (actor_id);
CREATE INDEX idx_audit_created ON audit_logs (created_at);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS users;
```

### 6. Example sqlc queries
```sql
-- db/queries/user.sql

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at'), sqlc.narg('cursor_id')))
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: CreateUser :one
INSERT INTO users (email, name, password, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    role = COALESCE(sqlc.narg('role'), role),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :exec
UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateAuditLog :exec
INSERT INTO audit_logs (entity_type, entity_id, action, actor_id, changes, ip_address)
VALUES ($1, $2, $3, $4, $5, $6);
```

### 7. Taskfile generate tasks
```yaml
# Add to Taskfile.yml
generate:
  desc: Run all code generators
  cmds:
    - task: generate:proto
    - task: generate:sqlc

generate:proto:
  desc: Generate from protobuf
  sources: [proto/**/*.proto, buf.gen.yaml]
  generates: [gen/proto/**/*.go]
  cmds:
    - buf lint
    - buf breaking --against '.git#branch=main' || true
    - buf generate

generate:sqlc:
  desc: Generate from SQL
  sources: [db/queries/*.sql, db/migrations/*.sql, sqlc.yaml]
  generates: [gen/sqlc/*.go]
  cmds:
    - sqlc generate
```

### 8. Verify generated code compiles
```bash
task generate
go build ./...
```

## Todo

- [x] buf.yaml with deps (googleapis, protovalidate)
- [x] buf.gen.yaml with plugins (go, connect, protovalidate, openapi, es)
- [x] Example user.proto with protovalidate rules
- [x] sqlc.yaml with pgx/v5 + UUID + JSONB overrides
- [x] Initial migration (users + audit_logs)
- [x] sqlc queries (CRUD + cursor pagination + soft delete)
- [x] `buf dep update` to lock deps
- [x] `task generate` runs both proto + sqlc
- [x] Verify generated Go code compiles
- [x] Verify OpenAPI spec generated

## Success Criteria

- `task generate:proto` → Go Connect RPC code + OpenAPI spec in `gen/`
- `task generate:sqlc` → type-safe Go queries in `gen/sqlc/`
- `go build ./...` passes with generated code
- Proto validation rules present in generated code
- Cursor pagination query works with sqlc

## Next Steps

→ Phase 4: Auth & Security (JWT, RBAC, password hashing, middleware chain)
