#!/bin/sh
set -eu

# Three fixed behaviors in agent-preflight.sh:
#
#   1. It must see a worktree DELETION (`git status --short` prefix ` D`) as a
#      concurrent write, not just a modification (` M`). Before the fix, `grep '^ M'`
#      let a deleted-on-disk tracked path pass through unseen exactly as a `git add -u`
#      would (docs/agent-memory/fixes.md, 2026-09-12).
#   2. Given an owned-path argument, a concurrently modified path OUTSIDE that argument
#      list must fail the check (exit non-zero); one INSIDE it must not. Before the fix,
#      this block only ever called `note`, so it could never set a non-zero exit no
#      matter what `git status` showed.
#   3. A failed `git ls-files src/` call must refuse instead of proving that no tracked
#      oracle path remains.
#
# Each case runs the real script (copied into an isolated fixture repo, per the
# gate-enumeration_test.sh pattern) against a small git history it controls.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
harness_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-agent-preflight-test.XXXXXX")
cleanup() { rm -rf "$harness_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

# A minimal PATH keeps the unrelated installed-binary check out of these assertions:
# this host has a real corvint on PATH, and building the fixture's stub cmd/corvint
# against it would report STALE and fail the whole script for a reason these cases
# aren't testing. /usr/bin:/bin still carries sh, git, grep, sed, cut, tr, wc, mktemp.
run_preflight() {
    fixture=$1; shift
    (cd "$fixture" && PATH=/usr/bin:/bin sh script/agent-preflight.sh "$@" 2>&1)
}

new_fixture() {
    name=$1
    fixture=$harness_root/$name
    mkdir -p "$fixture/script" "$fixture/src" "$fixture/cmd/corvint"
    cp "$source_root/script/agent-preflight.sh" "$fixture/script/"
    printf 'package main\n\nfunc main() {}\n' > "$fixture/cmd/corvint/main.go"
    cat > "$fixture/go.mod" <<'MOD'
module fixture

go 1.21
MOD
    git -C "$fixture" init -q
    git -C "$fixture" config user.email fixture@example.test
    git -C "$fixture" config user.name Fixture
    printf 'a\n' > "$fixture/tracked-a.txt"
    printf 'b\n' > "$fixture/tracked-b.txt"
    git -C "$fixture" add -A
    git -C "$fixture" commit -qm base
    printf '%s\n' "$fixture"
}

# Case 1: a tracked file deleted on disk (not staged) must register as a concurrent
# write, the same as a modification does.
fixture=$(new_fixture deletion)
rm -f "$fixture/tracked-a.txt"
output=$(run_preflight "$fixture") && status=0 || status=$?
printf '%s\n' "$output" | grep -q 'tracked-a.txt' \
    || fail "deletion: concurrent-writers output did not list the deleted path: $output"
printf '%s\n' "$output" | grep -q 'concurrent-writers.*tracked file(s) modified by someone' \
    || fail "deletion: concurrent-writers did not report the deleted path as a concurrent write: $output"
printf '  %-24s %s\n' "deletion" "deleted path counted as a concurrent write"

# Case 2a: with an owned-path argument, a concurrent modification OUTSIDE that argument
# must fail the whole script (non-zero exit).
fixture=$(new_fixture foreign-write)
printf 'a-changed\n' > "$fixture/tracked-a.txt"
output=$(run_preflight "$fixture" tracked-b.txt) && status=0 || status=$?
test "$status" -ne 0 \
    || fail "foreign-write: expected non-zero exit when a foreign path is modified, got 0: $output"
printf '%s\n' "$output" | grep -q 'FAIL\|fail' \
    || true # note() and fail() share a format; the exit code is the real assertion here.
printf '  %-24s %s\n' "foreign-write" "foreign concurrent write refused, exit $status"

# Case 2b: with the same owned-path argument, a concurrent modification INSIDE that
# argument must not fail the check -- it is the caller's own in-flight file.
fixture=$(new_fixture owned-write)
printf 'a-changed\n' > "$fixture/tracked-a.txt"
output=$(run_preflight "$fixture" tracked-a.txt) && status=0 || status=$?
test "$status" -eq 0 \
    || fail "owned-write: expected exit 0 when the only concurrent write is an owned path, got $status: $output"
printf '  %-24s %s\n' "owned-write" "owned concurrent write allowed, exit $status"

# Case 2c: with no owned-path argument at all (today's calling convention), a
# concurrent modification must stay informational -- exit 0 -- so this fix does not
# turn every existing bare invocation into a hard failure.
fixture=$(new_fixture no-args)
printf 'a-changed\n' > "$fixture/tracked-a.txt"
output=$(run_preflight "$fixture") && status=0 || status=$?
test "$status" -eq 0 \
    || fail "no-args: expected exit 0 with no owned-path argument (backward compatible), got $status: $output"
printf '  %-24s %s\n' "no-args" "no owned paths named: stays informational, exit $status"

# Case 3: failure to enumerate tracked oracle paths must refuse instead of treating src/ as empty.
real_git=$(command -v git)
mkdir -p "$harness_root/failing-git"
cat > "$harness_root/failing-git/git" <<'SH'
#!/bin/sh
for arg do
    if [ "$arg" = ls-files ]; then
        exit 128
    fi
done
exec "$REAL_GIT" "$@"
SH
chmod +x "$harness_root/failing-git/git"
fixture=$(new_fixture git-failure)
output=$(cd "$fixture" && REAL_GIT="$real_git" PATH="$harness_root/failing-git:/usr/bin:/bin" \
    sh script/agent-preflight.sh 2>&1) && status=0 || status=$?
test "$status" -ne 0 \
    || fail "git-failure: expected non-zero exit when git ls-files fails, got 0: $output"
printf '%s\n' "$output" | grep -q 'oracle-retired.*git ls-files failed' \
    || fail "git-failure: missing refusal diagnostic: $output"
printf '  %-24s %s\n' "git-failure" "git ls-files failure refused, exit $status"

printf 'agent-preflight_test: 7 assertions passed\n'
