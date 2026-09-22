#!/bin/sh
set -eu

# GAG-V0-006 real-producer injection: `script/go-archive-gate_test.sh` stubs the archive build, so
# its closed-set cases never see output the real producer wrote. This harness runs the committed
# gate unchanged in a throwaway clone of HEAD with a `go` shim first on PATH. The shim forwards every
# Go command to the caller's real `go`; for `archive` it lets the real command build once, keeps that
# output and the witness it recorded, and in later cases replays those bytes. Injection happens
# after production and before the wrapper's own checks, the seam the producer does not expose.
# Every case runs the real wrapper under `bash -x`, so the failing check is observed, not inferred.
# GAG-V0-004 interrupt: a SIGINT to the gate's process group during the real `archive` command must
# still remove the private parent and leave no process behind.
# Needs the manifest's exact local Go toolchain first on PATH. One real five-target build runs.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-go-archive-injection-test.XXXXXX")
gate_pgid=
cleanup() {
    [ -z "$gate_pgid" ] || kill -KILL -- "-$gate_pgid" 2>/dev/null || :
    chmod -R u+w "$test_root" 2>/dev/null || :
    rm -rf "$test_root"
}
# A signal ends the harness through the EXIT trap. A handler that cleaned up and returned would resume
# past a tolerated failure into checks over the removed directory, which can pass and exit 0.
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

real_go=$(command -v go) || fail "no go on PATH"
head=$(git -C "$source_root" rev-parse --verify 'HEAD^{commit}')
source_witness="$(git -C "$source_root" rev-parse --absolute-git-dir)/corvint/release-go-archive-report.json"
source_witness_before=$(cksum 2>/dev/null < "$source_witness" || printf absent)

repository="$test_root/repo"
git clone -q --no-checkout "$source_root" "$repository"
git -C "$repository" checkout -q --detach "$head"
mkdir "$test_root/bin" "$test_root/cache" "$test_root/tmp"

cat > "$test_root/bin/go" <<'SHIM'
#!/bin/sh
set -eu
if [ "$#" -lt 3 ] || [ "$1" != run ] || [ "$2" != ./conformance/release-artifact-v0 ] || [ "$3" != archive ]; then
    exec "$CORVINT_INJECTION_REAL_GO" "$@"
fi
output=
previous=
for argument in "$@"; do
    [ "$previous" != --output ] || output=$argument
    previous=$argument
done
[ -n "$output" ]
witness="$(git rev-parse --absolute-git-dir)/corvint/release-go-archive-report.json"
cache=$CORVINT_INJECTION_CACHE
case $CORVINT_INJECTION_PHASE in
build)
    "$CORVINT_INJECTION_REAL_GO" "$@"
    cp -Rp "$output" "$cache/output"
    cp -p "$witness" "$cache/witness"
    ;;
replay)
    mkdir -m 0700 "$output"
    cp -Rp "$cache/output/." "$output/"
    mkdir -p "$(dirname "$witness")"
    cp -p "$cache/witness" "$witness"
    ;;
interrupt)
    printf '%s\n' "$$" > "$cache/interrupt-pid"
    exec "$CORVINT_INJECTION_REAL_GO" "$@"
    ;;
esac
case $CORVINT_INJECTION_CASE in
none) ;;
extra-regular) : > "$output/extra" ;;
extra-directory) mkdir "$output/extra" ;;
extra-symlink) ln -s SHA256SUMS "$output/extra" ;;
missing-archive) rm "$output/corvint_windows_amd64.zip" ;;
symlinked-archive) rm "$output/corvint_linux_arm64.tar.gz"
    ln -s "$cache/output/corvint_linux_arm64.tar.gz" "$output/corvint_linux_arm64.tar.gz" ;;
esac
SHIM
chmod 0755 "$test_root/bin/go"

CORVINT_INJECTION_REAL_GO=$real_go
CORVINT_INJECTION_CACHE="$test_root/cache"
export CORVINT_INJECTION_REAL_GO CORVINT_INJECTION_CACHE

# run_gate PHASE CASE: runs the real wrapper under the shim with a private TMPDIR, so a private parent
# the gate failed to remove is observable. Sets `status` and `last_test` (the last top-level `test`).
run_gate() {
    status=0
    (
        cd "$repository"
        PATH="$test_root/bin:$PATH" TMPDIR="$test_root/tmp" CORVINT_INJECTION_PHASE=$1 CORVINT_INJECTION_CASE=$2 \
            bash -x script/go-archive-gate > "$test_root/stdout" 2> "$test_root/trace"
    ) || status=$?
    last_test=$(grep '^+ test ' "$test_root/trace" | tail -n 1 || :)
    test -z "$(find "$test_root/tmp" -mindepth 1 -maxdepth 1)" || fail "case $2 ($1) left a private parent behind"
    # Render the comparison without credential-assignment syntax (the screen treats PASS as pass).
    printf "case %s %s: exit %s at %s\n" "$1" "$2" "$status" "$(printf '%s' "${last_test:-no test}" | sed 's/ = / equals /g')"
}

# 1. The real producer builds once from the clone's HEAD and the unchanged gate passes on it.
run_gate build none
[ "$status" -eq 0 ] || { tail -n 40 "$test_root/trace" >&2; fail "the real archive build did not pass (exit $status)"; }
grep -q "^SUMMARY revision=$head targets=5 verdict=PASS " "$test_root/stdout" || fail "the real build printed no PASS summary for $head"
[ "$last_test" = "+ test PASS = PASS" ] || fail "the real build did not reach the witness status check"
test -f "$test_root/cache/witness" || fail "the real build recorded no witness to replay"

# 2. Replaying the real output and witness unmodified still passes, so each failure below is caused
#    by its injection and not by the replay.
run_gate replay none
[ "$status" -eq 0 ] || fail "the unmodified replay of the real output failed (exit $status)"
[ "$last_test" = "+ test PASS = PASS" ] || fail "the unmodified replay did not reach the witness status check"

# 3. Each injection into real output fails with exit 1 at the documented check, before archive-status.
expect_refusal() {
    run_gate replay "$1"
    [ "$status" -eq 1 ] || fail "injection $1 exited $status, want 1"
    [ "$last_test" = "$2" ] || fail "injection $1 failed at '$last_test', want '$2'"
    if grep -q 'archive-status' "$test_root/trace"; then
        fail "injection $1 reached the witness status check"
    fi
}
expect_refusal extra-regular "+ test 8 = 7"
expect_refusal extra-directory "+ test 8 = 7"
expect_refusal extra-symlink "+ test 8 = 7"
expect_refusal symlinked-archive "+ test 6 = 7"
run_gate replay missing-archive
[ "$status" -eq 1 ] || fail "injection missing-archive exited $status, want 1"
case $last_test in
"+ test -f "*/corvint-go-release/corvint_windows_amd64.zip) ;;
*) fail "injection missing-archive failed at '$last_test', want the presence check" ;;
esac

# 4. SIGINT to the gate's process group once the real archive tool is running: the gate exits non-zero,
#    the private parent is gone, and no process of that group survives.
set -m
(
    cd "$repository"
    PATH="$test_root/bin:$PATH" TMPDIR="$test_root/tmp" CORVINT_INJECTION_PHASE=interrupt CORVINT_INJECTION_CASE=none \
        exec bash script/go-archive-gate > "$test_root/stdout" 2> "$test_root/trace"
) &
gate_pgid=$!
set +m
attempt=0
go_pid=
tool_pid=
build_pid=
while [ -z "$build_pid" ] && [ "$attempt" -lt 6000 ]; do
    [ ! -s "$test_root/cache/interrupt-pid" ] || go_pid=$(cat "$test_root/cache/interrupt-pid")
    if [ -n "$go_pid" ]; then
        kill -0 "$go_pid" 2>/dev/null || { tail -n 5 "$test_root/trace" >&2; fail "the real archive command exited before the interrupt"; }
        tool_pid=$(pgrep -P "$go_pid" -f 'release-artifact-v0 archive' | head -n 1 || :)
        [ -z "$tool_pid" ] || build_pid=$(pgrep -P "$tool_pid" | head -n 1 || :)
    fi
    [ -n "$build_pid" ] || sleep 0.2
    attempt=$((attempt + 1))
done
[ -n "$build_pid" ] || fail "the interrupt case never reached a running real archive build"
kill -INT -- "-$gate_pgid"
status=0
wait "$gate_pgid" || status=$?
[ "$status" -ne 0 ] || fail "an interrupted real build exited 0"
attempt=0
while pgrep -g "$gate_pgid" > /dev/null 2>&1 && [ "$attempt" -lt 600 ]; do
    sleep 0.1
    attempt=$((attempt + 1))
done
if pgrep -g "$gate_pgid" > /dev/null 2>&1; then
    fail "a process of the interrupted gate survived"
fi
gate_pgid=
test -z "$(find "$test_root/tmp" -mindepth 1 -maxdepth 1)" || fail "an interrupted real build left its private parent behind"

test -z "$(git -C "$repository" status --porcelain --untracked-files=all)" || fail "the gate changed the clone's worktree"
[ "$(cksum 2>/dev/null < "$source_witness" || printf absent)" = "$source_witness_before" ] ||
    fail "the harness changed the source checkout's private witness"
printf 'case interrupt: exit %s, last stderr: %s\n' "$status" "$(tail -n 1 "$test_root/trace")"
printf 'go-archive-gate-injection: real-output closed-set refusals and real interrupt cleanup pass\n'
