#!/bin/sh
set -eu

# GAG-V0-001/002/004/005/008: private setup, offline environment, revision/output arguments,
# cleanup, and status propagation are observed at the wrapper's delegated-command boundary.
# GAG-V0-006 (case 3): the output is a closed set of seven entries of any type, so an extra entry of
# any type, a missing artifact, or an expected name that is a symlink fails.
# GAG-V0-007: a witness the current run did not record must not satisfy the gate's status check.
# `archive` records that witness best-effort, so a failed write otherwise leaves an earlier run's
# witness under the Git directory for the check to read. The archive build and the status command
# are stubbed on PATH here because the property under test is this wrapper's ordering, not the
# archive semantics that release-artifact-integrity-v0.md owns.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-go-archive-gate-test.XXXXXX")
gate_pid=
stub_pid=
cleanup() {
    [ -z "$gate_pid" ] || kill "$gate_pid" 2>/dev/null || :
    [ -z "$stub_pid" ] || kill "$stub_pid" 2>/dev/null || :
    rm -rf "$test_root"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

repository="$test_root/repo"
mkdir -p "$repository/script" "$test_root/bin"
cp "$source_root/script/go-archive-gate" "$repository/script/"
printf 'fixture\n' > "$repository/README"
git -C "$repository" init -q -b main
git -C "$repository" -c user.name=test -c user.email=test@example.invalid add .
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm fixture
witness="$repository/.git/corvint/release-go-archive-report.json"

# The stub stands in for `go run ./conformance/release-artifact-v0 <subcommand>`. When
# CORVINT_FIXTURE_WITNESS is unset the witness write "failed" and the subcommand still exits zero,
# which is the best-effort behavior the gate has to survive.
cat > "$test_root/bin/go" <<'STUB'
#!/bin/sh
set -eu
subcommand=$3
shift 3
case $subcommand in
archive)
    output=; revision=
    while [ $# -gt 0 ]; do
        case $1 in
        --output) output=$2; shift 2 ;;
        --revision) revision=$2; shift 2 ;;
        *) shift ;;
        esac
    done
    parent=$(dirname "$HOME")
    if [ -n "${CORVINT_FIXTURE_OBSERVE:-}" ]; then
        printf '%s\n' "$parent" > "$CORVINT_FIXTURE_OBSERVE"
    fi
    if [ "${CORVINT_FIXTURE_VALIDATE_ENV:-}" = 1 ]; then
        [ "$revision" = HEAD ]
        [ "$HOME" = "$parent/home" ]
        [ "$TMPDIR" = "$parent/tmp" ]
        [ "$GOTMPDIR" = "$parent/gotmp" ]
        [ "$GOCACHE" = "$parent/gocache" ]
        [ "$GOENV" = off ]
        [ "$GOFLAGS" = -mod=readonly ]
        [ "$GOPROXY" = off ]
        [ "$GOSUMDB" = off ]
        [ "$GOTOOLCHAIN" = local ]
        [ "$output" = "$parent/corvint-go-release" ]
        [ "$(find "$parent" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')" = 4 ]
        for directory in "$parent" "$HOME" "$TMPDIR" "$GOTMPDIR" "$GOCACHE"; do
            mode=$(stat -f '%Lp' "$directory" 2>/dev/null || stat -c '%a' "$directory")
            [ "$mode" = 700 ]
        done
    fi
    if [ -n "${CORVINT_FIXTURE_FAIL:-}" ]; then
        mkdir "$HOME/read-only"
        : > "$HOME/read-only/file"
        chmod 0500 "$HOME/read-only"
        exit "$CORVINT_FIXTURE_FAIL"
    fi
    if [ -n "${CORVINT_FIXTURE_WAIT:-}" ]; then
        printf '%s\n' "$$" > "$CORVINT_FIXTURE_WAIT"
        trap 'exit 143' TERM INT HUP
        while :; do sleep 1; done
    fi
    mkdir -p "$output"
    for name in SHA256SUMS corvint_darwin_amd64.tar.gz corvint_darwin_arm64.tar.gz \
        corvint_linux_amd64.tar.gz corvint_linux_arm64.tar.gz corvint_windows_amd64.zip \
        verification-report.json; do
        : > "$output/$name"
    done
    case ${CORVINT_FIXTURE_EXTRA:-} in
    directory) mkdir "$output/extra" ;;
    symlink) ln -s SHA256SUMS "$output/extra" ;;
    regular) : > "$output/extra" ;;
    missing) rm "$output/verification-report.json" ;;
    symlinked-expected) rm "$output/SHA256SUMS"; ln -s verification-report.json "$output/SHA256SUMS" ;;
    esac
    if [ -n "${CORVINT_FIXTURE_WITNESS:-}" ]; then
        mkdir -p "$(dirname "$CORVINT_FIXTURE_WITNESS")"
        recorded_commit=$(git rev-parse 'HEAD^{commit}')
        [ -z "${CORVINT_FIXTURE_STALE:-}" ] || recorded_commit=0000000000000000000000000000000000000000
        printf '%s %s %s\n' "$recorded_commit" "$(git rev-parse 'HEAD^{tree}')" "${CORVINT_FIXTURE_VERDICT:-PASS}" \
            > "$CORVINT_FIXTURE_WITNESS"
    else
        printf 'release-artifact-v0 archive: private witness not recorded\n' >&2
    fi
    ;;
archive-status)
    if [ -n "${CORVINT_FIXTURE_STATUS_WAIT:-}" ]; then
        printf '%s\n' "$$" > "$CORVINT_FIXTURE_STATUS_WAIT"
        while [ ! -e "$CORVINT_FIXTURE_STATUS_WAIT.release" ]; do sleep 0.05; done
    fi
    path=; revision=; tree=
    while [ $# -gt 0 ]; do
        case $1 in
        --witness) path=$2; shift 2 ;;
        --revision) revision=$2; shift 2 ;;
        --tree) tree=$2; shift 2 ;;
        *) shift ;;
        esac
    done
    if [ ! -f "$path" ]; then
        printf 'INVALID\n'
        exit 1
    fi
    read -r recorded_revision recorded_tree verdict < "$path"
    if [ "$recorded_revision" = "$revision" ] && [ "$recorded_tree" = "$tree" ]; then
        printf '%s\n' "$verdict"
    else
        printf 'STALE\n'
    fi
    ;;
esac
STUB
chmod 0755 "$test_root/bin/go"
PATH="$test_root/bin:$PATH"
export PATH

# 1. A passing run receives the private roots, modes, offline Go settings, revision, and output
#    location GAG-V0-001/002/005 require. Normal cleanup removes the observed private parent.
observe="$test_root/observed-parent"
CORVINT_FIXTURE_OBSERVE=$observe
CORVINT_FIXTURE_VALIDATE_ENV=1
CORVINT_FIXTURE_WITNESS=$witness
export CORVINT_FIXTURE_OBSERVE CORVINT_FIXTURE_VALIDATE_ENV CORVINT_FIXTURE_WITNESS
"$repository/script/go-archive-gate" > /dev/null 2>&1 ||
    fail "a run that recorded its own witness did not pass"
test -f "$witness" || fail "the passing run recorded no witness for the next case to inherit"
test ! -e "$(cat "$observe")" || fail "a passing run left its private parent behind"
unset CORVINT_FIXTURE_OBSERVE CORVINT_FIXTURE_VALIDATE_ENV

# 2. The witness write fails while that earlier witness, bound to the same revision and tree,
#    survives. The gate must refuse rather than report a pass on evidence this run did not produce.
unset CORVINT_FIXTURE_WITNESS
if "$repository/script/go-archive-gate" > /dev/null 2>&1; then
    fail "a witness left by an earlier run satisfied the status check"
fi
test ! -f "$witness" || fail "a witness the run did not record survived the gate"

# 3. A recorded PASS witness must not rescue an output that is not exactly the seven expected regular
#    files: an extra top-level directory, symlink or regular file, a missing artifact, and an expected
#    name that is a symlink each fail the closed-set check (GAG-V0-006).
CORVINT_FIXTURE_WITNESS=$witness
export CORVINT_FIXTURE_WITNESS
for extra in directory symlink regular missing symlinked-expected; do
    if CORVINT_FIXTURE_EXTRA=$extra "$repository/script/go-archive-gate" > /dev/null 2>&1; then
        fail "output variant $extra passed the closed-set check"
    fi
done

# 3b. A witness this run recorded must still carry PASS for the exact HEAD commit (GAG-V0-007): a
#     recorded non-PASS verdict and a witness bound to another revision each fail the status check.
if CORVINT_FIXTURE_VERDICT=FAIL "$repository/script/go-archive-gate" > /dev/null 2>&1; then
    fail "a recorded FAIL witness satisfied the status check"
fi
if CORVINT_FIXTURE_STALE=1 "$repository/script/go-archive-gate" > /dev/null 2>&1; then
    fail "a witness bound to another revision satisfied the status check"
fi

# 4. A build failure keeps its exit status and removes a private parent containing a read-only
#    directory. This covers GAG-V0-004's failure cleanup and GAG-V0-005's build-failure path.
CORVINT_FIXTURE_OBSERVE=$observe
CORVINT_FIXTURE_FAIL=23
export CORVINT_FIXTURE_OBSERVE CORVINT_FIXTURE_FAIL
status=0
"$repository/script/go-archive-gate" > /dev/null 2>&1 || status=$?
[ "$status" -eq 23 ] || fail "a build failure did not retain exit status 23 (got $status)"
test ! -e "$(cat "$observe")" || fail "a failed build left its private parent behind"
unset CORVINT_FIXTURE_OBSERVE CORVINT_FIXTURE_FAIL

# 5. TERM during the delegated build removes the private parent. Both the gate and its waiting stub
#    are owned here and are terminated before the assertion, so interruption leaves no descendant.
wait_file="$test_root/waiting-stub"
CORVINT_FIXTURE_OBSERVE=$observe CORVINT_FIXTURE_WAIT=$wait_file \
    "$repository/script/go-archive-gate" > /dev/null 2>&1 &
gate_pid=$!
attempt=0
while [ ! -s "$wait_file" ] && [ "$attempt" -lt 600 ]; do
    sleep 0.05
    attempt=$((attempt + 1))
done
[ -s "$wait_file" ] || fail "interrupt fixture did not reach the delegated build"
stub_pid=$(cat "$wait_file")
kill -TERM "$gate_pid"
kill -TERM "$stub_pid"
status=0
wait "$gate_pid" || status=$?
gate_pid=
[ "$status" -eq 143 ] || fail "an interrupted run did not retain exit status 143 (got $status)"
attempt=0
while kill -0 "$stub_pid" 2>/dev/null && [ "$attempt" -lt 100 ]; do
    sleep 0.05
    attempt=$((attempt + 1))
done
if kill -0 "$stub_pid" 2>/dev/null; then
    fail "the interrupted build stub survived"
fi
stub_pid=
test ! -e "$(cat "$observe")" || fail "an interrupted run left its private parent behind"

# 6. TERM to the gate alone while its status check still reports PASS exits 143 and removes the
#    private parent, instead of resuming after cleanup to a passing exit (GAG-V0-004).
status_wait="$test_root/waiting-status"
CORVINT_FIXTURE_OBSERVE=$observe CORVINT_FIXTURE_STATUS_WAIT=$status_wait \
    "$repository/script/go-archive-gate" > /dev/null 2>&1 &
gate_pid=$!
attempt=0
while [ ! -s "$status_wait" ] && [ "$attempt" -lt 600 ]; do
    sleep 0.05
    attempt=$((attempt + 1))
done
[ -s "$status_wait" ] || fail "signal fixture did not reach the status check"
kill -TERM "$gate_pid"
: > "$status_wait.release"
status=0
wait "$gate_pid" || status=$?
gate_pid=
[ "$status" -eq 143 ] || fail "a gate signalled during a passing status check exited $status, not 143"
test ! -e "$(cat "$observe")" || fail "a signalled run left its private parent behind"
test -z "$(git -C "$repository" status --porcelain --untracked-files=all)" ||
    fail "the wrapper cases changed the fixture worktree"

printf 'go-archive-gate: private setup, cleanup, failure, interruption, witness, and closed-set checks pass\n'
