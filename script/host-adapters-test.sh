#!/usr/bin/env bash
# host-adapters-test.sh runs integrations/host-adapters.test.mjs on its own, outside the Go wrapper
# TestHostAdapterJavaScriptHosts (V1-4147). The file needs two binaries the wrapper otherwise builds:
# the native adapter fixture (CORVINT_TEST_NATIVE_FIXTURE) and a real corvint
# (CORVINT_TEST_REAL_BINARY). This script builds both into a private temporary directory, which it
# removes on every exit path, and passes its arguments to node --test, for example
#   script/host-adapters-test.sh --test-name-pattern='opencode interruption'
# A binary already named by either variable is used as given instead of being built. A relative
# path in either variable, or in HOST_ADAPTERS_NODE when it contains a slash, is resolved against the
# caller's working directory before the script changes directory. Without node on PATH it prints
# NOT_AVAILABLE and exits 3, distinct from a test failure.
#
# Each build and the node suite run as a background job in their own process group (set -m), which
# the script waits on, so SIGINT or SIGTERM reaches it at once: the trap forwards the signal to that
# whole group, sends SIGKILL to the group after HOST_ADAPTERS_GRACE seconds (default 10) if any member
# remains, and only then exits, removing the scratch directory.
set -euo pipefail
set -m

root=$(cd "$(dirname "$0")/.." && pwd)
# absolute PATH prints PATH resolved against the caller's working directory.
absolute() {
  if [[ $1 == /* ]]; then
    printf '%s\n' "$1"
  else
    printf '%s/%s\n' "$PWD" "$1"
  fi
}
node=${HOST_ADAPTERS_NODE:-node}
if [[ $node == */* ]]; then
  node=$(absolute "$node")
fi
fixture=${CORVINT_TEST_NATIVE_FIXTURE:-}
if [[ -n $fixture ]]; then
  fixture=$(absolute "$fixture")
fi
binary=${CORVINT_TEST_REAL_BINARY:-}
if [[ -n $binary ]]; then
  binary=$(absolute "$binary")
fi
if ! command -v "$node" > /dev/null 2>&1; then
  printf 'host-adapters-test: NOT_AVAILABLE: %s is not on PATH; install Node.js to run integrations/host-adapters.test.mjs\n' "$node" >&2
  exit 3
fi
scratch=$(mktemp -d "${TMPDIR:-/tmp}/host-adapters-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
child=
# cancel SIGNAL CODE forwards SIGNAL to the running child's process group, escalates to SIGKILL after
# the grace period while any member remains, and exits CODE once the group is gone.
cancel() {
  trap - INT TERM
  if [[ -n $child ]]; then
    kill -"$1" -- -"$child" 2> /dev/null || true
    (
      sleep "${HOST_ADAPTERS_GRACE:-10}"
      kill -KILL -- -"$child" 2> /dev/null
    ) &
    local killer=$!
    wait "$child" 2> /dev/null || true
    if kill -0 -- -"$child" 2> /dev/null; then
      wait "$killer" 2> /dev/null || true
    else
      kill -- -"$killer" 2> /dev/null || true
    fi
  fi
  exit "$2"
}
trap 'cancel INT 130' INT
trap 'cancel TERM 143' TERM
# supervise COMMAND... runs COMMAND as the cancellable child and returns its status.
supervise() {
  "$@" &
  child=$!
  local rc=0
  wait "$child" || rc=$?
  child=
  return "$rc"
}
cd "$root"
if [[ -z $fixture ]]; then
  fixture=$scratch/adapter-fixture
  supervise env GOTOOLCHAIN=local go build -o "$fixture" ./integrations/testfixture
fi
if [[ -z $binary ]]; then
  binary=$scratch/corvint
  supervise env GOTOOLCHAIN=local go build -o "$binary" ./cmd/corvint
fi
cd "$root/integrations"
supervise env CORVINT_TEST_NATIVE_FIXTURE="$fixture" CORVINT_TEST_REAL_BINARY="$binary" GOTOOLCHAIN=local "$node" --test "$@" host-adapters.test.mjs
