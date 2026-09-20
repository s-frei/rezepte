#!/usr/bin/env bash
# Fails when the Docker build context no longer matches docker-context.txt.
# An allowlist rots the same way a denylist does, only in the safe direction:
# a build input silently dropped is a broken build, and a path silently added
# is bytes nobody meant to ship. Both show up here as a diff.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"

ACTUAL="$(mktemp)"
trap 'rm -f "$ACTUAL"' EXIT

"$ROOT/scripts/docker-context.sh" > "$ACTUAL"

if ! diff -u "$ROOT/docker-context.txt" "$ACTUAL"; then
	echo "docker:context:check: the build context changed" >&2
	echo "docker:context:check: review .dockerignore; if the change is wanted, refresh with" >&2
	echo "  mise run docker:context > docker-context.txt" >&2
	exit 1
fi
