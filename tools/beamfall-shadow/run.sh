#!/bin/sh
# Private convenience wrapper. The Go launcher creates a process group and
# forwards EXIT/INT/TERM into it before waiting/reaping the direct child.
set -eu
umask 077
shadow_tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-beamfall-shadow.XXXXXX")
wrapper_bin=$shadow_tmp/wrapper
child=
cleanup() { rm -rf "$shadow_tmp"; }
stop() {
  signal=$1
  if [ -n "$child" ]; then
    kill -"$signal" "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
    child=
  fi
  cleanup
}
trap 'status=$?; stop TERM; exit "$status"' EXIT
trap 'stop HUP; exit 129' HUP
trap 'stop INT; exit 130' INT
trap 'stop TERM; exit 143' TERM
TMPDIR=$shadow_tmp
export TMPDIR
"${SHADOW_GO:-go}" build -o "$wrapper_bin" ./tools/beamfall-shadow/wrapper
"$wrapper_bin" -- "${SHADOW_TARGET_GO:-go}" run ./tools/beamfall-shadow "$@" &
child=$!
wait "$child"
status=$?
child=
exit "$status"
