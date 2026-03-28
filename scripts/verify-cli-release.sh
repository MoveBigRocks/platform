#!/usr/bin/env bash

set -euo pipefail

export LC_ALL=C
export LANG=C

RELEASE_DIR=""
EXPECTED_VERSION=""
EXPECTED_GIT_SHA=""

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/verify-cli-release.sh RELEASE_DIR [--version VERSION] [--git-sha GIT_SHA]
EOF
}

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

verify_checksums_file() {
  local release_dir="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    (
      cd "$release_dir"
      sha256sum -c checksums.txt
    )
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    (
      cd "$release_dir"
      shasum -a 256 -c checksums.txt
    )
    return
  fi
  echo "sha256sum or shasum is required" >&2
  exit 1
}

if (($# == 0)); then
  usage
  exit 2
fi

RELEASE_DIR="$1"
shift

while (($# > 0)); do
  case "$1" in
    --version)
      EXPECTED_VERSION="${2:-}"
      shift 2
      ;;
    --git-sha)
      EXPECTED_GIT_SHA="${2:-}"
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

if [[ ! -d "$RELEASE_DIR" ]]; then
  echo "release directory not found: $RELEASE_DIR" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to validate release-manifest.json" >&2
  exit 1
fi

MANIFEST_PATH="$RELEASE_DIR/release-manifest.json"
CHECKSUMS_PATH="$RELEASE_DIR/checksums.txt"
VERIFICATION_PATH="$RELEASE_DIR/verification.json"

for required_path in "$MANIFEST_PATH" "$CHECKSUMS_PATH"; do
  if [[ ! -f "$required_path" ]]; then
    echo "required release artifact missing: $required_path" >&2
    exit 1
  fi
done

jq -e '.artifacts | type == "array" and length > 0' "$MANIFEST_PATH" >/dev/null

if [[ -n "$EXPECTED_VERSION" ]]; then
  jq -e --arg version "$EXPECTED_VERSION" '.version == $version' "$MANIFEST_PATH" >/dev/null
fi

if [[ -n "$EXPECTED_GIT_SHA" ]]; then
  jq -e --arg git_sha "$EXPECTED_GIT_SHA" '.git_sha == $git_sha' "$MANIFEST_PATH" >/dev/null
fi

artifact_count="$(jq -r '.artifacts | length' "$MANIFEST_PATH")"
checksums_count="$(wc -l <"$CHECKSUMS_PATH" | tr -d ' ')"
if [[ "$artifact_count" != "$checksums_count" ]]; then
  echo "artifact count mismatch: manifest=$artifact_count checksums=$checksums_count" >&2
  exit 1
fi

while IFS=$'\t' read -r file_name sha256; do
  artifact_path="$RELEASE_DIR/$file_name"
  if [[ ! -f "$artifact_path" ]]; then
    echo "release artifact listed in manifest is missing: $artifact_path" >&2
    exit 1
  fi

  expected_checksum="$(awk -v target="$file_name" '$2 == target {print $1}' "$CHECKSUMS_PATH")"
  if [[ -z "$expected_checksum" ]]; then
    echo "release artifact missing from checksums.txt: $file_name" >&2
    exit 1
  fi
  if [[ "$expected_checksum" != "$sha256" ]]; then
    echo "checksum mismatch between manifest and checksums.txt for $file_name" >&2
    exit 1
  fi

  actual_checksum="$(sha256_file "$artifact_path")"
  if [[ "$actual_checksum" != "$sha256" ]]; then
    echo "checksum mismatch for built artifact $file_name" >&2
    exit 1
  fi
done < <(jq -r '.artifacts[] | [.file, .sha256] | @tsv' "$MANIFEST_PATH")

verify_checksums_file "$RELEASE_DIR"

verified_at="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
manifest_version="$(jq -r '.version' "$MANIFEST_PATH")"
manifest_git_sha="$(jq -r '.git_sha' "$MANIFEST_PATH")"
jq -n \
  --arg verified_at "$verified_at" \
  --arg version "$manifest_version" \
  --arg git_sha "$manifest_git_sha" \
  --arg manifest_path "$(basename "$MANIFEST_PATH")" \
  --arg checksums_path "$(basename "$CHECKSUMS_PATH")" \
  --argjson artifact_count "$artifact_count" \
  '{
    verified_at: $verified_at,
    version: $version,
    git_sha: $git_sha,
    manifest_path: $manifest_path,
    checksums_path: $checksums_path,
    artifact_count: $artifact_count,
    manifest_valid: true,
    checksums_valid: true
  }' >"$VERIFICATION_PATH"

echo "wrote $(basename "$VERIFICATION_PATH")"
