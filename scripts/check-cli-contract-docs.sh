#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

GO_CACHE_DIR="${GOCACHE:-/tmp/mbr-go-cache}"
mkdir -p "$GO_CACHE_DIR"

GOCACHE="$GO_CACHE_DIR" go run ./cmd/tools/sync-agent-cli-doc --check

guidance_pattern='--license-token|MBR_LICENSE_TOKEN|public signed bundles|without an instance-bound token'

has_guidance() {
  local pattern="$1"
  local file="$2"
  if command -v rg >/dev/null 2>&1; then
    rg -qi -- "$pattern" "$file"
    return
  fi
  grep -Eiq -- "$pattern" "$file"
}

list_install_doc_refs() {
  if command -v rg >/dev/null 2>&1; then
    rg -l -- "mbr extensions install" README.md START_WITH_AN_AGENT.md docs
    return
  fi
  grep -R -l -E -- "mbr extensions install" README.md START_WITH_AN_AGENT.md docs
}

missing_guidance_files=()
while IFS= read -r file; do
  if ! has_guidance "$guidance_pattern" "$file"; then
    missing_guidance_files+=("$file")
  fi
done < <(list_install_doc_refs)

if [[ ${#missing_guidance_files[@]} -gt 0 ]]; then
  echo "FAILED: install examples missing public-bundle or license-token guidance:"
  printf '  %s\n' "${missing_guidance_files[@]}"
  exit 1
fi

echo "CLI contract doc check passed"
