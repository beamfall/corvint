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
# Environment: CORVINT_TEST_BINARY (a prebuilt corvint; fixture tests only),
# CORVINT_TEST_PROC_ROOT (a stand-in for /proc; fixture tests only) and
# CORVINT_GOCACHE (the build cache, default ${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache).

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
building=""
# process_table prints one "pid ppid" line per process from ps or, where ps is missing or
# fails, from /proc (V1-1400). A /proc stat record is "pid (comm) state ppid ...", and comm may
# hold spaces, parentheses and newlines, so the whole record is joined onto one line and ppid
# is read after the last ") ".
process_table() {
    if ps_table=$(ps -A -o pid= -o ppid= 2> /dev/null); then
        printf '%s\n' "$ps_table"
        return 0
    fi
    listed=""
    for stat in "${CORVINT_TEST_PROC_ROOT:-/proc}"/[0-9]*/stat; do
        line=""
        while IFS= read -r part || [ -n "$part" ]; do
            line="$line$part "
        done 2> /dev/null < "$stat" || continue
        fields=${line##*) }
        fields=${fields#* }
        printf '%s %s\n' "${line%% *}" "${fields%% *}"
        listed=1
    done
    [ -n "$listed" ]
}
# stop_tree freezes a process and every process below it, walking the process table until no
# new child appears (a stopped process cannot fork, and children are found before their parent
# dies and they are reparented), then terminates and resumes them all. A process still alive
# about two seconds later is killed. Without a process table only the build itself is killed.
stop_tree() {
    tree=$1
    pending=$1
    while [ -n "$pending" ]; do
        # shellcheck disable=SC2086
        kill -s STOP $pending 2> /dev/null || true
        if ! table=$(process_table); then
            echo "gate-affected: cannot list processes; killing the build only" >&2
            kill -s KILL "$1" 2> /dev/null || true
            return
        fi
        pending=$(printf '%s\n' "$table" | awk -v parents=" $pending " -v seen=" $tree " \
            'BEGIN { ORS = " " } index(parents, " " $2 " ") && !index(seen, " " $1 " ") { print $1 }')
        tree="$tree $pending"
        pending=${pending% }
    done
    # shellcheck disable=SC2086
    kill -s TERM $tree 2> /dev/null || true
    # shellcheck disable=SC2086
    kill -s CONT $tree 2> /dev/null || true
    # shellcheck disable=SC2086
    survivors=$(echo $tree | tr ' ' ',')
    tries=0
    while [ -n "$survivors" ] && [ "$tries" -lt 20 ]; do
        sleep 0.1
        tries=$((tries + 1))
        # A zombie has exited; a failed listing keeps the previous survivors.
        listed=$(ps -o pid= -o stat= -p "$survivors") || continue
        survivors=$(printf '%s\n' "$listed" | awk '$2 !~ /^Z/ { printf "%s%s", sep, $1; sep = "," }')
    done
    if [ -n "$survivors" ]; then
        # shellcheck disable=SC2046
        kill -s KILL $(echo "$survivors" | tr ',' ' ') 2> /dev/null || true
    fi
}
# A signal trap that returns lets the shell resume, so each signal exits with 128+N; EXIT
# cleans up. During the build every process below it is terminated: an asynchronous child of
# a non-interactive shell ignores SIGINT, and `go build` does not stop its compilers, so
# forwarding the signal alone would leave the build running (V1-1310). During `corvint test`
# the signal is forwarded to corvint, which kills its own process groups.
interrupt() {
    if [ -n "$building" ]; then
        # $! covers a signal that lands after the fork but before child is assigned.
        pid=${child:-${!:-}}
        if [ -n "$pid" ]; then
            stop_tree "$pid"
            wait "$pid" 2> /dev/null || true
        fi
    elif [ -n "$child" ]; then
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
    # The build runs as a waited background child, so a signal during it is handled at once
    # rather than deferred until a hung build returns.
    build_status=0
    building=1
    (cd "$source_root" && exec env GOCACHE="${CORVINT_GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}" \
        GOTOOLCHAIN=local go build -trimpath -o "$corvint" ./cmd/corvint) &
    child=$!
    wait "$child" || build_status=$?
    child=""
    building=""
    if [ "$build_status" -ne 0 ]; then
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
