#!/bin/sh
set -eu

# Covers the two paths docs/specs/sql-native-ratchet-gate-v0.md records as having no automated
# coverage (SNR-V0-001, SNR-V0-002), plus the source-byte ceiling (SNR-V0-005), the Core-base
# export (SNR-V0-007) and the causal_parent presence guard (SNR-V0-013). SNR-V0-005's ceiling
# check runs before any git archive, so a fresh, unrelated fixture repository can prove the
# boundary: at the ceiling the script proceeds past the Core-base export to the toolchain receipt
# build, which finds no main module in the fixture; one byte over fails silently at the ceiling test
# itself. SNR-V0-007's Core base is HEAD with the candidate's two source trees excluded; this
# runs the script's own archive command against a fixture commit and checks what it exports.
# SNR-V0-013's abstention branch is gated by a plain `git cat-file -e <rev>^{commit}` predicate;
# this proves that predicate is false for the pinned causal_parent (confirmed absent from this
# repository, "Verified current state") and true for a commit that exists. The toolchain-identity,
# build, and benchmark ratchets (SNR-V0-003, 004, 006-012, 014) and the Core binary comparison are exercised only by a real
# invocation (`make sql-native-ratchets`): they rebuild cmd/corvint twice and run 30 benchmark
# samples, which this fast harness must not do.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-sql-native-ratchets-test.XXXXXX")
spin_pid=
cleanup() {
	[ -z "$spin_pid" ] || kill "$spin_pid" 2>/dev/null || :
	rm -rf "$test_root"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

# Requirements SNR-V0-003/SNR-V0-004 pin toolchain identity by content: the source-byte check
# (SNR-V0-005) is reached only after that pin matches, so this harness needs the exact pinned
# go1.27.0 install, not whatever default `go` is first on PATH.
PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH
export PATH

repo="$test_root/repo"
mkdir -p "$repo/script" "$repo/experimental/analyzers/sqlnative" "$repo/cmd/corvint-analyzer-sql-native"
cp "$source_root/script/check-sql-native-ratchets.sh" "$repo/script/"
chmod +x "$repo/script/check-sql-native-ratchets.sh"
git -C "$repo" init -q -b main
mkdir -p "$repo/cmd/corvint"
: >"$repo/cmd/corvint/main.go"
: >"$repo/experimental/analyzers/sqlnative/a.go"
: >"$repo/cmd/corvint-analyzer-sql-native/main.go"
git -C "$repo" add cmd experimental
git -C "$repo" -c user.name=test -c user.email=test@example.invalid commit -q -m fixture
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir)

run() { (cd "$repo" && sh script/check-sql-native-ratchets.sh) 2>&1; }
wait_for() {
	i=0
	while eval "$1"; do
		i=$((i + 1))
		[ "$i" -lt 200 ] || fail "$2"
		sleep 0.05
	done
}

# Backgrounding through a `(cd ... && cmd) &` subshell leaves `$!` pointing at the wrong process
# under a non-interactive `sh <file>` (no job control): `cd` in the current shell instead, so the
# backgrounded command is the direct child and `$!` names it correctly.
start_dir=$(pwd)

# 1. SNR-V0-001: a lock already held refuses with exit 75 and reason=CONCURRENT_RUN.
mkdir "$git_dir/sql-native-ratchets.lock"
out=$(run) && fail "a held lock allowed a concurrent run"
printf '%s\n' "$out" | grep -q 'status=REJECTED reason=CONCURRENT_RUN' || fail "held lock did not report CONCURRENT_RUN: $out"
rmdir "$git_dir/sql-native-ratchets.lock"

# 2. SNR-V0-002: an unrecognized SQL_NATIVE_RATCHET_LOCK_TEST_HOLD value refuses with exit 64.
out=$( (cd "$repo" && SQL_NATIVE_RATCHET_LOCK_TEST_HOLD=bogus sh script/check-sql-native-ratchets.sh) 2>&1) \
	&& fail "an invalid lock-test value ran"
printf '%s\n' "$out" | grep -q 'status=REJECTED reason=INVALID_LOCK_TEST' || fail "invalid lock-test value did not report INVALID_LOCK_TEST: $out"

# 3. SNR-V0-002 `spin` holds the lock for a companion SNR-V0-001 CONCURRENT_RUN observation, and
# SNR-V0-001's trap releases the lock on TERM.
cd "$repo"
SQL_NATIVE_RATCHET_LOCK_TEST_HOLD=spin sh script/check-sql-native-ratchets.sh &
spin_pid=$!
cd "$start_dir"
# shellcheck disable=SC2016
wait_for '[ ! -d "$git_dir/sql-native-ratchets.lock" ]' "spin mode never took the lock"
out=$(run) && fail "a spinning holder allowed a concurrent run"
printf '%s\n' "$out" | grep -q 'status=REJECTED reason=CONCURRENT_RUN' || fail "concurrent spin holder did not report CONCURRENT_RUN: $out"
kill -TERM "$spin_pid"
wait "$spin_pid" 2>/dev/null || :
spin_pid=
# shellcheck disable=SC2016
wait_for '[ -d "$git_dir/sql-native-ratchets.lock" ]' "TERM did not release the lock"

# 4. SNR-V0-005: source bytes at the ceiling proceed past the check (and fail later, at the
# toolchain receipt build, which finds no main module); one byte over fails at the ceiling test itself,
# before any git archive or build is attempted.
analyzer_go="$repo/experimental/analyzers/sqlnative/a.go"
command_go="$repo/cmd/corvint-analyzer-sql-native/main.go"
fill() { head -c "$1" /dev/zero >"$2"; }
fill 32768 "$analyzer_go"
fill 32768 "$command_go"
out=$(run) && fail "a fixture with no go.mod produced success"
printf '%s\n' "$out" | grep -q 'cannot find main module' || fail "at-ceiling source did not proceed to the receipt build: $out"

fill 32769 "$analyzer_go"
out=$(run) && fail "over-ceiling source produced success"
[ -z "$out" ] || fail "over-ceiling source proceeded past its own check: $out"
fill 32768 "$analyzer_go"

# 5. SNR-V0-013: the abstention branch is gated by `git cat-file -e "$causal_parent^{commit}"`.
# Prove the predicate is false for the pinned causal_parent (confirmed absent from this
# repository) and true for a commit that exists, so a future edit that breaks the guard, or a
# future rebase that makes the pin resolve, is caught here.
# shellcheck disable=SC2016
grep -q 'git cat-file -e "\$causal_parent\^{commit}"' "$repo/script/check-sql-native-ratchets.sh" \
	|| fail "the causal_parent presence guard moved or changed; update this test"
pinned_causal_parent=$(sed -n 's/^causal_parent=//p' "$repo/script/check-sql-native-ratchets.sh")
git -C "$repo" cat-file -e "$pinned_causal_parent^{commit}" 2>/dev/null \
	&& fail "the pinned causal_parent unexpectedly resolved in a fresh fixture repository"
present=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" cat-file -e "$present^{commit}" 2>/dev/null \
	|| fail "a real commit was not detected as present by the same guard"

# 6. SNR-V0-007: the Core base is HEAD with exactly the candidate's two source trees excluded.
# shellcheck disable=SC2016
archive_line=$(grep '^git archive "\$core_base" -- ' "$repo/script/check-sql-native-ratchets.sh") \
	|| fail "the Core-base export moved or changed; update this test"
archive_args=${archive_line#git archive \"\$core_base\" }
archive_args=${archive_args% > *}
listing=$(cd "$repo" && eval "git archive HEAD $archive_args" | tar -t)
printf '%s\n' "$listing" | grep -qx 'cmd/corvint/main.go' || fail "the Core base omitted Core source: $listing"
printf '%s\n' "$listing" | grep -q 'sqlnative\|corvint-analyzer-sql-native' && fail "the Core base kept candidate source: $listing"

echo "check-sql-native-ratchets_test.sh: ok"
