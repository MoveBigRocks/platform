#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

GO_CACHE_DIR="${GOCACHE:-/tmp/mbr-go-cache}"
mkdir -p "$GO_CACHE_DIR"

GOCACHE="$GO_CACHE_DIR" go run ./cmd/tools/sync-agent-cli-doc --check

missing_token_files=()
while IFS= read -r file; do
  if ! rg -q -- "--license-token|MBR_LICENSE_TOKEN" "$file"; then
    missing_token_files+=("$file")
  fi
done < <(rg -l "mbr extensions install" README.md START_WITH_AN_AGENT.md docs)

if [[ ${#missing_token_files[@]} -gt 0 ]]; then
  echo "FAILED: install examples missing license-token guidance:"
  printf '  %s\n' "${missing_token_files[@]}"
  exit 1
fi

echo "CLI contract doc check passed"
