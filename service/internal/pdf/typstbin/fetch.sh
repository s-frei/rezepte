#!/bin/sh
# Usage: fetch.sh <goos> <goarch>
#
# Downloads the official Typst release archive for one Go platform, checks it
# against SHA256SUMS and writes the binary next to this script as
# typst_<goos>_<goarch>, which embed_<goos>_<goarch>.go embeds under the typst
# build tag. A build for this machine copies the pinned typst mise put on PATH
# instead (path a below); any other downloads it, and the archive stays beside
# it (gitignored), so a second run with a matching archive downloads nothing.
# A platform Typst publishes no binary for is skipped: that build embeds
# nothing and falls back to typst on PATH.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: fetch.sh <goos> <goarch>" >&2
	exit 2
fi
goos=$1
goarch=$2
cd "$(dirname "$0")"

case "$goos/$goarch" in
linux/amd64) target=x86_64-unknown-linux-musl ;;
linux/arm64) target=aarch64-unknown-linux-musl ;;
darwin/amd64) target=x86_64-apple-darwin ;;
darwin/arm64) target=aarch64-apple-darwin ;;
*)
	echo "fetch.sh: no Typst binary for $goos/$goarch; building without one"
	exit 0
	;;
esac

version=$(cat VERSION)
out="typst_${goos}_${goarch}"

# install_if_changed <file>: replace $out only when the content differs, so a
# repeated build leaves the embedded file untouched.
install_if_changed() {
	if [ -f "$out" ] && cmp -s "$1" "$out"; then
		rm -f "$1"
		return
	fi
	mv "$1" "$out"
}

# Path (a): the build is for this machine and PATH has the pinned typst. That
# is mise's install in dev and in every CI job, which mise takes from the
# release through aqua and checks against the checksums in mise.lock, so it
# is the same file the download below would produce. `mise which` names the
# install directly; without mise, PATH is asked. The real path is resolved so
# a mise shim (a mise binary) is never mistaken for it.
case "$(uname -s)/$(uname -m)" in
Linux/x86_64) host=linux/amd64 ;;
Linux/aarch64 | Linux/arm64) host=linux/arm64 ;;
Darwin/x86_64) host=darwin/amd64 ;;
Darwin/arm64) host=darwin/arm64 ;;
*) host= ;;
esac
local_typst=
if [ "$host" = "$goos/$goarch" ]; then
	if command -v mise >/dev/null 2>&1; then
		local_typst=$(mise which typst 2>/dev/null || true)
	fi
	if [ -z "$local_typst" ]; then
		local_typst=$(command -v typst || true)
	fi
fi
if [ -n "$local_typst" ]; then
	local_typst=$(realpath "$local_typst")
	if [ "$(basename "$local_typst")" = typst ] &&
		[ "$("$local_typst" --version 2>/dev/null | cut -d' ' -f2)" = "$version" ]; then
		cp "$local_typst" "$out.part"
		chmod 0644 "$out.part"
		install_if_changed "$out.part"
		echo "fetch.sh: using typst $version from $local_typst"
		exit 0
	fi
fi

# Path (b): download the release archive. bsdtar (macOS) reads xz itself;
# GNU tar runs the xz binary.
for tool in curl tar; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "fetch.sh: $tool is required to fetch Typst; install it and run again" >&2
		exit 1
	fi
done
if tar --version 2>/dev/null | grep -q GNU && ! command -v xz >/dev/null 2>&1; then
	echo "fetch.sh: xz is required to fetch Typst; install it and run again" >&2
	exit 1
fi

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

asset="typst-$target.tar.xz"
want=$(awk -v a="$asset" '$2 == a { print $1 }' SHA256SUMS)
if [ -z "$want" ]; then
	echo "fetch.sh: SHA256SUMS has no line for $asset" >&2
	exit 1
fi
archive="typst_${goos}_${goarch}.tar.xz"

if [ ! -f "$archive" ] || [ "$(sha256 "$archive")" != "$want" ]; then
	echo "fetch.sh: downloading Typst $version for $goos/$goarch"
	curl -fsSL --retry 3 -o "$archive.part" \
		"https://github.com/typst/typst/releases/download/v$version/$asset"
	got=$(sha256 "$archive.part")
	if [ "$got" != "$want" ]; then
		rm -f "$archive.part"
		echo "fetch.sh: $asset has sha256 $got, SHA256SUMS says $want" >&2
		exit 1
	fi
	mv "$archive.part" "$archive"
fi

tar -xJOf "$archive" "typst-$target/typst" > "$out.part"
install_if_changed "$out.part"
