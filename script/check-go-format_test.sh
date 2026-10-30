#!/bin/sh
set -eu

# Five cases, on a fixture repository: a clean tracked set passes, one unformatted
# tracked file fails and is named, an unformatted file that is not tracked is ignored,
# and a conflicted index is refused with a named message rather than a raw `git
# write-tree` plumbing error. The third case is why a scratch tree full of unformatted
# Go does not make the gate unrunnable; the fourth is why a merge in progress does not
# make it fail unreadably instead of cleanly; the fifth is why a failed `git ls-files` is not
# reported as an empty, clean tracked set.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-go-format-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/pkg"
cp "$source_root/script/check-go-format.sh" "$test_root/script/"
printf 'module example.test/fixture\n\ngo 1.27.0\n' > "$test_root/go.mod"

formatted() { printf 'package pkg\n\nfunc %s() int { return 1 }\n' "$1"; }
unformatted() { printf 'package pkg\n\nfunc %s() int {\nreturn 1\n}\n' "$1"; }

git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-go-format.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

# 1. A clean tracked set passes and says nothing.
formatted Clean > "$test_root/pkg/clean.go"
git -C "$test_root" add -A
output=$(run) || fail "a gofmt-clean tracked set did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 2. One unformatted tracked file fails, and the failure names it.
unformatted Ragged > "$test_root/pkg/ragged.go"
git -C "$test_root" add -A
if output=$(run); then
    fail "an unformatted tracked file passed the gate"
fi
case $output in
    *pkg/ragged.go*) ;;
    *) fail "the failure did not name the unformatted file: $output" ;;
esac
case $output in
    *pkg/clean.go*) fail "the failure named a file that is gofmt-clean: $output" ;;
esac

# 3. An unformatted file outside the tracked set is not the gate's business.
git -C "$test_root" rm -q --cached pkg/ragged.go
mkdir -p "$test_root/.scratch"
unformatted Scratch > "$test_root/.scratch/scratch.go"
output=$(run) || fail "untracked unformatted Go failed the gate: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 4. A conflicted index is refused with a named message, not a raw `git write-tree` error.
git -C "$test_root" commit -qm base
git -C "$test_root" branch -q side
git -C "$test_root" checkout -q side
formatted SideChange > "$test_root/pkg/clean.go"
git -C "$test_root" commit -qam side-change
git -C "$test_root" checkout -q -
formatted MainChange > "$test_root/pkg/clean.go"
git -C "$test_root" commit -qam main-change
git -C "$test_root" merge -q side >/dev/null 2>&1 || true
if output=$(run); then
    fail "a conflicted index passed the gate"
fi
case $output in
    *unmerged*) ;;
    *) fail "the failure did not name the unmerged index: $output" ;;
esac
case $output in
    *write-tree*) fail "the failure leaked a raw git write-tree error: $output" ;;
esac

# 5. A failed tracked-file enumeration is a gate failure, never an empty (clean) tracked set.
git -C "$test_root" merge --abort
unformatted Ragged > "$test_root/pkg/ragged.go"
git -C "$test_root" add -A
mkdir -p "$test_root/fakegit"
real_git=$(command -v git)
# shellcheck disable=SC2016
printf '#!/bin/sh\n[ "$3 $4" = "ls-files -z" ] && exit 128\nexec "%s" "$@"\n' "$real_git" \
    > "$test_root/fakegit/git"
chmod +x "$test_root/fakegit/git"
status=0
output=$(cd "$test_root" && PATH="$test_root/fakegit:$PATH" script/check-go-format.sh 2>&1) || status=$?
[ "$status" -ne 0 ] || fail "a failing git ls-files passed the gate: $output"

printf 'check-go-format_test: 5 cases passed\n'
