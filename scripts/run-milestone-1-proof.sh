#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/milestone-proof"
VERSION=""

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/run-milestone-1-proof.sh [--version VERSION] [--out PATH]
EOF
}

while (($# > 0)); do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --out)
      OUT_DIR="${2:-}"
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

if [[ -z "$VERSION" ]]; then
  VERSION="proof-$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
fi

mkdir -p "$OUT_DIR"

CLI_OUT_DIR="$OUT_DIR/cli-release"
SUMMARY_PATH="$OUT_DIR/summary.md"
GENERATED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
GIT_SHA="$(git -C "$ROOT_DIR" rev-parse HEAD)"

: "${STORAGE_TYPE:=filesystem}"
: "${STORAGE_PATH:=/tmp/mbr-proof}"
: "${FILESYSTEM_PATH:=${STORAGE_PATH}}"
: "${CACHE_ENABLED:=true}"
: "${EMAIL_BACKEND:=mock}"
: "${JWT_SECRET:=milestone-proof-secret-at-least-32-characters}"
: "${ENVIRONMENT:=test}"
: "${TRACING_ENABLED:=false}"
: "${ENABLE_METRICS:=false}"
: "${CLAMAV_ADDR:=}"

if [[ -n "${DATABASE_DSN:-}" ]]; then
  export DATABASE_DSN
  : "${TEST_DATABASE_ADMIN_DSN:=${DATABASE_DSN}}"
  export TEST_DATABASE_ADMIN_DSN
fi

export STORAGE_TYPE STORAGE_PATH FILESYSTEM_PATH
export CACHE_ENABLED EMAIL_BACKEND JWT_SECRET ENVIRONMENT TRACING_ENABLED ENABLE_METRICS CLAMAV_ADDR

run_step() {
  echo
  echo "==> $*"
  "$@"
}

cd "$ROOT_DIR"

run_step go test -count=1 ./internal/service/services ./internal/knowledge/services ./internal/platform/services ./cmd/api ./cmd/mbr
run_step bash scripts/check-cli-contract-docs.sh
run_step bash scripts/build-cli-release.sh --version "$VERSION" --out "$CLI_OUT_DIR"

cat >"$SUMMARY_PATH" <<EOF
# Milestone 1 Proof Summary

- generated_at: ${GENERATED_AT}
- git_sha: ${GIT_SHA}
- cli_release_dir: ${CLI_OUT_DIR}

## Commands Run

1. \`go test -count=1 ./internal/service/services ./internal/knowledge/services ./internal/platform/services ./cmd/api ./cmd/mbr\`
2. \`bash scripts/check-cli-contract-docs.sh\`
3. \`bash scripts/build-cli-release.sh --version ${VERSION} --out ${CLI_OUT_DIR}\`

## Evidence Docs

- [docs/MILESTONE_1_READINESS.md](../../docs/MILESTONE_1_READINESS.md)
- [docs/MILESTONE_1_PROOF.md](../../docs/MILESTONE_1_PROOF.md)
- [docs/FIRST_PARTY_PACK_READINESS.md](../../docs/FIRST_PARTY_PACK_READINESS.md)
- [docs/CLI_RELEASES.md](../../docs/CLI_RELEASES.md)
EOF

echo
echo "Proof summary: $SUMMARY_PATH"
