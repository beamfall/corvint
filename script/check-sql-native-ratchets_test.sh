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
# samples, which this fast harness must not do. Case 9 builds only the small receipt tool to check
# the host toolchain's pin row (SNR-V0-031).

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

# SNR-V0-003/SNR-V0-004 pin toolchain identity by content, and the source-byte check (SNR-V0-005)
# is reached only after the pin matches. The fixture copy of the script therefore carries one pin
# row for whatever toolchain this host discovers (SNR-V0-030), so the control-flow cases below do
# not depend on which admitted installation the host has; case 7 proves that an unpinned toolchain
# refuses with its typed reason, and case 8 that every real pin names a version the Makefile admits.
host_go_root=$(GOTOOLCHAIN=local go env GOROOT)
host_go_version=$(GOTOOLCHAIN=local "$host_go_root/bin/go" env GOVERSION)
host_go_sha=$(shasum -a 256 "$host_go_root/bin/go" | awk '{print $1}')

repo="$test_root/repo"
mkdir -p "$repo/script" "$repo/experimental/analyzers/sqlnative" "$repo/cmd/corvint-analyzer-sql-native"
# set_pins replaces the fixture script's pin table with the single row $1.
set_pins() {
	awk -v row="$1" '
		skip && /^'"'"'$/ { skip = 0; print row; print; next }
		skip { next }
		/^toolchain_pins='"'"'$/ { skip = 1 }
		{ print }
	' "$source_root/script/check-sql-native-ratchets.sh" >"$repo/script/check-sql-native-ratchets.sh"
	grep -qx "$1" "$repo/script/check-sql-native-ratchets.sh" || fail "the pin table moved or changed; update this test"
	chmod +x "$repo/script/check-sql-native-ratchets.sh"
}
zero=0000000000000000000000000000000000000000000000000000000000000000
set_pins "$host_go_version $host_go_sha $zero 0 $zero"
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

# 7. SNR-V0-030: a toolchain that matches no pin row (same version, other go binary digest)
# refuses with exactly one typed reason line and exit 78, before the source-byte check or any build.
set_pins "$host_go_version $zero $zero 0 $zero"
status=0
out=$(run) || status=$?
[ "$status" -eq 78 ] || fail "an unpinned go binary exited $status, want 78: $out"
[ "$(printf '%s\n' "$out" | grep -c 'status=REJECTED reason=TOOLCHAIN_MISMATCH')" -eq 1 ] \
	|| fail "an unpinned go binary did not report one TOOLCHAIN_MISMATCH line: $out"
printf '%s\n' "$out" | grep -q "detail=unpinned-$host_go_version-$host_go_sha" \
	|| fail "the TOOLCHAIN_MISMATCH line did not name the discovered toolchain: $out"
[ -d "$git_dir/sql-native-ratchets.lock" ] && fail "a toolchain refusal left the lock held"

# 8. SNR-V0-030: every real pin row names a Go version the Makefile go-version target admits.
admitted=$(sed -n '/^go-version:/,/^$/p' "$source_root/Makefile" | grep -o 'go1\.[0-9]*\.[0-9]*' | sort -u)
[ -n "$admitted" ] || fail "the Makefile go-version target moved or changed; update this test"
pinned=$(awk '/^toolchain_pins='"'"'$/{p=1;next} p&&/^'"'"'$/{exit} p&&NF{print $1}' "$source_root/script/check-sql-native-ratchets.sh")
[ -n "$pinned" ] || fail "the real pin table is empty or moved"
for version in $pinned; do
	printf '%s\n' "$admitted" | grep -qx "$version" || fail "pin $version is not admitted by the Makefile go-version target"
done

# 9. SNR-V0-031: no two real pin rows share a version and go binary digest, and a host whose GOROOT
# is the official golang.org/toolchain module of go1.27.3 for linux/amd64 matches a real row in
# all three digests, the receipt tool built with the gate's own flags. Any other host toolchain
# that matches a row is checked the same way; one that matches none is reported, not failed.
rows=$(awk '/^toolchain_pins='"'"'$/{p=1;next} p&&/^'"'"'$/{exit} p&&NF{print}' "$source_root/script/check-sql-native-ratchets.sh")
[ -z "$(printf '%s\n' "$rows" | awk '{print $1, $2}' | sort | uniq -d)" ] || fail "two pin rows share a version and go binary digest"
row=$(printf '%s\n' "$rows" | awk -v v="$host_go_version" -v d="$host_go_sha" '$1==v&&$2==d{print;exit}')
case $host_go_root in
*/golang.org/toolchain@v0.0.1-go1.27.3.linux-amd64)
	[ -n "$row" ] || fail "the official go1.27.3 linux/amd64 toolchain module at $host_go_root ($host_go_sha) has no pin row"
	;;
esac
if [ -n "$row" ]; then
	# shellcheck disable=SC2086
	set -- $row
	state="$test_root/receipt"
	mkdir -p "$state/home" "$state/gopath" "$state/modcache" "$state/cache"
	(cd "$source_root" && env -i PATH=/usr/bin:/bin HOME="$state/home" GOPATH="$state/gopath" GOMODCACHE="$state/modcache" GOCACHE="$state/cache" GOROOT="$host_go_root" GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local CGO_ENABLED=0 GOMAXPROCS=1 "$host_go_root/bin/go" build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$state/toolchain-receipt" ./cmd/corvint-go-toolchain-receipt) \
		|| fail "the receipt tool did not build on the host toolchain"
	[ "$(shasum -a 256 "$state/toolchain-receipt" | awk '{print $1}')" = "$3" ] || fail "the host toolchain's receipt tool digest differs from its pin row: $row"
	[ "$("$state/toolchain-receipt" "$host_go_root")" = "{\"Entries\":$4,\"SHA256\":\"$5\"}" ] || fail "the host GOROOT receipt differs from its pin row: $row"
	echo "check-sql-native-ratchets_test.sh: host toolchain $host_go_version $host_go_sha matches its pin row"
else
	echo "check-sql-native-ratchets_test.sh: host toolchain $host_go_version $host_go_sha has no pin row (NOT_OBSERVED)"
fi

echo "check-sql-native-ratchets_test.sh: ok"
