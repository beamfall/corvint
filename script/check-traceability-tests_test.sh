#!/bin/sh
set -eu

# The gate's test-function set is an index snapshot: an untracked declaration cannot satisfy a
# staged traceability row, while staging that same declaration makes it visible.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-traceability-tests-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/specs" "$test_root/internal/demo"
cp "$source_root/script/check-traceability-tests.sh" "$test_root/script/"
git -C "$test_root" init -q -b main

stage() { git -C "$test_root" -c user.name=t -c user.email=t@example.invalid add -- "$1"; }
run() { (cd "$test_root" && script/check-traceability-tests.sh 2>&1); }
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

# shellcheck disable=SC2016
printf '%s\n' '# Fixture' '' '## Traceability' '' \
  '| Requirement | Evidence |' '|---|---|' '| FIX-V0-001 | `TestGhostOnlyOnDisk` |' \
  > "$test_root/docs/specs/fixture.md"
stage docs/specs/fixture.md

printf '%s\n' 'package demo' '' 'func TestGhostOnlyOnDisk() {}' \
  > "$test_root/internal/demo/ghost_test.go"
output=$(run) && fail "an untracked test function satisfied a staged traceability row"
case "$output" in
  *"docs/specs/fixture.md:7: TestGhostOnlyOnDisk"*) ;;
  *) fail "the unresolved function was not reported: $output" ;;
esac

stage internal/demo/ghost_test.go
run >/dev/null || fail "a staged test function did not satisfy the traceability row"

echo "check-traceability-tests_test.sh: ok"
