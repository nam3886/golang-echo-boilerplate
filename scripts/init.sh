#!/usr/bin/env bash
set -euo pipefail

# Cross-platform sed -i
sedi() {
  if sed --version >/dev/null 2>&1; then
    sed -i "$@"   # GNU sed
  else
    sed -i '' "$@" # BSD sed (macOS)
  fi
}

# Read current module path from go.mod
OLD_MODULE=$(head -1 go.mod | awk '{print $2}')
OLD_APP=$(basename "$OLD_MODULE")

# Accept args or prompt
MODULE_PATH="${1:-}"
APP_NAME="${2:-}"

if [[ -z "$MODULE_PATH" ]]; then
  read -rp "New Go module path (e.g. github.com/myorg/myproject): " MODULE_PATH
fi
if [[ -z "$MODULE_PATH" ]]; then
  echo "Error: module path is required." >&2
  exit 1
fi
if [[ -z "$APP_NAME" ]]; then
  DEFAULT_APP=$(basename "$MODULE_PATH")
  read -rp "App name [$DEFAULT_APP]: " APP_NAME
  APP_NAME="${APP_NAME:-$DEFAULT_APP}"
fi

if [[ "$MODULE_PATH" == "$OLD_MODULE" ]]; then
  echo "Module path is already $OLD_MODULE — nothing to do."
  exit 0
fi

echo "Renaming module: $OLD_MODULE → $MODULE_PATH"
echo "Renaming app: $OLD_APP → $APP_NAME"

# Replace Go module path in source files
find . \( -name '*.go' -o -name '*.proto' \) \
  -not -path './plans/*' -not -path './.claude/*' -not -path './vendor/*' \
  -exec grep -l "$OLD_MODULE" {} + | while IFS= read -r f; do
  sedi "s|$OLD_MODULE|$MODULE_PATH|g" "$f"
done

# Update go.mod module declaration
go mod edit -module "$MODULE_PATH"

# Replace app name in config files
sedi "s|APP_NAME: $OLD_APP|APP_NAME: $APP_NAME|g" Taskfile.yml
sedi "s|APP_NAME=$OLD_APP|APP_NAME=$APP_NAME|g" .env .env.example 2>/dev/null || true

# Update Docker image reference
ORG=$(echo "$MODULE_PATH" | cut -d'/' -f2)
sedi "s|ghcr.io/[^/]*/[^}]*|ghcr.io/$ORG/$APP_NAME|g" deploy/docker-compose.yml

# Update database name references
sedi "s|${OLD_APP//-/_}_dev|${APP_NAME//-/_}_dev|g" Taskfile.yml
sedi "s|${OLD_APP//-/_}_dev|${APP_NAME//-/_}_dev|g" .env .env.example 2>/dev/null || true

# Replace in documentation
find docs -name '*.md' -exec grep -l "$OLD_MODULE\|$OLD_APP" {} + 2>/dev/null | while IFS= read -r f; do
  sedi "s|$OLD_MODULE|$MODULE_PATH|g" "$f"
  sedi "s|$OLD_APP|$APP_NAME|g" "$f"
done
if grep -q "$OLD_MODULE\|$OLD_APP" README.md 2>/dev/null; then
  sedi "s|$OLD_MODULE|$MODULE_PATH|g" README.md
  sedi "s|$OLD_APP|$APP_NAME|g" README.md
fi

# Update JWT audience constant
sedi "s|jwtAudience = \"$OLD_APP\"|jwtAudience = \"$APP_NAME\"|g" internal/shared/auth/jwt.go
# Update JWT test audience assertion
sedi "s|$OLD_APP|$APP_NAME|g" internal/shared/auth/jwt_test.go
# Update config default
sedi "s|envDefault:\"$OLD_APP\"|envDefault:\"$APP_NAME\"|g" internal/shared/config/config.go

# Regenerate proto (go_package changed)
echo "Regenerating proto..."
buf generate || echo "Warning: buf generate failed. Run 'task generate:proto' manually."

# Validate
echo "Running go mod tidy..."
go mod tidy

echo ""
echo "Done! Module renamed to: $MODULE_PATH"
echo "Done! App renamed to: $APP_NAME"
echo ""
echo "Next steps:"
echo "  1. go build ./...    — verify compilation"
echo "  2. go test ./...     — verify tests"
echo "  3. git add -A && git commit -m 'chore: initialize project as $APP_NAME'"
