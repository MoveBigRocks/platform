#!/bin/sh

set -eu

SEMANTIC_PATTERN='^(fixup! |squash! )?(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([[:alnum:]./_-]+\))?(!)?: .+'

read_message() {
	if [ "$#" -gt 0 ] && [ -f "$1" ]; then
		cat "$1"
		return
	fi

	if [ "$#" -gt 0 ]; then
		printf '%s\n' "$*"
		return
	fi

	cat
}

message="$(read_message "$@")"
subject="$(printf '%s\n' "$message" | sed -n '1p')"

if [ -z "$subject" ]; then
	echo "ERROR: Commit message subject is required." >&2
	exit 1
fi

if printf '%s\n' "$subject" | grep -Eq '^(Merge |Revert ")'; then
	exit 0
fi

if ! printf '%s\n' "$subject" | grep -Eq "$SEMANTIC_PATTERN"; then
	cat >&2 <<'EOF'
ERROR: Commit message must follow the semantic format:
  <type>(optional-scope): short summary

Examples:
  feat(auth): add SSO callback support
  fix: prevent duplicate webhook delivery
  chore(ci): cache Go modules in CI

Allowed types:
  build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test
EOF
	printf 'Received: %s\n' "$subject" >&2
	exit 1
fi
