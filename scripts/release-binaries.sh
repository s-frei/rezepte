#!/usr/bin/env bash
# Builds the published release archives into dist/. The SPA is platform
# independent and is built once by this task's `depends`, so each target here
# is a Go cross-compile and nothing more. See
# docs/memory/content/architecture/releases.mdx.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"

# The workflow passes the tag; a local run falls back to the nearest tag, or
# the commit when the repository has no tag yet, or `dev` in an exported source
# tree with no .git at all. Same derivation as //service:build:only.
VERSION="${REZEPTE_VERSION:-$(git -C "$ROOT" describe --tags --always 2>/dev/null || echo dev)}"
VERSION="${VERSION#v}"

DIST="$ROOT/dist"
rm -rf "$DIST"
mkdir -p "$DIST"

# Only the archives and checksums.txt may end up in dist/, so the workflow can
# upload dist/* without naming each file. The loose binaries are staged here.
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

for target in linux/amd64 linux/arm64 darwin/arm64 darwin/amd64; do
	os="${target%/*}"
	arch="${target#*/}"

	echo "release:binaries: building $os/$arch"
	GOOS="$os" GOARCH="$arch" mise run //service:build:only

	install -m 0755 "$ROOT/service/bin/rezepte" "$STAGE/rezepte"
	tar -czf "$DIST/rezepte_${VERSION}_${os}_${arch}.tar.gz" -C "$STAGE" rezepte
	rm -f "$STAGE/rezepte"
done

# The loop ends on darwin/amd64, so service/bin/rezepte is now a binary for
# whatever platform came last - "cannot execute binary file" for anyone who
# runs it, or a silent Rosetta start on an arm64 Mac. The archives are this
# task's output; the shared path belongs to `mise run build`, so leave it
# empty rather than misleading. Rebuilding for the host would cost a build
# for nothing.
rm -f "$ROOT/service/bin/rezepte"

# shasum rather than sha256sum: it exists on both macOS and the CI runner.
cd "$DIST"
shasum -a 256 ./*.tar.gz > checksums.txt

echo "release:binaries: wrote $VERSION to $DIST"
ls -1 "$DIST"
