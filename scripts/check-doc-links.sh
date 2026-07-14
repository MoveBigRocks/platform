#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

tmp_hits="$(mktemp)"
trap 'rm -f "$tmp_hits"' EXIT

findings=0

while IFS= read -r source_file; do
  while IFS= read -r raw_match; do
    [[ -z "$raw_match" ]] && continue

    # The line-number prefix and any colons inside a URL are skipped by the
    # paren extraction below, which reads the target between ( and ).
    match="$raw_match"

    target="${match#*\(}"
    target="${target%\)*}"
    target="${target%%#*}"
    target="$(echo "$target" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

    [[ -z "$target" ]] && continue

    if [[ "$target" == http://* || "$target" == https://* || "$target" == mailto:* || "$target" == tel:* || "$target" == javascript:* || "$target" == "#*" || "$target" == *"://"* ]]; then
      continue
    fi
    if [[ "$target" == data:* || "$target" == blob:* ]]; then
      continue
    fi

    if [[ "$target" == "/"* ]]; then
      resolved="$REPO_ROOT$target"
    else
      resolved="$(dirname "$source_file")/$target"
    fi
    # Normalize the path portably (BSD realpath has no -m); keep it relative to
    # the repo root, which is the current directory.
    resolved="$(python3 -c 'import os,sys; print(os.path.normpath(sys.argv[1]))' "$resolved")"

    if [[ ! -e "$resolved" ]]; then
      printf '%s -> %s\n' "$source_file" "$target" >> "$tmp_hits"
      findings=1
    fi
  done < <(rg -o -n '\[[^]]+\]\([^)]*\)' "$source_file")
done < <(rg --files -g '*.md' -g '*.mdx')

if [[ "$findings" -ne 0 ]]; then
  echo 'FAILED: Broken local markdown links found.'
  echo 'Fix these references before continuing:'
  sort -u "$tmp_hits"
  exit 1
fi

echo 'Docs link check passed'
