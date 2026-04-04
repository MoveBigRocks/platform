#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/milestone-proof"
VERSION=""
WORKSPACE_ROOT="${MBR_WORKSPACE_ROOT:-${MOVEBIGROCKS_WORKSPACE_ROOT:-$(cd "$ROOT_DIR/.." && pwd)}}"
FIRST_PARTY_EXTENSIONS_ROOT="${FIRST_PARTY_EXTENSIONS_ROOT:-$WORKSPACE_ROOT/extensions}"
PRIVATE_EXTENSIONS_ROOT="${PRIVATE_EXTENSIONS_ROOT:-$WORKSPACE_ROOT/private-extensions}"
EXTENSION_SDK_ROOT="${EXTENSION_SDK_ROOT:-$WORKSPACE_ROOT/extension-sdk}"

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
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

CLI_OUT_DIR="$OUT_DIR/cli-release"
PROOF_BIN_DIR="$OUT_DIR/bin"
LOCAL_MBR_BIN="$PROOF_BIN_DIR/mbr"
EXTENSIONS_VALIDATION_DIR="$OUT_DIR/extensions-validation"
EXTENSIONS_VALIDATION_LOG="$EXTENSIONS_VALIDATION_DIR/validate-first-party.log"
FIRST_PARTY_VALIDATION_SCRIPT="$FIRST_PARTY_EXTENSIONS_ROOT/scripts/validate-first-party.sh"
FIRST_PARTY_CATALOG_PATH="$FIRST_PARTY_EXTENSIONS_ROOT/catalog/public-bundles.json"
ARCHIVED_PUBLICATION_EVIDENCE_DIR="$ROOT_DIR/docs/evidence/public-bundle-publication"
CANONICAL_WORKSPACE_REFS_MANIFEST="${CANONICAL_WORKSPACE_REFS_MANIFEST:-$ROOT_DIR/docs/evidence/canonical-workspace-refs.json}"
BOOTSTRAP_PROOF_DIR="$OUT_DIR/runtime-bootstrap"
BOOTSTRAP_PROOF_PATH="$BOOTSTRAP_PROOF_DIR/mbr-instance.json"
ATS_SCENARIO_DIR="$OUT_DIR/ats-scenario"
ATS_SCENARIO_PATH="$ATS_SCENARIO_DIR/ats-scenario.json"
INTEGRATION_LOG_PATH="$OUT_DIR/integration-go-test.log"
PUBLIC_BUNDLE_PUBLICATION_DIR="$OUT_DIR/public-bundle-publication"
PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH="$PUBLIC_BUNDLE_PUBLICATION_DIR/publication-plan.json"
PUBLIC_BUNDLE_PUBLICATION_MANIFEST_ARCHIVE_PATH="$PUBLIC_BUNDLE_PUBLICATION_DIR/publication-evidence-manifest.json"
PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR="$PUBLIC_BUNDLE_PUBLICATION_DIR/release-evidence"
FETCHED_PUBLICATION_EVIDENCE_DIR="$PUBLIC_BUNDLE_PUBLICATION_DIR/fetched-evidence"
PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH="$PUBLIC_BUNDLE_PUBLICATION_DIR/evidence-verification.json"
WORKFLOW_PROOF_DIR="$OUT_DIR/workflow-proof"
PROOF_GO_WORK_DIR="$OUT_DIR/go-work"
PROOF_GO_WORK_PATH="$PROOF_GO_WORK_DIR/go.work"
WORKSPACE_REFS_DIR="$OUT_DIR/workspace-refs"
WORKSPACE_REFS_MANIFEST_ARCHIVE_PATH="$WORKSPACE_REFS_DIR/canonical-workspace-refs.json"
WORKSPACE_REFS_VERIFICATION_PATH="$WORKSPACE_REFS_DIR/canonical-workspace-refs-verification.json"
CASE_REPLY_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-reply-send.json"
CASE_COMMAND_CREATE_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-command-create.json"
CASE_COMMAND_FAILURE_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-command-failure-visible.json"
CASE_OPERATOR_MANUAL_CREATE_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-operator-manual-create.json"
CASE_OPERATOR_WORK_MANAGEMENT_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-operator-work-management.json"
CASE_OPERATOR_REPLY_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-operator-reply.json"
CASE_OPERATOR_HANDOFF_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-operator-handoff.json"
CASE_OPERATOR_STATUS_TRANSITION_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-operator-status-transition.json"
CASE_OPERATOR_ATTACHMENT_UPLOAD_PROOF_PATH="$WORKFLOW_PROOF_DIR/case-operator-attachment-upload.json"
CONVERSATION_OPERATOR_REPLY_PROOF_PATH="$WORKFLOW_PROOF_DIR/conversation-operator-reply.json"
CONVERSATION_OPERATOR_HANDOFF_PROOF_PATH="$WORKFLOW_PROOF_DIR/conversation-operator-handoff.json"
CONVERSATION_OPERATOR_ESCALATION_PROOF_PATH="$WORKFLOW_PROOF_DIR/conversation-operator-escalation.json"
PUBLIC_CONVERSATION_INTAKE_PROOF_PATH="$WORKFLOW_PROOF_DIR/public-conversation-intake.json"
EMAIL_COMMAND_FAILURE_PROOF_PATH="$WORKFLOW_PROOF_DIR/email-command-failure-visible.json"
INBOUND_NEW_EMAIL_PROOF_PATH="$WORKFLOW_PROOF_DIR/inbound-new-email-case-create.json"
INBOUND_EMAIL_ATTACHMENTS_PROOF_PATH="$WORKFLOW_PROOF_DIR/inbound-email-attachments.json"
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
FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST_DISPLAY="${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST:-<not supplied>}"
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
: "${MBR_REQUIRE_WORKSPACE_REFS:=true}"

if [[ -n "${DATABASE_DSN:-}" ]]; then
  export DATABASE_DSN
  : "${TEST_DATABASE_ADMIN_DSN:=${DATABASE_DSN}}"
  export TEST_DATABASE_ADMIN_DSN
fi

export STORAGE_TYPE STORAGE_PATH FILESYSTEM_PATH
export CACHE_ENABLED EMAIL_BACKEND JWT_SECRET ENVIRONMENT TRACING_ENABLED ENABLE_METRICS CLAMAV_ADDR
export MBR_WORKSPACE_ROOT="$WORKSPACE_ROOT"
export MBR_REQUIRE_WORKSPACE_REFS

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

artifact_rel() {
  local path="$1"
  if [[ "$path" == "$OUT_DIR" ]]; then
    echo "."
    return
  fi
  if [[ "$path" == "$OUT_DIR/"* ]]; then
    echo "${path#$OUT_DIR/}"
    return
  fi
  echo "$path"
}

workspace_includes_module_dir() {
  local gowork_path="$1"
  local module_dir="$2"
  local gowork_dir resolved_module resolved_use_path disk_path

  if [[ ! -f "$gowork_path" ]]; then
    return 1
  fi

  resolved_module="$(cd "$module_dir" && pwd)"
  gowork_dir="$(cd "$(dirname "$gowork_path")" && pwd)"
  while IFS= read -r disk_path; do
    [[ -z "$disk_path" ]] && continue
    if [[ "$disk_path" = /* ]]; then
      resolved_use_path="$(cd "$disk_path" && pwd)"
    else
      resolved_use_path="$(cd "$gowork_dir/$disk_path" && pwd)"
    fi
    if [[ "$resolved_use_path" == "$resolved_module" ]]; then
      return 0
    fi
  done < <(go work edit -json "$gowork_path" | jq -r '.Use[]?.DiskPath')

  return 1
}

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "required file not found: $path" >&2
    exit 1
  fi
}

require_dir() {
  local path="$1"
  if [[ ! -d "$path" ]]; then
    echo "required directory not found: $path" >&2
    exit 1
  fi
}

ensure_go_workspace() {
  if [[ "${GOWORK:-}" == "off" ]]; then
    return
  fi
  if [[ -n "${GOWORK:-}" && -f "$GOWORK" ]]; then
    if workspace_includes_module_dir "$GOWORK" "$ROOT_DIR"; then
      return
    fi
  fi
  if [[ -f "$WORKSPACE_ROOT/go.work" ]]; then
    if workspace_includes_module_dir "$WORKSPACE_ROOT/go.work" "$ROOT_DIR"; then
      export GOWORK="$WORKSPACE_ROOT/go.work"
      return
    fi
  fi

  local modules=("$ROOT_DIR")
  if [[ -f "$FIRST_PARTY_EXTENSIONS_ROOT/go.mod" ]]; then
    modules+=("$FIRST_PARTY_EXTENSIONS_ROOT")
  fi
  if [[ -f "$PRIVATE_EXTENSIONS_ROOT/go.mod" ]]; then
    modules+=("$PRIVATE_EXTENSIONS_ROOT")
  fi
  if [[ -f "$EXTENSION_SDK_ROOT/go.mod" ]]; then
    modules+=("$EXTENSION_SDK_ROOT")
  fi

  mkdir -p "$PROOF_GO_WORK_DIR"
  rm -f "$PROOF_GO_WORK_PATH" "$PROOF_GO_WORK_DIR/go.work.sum"
  go -C "$PROOF_GO_WORK_DIR" work init "${modules[@]}"
  export GOWORK="$PROOF_GO_WORK_PATH"
}

cd "$ROOT_DIR"

mkdir -p "$PROOF_BIN_DIR" "$EXTENSIONS_VALIDATION_DIR" "$BOOTSTRAP_PROOF_DIR" "$ATS_SCENARIO_DIR" "$PUBLIC_BUNDLE_PUBLICATION_DIR" "$WORKFLOW_PROOF_DIR"
require_dir "$WORKSPACE_ROOT"
require_dir "$FIRST_PARTY_EXTENSIONS_ROOT"
require_dir "$EXTENSION_SDK_ROOT"
require_file "$FIRST_PARTY_VALIDATION_SCRIPT"
require_file "$FIRST_PARTY_CATALOG_PATH"
require_file "$ROOT_DIR/scripts/verify-publication-evidence.sh"
require_file "$ROOT_DIR/scripts/verify-canonical-workspace-refs.sh"
require_file "$CANONICAL_WORKSPACE_REFS_MANIFEST"
ensure_go_workspace
mkdir -p "$WORKSPACE_REFS_DIR"
cp "$CANONICAL_WORKSPACE_REFS_MANIFEST" "$WORKSPACE_REFS_MANIFEST_ARCHIVE_PATH"
run_step bash scripts/verify-canonical-workspace-refs.sh \
  --manifest "$WORKSPACE_REFS_MANIFEST_ARCHIVE_PATH" \
  --workspace-root "$WORKSPACE_ROOT" \
  --out "$WORKSPACE_REFS_VERIFICATION_PATH"

run_step go test -count=1 ./internal/service/services ./internal/knowledge/services ./internal/platform/services ./cmd/api ./cmd/mbr
run_step bash scripts/check-cli-contract-docs.sh
run_step go build -trimpath -o "$LOCAL_MBR_BIN" ./cmd/mbr
run_step env WORKFLOW_PROOF_DIR="$WORKFLOW_PROOF_DIR" go test -count=1 ./internal/knowledge/services
run_step_capture "$INTEGRATION_LOG_PATH" env WORKFLOW_PROOF_DIR="$WORKFLOW_PROOF_DIR" go test -tags=integration -count=1 ./...
run_step bash -lc "cd \"$FIRST_PARTY_EXTENSIONS_ROOT\" && go test ./ats/runtime ./cmd/ats-runtime ./tools/ats-scenario-proof -count=1"
run_step bash -lc "cd \"$FIRST_PARTY_EXTENSIONS_ROOT\" && go run ./tools/publication-evidence --mode plan --source-root \"$FIRST_PARTY_EXTENSIONS_ROOT\" --out \"$PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH\""
run_step go run ./tools/runtime-bootstrap-proof --out "$BOOTSTRAP_PROOF_PATH" --version "$VERSION" --git-sha "$GIT_SHA" --build-date "$GENERATED_AT"
run_step bash -lc "cd \"$FIRST_PARTY_EXTENSIONS_ROOT\" && go run ./tools/ats-scenario-proof --out \"$ATS_SCENARIO_PATH\" --version \"$VERSION\" --git-sha \"$GIT_SHA\" --build-date \"$GENERATED_AT\""
require_file "$ATS_SCENARIO_PATH"
run_step bash -lc "jq -e '.attachment.id == .case.customFields.ats_applicant_resume_attachment_id and .attachment.caseId == .case.id and .attachment.visibleCount >= 1 and .attachment.status == \"clean\" and (.case.customFields.ats_applicant_portfolio_url | type == \"string\" and length > 0)' \"$ATS_SCENARIO_PATH\" >/dev/null"
require_file "$CASE_REPLY_PROOF_PATH"
require_file "$CASE_COMMAND_CREATE_PROOF_PATH"
require_file "$CASE_COMMAND_FAILURE_PROOF_PATH"
require_file "$CASE_OPERATOR_MANUAL_CREATE_PROOF_PATH"
require_file "$CASE_OPERATOR_WORK_MANAGEMENT_PROOF_PATH"
require_file "$CASE_OPERATOR_REPLY_PROOF_PATH"
require_file "$CASE_OPERATOR_HANDOFF_PROOF_PATH"
require_file "$CASE_OPERATOR_STATUS_TRANSITION_PROOF_PATH"
require_file "$CASE_OPERATOR_ATTACHMENT_UPLOAD_PROOF_PATH"
require_file "$CONVERSATION_OPERATOR_REPLY_PROOF_PATH"
require_file "$CONVERSATION_OPERATOR_HANDOFF_PROOF_PATH"
require_file "$CONVERSATION_OPERATOR_ESCALATION_PROOF_PATH"
require_file "$PUBLIC_CONVERSATION_INTAKE_PROOF_PATH"
require_file "$EMAIL_COMMAND_FAILURE_PROOF_PATH"
require_file "$INBOUND_NEW_EMAIL_PROOF_PATH"
require_file "$INBOUND_EMAIL_ATTACHMENTS_PROOF_PATH"
require_file "$INBOUND_REPLY_THREADING_PROOF_PATH"
require_file "$FORM_NOTIFICATION_PROOF_PATH"
require_file "$RULE_EMAIL_PROOF_PATH"
require_file "$KNOWLEDGE_NOTIFICATION_PROOF_PATH"
require_file "$NOTIFICATION_COMMAND_FAILURE_PROOF_PATH"
run_step bash -lc "jq -e '.participant_kind == \"user\" or .participant_kind == \"agent\"' \"$CONVERSATION_OPERATOR_REPLY_PROOF_PATH\" >/dev/null"
run_step bash -lc "jq -e '.role == \"assistant\" and .visibility == \"customer\" and (.reply_message_id | type == \"string\" and length > 0) and (.reply_participant_id | type == \"string\" and length > 0) and (.queue_id | type == \"string\" and length > 0)' \"$CONVERSATION_OPERATOR_REPLY_PROOF_PATH\" >/dev/null"
run_step bash -lc "jq -e '(.target_queue_id | type == \"string\" and length > 0) and (.target_team_id | type == \"string\" and length > 0) and (.target_user_id | type == \"string\" and length > 0) and (.performed_by_id | type == \"string\" and length > 0)' \"$CONVERSATION_OPERATOR_HANDOFF_PROOF_PATH\" >/dev/null"
run_step bash -lc "jq -e '.linked_case_id == .case_id and .case_origin_conversation == .session_id and .conversation_status == \"escalated\" and (.case_queue_id | type == \"string\" and length > 0)' \"$CONVERSATION_OPERATOR_ESCALATION_PROOF_PATH\" >/dev/null"
run_step bash -lc "jq -e '.status == \"waiting\" and (.queue_item_id | type == \"string\" and length > 0) and (.operator_reply_id | type == \"string\" and length > 0) and (.follow_up_message_id | type == \"string\" and length > 0) and (.message_count >= 3)' \"$PUBLIC_CONVERSATION_INTAKE_PROOF_PATH\" >/dev/null"
if [[ -z "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" && -n "$FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST" ]]; then
  run_step bash scripts/fetch-publication-evidence.sh --manifest "$FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST" --out "$FETCHED_PUBLICATION_EVIDENCE_DIR"
  FIRST_PARTY_PUBLICATION_EVIDENCE_DIR="$FETCHED_PUBLICATION_EVIDENCE_DIR"
fi
if [[ -z "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" && -d "$ARCHIVED_PUBLICATION_EVIDENCE_DIR" ]]; then
  FIRST_PARTY_PUBLICATION_EVIDENCE_DIR="$ARCHIVED_PUBLICATION_EVIDENCE_DIR"
fi
if [[ -n "$FIRST_PARTY_PUBLICATION_EVIDENCE_DIR" ]]; then
  publication_manifest_path="${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST:-$ROOT_DIR/docs/evidence/public-bundle-publication-runs.json}"
  require_file "$publication_manifest_path"
  cp "$publication_manifest_path" "$PUBLIC_BUNDLE_PUBLICATION_MANIFEST_ARCHIVE_PATH"
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
  if [[ "${#compare_args[@]}" -gt 0 ]]; then
    run_step bash scripts/verify-publication-evidence.sh \
      --manifest "$PUBLIC_BUNDLE_PUBLICATION_MANIFEST_ARCHIVE_PATH" \
      --plan "$PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH" \
      --evidence-dir "$PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR" \
      "${compare_args[@]}" \
      --out "$PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH"
  else
    run_step bash scripts/verify-publication-evidence.sh \
      --manifest "$PUBLIC_BUNDLE_PUBLICATION_MANIFEST_ARCHIVE_PATH" \
      --plan "$PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH" \
      --evidence-dir "$PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR" \
      --out "$PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH"
  fi
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
- cli_release_dir: $(artifact_rel "$CLI_OUT_DIR")
- cli_release_verification: $(artifact_rel "$CLI_OUT_DIR/verification.json")
- local_mbr_bin: $(artifact_rel "$LOCAL_MBR_BIN")
- integration_log: $(artifact_rel "$INTEGRATION_LOG_PATH")
- runtime_bootstrap_artifact: $(artifact_rel "$BOOTSTRAP_PROOF_PATH")
- ats_scenario_artifact: $(artifact_rel "$ATS_SCENARIO_PATH")
- canonical_workspace_refs_manifest: $(artifact_rel "$WORKSPACE_REFS_MANIFEST_ARCHIVE_PATH")
- canonical_workspace_refs_verification: $(artifact_rel "$WORKSPACE_REFS_VERIFICATION_PATH")
- workflow_proof_dir: $(artifact_rel "$WORKFLOW_PROOF_DIR")
- go_work_path: $(artifact_rel "${GOWORK:-off}")
- workflow_case_reply_artifact: $(artifact_rel "$CASE_REPLY_PROOF_PATH")
- workflow_case_command_create_artifact: $(artifact_rel "$CASE_COMMAND_CREATE_PROOF_PATH")
- workflow_case_command_failure_artifact: $(artifact_rel "$CASE_COMMAND_FAILURE_PROOF_PATH")
- workflow_case_operator_manual_create_artifact: $(artifact_rel "$CASE_OPERATOR_MANUAL_CREATE_PROOF_PATH")
- workflow_case_operator_work_management_artifact: $(artifact_rel "$CASE_OPERATOR_WORK_MANAGEMENT_PROOF_PATH")
- workflow_case_operator_reply_artifact: $(artifact_rel "$CASE_OPERATOR_REPLY_PROOF_PATH")
- workflow_case_operator_handoff_artifact: $(artifact_rel "$CASE_OPERATOR_HANDOFF_PROOF_PATH")
- workflow_case_operator_status_transition_artifact: $(artifact_rel "$CASE_OPERATOR_STATUS_TRANSITION_PROOF_PATH")
- workflow_case_operator_attachment_upload_artifact: $(artifact_rel "$CASE_OPERATOR_ATTACHMENT_UPLOAD_PROOF_PATH")
- workflow_conversation_operator_reply_artifact: $(artifact_rel "$CONVERSATION_OPERATOR_REPLY_PROOF_PATH")
- workflow_conversation_operator_handoff_artifact: $(artifact_rel "$CONVERSATION_OPERATOR_HANDOFF_PROOF_PATH")
- workflow_conversation_operator_escalation_artifact: $(artifact_rel "$CONVERSATION_OPERATOR_ESCALATION_PROOF_PATH")
- workflow_public_conversation_intake_artifact: $(artifact_rel "$PUBLIC_CONVERSATION_INTAKE_PROOF_PATH")
- workflow_email_command_failure_artifact: $(artifact_rel "$EMAIL_COMMAND_FAILURE_PROOF_PATH")
- workflow_inbound_case_create_artifact: $(artifact_rel "$INBOUND_NEW_EMAIL_PROOF_PATH")
- workflow_inbound_email_attachments_artifact: $(artifact_rel "$INBOUND_EMAIL_ATTACHMENTS_PROOF_PATH")
- workflow_inbound_reply_threading_artifact: $(artifact_rel "$INBOUND_REPLY_THREADING_PROOF_PATH")
- workflow_form_notification_artifact: $(artifact_rel "$FORM_NOTIFICATION_PROOF_PATH")
- workflow_rule_email_artifact: $(artifact_rel "$RULE_EMAIL_PROOF_PATH")
- workflow_knowledge_notification_artifact: $(artifact_rel "$KNOWLEDGE_NOTIFICATION_PROOF_PATH")
- workflow_notification_command_failure_artifact: $(artifact_rel "$NOTIFICATION_COMMAND_FAILURE_PROOF_PATH")
- extensions_validation_dir: $(artifact_rel "$EXTENSIONS_VALIDATION_DIR")
- public_bundle_publication_plan: $(artifact_rel "$PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH")
- public_bundle_publication_manifest: $(artifact_rel "$PUBLIC_BUNDLE_PUBLICATION_MANIFEST_ARCHIVE_PATH")
- public_bundle_release_evidence_dir: $(artifact_rel "$PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR")
- public_bundle_evidence_verification: $(artifact_rel "$PUBLIC_BUNDLE_EVIDENCE_VERIFICATION_PATH")

## Commands Run

1. \`go test -count=1 ./internal/service/services ./internal/knowledge/services ./internal/platform/services ./cmd/api ./cmd/mbr\`
2. \`bash scripts/verify-canonical-workspace-refs.sh --manifest ${WORKSPACE_REFS_MANIFEST_ARCHIVE_PATH} --workspace-root ${WORKSPACE_ROOT} --out ${WORKSPACE_REFS_VERIFICATION_PATH}\`
3. \`bash scripts/check-cli-contract-docs.sh\`
4. \`go build -trimpath -o ${LOCAL_MBR_BIN} ./cmd/mbr\`
5. \`env WORKFLOW_PROOF_DIR=${WORKFLOW_PROOF_DIR} go test -count=1 ./internal/knowledge/services\`
6. \`env WORKFLOW_PROOF_DIR=${WORKFLOW_PROOF_DIR} go test -tags=integration -count=1 ./...\`
7. \`go work\` bootstrap for proof when no workspace-level \`go.work\` exists, exported as \`${GOWORK:-off}\`
8. \`(cd ${FIRST_PARTY_EXTENSIONS_ROOT} && go test ./ats/runtime ./cmd/ats-runtime ./tools/ats-scenario-proof -count=1)\`
9. \`(cd ${FIRST_PARTY_EXTENSIONS_ROOT} && go run ./tools/publication-evidence --mode plan --source-root ${FIRST_PARTY_EXTENSIONS_ROOT} --out ${PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH})\`
10. \`go run ./tools/runtime-bootstrap-proof --out ${BOOTSTRAP_PROOF_PATH} --version ${VERSION} --git-sha ${GIT_SHA} --build-date ${GENERATED_AT}\`
11. \`(cd ${FIRST_PARTY_EXTENSIONS_ROOT} && go run ./tools/ats-scenario-proof --out ${ATS_SCENARIO_PATH} --version ${VERSION} --git-sha ${GIT_SHA} --build-date ${GENERATED_AT})\`
12. \`bash scripts/fetch-publication-evidence.sh --manifest ${FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST_DISPLAY} --out ${FETCHED_PUBLICATION_EVIDENCE_DIR}\` when a manifest is supplied
13. \`bash scripts/verify-publication-evidence.sh --manifest ${PUBLIC_BUNDLE_PUBLICATION_MANIFEST_ARCHIVE_PATH} --plan ${PUBLIC_BUNDLE_PUBLICATION_PLAN_PATH} --evidence-dir ${PUBLIC_BUNDLE_RELEASE_EVIDENCE_DIR}\`
14. \`MBR_BIN=${LOCAL_MBR_BIN} bash ${FIRST_PARTY_VALIDATION_SCRIPT}\`
15. \`bash scripts/build-cli-release.sh --version ${VERSION} --out ${CLI_OUT_DIR}\`
16. \`bash scripts/verify-cli-release.sh ${CLI_OUT_DIR} --version ${VERSION} --git-sha ${GIT_SHA}\`

## Evidence Docs

- [docs/MILESTONE_1_READINESS.md](../../docs/MILESTONE_1_READINESS.md)
- [docs/MILESTONE_1_PROOF.md](../../docs/MILESTONE_1_PROOF.md)
- [docs/FIRST_PARTY_EXTENSION_READINESS.md](../../docs/FIRST_PARTY_EXTENSION_READINESS.md)
- [docs/CLI_RELEASES.md](../../docs/CLI_RELEASES.md)
EOF

echo
echo "Proof summary: $SUMMARY_PATH"
