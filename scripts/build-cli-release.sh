#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/cli-release"
VERSION=""

export LC_ALL=C
export LANG=C

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/build-cli-release.sh [--version VERSION] [--out PATH]
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

if ! command -v zip >/dev/null 2>&1; then
  echo "zip is required to package Windows CLI archives" >&2
  exit 1
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return
  fi
  echo "sha256sum or shasum is required" >&2
  exit 1
}

if [[ -z "$VERSION" ]]; then
  if git -C "$ROOT_DIR" describe --tags --exact-match >/dev/null 2>&1; then
    VERSION="$(git -C "$ROOT_DIR" describe --tags --exact-match)"
  else
    VERSION="dev-$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
  fi
fi

GIT_SHA="$(git -C "$ROOT_DIR" rev-parse HEAD)"
BUILD_DATE="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

mkdir -p "$OUT_DIR"

BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

declare -a artifacts=()

build_archive() {
  local goos="$1"
  local goarch="$2"
  local archive_base="mbr_${VERSION}_${goos}_${goarch}"
  local stage_dir="$BUILD_DIR/${goos}_${goarch}"
  local binary_name="mbr"
  local archive_name=""
  local archive_path=""

  mkdir -p "$stage_dir"
  if [[ "$goos" == "windows" ]]; then
    binary_name="mbr.exe"
  fi

  (
    cd "$ROOT_DIR"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -o "$stage_dir/$binary_name" ./cmd/mbr
  )

  if [[ "$goos" == "windows" ]]; then
    archive_name="${archive_base}.zip"
    archive_path="$OUT_DIR/$archive_name"
    (
      cd "$stage_dir"
      zip -q "$archive_path" "$binary_name"
    )
  else
    archive_name="${archive_base}.tar.gz"
    archive_path="$OUT_DIR/$archive_name"
    tar -C "$stage_dir" -czf "$archive_path" "$binary_name"
  fi

  artifacts+=("$goos|$goarch|$archive_name|$(sha256_file "$archive_path")")
  echo "built $archive_name"
}

build_archive darwin amd64
build_archive darwin arm64
build_archive linux amd64
build_archive linux arm64
build_archive windows amd64
build_archive windows arm64

checksums_path="$OUT_DIR/checksums.txt"
manifest_path="$OUT_DIR/release-manifest.json"

{
  for artifact in "${artifacts[@]}"; do
    IFS='|' read -r goos goarch file_name sha256 <<<"$artifact"
    printf '%s  %s\n' "$sha256" "$file_name"
  done
} >"$checksums_path"

{
  echo "{"
  echo "  \"version\": \"${VERSION}\","
  echo "  \"git_sha\": \"${GIT_SHA}\","
  echo "  \"build_date\": \"${BUILD_DATE}\","
  echo "  \"artifacts\": ["

  separator=""
  for artifact in "${artifacts[@]}"; do
    IFS='|' read -r goos goarch file_name sha256 <<<"$artifact"
    printf '%s    {\n' "$separator"
    printf '      "os": "%s",\n' "$goos"
    printf '      "arch": "%s",\n' "$goarch"
    printf '      "file": "%s",\n' "$file_name"
    printf '      "sha256": "%s"\n' "$sha256"
    printf '    }'
    separator=",\n"
  done

  echo
  echo "  ]"
  echo "}"
} >"$manifest_path"

echo "wrote $(basename "$checksums_path")"
echo "wrote $(basename "$manifest_path")"
