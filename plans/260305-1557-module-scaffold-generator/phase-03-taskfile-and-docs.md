# Phase 3: Taskfile + Docs Update

**Priority:** Medium | **Status:** completed | **Effort:** 30min

## Overview

Add `module:create` task to Taskfile.yml and update `adding-a-module.md` to reference the scaffold tool.

## Related Code Files

### Modify
- `Taskfile.yml` — add `module:create` task
- `docs/adding-a-module.md` — add scaffold section at top

## Implementation Steps

### 1. Taskfile.yml — Add task

```yaml
module:create:
  desc: Scaffold a new CRUD module
  cmds:
    - go run ./cmd/scaffold -name={{.name}} {{if .plural}}-plural={{.plural}}{{end}}
    - task: generate
  requires:
    vars: [name]
```

Place after `generate:mocks` task (code generation section).

### 2. docs/adding-a-module.md — Add scaffold section

Add at top of file (after title), before Step 1:

```markdown
## Quick Start (Recommended)

Run the scaffold generator to create all module files:

    task module:create name=product

For custom plural naming:

    task module:create name=category plural=categories

This creates 17 files + runs code generation. Then:
1. Customize proto fields in `proto/{name}/v1/{name}.proto`
2. Customize DB columns in `db/migrations/{timestamp}_create_{plural}.sql`
3. Customize SQL queries in `db/queries/{name}.sql`
4. Run `task generate` after customizing proto/SQL
5. Update domain entity, handlers, and adapters to match new fields
6. Register module in `cmd/server/main.go`
7. Run `task migrate:up && task check`

## Manual Steps (Reference)

The sections below detail what the scaffold generates, for reference.
```

## Success Criteria

- [x] `task module:create name=product` works end-to-end
- [x] `task -l` shows module:create with description
- [x] docs/adding-a-module.md has scaffold section at top

## Implementation Notes

- Taskfile.yml updated with module:create task
- docs/adding-a-module.md updated with Quick Start section
- Task properly chains scaffold → generate steps
