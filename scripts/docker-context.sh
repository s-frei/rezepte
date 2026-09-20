#!/usr/bin/env bash
# Prints every path the Docker build context contains, two levels deep, so
# `docker:context:check` can diff it against the tracked docker-context.txt.
# What that catches is an edit to .dockerignore and a build input that has
# disappeared; it cannot catch a new top-level directory, because
# .dockerignore starts with `*` and nothing unlisted ever enters the context
# at any depth. The listing's other job is being reviewable: the context's
# contents sit in a tracked file instead of only in a build. Two levels is
# the compromise that keeps it readable - a new component under
# frontend/src does not churn it. See
# docs/memory/content/architecture/releases.mdx.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"

OUT="$(mktemp -d)"
ERR="$(mktemp)"
trap 'rm -rf "$OUT" "$ERR"' EXIT

# Normal build chatter would land in the listing's caller, so stderr is held
# back - but only until the build fails. Swallowing it outright made a
# stopped Docker daemon look exactly like a changed context: no output, exit 1.
if ! docker buildx build --target build-context --output "type=local,dest=$OUT" "$ROOT" >/dev/null 2>"$ERR"; then
	echo "docker:context: the build context export failed" >&2
	cat "$ERR" >&2
	exit 1
fi

cd "$OUT"
find . -maxdepth 2 | LC_ALL=C sort
