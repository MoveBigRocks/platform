#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST_PATH="$ROOT_DIR/docs/evidence/canonical-workspace-refs.json"
WORKSPACE_ROOT=""
VERIFICATION_OUT=""

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/bootstrap-canonical-workspace.sh --workspace-root PATH [--manifest PATH] [--verify-out PATH]
EOF
}

while (($# > 0)); do
  case "$1" in
    --workspace-root)
      WORKSPACE_ROOT="${2:-}"
      shift 2
      ;;
    --manifest)
      MANIFEST_PATH="${2:-}"
      shift 2
      ;;
    --verify-out)
      VERIFICATION_OUT="${2:-}"
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

if [[ -z "$WORKSPACE_ROOT" ]]; then
  usage
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to bootstrap the canonical workspace" >&2
  exit 1
fi
if ! command -v git >/dev/null 2>&1; then
  echo "git is required to bootstrap the canonical workspace" >&2
  exit 1
fi
if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "canonical workspace refs manifest not found: $MANIFEST_PATH" >&2
  exit 1
fi

WORKSPACE_ROOT="$(mkdir -p "$WORKSPACE_ROOT" && cd "$WORKSPACE_ROOT" && pwd)"
if [[ -z "$VERIFICATION_OUT" ]]; then
  VERIFICATION_OUT="$WORKSPACE_ROOT/canonical-workspace-refs-verification.json"
fi

while IFS=$'\t' read -r name repository rel_path ref; do
  target="$WORKSPACE_ROOT/$rel_path"
  if [[ -e "$target" && ! -e "$target/.git" ]]; then
    echo "target exists but is not a git checkout: $target" >&2
    exit 1
  fi

  if [[ ! -d "$target" ]]; then
    mkdir -p "$(dirname "$target")"
    git clone "https://github.com/${repository}.git" "$target" >/dev/null
  fi

  if [[ -n "$(git -C "$target" status --short)" ]]; then
    echo "refusing to change dirty checkout: $target" >&2
    exit 1
  fi

  git -C "$target" fetch --tags origin >/dev/null
  git -C "$target" checkout --detach "$ref" >/dev/null
  echo "Prepared ${name} at ${target} (${ref})"
done < <(jq -r '.repositories | to_entries[] | [.key, .value.repository, .value.path, .value.ref] | @tsv' "$MANIFEST_PATH")

bash "$ROOT_DIR/scripts/verify-canonical-workspace-refs.sh" \
  --manifest "$MANIFEST_PATH" \
  --workspace-root "$WORKSPACE_ROOT" \
  --out "$VERIFICATION_OUT"

cat <<EOF

Canonical workspace ready: $WORKSPACE_ROOT
Verification: $VERIFICATION_OUT

Next step:
  MBR_WORKSPACE_ROOT="$WORKSPACE_ROOT" make -C "$ROOT_DIR" milestone-proof
EOF
