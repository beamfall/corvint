#!/bin/sh
set -eu

# gate-affected.sh is the fast tier of the test gate: it asks `corvint affected` which Go
# packages the change plausibly reaches and runs `go test` over that union instead of `./...`.
# It is not the push gate (`make gate` is, unchanged) and it never trusts a narrowed set it
# cannot justify: every reason to doubt the plan falls back to the full go-test command, and
# the selection or the fallback reason is printed so a run can be audited afterwards.
#
# The union (AFP-V0-012) is provider.go.packages plus what tools/gate-affected-select derives
# from a static index of the repository for every dirty path: a Go source's package and its
# reverse importers (so a deleted file still reaches them); every package enclosing any other
# path; every package whose string literals name the path (internal/specindex names
# docs/specs; the CEM sidecar only by a literal that resolves to it); and, whenever any path is dirty, every package whose reads no literal bounds
# (its tests locate the root with `..`, runtime.Caller, os.Getwd or git, or it depends on
# non-test code that does). Fallback, in order of detection: the plan cannot be produced;
# provider.go.state is not RUNNABLE; the Go frontier is unknown at module level (module path
# unresolved, a bounded directory walk, an unparsed source, cgo); the root module definition
# itself is dirty; a dirty or unattributed path carries a control character; the repository
# cannot be indexed (a Go file whose imports do not parse, or the walk's bounds); a package
# path is not a plain import path; the union is empty while the diff is not; or the
# selector's last line is not a verdict.
#
# Usage: script/gate-affected.sh [BASE]. With BASE (a commit or ref), the committed tree diff
# BASE..HEAD joins the worktree dirty set (`corvint affected --base`), so a branch is tested
# as a whole; BASE is resolved here to a full commit id and printed, and one that does not
# resolve falls back.
#
# Environment: GO_TEST_COMMAND (the go-test target's command without its package pattern),
# CORVINT_PLANNER (the planner command; defaults to `go run ./cmd/corvint` in the working
# directory), and the working directory is the repository under test.

go_test_command=${GO_TEST_COMMAND:-"GOTOOLCHAIN=local go test -p 1 -count=1 -timeout ${GO_TEST_TIMEOUT:-30m}"}
planner=${CORVINT_PLANNER:-"GOTOOLCHAIN=local go run ./cmd/corvint"}
base_request=${1:-}

fallback() {
    printf 'gate-affected: FALLBACK %s; running the full go-test command\n' "$1" >&2
    printf 'gate-affected: run %s ./...\n' "$go_test_command"
    eval "$go_test_command ./..."
    exit $?
}

plan=$(mktemp "${TMPDIR:-/tmp}/corvint-gate-affected.XXXXXX")
# A signal trap that returns lets the shell resume, so an interrupted planner would read as
# "affected failed" and start the full go-test run; each signal exits instead, and EXIT cleans up.
trap 'rm -f "$plan"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

range_flag=""
if [ -n "$base_request" ]; then
    case "$base_request" in
        -*) fallback "base $base_request is not a revision" ;;
    esac
    if ! base=$(git rev-parse --verify --quiet "$base_request^{commit}"); then
        fallback "base $base_request does not resolve to a commit"
    fi
    printf 'gate-affected: base %s (%s)\n' "$base" "$base_request"
    range_flag="--base $base"
fi

if ! eval "$planner affected $range_flag" > "$plan"; then
    fallback "corvint affected failed"
fi

module=$(sed -n 's/^module[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' go.mod | head -n 1)
if [ -z "$module" ]; then
    fallback "go.mod declares no module path"
fi

# The selection step (tools/gate-affected-select, run from this script's own checkout so the
# repository under test needs no copy of it) reads the affected-plan/0 receipt and prints one
# audit line per decision (`selected`, `frontier`, `data`, `reader`, `unresolved`), then one
# verdict as the last line: `run <pkgs>`, `FALLBACK <reason>`, or `NOTHING <reason>`. Package
# paths are validated before they reach a shell word. A receipt the selector cannot read
# falls back.
source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
if ! selection=$(GOTOOLCHAIN=local go -C "$source_root" run ./tools/gate-affected-select "$plan" "$module" "$(pwd -P)"); then
    fallback "the affected-plan receipt could not be read"
fi

printf '%s\n' "$selection" | sed '$d' | sed 's/^/gate-affected: /'
verdict=$(printf '%s\n' "$selection" | sed -n '$p')
case "$verdict" in
    FALLBACK\ *)
        fallback "${verdict#FALLBACK }"
        ;;
    NOTHING\ *)
        printf 'gate-affected: %s\n' "${verdict#NOTHING }"
        exit 0
        ;;
    run\ *) ;;
    *)
        fallback "the selector's last line is not a verdict"
        ;;
esac

packages=${verdict#run }
printf 'gate-affected: run %s %s\n' "$go_test_command" "$packages"
eval "$go_test_command $packages"
