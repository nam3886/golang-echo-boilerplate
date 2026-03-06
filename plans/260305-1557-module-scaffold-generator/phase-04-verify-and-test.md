# Phase 4: Verify + Test

**Priority:** High | **Status:** completed | **Effort:** 1h

## Overview

End-to-end verification: scaffold a test module, compile, lint, run tests, then clean up.

## Implementation Steps

### 1. Scaffold test module

```bash
task module:create name=product
```

Verify: 17 files created + codegen output clean.

### 2. Compile check

```bash
go build ./internal/modules/product/...
go vet ./internal/modules/product/...
```

### 3. Lint

```bash
buf lint
golangci-lint run ./internal/modules/product/...
```

### 4. Unit tests

```bash
go test -race -count=1 ./internal/modules/product/...
```

### 5. Fix any issues

Iterate on templates until all checks pass.

### 6. Conflict detection test

```bash
# Second run should fail
task module:create name=product
# Expected: error "files already exist"
```

### 7. Clean up test module

Remove scaffolded product module files (proto, migration, queries, internal/modules/product/).

### 8. Verify scaffold CLI builds

```bash
go build ./cmd/scaffold
```

## Success Criteria

- [x] Scaffold generates all files without error
- [x] Generated Go code compiles (go build + go vet)
- [x] Proto passes buf lint
- [x] Go passes golangci-lint
- [x] Unit tests compile and run
- [x] Conflict detection works on re-run
- [x] Scaffold CLI itself compiles cleanly
- [x] Clean up leaves no artifacts

## Risk Mitigation

- sqlc codegen order verified — migrations properly sequenced
- Proto import paths match buf.yaml module config
- mockgen destination path correct relative to domain/repository.go

## Implementation Notes

- All 4 success criteria fully verified
- Code review feedback applied (H-1, M1-M4)
- Conflict detection tested and working
- Test module cleaned up, no artifacts left
