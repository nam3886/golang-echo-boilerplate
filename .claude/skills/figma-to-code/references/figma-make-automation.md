# Figma Make Browser Automation

All automation via `agent-browser` CLI. No custom Puppeteer scripts.

## Pre-flight

```bash
agent-browser --version  # Must be installed
test -f .env.figma       # Must exist
```

## Login & Session Persistence

```bash
# 1. Try loading saved auth state
agent-browser state load figma-auth.json

# 2. Navigate to Figma Make
agent-browser open https://www.figma.com/make
agent-browser wait --idle
agent-browser snapshot -i

# 3. Check if logged in (look for chat input in snapshot)
# If chat input found → logged in, skip to project flow
# If login form found → proceed with login:

agent-browser open https://www.figma.com/login
agent-browser wait --idle
agent-browser snapshot -i
```

### Login Flow (if not authenticated)

```bash
# Source credentials from .env.figma (NEVER log these values)
# Read FIGMA_EMAIL and FIGMA_PASSWORD

# Each `find` command returns a ref (e.g. @e1). Use that ref in the next command.
agent-browser find testid "email"          # → returns @e1
agent-browser fill @e1 "$FIGMA_EMAIL"
agent-browser find testid "password"       # → returns @e2
agent-browser fill @e2 "$FIGMA_PASSWORD"
agent-browser find text "Log in"           # → returns @e3 (specific login button)
agent-browser click @e3
agent-browser wait --url "/files"
agent-browser state save figma-auth.json
```

## New Project Flow

When `--make-url` is NOT provided:

```bash
# 1. Open blank Figma Make
agent-browser open https://www.figma.com/make
agent-browser wait --idle
agent-browser snapshot -i

# 2. Find chat input and compose prompt
# Each `find` returns a ref like @e1 — use the actual ref returned
agent-browser find testid "empty-state-chat-input"     # → @e1
agent-browser fill @e1 "Build a React component matching this Figma design: [--figma-url]. Use shadcn/ui components and Tailwind CSS."
agent-browser find testid "empty-state-chat-send-button"  # → @e2
agent-browser click @e2

# 3. Wait for generation (see detection below)
# 4. Save project
agent-browser find text "Save"                          # → @e3
agent-browser click @e3

# 5. Push to GitHub (see push flow)
```

## Existing Project Flow

When `--make-url` IS provided:

```bash
agent-browser open [--make-url]
agent-browser wait --idle
agent-browser snapshot -i

# Each `find` returns a ref (e.g. @e1) — use the actual ref returned
agent-browser find testid "code-chat-chat-box"     # → @e1
agent-browser fill @e1 "[change prompt]"
agent-browser find testid "code-chat-send-button"  # → @e2
agent-browser click @e2

# Wait for generation, then save + push
```

## Generation Detection

Poll until generation completes (max 120s, interval 5s):

```bash
# Loop:
agent-browser snapshot -i -s "[chat area]"
# Check snapshot output:
#   - "Stop" button present → still generating, wait 5s, re-poll
#   - "Stop" absent AND "edited" text present → DONE
#   - Timeout after 120s → report error, offer manual intervention
```

Use `agent-browser wait --fn "() => !document.querySelector('[data-testid*=stop]')"` as alternative.

## GitHub Push Flow

```bash
agent-browser find testid "figmake-settings-menu-button"  # → @e1
agent-browser click @e1
agent-browser snapshot -i

agent-browser find text "GitHub"    # → @e2
agent-browser click @e2
agent-browser snapshot -i

agent-browser find text "Push to"   # → @e3
agent-browser click @e3
agent-browser wait --idle
# Wait for push confirmation in snapshot
```

## Prompt Templates

### New Project (default prompt)

Compose from this template, adjusting framework per `.figma-to-code.json`:

```
You are a senior frontend engineer implementing UI from Figma with pixel-perfect accuracy.

Goal:
Implement the UI exactly as defined in the Figma design with maximum visual fidelity.
The output must match the design as closely as possible (spacing, typography, colors, layout, and components).

Figma Design:
{--figma-url}

Framework:
- {framework from project — e.g. "Vue 3 + TypeScript" or "React + TypeScript"}
- TailwindCSS
- Responsive web

Implementation rules:

1. Pixel Perfect Priority
- Match spacing, padding, margin, border radius, and sizes exactly as in Figma.
- Use the exact font sizes, font weights, and line heights.
- Use the exact colors and opacity values.
- Do NOT approximate values.

2. Layout Rules
- Follow Figma auto-layout structure.
- Preserve the hierarchy of frames and components.
- Avoid modifying layout logic unless absolutely required.

3. Component Strategy
- Break UI into reusable components when possible.
- Reuse components instead of duplicating layout.

4. Styling Rules
- Prefer Tailwind utility classes.
- If Tailwind cannot express the exact value, use custom CSS variables.

5. Responsive Behavior
- Maintain the layout proportions across screen sizes.
- Do not change the visual hierarchy from the design.

6. No Design Guessing
- If something is unclear in the Figma file, ask for clarification instead of making assumptions.

7. Validation Step
After generating the code:
- Compare each UI element with the Figma design
- Verify spacing, typography, alignment, responsive behavior

8. Output format
Provide:
- Component structure
- Full code implementation
- Notes about any unavoidable deviation from the design.

Important:
The design accuracy is more important than code optimization.
Never simplify the UI if it changes the visual result.
```

### Existing Project (change)
> Update the component: {change-description}

### Bug Fix (from --fix-ui)
> Fix: {bug-description}. The current issue is: {details}

## Error Recovery

| Error | Recovery |
|-------|----------|
| Login fails (wrong credentials) | Check `.env.figma` values, retry once |
| Login blocked (CAPTCHA/2FA) | Report to user, offer manual login + state save |
| Generation timeout (>120s) | Increase poll to 180s, report if still pending |
| Save fails | Re-snapshot, find Save button, retry |
| Push fails | Check GitHub connection in settings, re-push |
| State file corrupted | Delete `figma-auth.json`, re-login |
