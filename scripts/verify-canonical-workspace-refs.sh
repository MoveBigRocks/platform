#!/usr/bin/env bash

set -euo pipefail

MANIFEST_PATH=""
WORKSPACE_ROOT=""
OUT_PATH=""

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/verify-canonical-workspace-refs.sh \
  --manifest PATH \
  --workspace-root PATH \
  [--out PATH]
EOF
}

while (($# > 0)); do
  case "$1" in
    --manifest)
      MANIFEST_PATH="${2:-}"
      shift 2
      ;;
    --workspace-root)
      WORKSPACE_ROOT="${2:-}"
      shift 2
      ;;
    --out)
      OUT_PATH="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$MANIFEST_PATH" || -z "$WORKSPACE_ROOT" ]]; then
  usage
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to verify canonical workspace refs" >&2
  exit 1
fi

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "canonical workspace refs manifest not found: $MANIFEST_PATH" >&2
  exit 1
fi

if [[ ! -d "$WORKSPACE_ROOT" ]]; then
  echo "workspace root not found: $WORKSPACE_ROOT" >&2
  exit 1
fi

tmp_results="$(mktemp)"
cleanup() {
  rm -f "$tmp_results"
}
trap cleanup EXIT
printf '[]\n' >"$tmp_results"

append_result() {
  local row_json="$1"
  local tmp_next
  tmp_next="$(mktemp)"
  jq --argjson row "$row_json" '. + [$row]' "$tmp_results" >"$tmp_next"
  mv "$tmp_next" "$tmp_results"
}

while IFS= read -r repo; do
  name="$(jq -r '.key' <<<"$repo")"
  repository="$(jq -r '.value.repository' <<<"$repo")"
  path_rel="$(jq -r '.value.path' <<<"$repo")"
  expected_ref="$(jq -r '.value.ref' <<<"$repo")"
  checkout_path="$WORKSPACE_ROOT/$path_rel"

  if [[ ! -d "$checkout_path" ]]; then
    echo "canonical workspace checkout missing for ${name}: ${checkout_path}" >&2
    exit 1
  fi

  if ! git -C "$checkout_path" rev-parse --git-dir >/dev/null 2>&1; then
    echo "canonical workspace checkout is not a git repository for ${name}: ${checkout_path}" >&2
    exit 1
  fi

  actual_ref="$(git -C "$checkout_path" rev-parse HEAD)"
  if [[ "$actual_ref" != "$expected_ref" ]]; then
    echo "canonical workspace ref mismatch for ${name}: expected ${expected_ref}, got ${actual_ref}" >&2
    exit 1
  fi

  row_json="$(jq -n \
    --arg name "$name" \
    --arg repository "$repository" \
    --arg path "$checkout_path" \
    --arg expectedRef "$expected_ref" \
    --arg actualRef "$actual_ref" \
    '{
      name: $name,
      repository: $repository,
      path: $path,
      expectedRef: $expectedRef,
      actualRef: $actualRef,
      match: true
    }')"
  append_result "$row_json"
done < <(jq -c '.repositories | to_entries[]' "$MANIFEST_PATH")

count="$(jq 'length' "$tmp_results")"

if [[ -n "$OUT_PATH" ]]; then
  mkdir -p "$(dirname "$OUT_PATH")"
  jq -n \
    --arg verifiedAt "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
    --arg manifestPath "$MANIFEST_PATH" \
    --arg workspaceRoot "$WORKSPACE_ROOT" \
    --slurpfile repositories "$tmp_results" \
    '{
      verifiedAt: $verifiedAt,
      manifestPath: $manifestPath,
      workspaceRoot: $workspaceRoot,
      repositories: $repositories[0]
    }' >"$OUT_PATH"
fi

echo "Verified ${count} canonical workspace checkout(s) from ${MANIFEST_PATH}"
