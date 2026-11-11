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
  *"docs/specs/fixture.md:7: TestGhostOnlyOnDisk (declared only in the unstaged working-tree file internal/demo/ghost_test.go; the check reads the index, so stage it: git add -- internal/demo/ghost_test.go)"*) ;;
  *) fail "the unresolved function was not reported with its unstaged file (TTG-V0-012): $output" ;;
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
  *"declared only in the unstaged"*) fail "a never-declared function was named as unstaged: $output" ;;
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

# TTG-V0-917 (V1-4048): a backticked word of the test-name shape followed by `(not a test)`, such as
# a fixture name, is not checked; the same word without the marker on the next row still fails.
# shellcheck disable=SC2016
printf '%s\n' '# Prose' '' '## Traceability' '' \
  '| FIX-V0-009 | the fixture `TestLine` (not a test) and TestCol (not a test) | `TestGhostOnlyOnDisk` |' \
  > "$test_root/docs/specs/prose.md"
stage docs/specs/prose.md
run >/dev/null || fail "a word marked (not a test) was checked as a test function: $(run)"
# shellcheck disable=SC2016
printf '%s\n' '| FIX-V0-010 | the fixture `TestLine` |' >> "$test_root/docs/specs/prose.md"
stage docs/specs/prose.md
output=$(run) && fail "an unmarked fixture word passed"
case "$output" in
  *"docs/specs/prose.md:6: TestLine"*) ;;
  *) fail "the unmarked fixture word was not reported: $output" ;;
esac
case "$output" in *TestCol*) fail "the marked word was reported: $output" ;; esac
git -C "$test_root" rm -q --cached -- docs/specs/prose.md
/bin/rm -f "$test_root/docs/specs/prose.md"

# TTG-V0-951: a spec with a `Delivery status: retired` header is not checked and is counted; the
# same body under another status is checked.
# shellcheck disable=SC2016
printf '%s\n' '# Retired' '' 'Delivery status: retired' '' '## Traceability' '' \
  '| FIX-V0-011 | `TestLeftWithItsCode` |' > "$test_root/docs/specs/retired.md"
stage docs/specs/retired.md
output=$(run) || fail "a retired spec was checked (TTG-V0-951): $output"
case "$output" in
  *"1 retired spec(s) not checked"*) ;;
  *) fail "the retired spec was not counted: $output" ;;
esac
# shellcheck disable=SC2016
printf '%s\n' '# Retired' '' 'Delivery status: implemented' '' '## Traceability' '' \
  '| FIX-V0-011 | `TestLeftWithItsCode` |' > "$test_root/docs/specs/retired.md"
stage docs/specs/retired.md
output=$(run) && fail "an undeclared name in a spec that is not retired passed"
case "$output" in
  *"docs/specs/retired.md:7: TestLeftWithItsCode"*) ;;
  *) fail "the live spec row was not reported: $output" ;;
esac

# TTG-V0-952: `(removed)` after an undeclared name passes and is counted; after a declared name it
# is refused.
# shellcheck disable=SC2016
printf '%s\n' '# Retired' '' 'Delivery status: implemented' '' '## Traceability' '' \
  '| FIX-V0-011 | `TestLeftWithItsCode` (removed) |' > "$test_root/docs/specs/retired.md"
stage docs/specs/retired.md
output=$(run) || fail "a name marked (removed) was reported (TTG-V0-952): $output"
case "$output" in
  *"1 marked removed"*) ;;
  *) fail "the removed marker was not counted: $output" ;;
esac
# shellcheck disable=SC2016
printf '%s\n' '| FIX-V0-012 | `TestGhostOnlyOnDisk` (removed) |' >> "$test_root/docs/specs/retired.md"
stage docs/specs/retired.md
output=$(run) && fail "a declared test marked (removed) passed"
case "$output" in
  *"docs/specs/retired.md:8: TestGhostOnlyOnDisk is marked (removed) but is declared; drop the marker"*) ;;
  *) fail "the declared test marked (removed) was not named: $output" ;;
esac
git -C "$test_root" rm -q --cached -- docs/specs/retired.md
/bin/rm -f "$test_root/docs/specs/retired.md"

# TTG-V0-012 (V1-3235): a declaration added to a tracked file but not staged is named with its file
# and the staging fix; staging it resolves the row.
printf '%s\n' 'func TestAddedUnstaged() {}' >> "$test_root/internal/demo/ghost_test.go"
# shellcheck disable=SC2016
printf '%s\n' '# Added' '' '## Traceability' '' '| FIX-V0-006 | `TestAddedUnstaged` |' > "$test_root/docs/specs/added.md"
stage docs/specs/added.md
output=$(run) && fail "an unstaged edit satisfied a staged traceability row"
case "$output" in
  *"docs/specs/added.md:5: TestAddedUnstaged (declared only in the unstaged working-tree file internal/demo/ghost_test.go;"*) ;;
  *) fail "the unstaged edit was not named: $output" ;;
esac
stage internal/demo/ghost_test.go
run >/dev/null || fail "the staged edit did not satisfy the row: $(run)"

# TTG-V0-960, TTG-V0-961 (V1-4803): a citation `TestName/REQ-ID` resolves only when a declaration of
# TestName holds REQ-ID in its body, from its `func` line to the next column-0 `}`. A doc comment, a
# later function and a longer ID do not count; a second declaration in another file does. A suffix
# that is not a requirement ID, and a name the existing rules exempt, are checked as before.
printf '%s\n' 'package demo' '' '// TestSub covers REQ-9 only in this doc comment.' 'func TestSub() {' \
  '	_ = "REQ-1_case"' '}' '' 'func TestLonger() {' '	_ = "REQ-12 and REQ-7"' '}' '' \
  'func TestOne(t *testing.T) { t.Skip() }' '' 'func TestAfterOne() {' '	_ = "REQ-31"' '}' '' \
  'func TestOneHolds(t *testing.T) { _ = "REQ-32" }' > "$test_root/internal/demo/sub_test.go"
# A brace inside a string, rune or comment on the declaration line opens and closes nothing: a
# one-line test holding a quoted `{` does not take the next function as its body, and a declaration
# line whose only closing brace is quoted is not a one-line test (TTG-V0-961).
printf '%s\n' 'package demo' '' 'func TestQuoteOpen(t *testing.T) { _ = "{" }' '' \
  'func TestAfterQuote() {' '	_ = "REQ-34"' '}' '' "func TestQuoteClose(t *testing.T) { r := '}' // }" \
  '	_, _ = r, "REQ-35"' '}' > "$test_root/internal/demo/quote_test.go"
# A column-0 `}` inside a raw string ends the body lexically (TTG-V0-961 failure mode); the refusal
# says where the body was read to.
printf '%s\n' 'package demo' '' 'func TestRaw() {' '	_ = `' '}' '`' '	_ = "REQ-33"' '}' \
  > "$test_root/internal/demo/raw_test.go"
# shellcheck disable=SC2016
printf '%s\n' '# Sub' '' '## Traceability' '' '| Requirement | Evidence |' '|---|---|' \
  '| REQ-1 | `TestSub/REQ-1` resolves |' \
  '| FIX-V0-013 | `TestSub/happy_path`, `pkg.TestSub/REQ-404`, `TestSubPlanned` (PLANNED), `TestSubWord` (not a test), `TestSubGone` (removed) |' \
  > "$test_root/docs/specs/sub.md"
stage internal/demo/sub_test.go
stage internal/demo/raw_test.go
stage internal/demo/quote_test.go
stage docs/specs/sub.md
output=$(run) || fail "a subtest citation whose body holds the ID failed (TTG-V0-960): $output"
case "$output" in
  *"traceability tests: 1 planned"*"traceability subtest citations: 1 checked"*) ;;
  *) fail "the subtest citation was not counted once after the unchanged summary line (TTG-V0-963): $output" ;;
esac
# shellcheck disable=SC2016
printf '%s\n' '| REQ-1 | `TestLonger/REQ-1` holds only REQ-12 |' '| REQ-9 | `TestSub/REQ-9` |' \
  '| REQ-7 | `TestSub/REQ-7` |' '| REQ-8 | `TestNoSuchSub/REQ-8` |' '| REQ-31 | `TestOne/REQ-31` |' \
  '| REQ-32 | `TestOneHolds/REQ-32` |' '| REQ-33 | `TestRaw/REQ-33` |' \
  '| REQ-34 | `TestQuoteOpen/REQ-34` |' '| REQ-35 | `TestQuoteClose/REQ-35` |' >> "$test_root/docs/specs/sub.md"
stage docs/specs/sub.md
output=$(run) && fail "a subtest citation whose body lacks the ID passed (TTG-V0-961)"
for expected in \
  'docs/specs/sub.md:9: TestLonger/REQ-1 (no declaration of TestLonger contains REQ-1; each body was read from its func line to the first column-0 }: internal/demo/sub_test.go)' \
  'docs/specs/sub.md:10: TestSub/REQ-9 (no declaration of TestSub contains REQ-9; each body was read from its func line to the first column-0 }: internal/demo/sub_test.go)' \
  'docs/specs/sub.md:11: TestSub/REQ-7 (no declaration of TestSub contains REQ-7; each body was read from its func line to the first column-0 }: internal/demo/sub_test.go)' \
  'docs/specs/sub.md:13: TestOne/REQ-31 (no declaration of TestOne contains REQ-31; each body was read from its func line to the first column-0 }: internal/demo/sub_test.go)' \
  'docs/specs/sub.md:15: TestRaw/REQ-33 (no declaration of TestRaw contains REQ-33; each body was read from its func line to the first column-0 }: internal/demo/raw_test.go)' \
  'docs/specs/sub.md:16: TestQuoteOpen/REQ-34 (no declaration of TestQuoteOpen contains REQ-34; each body was read from its func line to the first column-0 }: internal/demo/quote_test.go)' \
  'docs/specs/sub.md:12: TestNoSuchSub
'; do
  case "$output
" in
    *"  $expected"*) ;;
    *) fail "the row was not reported as: $expected: $output" ;;
  esac
done
case "$output" in *"TestNoSuchSub/REQ-8"*) fail "an undeclared name got a second, subtest failure: $output" ;; esac
case "$output" in *TestOneHolds*) fail "a one-line declaration holding the ID was refused: $output" ;; esac
case "$output" in *TestQuoteClose*) fail "a declaration whose only closing brace is quoted was read as a one-line test: $output" ;; esac
case "$output" in *"traceability subtest citations: 9 checked"*) ;; *) fail "the subtest citations were not counted: $output" ;; esac
printf '%s\n' '//go:build other' '' 'package demo' '' 'func TestSub() {' '	_ = "REQ-7"' '}' \
  > "$test_root/internal/demo/sub_other_test.go"
stage internal/demo/sub_other_test.go
output=$(run) && fail "the remaining subtest failures passed"
case "$output" in *"TestSub/REQ-7"*) fail "a second declaration holding the ID did not resolve the citation: $output" ;; esac
for expected in TestLonger/REQ-1 TestOne/REQ-31 TestRaw/REQ-33; do
  case "$output" in *"$expected"*) ;; *) fail "the $expected row stopped failing: $output" ;; esac
done
git -C "$test_root" rm -q --cached -- docs/specs/sub.md
/bin/rm -f "$test_root/docs/specs/sub.md"

# TTG-V0-950 (V1-3586): a spec-first commit citing a test that the next commit adds fails the index
# check with the range hint, and passes --range BASE..HEAD, which resolves every commit's rows
# against the change head; a cited test that never lands fails at the head and is named once, at
# the first commit that cites it.
commit() { git -C "$test_root" -c user.name=t -c user.email=t@example.invalid commit -q -m "$1"; }
commit base
base=$(git -C "$test_root" rev-parse HEAD)
# shellcheck disable=SC2016
printf '%s\n' '# Later' '' '## Traceability' '' '| FIX-V0-007 | `TestLandsNext` |' > "$test_root/docs/specs/later.md"
stage docs/specs/later.md
output=$(run) && fail "the spec-only commit passed the index check without its test"
case "$output" in
  *"docs/specs/later.md:5: TestLandsNext"*"TRACE_RANGE=BASE..HEAD"*"(TTG-V0-950)"*) ;;
  *) fail "the spec-only failure lacks the range hint: $output" ;;
esac
commit spec
spec_commit=$(git -C "$test_root" rev-parse HEAD)
printf '%s\n' 'package demo' '' 'func TestLandsNext() {}' > "$test_root/internal/demo/later_test.go"
stage internal/demo/later_test.go
commit test
range() { (cd "$test_root" && script/check-traceability-tests.sh --range "$1" 2>&1); }
range "$base..HEAD" > /dev/null || fail "a test added by the next commit failed the range check: $(range "$base..HEAD")"
(cd "$test_root" && script/check-traceability-tests.sh --range "$base..$spec_commit") > /dev/null 2>&1 &&
  fail "a range ending before the test landed passed"
# shellcheck disable=SC2016
printf '%s\n' '# Never' '' '## Traceability' '' '| FIX-V0-008 | `TestNeverLands` |' > "$test_root/docs/specs/never.md"
stage docs/specs/never.md
commit never
never_commit=$(git -C "$test_root" rev-parse HEAD)
printf 'x\n' > "$test_root/internal/demo/note.txt"
stage internal/demo/note.txt
commit unrelated
output=$(range "$base..HEAD") && fail "a cited test that never lands passed the range check"
case "$output" in
  *"$(printf '%.12s' "$never_commit") docs/specs/never.md:5: TestNeverLands"*) ;;
  *) fail "the never-landing test was not named at its first commit: $output" ;;
esac
test "$(printf '%s\n' "$output" | grep -c TestNeverLands)" = 1 || fail "the never-landing row was named more than once: $output"
case "$output" in *TestLandsNext*) fail "the landed test was reported: $output" ;; esac
output=$(range "$base") && fail "a range without .. was accepted"
case "$output" in *"--range must be BASE..HEAD"*) ;; *) fail "the malformed range was not named: $output" ;; esac

# TTG-V0-962, TTG-V0-963 (V1-4803): the range form compares BASE's tree with HEAD's and fails for a
# requirement whose last test citation the range removed, naming its citation at BASE. A second
# citation, a retired spec, a requirement no longer stated, or `(untested: REASON)` with a reason
# passes; an empty reason does not. The index form does not apply the rule.
# A retired spec exempts its requirement only when every spec that cited it at BASE is retired at
# HEAD and no row of a spec not retired holds the ID.
lc=$test_root/last
mkdir -p "$lc/script" "$lc/docs/specs" "$lc/internal/demo"
cp "$source_root/script/check-traceability-tests.sh" "$lc/script/"
git -C "$lc" init -q -b main
lc_commit() { git -C "$lc" add -A && git -C "$lc" -c user.name=t -c user.email=t@example.invalid commit -q -m "$1"; }
lc_range() { (cd "$lc" && script/check-traceability-tests.sh --range "$lc_base..HEAD" 2>&1); }
lc_tests() { printf '%s\n' 'package demo' '' "$@" > "$lc/internal/demo/a_test.go"; }
# shellcheck disable=SC2016
lc_spec() {
  { printf '%s\n' '# Req' '' "$1" '' '## Requirements' '' '- `REQ-2`: second.' '- `REQ-3`: third.'
    test -z "$2" || printf '%s\n' "$2"
    printf '%s\n' '' '## Traceability' '' '| Requirement | Evidence |' '|---|---|' "$3" "$4" '| REQ-3 | `TestY` |'
  } > "$lc/docs/specs/req.md"
}
lc_tests 'func TestX() {}' 'func TestY() {}'
# shellcheck disable=SC2016
lc_spec 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | `TestX` |' '| REQ-2 | `TestX`, `TestY` |'
lc_commit base
lc_base=$(git -C "$lc" rev-parse HEAD)
lc_scenario() {
  git -C "$lc" reset -q --hard "$lc_base"
  lc_tests 'func TestY() {}'
  # shellcheck disable=SC2016
  lc_spec "$1" "$2" "$3" '| REQ-2 | `TestY` |'
  lc_commit "$4"
}
# shellcheck disable=SC2016
lc_scenario 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | no test |' 'delete TestX'
output=$(lc_range) && fail "deleting the only citing test of a requirement passed the range form (TTG-V0-962)"
case "$output" in
  *"REQ-1: last test citation removed ($(printf '%.12s' "$lc_base") docs/specs/req.md:15: TestX)"*) ;;
  *) fail "the lost requirement was not named with its citation at BASE: $output" ;;
esac
case "$output" in *REQ-2:*) fail "a requirement that kept a second citation was reported: $output" ;; esac
case "$output" in
  *"traceability last citations: 3 requirements cited at BASE, 0 untested"*) ;;
  *) fail "the last-citation counts were not printed: $output" ;;
esac
(cd "$lc" && script/check-traceability-tests.sh > /dev/null 2>&1) || fail "the index form applied the last-citation rule"
# shellcheck disable=SC2016
lc_scenario 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | (untested: covered by the measured run) |' 'state it'
output=$(lc_range) || fail "a removal stated with (untested: REASON) failed (TTG-V0-963): $output"
case "$output" in
  *"3 requirements cited at BASE, 1 untested"*) ;;
  *) fail "the untested requirement was not counted: $output" ;;
esac
# shellcheck disable=SC2016
lc_scenario 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | (untested: ) |' 'no reason'
output=$(lc_range) && fail "an (untested: ) marker without a reason passed"
case "$output" in *"REQ-1: last test citation removed"*) ;; *) fail "the reasonless marker did not name REQ-1: $output" ;; esac
lc_scenario 'Delivery status: implemented' '' '' 'drop the requirement'
output=$(lc_range) || fail "a requirement no longer stated in any spec was reported: $output"
# shellcheck disable=SC2016
lc_scenario 'Delivery status: retired' '- `REQ-1`: first.' '| REQ-1 | no test |' 'retire the spec'
output=$(lc_range) || fail "a requirement of a retired spec was reported: $output"
# A retired spec's requirement stays exempt when another spec, not retired, still names the ID in
# prose: every spec that cited it at BASE is retired at HEAD and no checked row holds it.
lc_other() { printf '%s\n' '# Other' '' 'Delivery status: implemented' '' "$@" > "$lc/docs/specs/other.md"; }
lc_other 'REQ-1 was removed with its spec.'
lc_commit 'name the retired requirement in prose'
output=$(lc_range) || fail "prose naming a retired spec's requirement was reported (TTG-V0-962): $output"
# A traceability row of a spec that is not retired keeps the requirement alive.
lc_other '## Traceability' '' '| Requirement | Evidence |' '|---|---|' '| REQ-1 | no test |'
lc_commit 'hold the retired requirement in a checked row'
output=$(lc_range) && fail "a checked row holding a retired spec's requirement passed (TTG-V0-962)"
case "$output" in *"REQ-1: last test citation removed"*) ;; *) fail "the row-held requirement was not named: $output" ;; esac
# A second spec that cited the requirement at BASE and is not retired at HEAD keeps the rule.
git -C "$lc" reset -q --hard "$lc_base"
# shellcheck disable=SC2016
lc_other '## Traceability' '' '| Requirement | Evidence |' '|---|---|' '| REQ-1 | `TestX` |'
lc_commit 'second citing spec'
lc_first_base=$lc_base
lc_base=$(git -C "$lc" rev-parse HEAD)
lc_tests 'func TestY() {}'
# shellcheck disable=SC2016
lc_spec 'Delivery status: retired' '- `REQ-1`: first.' '| REQ-1 | no test |' '| REQ-2 | `TestY` |'
lc_other 'REQ-1 lost its row here.'
lc_commit 'retire one of two citing specs'
output=$(lc_range) && fail "a requirement one live spec cited at BASE passed after the other spec retired (TTG-V0-962)"
case "$output" in
  *"REQ-1: last test citation removed ($(printf '%.12s' "$lc_base") docs/specs/other.md:9: TestX)"*) ;;
  *) fail "the requirement of the spec that is not retired was not named: $output" ;;
esac
lc_base=$lc_first_base
# A citation demoted to (PLANNED) does not resolve, so the requirement is no longer cited at HEAD.
# shellcheck disable=SC2016
lc_scenario 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | `TestX` (PLANNED) |' 'demote TestX'
output=$(lc_range) && fail "a requirement whose only citation was demoted to (PLANNED) passed (TTG-V0-962)"
case "$output" in *"REQ-1: last test citation removed"*) ;; *) fail "the demoted requirement was not named: $output" ;; esac
# shellcheck disable=SC2016
lc_scenario 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | `TestX` (PLANNED) (untested: rewritten next) |' 'demote and state'
output=$(lc_range) || fail "a demoted citation stated with (untested: REASON) failed: $output"
case "$output" in *"1 untested"*) ;; *) fail "the demoted, stated requirement was not counted untested: $output" ;; esac
git -C "$lc" reset -q --hard "$lc_base"
lc_tests 'func TestX() {}' 'func TestY() {}' 'func TestZ() {' '	_ = "REQ-3"' '}'
# shellcheck disable=SC2016
lc_spec 'Delivery status: implemented' '- `REQ-1`: first.' '| REQ-1 | `TestX` |' '| REQ-2 | `TestX`, `TestZ/REQ-3` |'
# shellcheck disable=SC2016
printf '%s\n' '- `REQ-4`: fourth, whose BASE citation never resolved.' '| REQ-4 | `TestZ/REQ-4` |' >> "$lc/docs/specs/req.md"
lc_commit 'subtest citation'
lc_base=$(git -C "$lc" rev-parse HEAD)
lc_tests 'func TestX() {}' 'func TestZ() {' '	_ = "REQ-3"' '}'
# shellcheck disable=SC2016
printf '%s\n' '# Req' '' 'Delivery status: implemented' '' '## Requirements' '' '- `REQ-1`: first.' '- `REQ-2`: second.' \
  '- `REQ-3`: third.' '' '## Traceability' '' '| Requirement | Evidence |' '|---|---|' '| REQ-1 | `TestX` |' \
  '| REQ-2 | `TestX`, `TestZ/REQ-3` |' '| REQ-3 | none |' '- `REQ-4`: fourth.' '| REQ-4 | none |' > "$lc/docs/specs/req.md"
lc_commit 'delete TestY'
output=$(lc_range) || fail "a resolving subtest citation did not keep its requirement cited, or a BASE subtest citation that never resolved counted (TTG-V0-962): $output"

# TTG-V0-962: the comparison uses the merge base. A stale sibling BASE that cited REQ-4 with a test
# HEAD never had reports nothing, because HEAD branched before it; a BASE that is not a commit and
# a BASE that shares no history with HEAD are refused with exit 2, naming the revisions.
git -C "$lc" checkout -q -b sibling "$lc_base"
lc_tests 'func TestX() {}' 'func TestY() {}' 'func TestW() {}' 'func TestZ() {' '	_ = "REQ-3"' '}'
# shellcheck disable=SC2016
printf '%s\n' '| REQ-4 | `TestW` |' >> "$lc/docs/specs/req.md"
lc_commit 'sibling cites REQ-4'
sibling=$(git -C "$lc" rev-parse HEAD)
git -C "$lc" checkout -q main
output=$( (cd "$lc" && script/check-traceability-tests.sh --range "$sibling..HEAD" 2>&1) ) ||
  fail "a stale sibling BASE was compared as itself, not at the merge base (TTG-V0-962): $output"
case "$output" in *"REQ-4"*) fail "the sibling's REQ-4 was reported: $output" ;; esac
status=0
output=$( (cd "$lc" && script/check-traceability-tests.sh --range "nosuchrev..HEAD" 2>&1) ) || status=$?
test "$status" = 2 || fail "a BASE that is not a commit did not exit 2 (exit $status): $output"
case "$output" in *"--range base nosuchrev is not a commit"*) ;; *) fail "the unresolved BASE was not named: $output" ;; esac
orphan=$(git -C "$lc" -c user.name=t -c user.email=t@example.invalid commit-tree "$(git -C "$lc" hash-object -t tree /dev/null)" -m orphan)
status=0
output=$( (cd "$lc" && script/check-traceability-tests.sh --range "$orphan..main" 2>&1) ) || status=$?
test "$status" = 2 || fail "a BASE with no merge base did not exit 2 (exit $status): $output"
case "$output" in
  *"base $orphan ($orphan) and head main ($(git -C "$lc" rev-parse main)) have no merge base"*) ;;
  *) fail "the refusal did not name both revisions: $output" ;;
esac

echo "check-traceability-tests_test.sh: ok"
