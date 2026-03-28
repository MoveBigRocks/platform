#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/milestone-proof"
VERSION=""
FIRST_PARTY_EXTENSIONS_ROOT="${FIRST_PARTY_EXTENSIONS_ROOT:-$(cd "$ROOT_DIR/.." && pwd)/extensions}"

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
PROOF_BIN_DIR="$OUT_DIR/bin"
LOCAL_MBR_BIN="$PROOF_BIN_DIR/mbr"
EXTENSIONS_VALIDATION_DIR="$OUT_DIR/extensions-validation"
EXTENSIONS_VALIDATION_LOG="$EXTENSIONS_VALIDATION_DIR/validate-first-party.log"
FIRST_PARTY_VALIDATION_SCRIPT="$FIRST_PARTY_EXTENSIONS_ROOT/scripts/validate-first-party.sh"
FIRST_PARTY_CATALOG_PATH="$FIRST_PARTY_EXTENSIONS_ROOT/catalog/public-bundles.json"
ARCHIVED_PUBLICATION_EVIDENCE_DIR="$ROOT_DIR/docs/evidence/public-bundle-publication"
BOOTSTRAP_PROOF_DIR="$OUT_DIR/runtime-bootstrap"
BOOTSTRAP_PROOF_PATH="$BOOTSTRAP_PROOF_DIR/mbr-instance.json"
ATS_SCENARIO_DIR="$OUT_DIR/ats-scenario"
ATS_SCENARIO_PATH="$ATS_SCENARIO_DIR/ats-scenario.json"
INTEGRATION_LOG_PATH="$OUT_DIR/integration-go-test.log"
PUBLIC_BUNDLE_PUBLICATION_DIR="$OUT_DIR/public-bundle-publication"
PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH="$PUBLIC_BUNDLE_PUBLICATION_DIR/publication-plan.json"
PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR="$PUBLIC_BUNDLE_PUBLICATION_DIR/release-evidence"
FETCHED_PUBLICATION_EVIDENCE_DIR="$PUBLIC_BUNDLE_PUBLICATION_DIR/fetched-evidence"
PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH="$PUBLIC_BUNDLE_PUBLICATION_DIR/evidence-verification.json"
WORKFLOW_PROOF_DIR="$OUT_DIR/workflow-proof"
CASE_REPLY_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-reply-send.json"
EMAIL_COMMAND_FAILURE_PROOF_PATH="$WORKFLOW_PROOF_DIR/email-command-failure-visible.json"
INBOUND_NEW_EMAIL_PROOF_PATH="$WORKFLOW_PROOF_DIR/inbound-new-email-case-create.json"
INBOUND_REPLY_THREADING_PROOF_PATH="$WORKFLOW_PROOF_DIR/inbound-reply-threading.json"
FORM_NOTIFICATION_PROOF_PATH="$WORKFLOW_PROOF_DIR/public-form-case-notification.json"
RULE_EMAIL_PROOF_PATH="$WORKFLOW_PROOF_DIR/rule-send-email.json"
KNOWLEDGE_NOTIFICATION_PROOF_PATH="$WORKFLOW_PROOF_DIR/knowledge-review-notification.json"
NOTIFICATION_COMMAND_FAILURE_PROOF_PATH="$WORKFLOW_PROOF_DIR/notification-command-failure-visible.json"
SUMMARY_PATH="$OUT_DIR/summary.md"
GENERATED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
GIT_SHA="$(git -C "$ROOT_DIR" rev-parse HEAD)"
FIRST_PARTY_PUBLICATION_EVIDENCE_DIR="${FIRST_PARTY_PUBLICATION_EVIDENCE_DIR:-}"
FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST="${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST:-}"
REQUIRE_PUBLICATION_EVIDENCE="${REQUIRE_PUBLICATION_EVIDENCE:-false}"

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

run_step_capture() {
  local output_path="$1"
  shift
  echo
  echo "==> $*"
  "$@" 2>&1 | tee "$output_path"
}

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "required file not found: $path" >&2
    exit 1
  fi
}

cd "$ROOT_DIR"

mkdir -p "$PROOF_BIN_DIR" "$EXTENSIONS_VALIDATION_DIR" "$BOOTSTRAP_PROOF_DIR" "$ATS_SCENARIO_DIR" "$PUBLIC_BUNDLE_PUBLICATION_DIR" "$WORKFLOW_PROOF_DIR"
require_file "$FIRST_PARTY_VALIDATION_SCRIPT"
require_file "$FIRST_PARTY_CATALOG_PATH"
require_file "$ROOT_DIR/scripts/verify-publication-evidence.sh"

run_step go test -count=1 ./internal/service/services ./internal/knowledge/services ./internal/platform/services ./cmd/api ./cmd/mbr
run_step bash scripts/check-cli-contract-docs.sh
run_step go build -trimpath -o "$LOCAL_MBR_BIN" ./cmd/mbr
run_step env WORKFLOW_PROOF_DIR="$WORKFLOW_PROOF_DIR" go test -count=1 ./internal/knowledge/services
run_step_capture "$INTEGRATION_LOG_PATH" env WORKFLOW_PROOF_DIR="$WORKFLOW_PROOF_DIR" go test -tags=integration -count=1 ./...
run_step bash -lc "cd \"$FIRST_PARTY_EXTENSIONS_ROOT\" && go test ./ats/runtime ./cmd/ats-runtime ./tools/ats-scenario-proof -count=1"
run_step bash -lc "cd \"$FIRST_PARTY_EXTENSIONS_ROOT\" && go run ./tools/publication-evidence --mode plan --source-root \"$FIRST_PARTY_EXTENSIONS_ROOT\" --out \"$PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH\""
run_step go run ./tools/runtime-bootstrap-proof --out "$BOOTSTRAP_PROOF_PATH" --version "$VERSION" --git-sha "$GIT_SHA" --build-date "$GENERATED_AT"
run_step bash -lc "cd \"$FIRST_PARTY_EXTENSIONS_ROOT\" && go run ./tools/ats-scenario-proof --out \"$ATS_SCENARIO_PATH\" --version \"$VERSION\" --git-sha \"$GIT_SHA\" --build-date \"$GENERATED_AT\""
require_file "$CASE_REPLY_PROOF_PATH"
require_file "$EMAIL_COMMAND_FAILURE_PROOF_PATH"
require_file "$INBOUND_NEW_EMAIL_PROOF_PATH"
require_file "$INBOUND_REPLY_THREADING_PROOF_PATH"
require_file "$FORM_NOTIFICATION_PROOF_PATH"
require_file "$RULE_EMAIL_PROOF_PATH"
require_file "$KNOWLEDGE_NOTIFICATION_PROOF_PATH"
require_file "$NOTIFICATION_COMMAND_FAILURE_PROOF_PATH"
if [[ -z "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" && -n "$FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST" ]]; then
  run_step bash scripts/fetch-publication-evidence.sh --manifest "$FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST" --out "$FETCHED_PUBLICATION_EVIDENCE_DIR"
  FIRST_PARTY_PUBLICATION_EVIDENCE_DIR="$FETCHED_PUBLICATION_EVIDENCE_DIR"
fi
if [[ -z "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" && -d "$ARCHIVED_PUBLICATION_EVIDENCE_DIR" ]]; then
  FIRST_PARTY_PUBLICATION_EVIDENCE_DIR="$ARCHIVED_PUBLICATION_EVIDENCE_DIR"
fi
if [[ -n "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" ]]; then
  mkdir -p "$PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR"
  shopt -s nullglob
  evidence_files=("$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR"/*.publication-evidence.json)
  shopt -u nullglob
  if [[ "${#evidence_files[@]}" -eq 0 ]]; then
    echo "no publication evidence files found in ${FIRST_PARTY_PUBLICATION_EVIDENCE_DIR}" >&2
    exit 1
  fi
  cp "${evidence_files[@]}" "$PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR/"
  compare_args=()
  if [[ "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" == "$FETCHED_PUBLICATION_EVIDENCE_DIR" && -d "$ARCHIVED_PUBLICATION_EVIDENCE_DIR" ]]; then
    compare_args=(--compare-dir "$ARCHIVED_PUBLICATION_EVIDENCE_DIR")
  fi
  run_step bash scripts/verify-publication-evidence.sh \
    --manifest "${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST:-$ROOT_DIR/docs/evidence/public-bundle-publication-runs.json}" \
    --plan "$PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH" \
    --evidence-dir "$PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR" \
    "${compare_args[@]}" \
    --out "$PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH"
  require_file "$PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH"
elif [[ "$REQUIRE_PUBLICATION_EVIDENCE" == "true" ]]; then
  echo "publication evidence is required but no evidence directory or manifest was provided" >&2
  exit 1
fi
cp "$FIRST_PARTY_CATALOG_PATH" "$EXTENSIONS_VALIDATION_DIR/public-bundles.json"
run_step_capture "$EXTENSIONS_VALIDATION_LOG" env MBR_BIN="$LOCAL_MBR_BIN" bash "$FIRST_PARTY_VALIDATION_SCRIPT"
run_step bash scripts/build-cli-release.sh --version "$VERSION" --out "$CLI_OUT_DIR"
run_step bash scripts/verify-cli-release.sh "$CLI_OUT_DIR" --version "$VERSION" --git-sha "$GIT_SHA"

cat >"$SUMMARY_PATH" <<EOF
# Milestone 1 Proof Summary

- generated_at: ${GENERATED_AT}
- git_sha: ${GIT_SHA}
- cli_release_dir: ${CLI_OUT_DIR}
- cli_release_verification: ${CLI_OUT_DIR}/verification.json
- local_mbr_bin: ${LOCAL_MBR_BIN}
- integration_log: ${INTEGRATION_LOG_PATH}
- runtime_bootstrap_artifact: ${BOOTSTRAP_PROOF_PATH}
- ats_scenario_artifact: ${ATS_SCENARIO_PATH}
- workflow_proof_dir: ${WORKFLOW_PROOF_DIR}
- workflow_case_reply_artifact: ${CASE_REPLY_PROOF_PATH}
- workflow_email_command_failure_artifact: ${EMAIL_COMMAND_FAILURE_PROOF_PATH}
- workflow_inbound_case_create_artifact: ${INBOUND_NEW_EMAIL_PROOF_PATH}
- workflow_inbound_reply_threading_artifact: ${INBOUND_REPLY_THREADING_PROOF_PATH}
- workflow_form_notification_artifact: ${FORM_NOTIFICATION_PROOF_PATH}
- workflow_rule_email_artifact: ${RULE_EMAIL_PROOF_PATH}
- workflow_knowledge_notification_artifact: ${KNOWLEDGE_NOTIFICATION_PROOF_PATH}
- workflow_notification_command_failure_artifact: ${NOTIFICATION_COMMAND_FAILURE_PROOF_PATH}
- extensions_validation_dir: ${EXTENSIONS_VALIDATION_DIR}
- public_bundle_publication_plan: ${PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH}
- public_bundle_release_evidence_dir: ${PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR}
- public_bundle_evidence_verification: ${PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH}

## Commands Run

1. \`go test -count=1 ./internal/service/services ./internal/knowledge/services ./internal/platform/services ./cmd/api ./cmd/mbr\`
2. \`bash scripts/check-cli-contract-docs.sh\`
3. \`go build -trimpath -o ${LOCAL_MBR_BIN} ./cmd/mbr\`
4. \`env WORKFLOW_PROOF_DIR=${WORKFLOW_PROOF_DIR} go test -count=1 ./internal/knowledge/services\`
5. \`env WORKFLOW_PROOF_DIR=${WORKFLOW_PROOF_DIR} go test -tags=integration -count=1 ./...\`
6. \`(cd ${FIRST_PARTY_EXTENSIONS_ROOT} && go test ./ats/runtime ./cmd/ats-runtime ./tools/ats-scenario-proof -count=1)\`
7. \`(cd ${FIRST_PARTY_EXTENSIONS_ROOT} && go run ./tools/publication-evidence --mode plan --source-root ${FIRST_PARTY_EXTENSIONS_ROOT} --out ${PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH})\`
8. \`go run ./tools/runtime-bootstrap-proof --out ${BOOTSTRAP_PROOF_PATH} --version ${VERSION} --git-sha ${GIT_SHA} --build-date ${GENERATED_AT}\`
9. \`(cd ${FIRST_PARTY_EXTENSIONS_ROOT} && go run ./tools/ats-scenario-proof --out ${ATS_SCENARIO_PATH} --version ${VERSION} --git-sha ${GIT_SHA} --build-date ${GENERATED_AT})\`
10. \`bash scripts/fetch-publication-evidence.sh --manifest ${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST} --out ${FETCHED_PUBLICATION_EVIDENCE_DIR}\` when a manifest is supplied
11. \`bash scripts/verify-publication-evidence.sh --manifest ${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST:-docs/evidence/public-bundle-publication-runs.json} --plan ${PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH} --evidence-dir ${PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR}\`
12. \`MBR_BIN=${LOCAL_MBR_BIN} bash ${FIRST_PARTY_VALIDATION_SCRIPT}\`
13. \`bash scripts/build-cli-release.sh --version ${VERSION} --out ${CLI_OUT_DIR}\`
14. \`bash scripts/verify-cli-release.sh ${CLI_OUT_DIR} --version ${VERSION} --git-sha ${GIT_SHA}\`

## Evidence Docs

- [docs/MILESTONE_1_READINESS.md](../../docs/MILESTONE_1_READINESS.md)
- [docs/MILESTONE_1_PROOF.md](../../docs/MILESTONE_1_PROOF.md)
- [docs/FIRST_PARTY_PACK_READINESS.md](../../docs/FIRST_PARTY_PACK_READINESS.md)
- [docs/CLI_RELEASES.md](../../docs/CLI_RELEASES.md)
EOF

echo
echo "Proof summary: $SUMMARY_PATH"
