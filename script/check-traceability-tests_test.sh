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

# A traceability section closes only at a heading at or above its opening depth; a deeper heading,
# including a repeated deeper traceability heading, keeps it open (TTG-V0-003).
# shellcheck disable=SC2016
row='| FIX-V0-002 | `TestNeverDeclared` |'
printf '%s\n' '# Control' '' '## Traceability' '' "$row" > "$test_root/docs/specs/control.md"
printf '%s\n' '# Nested' '' '## Traceability' '' '### Nested traceability' '' '### Other witnesses' '' \
  "$row" '' '### Traceability again' '' '#### Deeper still' '' "$row" > "$test_root/docs/specs/nested.md"
printf '%s\n' '# Closed' '' '## Traceability' '' '### Nested traceability' '' '## Outside at its depth' '' \
  "$row" '' '### Traceability' '' '# Outside above its depth' '' "$row" > "$test_root/docs/specs/closed.md"
stage docs/specs
output=$(run) && fail "an undeclared test function in a traceability section passed"
for expected in control.md:5 nested.md:9 nested.md:15; do
  case "$output" in
    *"docs/specs/$expected: TestNeverDeclared"*) ;;
    *) fail "docs/specs/$expected was not reported: $output" ;;
  esac
done
case "$output" in
  *closed.md*) fail "a row after its traceability section closed was inspected: $output" ;;
esac

echo "check-traceability-tests_test.sh: ok"
