# Config Schema Reference

## `.figma-to-code.json`

Project-level config file. Must exist in project root before running `/figma-to-code`.

### Full Schema

```json
{
  "project": "project-name",
  "figma": {
    "makeRepo": "user/repo-name",
    "makeUrl": "https://www.figma.com/make/...",
    "defaultModel": "Default"
  },
  "paths": {
    "components": "src/components"
  },
  "designSystem": {
    "colorTokens": "src/styles/tokens.ts",
    "typography": "src/styles/typography.ts",
    "baseComponents": "src/components/ui"
  }
}
```

### Field Descriptions

| Field | Required | Description |
|-------|----------|-------------|
| `project` | Yes | Project identifier for logging |
| `figma.makeRepo` | Yes | GitHub repo where Figma Make pushes code (user/repo) |
| `figma.makeUrl` | No | Existing Figma Make project URL (reuse across runs) |
| `figma.defaultModel` | No | Figma Make AI model selection (default: "Default") |
| `paths.components` | Yes | Target directory for UI components |
| `designSystem.colorTokens` | No | Path to color token definitions |
| `designSystem.typography` | No | Path to typography definitions |
| `designSystem.baseComponents` | No | Path to base UI component library |

### Validation Rules

- `project` and `figma.makeRepo` are required
- `paths.components` is required (must be relative path)
- All relative paths resolve from project root

## `.env.figma`

Credentials file. **MUST be gitignored.**

### Template

```env
FIGMA_EMAIL=your@email.com
FIGMA_PASSWORD=your-password
GITHUB_PAT=ghp_xxxxx
GITHUB_USER=your-username
```

### Gitignore Verification

Before running, verify `.env.figma` is in `.gitignore`:
```bash
grep -q ".env.figma" .gitignore || echo "WARNING: .env.figma not in .gitignore!"
```

### Example Configs

#### React + Vite Project
```json
{
  "project": "gnha-web",
  "figma": { "makeRepo": "user/figma-make-output" },
  "paths": { "components": "src/components" },
  "designSystem": { "baseComponents": "src/components/ui" }
}
```

#### Next.js Project
```json
{
  "project": "my-app",
  "figma": { "makeRepo": "user/figma-make-output" },
  "paths": { "components": "src/components" },
  "designSystem": { "colorTokens": "src/styles/tokens.ts" }
}
```
