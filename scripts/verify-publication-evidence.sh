#!/usr/bin/env bash

set -euo pipefail

MANIFEST_PATH=""
PLAN_PATH=""
EVIDENCE_DIR=""
COMPARE_DIR=""
OUT_PATH=""

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/verify-publication-evidence.sh \
  --manifest PATH \
  --plan PATH \
  --evidence-dir PATH \
  [--compare-dir PATH] \
  [--out PATH]
EOF
}

while (($# > 0)); do
  case "$1" in
    --manifest)
      MANIFEST_PATH="${2:-}"
      shift 2
      ;;
    --plan)
      PLAN_PATH="${2:-}"
      shift 2
      ;;
    --evidence-dir)
      EVIDENCE_DIR="${2:-}"
      shift 2
      ;;
    --compare-dir)
      COMPARE_DIR="${2:-}"
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

if [[ -z "$MANIFEST_PATH" || -z "$PLAN_PATH" || -z "$EVIDENCE_DIR" ]]; then
  usage
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to verify publication evidence" >&2
  exit 1
fi

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "publication evidence manifest not found: $MANIFEST_PATH" >&2
  exit 1
fi

if [[ ! -f "$PLAN_PATH" ]]; then
  echo "publication plan not found: $PLAN_PATH" >&2
  exit 1
fi

if [[ ! -d "$EVIDENCE_DIR" ]]; then
  echo "publication evidence directory not found: $EVIDENCE_DIR" >&2
  exit 1
fi

if [[ -n "$COMPARE_DIR" && ! -d "$COMPARE_DIR" ]]; then
  echo "compare directory not found: $COMPARE_DIR" >&2
  exit 1
fi

repository="$(jq -r '.repository // "MoveBigRocks/extensions"' "$MANIFEST_PATH")"
expected_count="$(jq -r '.bundles | length' "$MANIFEST_PATH")"
actual_count="$(find "$EVIDENCE_DIR" -maxdepth 1 -type f -name '*.publication-evidence.json' | wc -l | tr -d ' ')"

if [[ "$actual_count" != "$expected_count" ]]; then
  echo "publication evidence count mismatch: expected ${expected_count}, found ${actual_count} in ${EVIDENCE_DIR}" >&2
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

while IFS= read -r bundle; do
  slug="$(jq -r '.slug' <<<"$bundle")"
  manifest_version="$(jq -r '.version' <<<"$bundle")"
  publish_tag="$(jq -r '.publishTag' <<<"$bundle")"
  run_id="$(jq -r '.runID' <<<"$bundle")"

  evidence_path="$EVIDENCE_DIR/${slug}.publication-evidence.json"
  if [[ ! -f "$evidence_path" ]]; then
    echo "publication evidence file missing for ${slug}: ${evidence_path}" >&2
    exit 1
  fi

  plan_bundle="$(jq -c --arg slug "$slug" '.bundles[] | select(.slug == $slug)' "$PLAN_PATH")"
  if [[ -z "$plan_bundle" ]]; then
    echo "bundle ${slug} not found in publication plan ${PLAN_PATH}" >&2
    exit 1
  fi

  plan_version="$(jq -r '.version' <<<"$plan_bundle")"
  if [[ "${manifest_version#v}" != "$plan_version" ]]; then
    echo "manifest version mismatch for ${slug}: manifest=${manifest_version} plan=${plan_version}" >&2
    exit 1
  fi

  release_tag="$(jq -r '.releaseTag' <<<"$plan_bundle")"
  bundle_ref="$(jq -r '.bundleRef' <<<"$plan_bundle")"
  runtime_ref="$(jq -r '.runtimeRef // ""' <<<"$plan_bundle")"
  release_channel="$(jq -r '.releaseChannel' <<<"$plan_bundle")"
  runtime_class="$(jq -r '.runtimeClass // ""' <<<"$plan_bundle")"
  runtime_protocol="$(jq -r '.runtimeProtocol // ""' <<<"$plan_bundle")"
  storage_class="$(jq -r '.storageClass // ""' <<<"$plan_bundle")"
  schema_name="$(jq -r '.schemaName // ""' <<<"$plan_bundle")"
  schema_target_version="$(jq -r '.schemaTargetVersion // ""' <<<"$plan_bundle")"
  scope="$(jq -r '.scope // ""' <<<"$plan_bundle")"
  kind="$(jq -r '.kind // ""' <<<"$plan_bundle")"
  risk="$(jq -r '.risk // ""' <<<"$plan_bundle")"

  jq -e \
    --arg slug "$slug" \
    --arg version "$plan_version" \
    --arg publishTag "$publish_tag" \
    --arg repository "$repository" \
    --arg runID "$run_id" \
    --arg releaseTag "$release_tag" \
    --arg bundleRef "$bundle_ref" \
    --arg runtimeRef "$runtime_ref" \
    --arg releaseChannel "$release_channel" \
    --arg runtimeClass "$runtime_class" \
    --arg runtimeProtocol "$runtime_protocol" \
    --arg storageClass "$storage_class" \
    --arg schemaName "$schema_name" \
    --arg schemaTargetVersion "$schema_target_version" \
    --arg scope "$scope" \
    --arg kind "$kind" \
    --arg risk "$risk" \
    '
      .extension.slug == $slug and
      .extension.version == $version and
      .extension.releaseTag == $releaseTag and
      .extension.releaseChannel == $releaseChannel and
      .extension.kind == $kind and
      .extension.scope == $scope and
      .extension.risk == $risk and
      .extension.runtimeClass == $runtimeClass and
      .extension.runtimeProtocol == $runtimeProtocol and
      .extension.storageClass == $storageClass and
      .extension.schemaName == $schemaName and
      .extension.schemaTargetVersion == $schemaTargetVersion and
      .extension.bundleRef == $bundleRef and
      ((.extension.runtimeRef // "") == $runtimeRef) and
      .publication.repository == $repository and
      .publication.publishTag == $publishTag and
      .publication.bundleRef == $bundleRef and
      ((.publication.runtimeRef // "") == $runtimeRef) and
      (.publication.workflowRunUrl | contains("/runs/" + $runID)) and
      (.publication.bundleDigest | type == "string" and length > 0) and
      ((.publication.runtimeDigest // "") | type == "string")
    ' "$evidence_path" >/dev/null

  archived_match="not_compared"
  if [[ -n "$COMPARE_DIR" ]]; then
    compare_path="$COMPARE_DIR/${slug}.publication-evidence.json"
    if [[ ! -f "$compare_path" ]]; then
      echo "compare evidence file missing for ${slug}: ${compare_path}" >&2
      exit 1
    fi
    if ! cmp -s "$evidence_path" "$compare_path"; then
      echo "publication evidence mismatch between ${evidence_path} and ${compare_path}" >&2
      diff -u "$compare_path" "$evidence_path" >&2 || true
      exit 1
    fi
    archived_match="exact"
  fi

  row_json="$(jq -n \
    --arg slug "$slug" \
    --arg version "$plan_version" \
    --arg publishTag "$publish_tag" \
    --arg runID "$run_id" \
    --arg releaseTag "$release_tag" \
    --arg bundleRef "$bundle_ref" \
    --arg runtimeRef "$runtime_ref" \
    --arg evidenceFile "$(basename "$evidence_path")" \
    --arg archivedMatch "$archived_match" \
    '{
      slug: $slug,
      version: $version,
      publishTag: $publishTag,
      runID: $runID,
      releaseTag: $releaseTag,
      evidenceFile: $evidenceFile,
      bundleRef: $bundleRef,
      runtimeRef: $runtimeRef,
      archivedMatch: $archivedMatch
    }')"
  append_result "$row_json"
done < <(jq -c '.bundles[]' "$MANIFEST_PATH")

if [[ -n "$OUT_PATH" ]]; then
  mkdir -p "$(dirname "$OUT_PATH")"
  jq -n \
    --arg verifiedAt "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
    --arg repository "$repository" \
    --arg manifestPath "$MANIFEST_PATH" \
    --arg planPath "$PLAN_PATH" \
    --arg evidenceDir "$EVIDENCE_DIR" \
    --arg compareDir "$COMPARE_DIR" \
    --slurpfile bundles "$tmp_results" \
    '{
      verifiedAt: $verifiedAt,
      repository: $repository,
      manifestPath: $manifestPath,
      planPath: $planPath,
      evidenceDir: $evidenceDir,
      compareDir: (if $compareDir == "" then null else $compareDir end),
      bundleCount: ($bundles[0] | length),
      bundles: $bundles[0]
    }' >"$OUT_PATH"
fi

echo "Verified ${expected_count} publication evidence file(s) in ${EVIDENCE_DIR}"
