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

# DCW-V0-356 (V1-2855): a wildcard row is refused with the full-name rule and the declared
# functions it would have matched; a prefix matching nothing says so.
/bin/rm -f "$test_root/docs/specs/control.md" "$test_root/docs/specs/nested.md" "$test_root/docs/specs/closed.md"
printf '%s\n' 'package demo' '' 'func TestTCDV0174_A() {}' 'func TestTCDV0174_B() {}' 'func TestOther() {}' \
  > "$test_root/internal/demo/wild_test.go"
# shellcheck disable=SC2016
printf '%s\n' '# Wild' '' '## Traceability' '' '| FIX-V0-003 | `TestTCDV0174_*` |' '| FIX-V0-004 | `TestNoSuch_*` |' \
  > "$test_root/docs/specs/wild.md"
git -C "$test_root" add -A
output=$(run) && fail "a wildcard traceability row passed"
expected='traceability tests: unresolved test functions:
  docs/specs/wild.md:5: TestTCDV0174_* is a wildcard; name each test function in full (2 declared: TestTCDV0174_A, TestTCDV0174_B)
  docs/specs/wild.md:6: TestNoSuch_* is a wildcard; name each test function in full (no declared function has this prefix)
traceability tests: rows must name declared Test functions in full; wildcard and prefix forms are not resolved'
case "$output" in
  *"$expected"*) ;;
  *) fail "the wildcard refusal did not state the full-name rule: $output" ;;
esac
# shellcheck disable=SC2016
printf '%s\n' '# Wild' '' '## Traceability' '' '| FIX-V0-003 | `TestTCDV0174_A`, `TestTCDV0174_B` |' \
  > "$test_root/docs/specs/wild.md"
stage docs/specs/wild.md
run >/dev/null || fail "the row naming each function in full did not pass"

# TTG-V0-011 (V1-3143): a package-qualified Go identifier such as `toolgroup.TestValidity` in a row
# is not a test-function citation; a bare undeclared name on the same row is still reported.
# shellcheck disable=SC2016
printf '%s\n' '# Qualified' '' '## Traceability' '' \
  '| FIX-V0-005 | `internal/toolgroup` type `toolgroup.TestValidity`, (*v).TestRun | `TestOther` |' \
  > "$test_root/docs/specs/qualified.md"
stage docs/specs/qualified.md
run >/dev/null || fail "a package-qualified Test identifier was checked as a test function: $(run)"
# shellcheck disable=SC2016
printf '%s\n' '# Qualified' '' '## Traceability' '' \
  '| FIX-V0-005 | `toolgroup.TestValidity` | `TestValidity` |' > "$test_root/docs/specs/qualified.md"
stage docs/specs/qualified.md
output=$(run) && fail "a bare undeclared name beside a qualified one passed"
case "$output" in
  *"docs/specs/qualified.md:5: TestValidity"*) ;;
  *) fail "the bare undeclared name was not reported: $output" ;;
esac
/bin/rm -f "$test_root/docs/specs/qualified.md"
stage docs/specs/qualified.md

echo "check-traceability-tests_test.sh: ok"
