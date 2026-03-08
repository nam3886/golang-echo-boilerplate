---
name: ck:figma-to-code
description: "Generate UI code from Figma designs via Figma Make, then pull and refactor into clean, production-ready components. Use when implementing UI from Figma designs."
argument-hint: "--figma-url <URL> --target <path> [--make-url <URL>] [--fix-ui <description>]"
---

# /figma-to-code - Figma Design to Clean Components

Generates UI code via Figma Make browser automation, pulls the output, and refactors it into clean, project-ready components.

## Arguments

| Arg | Required | Description |
|-----|----------|-------------|
| `--figma-url` | Yes | Figma design URL to generate code from |
| `--target` | Yes | Target directory for output components |
| `--make-url` | No | Existing Figma Make project URL (skip new project creation) |
| `--fix-ui` | No | Re-enter Figma Make with a UI fix prompt |

## Pre-flight Checks

Before any phase, verify:
1. `.figma-to-code.json` exists in project root — see `references/config-schema.md`
2. `.env.figma` exists and is gitignored — see `references/config-schema.md`
3. `agent-browser` CLI installed: `agent-browser --version`

If any check fails, report to user with fix instructions. Do NOT proceed.

## Phase Router

| Condition | Action |
|-----------|--------|
| `--fix-ui` provided | Phase 1 (existing project) -> Phase 2. See `references/bug-fix-loops.md` |
| `--make-url` provided | Skip Phase 1 creation, start Phase 2 (pull from existing) |
| Default | Phase 1 -> Phase 2 (full pipeline) |

## Phases

### Phase 1: Figma Make Generation
Generate UI code from Figma design via browser automation.
- Login to Figma (with session persistence)
- Create new or update existing Figma Make project
- Send design URL + generation prompt
- Wait for generation completion
- Save and push to GitHub

See `references/figma-make-automation.md` for step-by-step instructions.

### Phase 2: Pull & Refactor
Pull generated code from GitHub and refactor until clean.
- Clone/pull from Figma Make GitHub repo
- Rename Vietnamese component names to English
- Split monolith components (<200 lines each)
- Map hardcoded values to project design tokens
- Replace absolute positioning with flex/grid
- Clean up SVG files with meaningful names
- Copy to `--target` path
- Verify TypeScript compiles

See `references/refactor-strategy.md` for pipeline details.

## Config

Project config: `.figma-to-code.json` — see `references/config-schema.md` for full schema.

## Security Policy

- NEVER log, display, or commit credentials from `.env.figma`
- Auth state files (`figma-auth.json`) must be gitignored
- All screenshots stored in `/tmp`, cleaned up after use
