#!/usr/bin/env bash
#MISE description="Create the GitHub release for the current tag and upload dist/"
#
# Publishes the GitHub release for a version tag and attaches everything in
# dist/. A task rather than inline workflow YAML, so the whole release is
# reproducible locally, like the archives it uploads. See
# docs/memory/content/architecture/releases.mdx.
set -euo pipefail

# mise runs a root file task from the repository root.
DIST="$PWD/dist"

# The workflow passes the tag in REZEPTE_VERSION, the same variable
# //:release:binaries reads, so the archives and the release are stamped from
# one value. Without it, fall back to the tag that points at HEAD and only
# that: that task may fall back to a bare commit for a local build, but a
# release must never invent a tag name that does not exist.
VERSION="${REZEPTE_VERSION:-$(git describe --tags --exact-match 2>/dev/null || true)}"
if [ -z "$VERSION" ]; then
	echo "release:publish: no tag - set REZEPTE_VERSION=vX.Y.Z or run on a tagged commit" >&2
	exit 1
fi

DIST="$ROOT/dist"
# An empty dist/ would publish an assetless release that still reads as a
# finished one, and re-running cannot repair it because the tag is then taken.
if [ -z "$(ls -A "$DIST" 2>/dev/null || true)" ]; then
	echo "release:publish: $DIST is empty - run 'mise run release:binaries' first" >&2
	exit 1
fi

# Semver puts prerelease identifiers after a hyphen, so v1.5.0-rc1 is a
# prerelease and v1.5.0 is not. Without --prerelease the release candidate
# becomes /releases/latest - the URL the user docs send people to for the
# binaries - while the image job deliberately withholds :latest from it, and
# one tag would hand Docker users stable and binary users the candidate.
# Positional parameters rather than an array: they carry an empty list safely
# under `set -u` in every bash this script may meet.
set --
case "$VERSION" in
*-*)
	set -- --prerelease
	echo "release:publish: $VERSION is a prerelease"
	;;
esac

echo "release:publish: creating release $VERSION from $DIST"
gh release create "$VERSION" "$@" --generate-notes "$DIST"/*
