#!/bin/sh
set -eu

# Four cases, on a fixture repository: a clean `* -text` tree passes, an attribute that escapes
# the repository-wide rule fails and is named, a CRLF-committed blob fails and is named, and the
# two exempt interop fixture paths stay allowed. The last is the one that matters, because those
# two fixtures exist to carry non-LF endings and a check that rejected them would be unrunnable.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-eol-policy-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs"
cp "$source_root/script/check-eol-policy.sh" "$test_root/script/"
printf '* -text\n' > "$test_root/.gitattributes"

git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-eol-policy.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

# 1. A clean tracked set passes.
printf 'one\ntwo\n' > "$test_root/docs/lf.md"
git -C "$test_root" add -A
output=$(run) || fail "a clean -text tree did not pass: $output"
case $output in
    *'tracked paths clean'*) ;;
    *) fail "a passing run did not report the tracked count: $output" ;;
esac

# 2. An attribute that escapes `* -text` fails, and the failure names the path.
printf '* -text\ndocs/*.md text\n' > "$test_root/.gitattributes"
git -C "$test_root" add -A
if output=$(run); then
    fail "a path outside the repository-wide -text rule passed the gate"
fi
case $output in
    *docs/lf.md*) ;;
    *) fail "the failure did not name the path with the drifted attribute: $output" ;;
esac
printf '* -text\n' > "$test_root/.gitattributes"
git -C "$test_root" add -A

# 3. A CRLF-committed blob fails, and the failure names it.
printf 'one\r\ntwo\r\n' > "$test_root/docs/crlf.md"
git -C "$test_root" add -A
if output=$(run); then
    fail "a CRLF-committed blob passed the gate"
fi
case $output in
    *docs/crlf.md*) ;;
    *) fail "the failure did not name the CRLF blob: $output" ;;
esac
case $output in
    *docs/lf.md*) fail "the failure named an LF-clean path: $output" ;;
esac
git -C "$test_root" rm -q --cached docs/crlf.md
rm -f "$test_root/docs/crlf.md"

# 4. The two exempt interop fixtures keep their non-LF endings.
mkdir -p "$test_root/interop/cem-0.1/repository/base/src" "$test_root/interop/cem-0.1/patches"
printf 'one\r\ntwo\r\n' > "$test_root/interop/cem-0.1/repository/base/src/ending.txt"
printf 'one\r\ntwo\n' > "$test_root/interop/cem-0.1/patches/line-ending.patch"
git -C "$test_root" add -A
output=$(run) || fail "the exempt interop fixtures failed the gate: $output"
case $output in
    *'2 exempt fixture(s)'*) ;;
    *) fail "the passing run did not count both exempt fixtures: $output" ;;
esac

printf 'check-eol-policy_test: 4 cases passed\n'
