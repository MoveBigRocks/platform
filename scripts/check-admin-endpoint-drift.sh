#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEMPLATES_DIR="$REPO_ROOT/web/admin-panel/templates"

if [ ! -d "$TEMPLATES_DIR" ]; then
  exit 0
fi

# Detect deprecated admin-facing endpoint patterns that should no longer be used in admin templates.
LEGACY_RE='"?/api/(workspaces|users|rules|forms|applications|analytics|oauth|authorizations|clients|graphql)|/api/\w+'

cd "$REPO_ROOT"

HITS=$(rg -n --no-heading -g '*.html' -e "$LEGACY_RE" "$TEMPLATES_DIR" || true)

if [ -n "$HITS" ]; then
  echo "FAILED: deprecated admin template routes detected in web/admin-panel/templates"
  echo "Expected internal routes: /admin/actions/* and /admin/graphql"
  echo ""
  echo "$HITS"
  exit 1
fi

echo "Admin template endpoint check passed"
