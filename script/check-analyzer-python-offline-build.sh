#!/bin/sh
set -eu

root=${1:?repository root required}
test -d "$root"
test ! -e "$root/vendor"

module_cache=$(mktemp -d "${TMPDIR:-/tmp}/corvint-analyzer-python-modcache.XXXXXX")
build_cache=$(mktemp -d "${TMPDIR:-/tmp}/corvint-analyzer-python-buildcache.XXXXXX")
binary=$(mktemp "${TMPDIR:-/tmp}/corvint-analyzer-python-offline.XXXXXX")
cleanup() {
	rm -rf "$module_cache" "$build_cache"
	rm -f "$binary"
}
# A signal trap that returns lets the script resume and report success, so each signal exits 128+N
# and the EXIT trap cleans up.
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

cd "$root"
GOMODCACHE="$module_cache" GOCACHE="$build_cache" GOPROXY=off GOSUMDB=off GOWORK=off GOFLAGS=-mod=mod go build -o "$binary" ./cmd/corvint-analyzer-python
GOMODCACHE="$module_cache" GOCACHE="$build_cache" GOPROXY=off GOSUMDB=off GOWORK=off GOFLAGS=-mod=mod go list -deps ./cmd/corvint > "$build_cache/corvint.deps"
# A pipeline beginning with `!` is exempt from `set -e`, so the refusal must exit explicitly.
if grep -qx 'github.com/Beamfall/corvint/internal/analyzerpython' "$build_cache/corvint.deps"; then
	echo "check-analyzer-python-offline-build: Core build depends on github.com/Beamfall/corvint/internal/analyzerpython" >&2
	exit 1
fi
# go may stamp the main module's own pseudo-version into the cache; only
# foreign module artifacts indicate a network download.
foreign=$(find "$module_cache/cache/download" -type f 2>/dev/null | grep -v '/github.com/!beamfall/corvint/@v/' || true)
test -z "$foreign"
