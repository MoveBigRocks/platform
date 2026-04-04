#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "${repo_root}"

unexpected_runtime_files="$(
  find internal/extensionhost/runtime -maxdepth 1 -type f -name '*.go' \
    ! -name 'background.go' \
    ! -name 'registry.go' \
    ! -name 'runtime.go' \
    ! -name 'socket_transport.go' \
    ! -name '*_test.go' \
    -print
)"

if [[ -n "${unexpected_runtime_files}" ]]; then
  echo "Platform runtime boundary violation: unexpected extension-specific source under internal/extensionhost/runtime" >&2
  echo "${unexpected_runtime_files}" >&2
  exit 1
fi

if rg -n --glob '*.go' --glob '!**/*_test.go' 'github\.com/movebigrocks/(extensions|private-extensions)/' .; then
  echo >&2
  echo "Platform must not import github.com/movebigrocks/extensions/... or github.com/movebigrocks/private-extensions/..." >&2
  echo "Keep platform generic and host extensions through manifests, routes, and runtime protocols." >&2
  exit 1
fi

echo "Platform extension boundary check passed."
