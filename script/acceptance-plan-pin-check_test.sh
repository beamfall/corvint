#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# ACR-V1-001: tests for script/acceptance-plan-pin-check.sh over a temporary repository.

here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
check=$here/acceptance-plan-pin-check.sh
root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-plan-pin-test.XXXXXX")
trap 'rm -rf "$root"' EXIT INT TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

mkdir -p "$root/docs/specs" "$root/docs/decisions"
plan=$root/docs/specs/PRODUCT-PLAN-1.0.md
record=docs/decisions/0001-accept.md
git -C "$root" init -q
write_plan() {
  printf '%s\n' '# Plan' '' \
    '- `PROD-A-001`: (proposed 2026-11-09) Corvint MUST name the tests' \
    '  a change requires.' \
    'See `docs/decisions/PROPOSED-accept-product-plan-1.0.md` and (`AGENTS.md:176@b8bc96f4`).' \
    > "$plan"
}
write_plan
blob=$(git -C "$root" hash-object -w "$plan")
printf -- '- Accepts: the plan at commit `x`,\n  where the plan file is blob `%s`. More.\n' "$blob" \
  > "$root/$record"

# 1. The pinned text passes.
out=$("$check" "$root" "$record") || fail "the pinned plan exited $?: $out"

# 2. A new acceptance marker, a re-wrapped line, a moved citation line and the record's own
# path are no difference.
printf '%s\n' '# Plan' '' \
  '- `PROD-A-001`: (accepted, decision 0001; V1-0001) Corvint MUST name' \
  '  the tests a change requires.' \
  "See \`$record\` and (\`AGENTS.md:155-160@b8bc96f4\`)." > "$plan"
out=$("$check" "$root" "$record") || fail "an ignored edit exited $?: $out"

# 3. A changed word, a changed citation hash and a marker of another form are each stale.
for edit in 's/MUST name/SHOULD name/' 's/@b8bc96f4/@b8bc96f5/' 's/(proposed 2026-11-09)/(accepted)/'; do
  write_plan
  sed "$edit" "$plan" > "$plan.new" && mv "$plan.new" "$plan"
  status=0
  out=$("$check" "$root" "$record") || status=$?
  test "$status" = 1 || fail "edit $edit exited $status, want 1"
  case $out in
    "STALE_PLAN	$record	blob $blob"*) ;;
    *) fail "edit $edit printed: $out" ;;
  esac
done

# 4. A record with no blob, and a blob the clone does not hold, exit 2.
write_plan
printf 'no pin here\n' > "$root/$record"
status=0
"$check" "$root" "$record" > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "a record with no blob exited $status, want 2"
printf 'the plan file is blob `%s`.\n' 0123456789012345678901234567890123456789 > "$root/$record"
status=0
"$check" "$root" "$record" > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "an absent blob exited $status, want 2"

printf 'ok\n'
