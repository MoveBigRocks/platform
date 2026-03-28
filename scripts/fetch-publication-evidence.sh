#!/usr/bin/env bash

set -euo pipefail

MANIFEST_PATH=""
OUT_DIR=""

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/fetch-publication-evidence.sh --manifest PATH --out PATH
EOF
}

while (($# > 0)); do
  case "$1" in
    --manifest)
      MANIFEST_PATH="${2:-}"
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

if [[ -z "$MANIFEST_PATH" || -z "$OUT_DIR" ]]; then
  usage
  exit 2
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh CLI is required to download publication evidence" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to parse the publication evidence manifest" >&2
  exit 1
fi

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "publication evidence manifest not found: $MANIFEST_PATH" >&2
  exit 1
fi

REPOSITORY="$(jq -r '.repository // "MoveBigRocks/extensions"' "$MANIFEST_PATH")"
mkdir -p "$OUT_DIR"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

while IFS= read -r bundle; do
  slug="$(jq -r '.slug' <<<"$bundle")"
  run_id="$(jq -r '.runID' <<<"$bundle")"
  artifact_name="$(jq -r '.artifactName // empty' <<<"$bundle")"
  publish_tag="$(jq -r '.publishTag' <<<"$bundle")"

  if [[ -z "$artifact_name" ]]; then
    artifact_name="public-bundle-${slug}-${publish_tag}"
  fi

  bundle_dir="$TMP_DIR/$slug"
  mkdir -p "$bundle_dir"

  echo "Downloading ${artifact_name} from ${REPOSITORY} run ${run_id}..."
  gh run download "$run_id" --repo "$REPOSITORY" --name "$artifact_name" --dir "$bundle_dir" >/dev/null

  evidence_path="$(find "$bundle_dir" -type f -name '*.publication-evidence.json' -print -quit)"
  if [[ -z "$evidence_path" ]]; then
    echo "publication evidence file not found in artifact ${artifact_name} from run ${run_id}" >&2
    exit 1
  fi

  cp "$evidence_path" "$OUT_DIR/"
done < <(jq -c '.bundles[]' "$MANIFEST_PATH")

count="$(find "$OUT_DIR" -maxdepth 1 -type f -name '*.publication-evidence.json' | wc -l | tr -d ' ')"
if [[ "$count" -eq 0 ]]; then
  echo "no publication evidence files were downloaded" >&2
  exit 1
fi

echo "Downloaded ${count} publication evidence file(s) into ${OUT_DIR}"
