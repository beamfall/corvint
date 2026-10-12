#!/bin/sh
set -eu

# gate-affected-corvint.sh is the fast tier of the test gate on the session test runner
# (STR-V0-012 rollout step 2): it builds `corvint` from this checkout into a private
# temporary directory and runs `corvint test --base BASE` in the working directory, so the
# planner, the AFP-V0-012 attribution, the declared full scope and the runner are the
# reviewed source under test, never a stale binary on PATH. `corvint test` prints its
# selection and every FALLBACK reason itself and writes its receipt under the Git directory.
# Its exit status is passed through: 0 PASS, 1 FAIL, 2 usage, 3 INCOMPLETE, 128+n signal.
# script/gate-affected.sh, the previous implementation, stays in the tree as the rollback.
#
# Usage: script/gate-affected-corvint.sh [BASE]. BASE (a commit or ref) is resolved here to
# a full commit id and printed; the committed tree diff BASE..HEAD joins the dirty worktree.
# An empty BASE is HEAD, so only the dirty worktree is tested. A BASE that does not resolve
# is a usage error (exit 2): the run never widens or narrows on a guessed base.
#
# Environment: CORVINT_TEST_BINARY (a prebuilt corvint; fixture tests only) and
# CORVINT_GOCACHE (the build cache, default /tmp/corvint-go-build-cache).

base_request=${1:-}
source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)

case "$base_request" in
    -*)
        printf 'gate-affected: base %s is not a revision\n' "$base_request" >&2
        exit 2
        ;;
    "") base_request=HEAD ;;
esac
if ! base=$(git rev-parse --verify --quiet "$base_request^{commit}"); then
    printf 'gate-affected: base %s does not resolve to a commit\n' "$base_request" >&2
    exit 2
fi
printf 'gate-affected: base %s (%s)\n' "$base" "$base_request"

work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-gate-affected.XXXXXX")
child=""
# A signal trap that returns lets the shell resume, so each signal is forwarded to the
# running corvint (which kills its own process groups) and exits; EXIT cleans up.
interrupt() {
    if [ -n "$child" ]; then
        kill -s "$1" "$child" 2> /dev/null || true
        wait "$child" 2> /dev/null || true
    fi
    exit "$2"
}
trap 'rm -rf "$work"' EXIT
trap 'interrupt HUP 129' HUP
trap 'interrupt INT 130' INT
trap 'interrupt TERM 143' TERM

corvint=${CORVINT_TEST_BINARY:-}
if [ -z "$corvint" ]; then
    corvint="$work/corvint"
    # The build runs as a waited background child too, so a signal during it is forwarded
    # rather than deferred until a hung build returns.
    build=0
    (cd "$source_root" && exec env GOCACHE="${CORVINT_GOCACHE:-/tmp/corvint-go-build-cache}" \
        GOTOOLCHAIN=local go build -trimpath -o "$corvint" ./cmd/corvint) &
    child=$!
    wait "$child" || build=$?
    child=""
    if [ "$build" -ne 0 ]; then
        printf 'gate-affected: corvint could not be built from %s\n' "$source_root" >&2
        exit 2
    fi
fi

status=0
"$corvint" test --base "$base" &
child=$!
wait "$child" || status=$?
child=""
exit "$status"
