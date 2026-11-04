#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Covers LVT-V0-003 and LVT-V0-004 (V1-4046): the requirement-ID reservation ledger refuses a
# block another lane reserved, a block already used at the base or defined on an open branch, and a
# whole prefix whose family another lane holds; it is shared by every worktree, releases by owner,
# and fails closed on a malformed ledger or a held lock.
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-requirement-id-reservations-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
repo=$test_root/repo
mkdir -p "$repo/script" "$repo/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$source_root/script/requirement-id-collisions.sh" \
  "$source_root/script/requirement-ids-free.sh" "$source_root/script/requirement-id-reservations.sh" "$repo/script/"
git -C "$repo" init -q -b main
g() { git -C "$repo" -c user.name=t -c user.email=t@example.invalid "$@"; }
define() {
  # shellcheck disable=SC2016
  printf -- '- `%s`: %s\n' "$2" "$3" >> "$repo/docs/specs/$1.md"
  g add -- "docs/specs/$1.md"
  (cd "$repo" && script/gen-spec-requirements.sh > docs/specs/REQUIREMENTS.tsv)
  g add -- docs/specs/REQUIREMENTS.tsv
  g commit -q -m "define $2"
}
g add -- script
define self ABC-V0-101 'a base requirement.'
r() { (cd "${dir:-$repo}" && script/requirement-id-reservations.sh "$@" 2>&1); }
ledger=$(cd "$repo" && git rev-parse --path-format=absolute --git-common-dir)/corvint/requirement-id-reservations.tsv
status() { "$@" > "$test_root/out" 2>&1 && echo 0 || echo $?; }

out=$(r check ABC-V0 200 219) || fail "an absent ledger refused a check: $out"
[[ $out == "unreserved ABC-V0-200..ABC-V0-219 in $ledger" ]] || fail "absent ledger check: $out"
out=$(r reserve ABC-V0 200 219 lane-a --base main) || fail "a free block was not reserved: $out"
[[ $out == *"reserved-ok ABC-V0-200..ABC-V0-219 lane-a in $ledger"* ]] || fail "reserve output: $out"
[[ $(head -1 "$ledger") == '# requirement-id-reservations/1' && $(grep -c . "$ledger") == 2 ]] || fail "ledger rows: $(cat "$ledger")"
out=$(r reserve ABC-V0 200 219 lane-a --base main) || fail "a repeated reserve by its owner failed: $out"
[[ $out == *'already-reserved ABC-V0-200..ABC-V0-219 lane-a'* && $(grep -c . "$ledger") == 2 ]] || fail "repeat reserve: $out"

# Another lane's overlapping block is refused and named; an adjacent one is reserved.
[[ $(status r reserve ABC-V0 210 2300 lane-b --base main) == 1 ]] || fail "an overlapping block was reserved: $(cat "$test_root/out")"
grep -q '^reserved ABC-V0-200..ABC-V0-219 lane-a 20' "$test_root/out" || fail "the overlap was not named: $(cat "$test_root/out")"
out=$(r reserve ABC-V0 2200 2229 lane-b --base main) || fail "a free four-digit block was refused: $out"
# check: any owner's reservation is reserved; --owner skips the caller's own.
[[ $(status r check ABC-V0 205 205) == 1 ]] && grep -q '^reserved ABC-V0-200..ABC-V0-219 lane-a ' "$test_root/out" ||
  fail "check without an owner passed a reserved ID: $(cat "$test_root/out")"
out=$(r check ABC-V0 200 219 --owner lane-a) || fail "the owner's own block was refused: $out"
[[ $(status r check ABC-V0 2229 2240 --owner lane-a) == 1 ]] && grep -q '^reserved ABC-V0-2200..ABC-V0-2229 lane-b ' "$test_root/out" ||
  fail "lane-b's block was not reported to lane-a: $(cat "$test_root/out")"
out=$(r check ABC-V0 220 2199) || fail "a gap between reservations was refused: $out"

# A whole prefix holds its family: any version, any block, and the prefix itself.
out=$(r reserve NEW-V0 lane-c --base main) || fail "a free prefix was not reserved: $out"
[[ $out == *'reserved-ok prefix NEW-V0 lane-c'* ]] || fail "prefix reserve output: $out"
[[ $(status r reserve NEW-V1 1 9 lane-d --base main) == 1 ]] && grep -q '^reserved prefix NEW-V0 lane-c ' "$test_root/out" ||
  fail "a block in a reserved family was reserved: $(cat "$test_root/out")"
[[ $(status r check NEW-V0) == 1 ]] || fail "a reserved prefix passed its check"
[[ $(status r reserve ABC-V1 lane-d --base main) == 1 ]] && grep -q '^reserved ABC-V0-' "$test_root/out" ||
  fail "a prefix whose family holds block reservations was reserved: $(cat "$test_root/out")"

# Fault injection: IDs already used at the base, or defined on an unmerged branch, are not reserved.
[[ $(status r reserve ABC-V0 100 109 lane-d --base main) == 1 ]] && grep -q '^used ABC-V0-101 ' "$test_root/out" ||
  fail "a block used at the base was reserved: $(cat "$test_root/out")"
g checkout -q -b lane/open main
define self ABC-V0-300 'an unmerged clause.'
g checkout -q main
[[ $(status r reserve ABC-V0 300 309 lane-d --base main) == 1 ]] && grep -q '^used ABC-V0-300 lane/open ' "$test_root/out" ||
  fail "a block an open branch defines was reserved: $(cat "$test_root/out")"
[[ $(status r reserve OPN-V0 lane-d --base main) == 0 ]] || fail "a free prefix was refused: $(cat "$test_root/out")"
[[ $(grep -c . "$ledger") == 5 ]] || fail "a refused reserve wrote the ledger: $(cat "$ledger")"

# The ledger lives in the common Git directory, so a linked worktree sees and writes the same rows.
g worktree add -q --detach "$test_root/wt" main
out=$(dir=$test_root/wt r list) || fail "list from a worktree: $out"
[[ $(grep -c . <<< "$out") == 4 ]] || fail "the worktree saw another ledger: $out"
[[ $(dir=$test_root/wt status r check ABC-V0 200 200) == 1 ]] || fail "the worktree missed lane-a's reservation"

# release removes only the named owner's rows (only the named block when given).
out=$(r release lane-b ABC-V0 2200 2229) || fail "release of one block failed: $out"
[[ $out == 'released 1' ]] || fail "release output: $out"
out=$(r release lane-a) || fail "release by owner failed: $out"
[[ $out == 'released 1' && $(grep -c . "$ledger") == 3 ]] || fail "release left: $out $(cat "$ledger")"
out=$(r release lane-a) || fail "an empty release failed: $out"
[[ $out == 'released 0' ]] || fail "empty release output: $out"
out=$(r reserve ABC-V0 210 219 lane-b --base main) || fail "a released block was not reservable: $out"

# Fail closed: a held lock and a malformed ledger refuse with exit 2 and a fix.
mkdir "$ledger.lock"
[[ $(status r reserve ABC-V0 400 409 lane-e --base main) == 2 ]] &&
  grep -q 'REFUSE ledger-locked .*rmdir' "$test_root/out" || fail "a held lock did not refuse: $(cat "$test_root/out")"
rmdir "$ledger.lock"
cp "$ledger" "$test_root/ledger.good"
printf 'ABC-V0\t1\n' >> "$ledger"
[[ $(status r check ABC-V0 1 2) == 2 ]] && grep -q 'REFUSE ledger-malformed' "$test_root/out" ||
  fail "a malformed row did not refuse: $(cat "$test_root/out")"
printf 'not a header\n' > "$ledger"
[[ $(status r list) == 2 ]] || fail "a missing header did not refuse"
: > "$ledger"
[[ $(status r list) == 2 ]] || fail "an empty ledger did not refuse"
cp "$test_root/ledger.good" "$ledger"
out=$(REQUIREMENT_ID_RESERVATIONS=$test_root/shared.tsv r check ABC-V0 210 219) || fail "REQUIREMENT_ID_RESERVATIONS did not select another ledger: $out"
for bad in 'reserve abc 1 2 x' 'reserve ABC-V0 5 2 x' 'reserve ABC-V0 1 10000 x' 'reserve ABC-V0 1 2 bad/owner' 'check' 'release' 'bogus'; do
  read -r -a argv <<< "$bad"
  [[ $(status r "${argv[@]}") == 2 ]] || fail "bad input '$bad' did not exit 2: $(cat "$test_root/out")"
done
# Next-free queries skip base definitions, open-ref definitions and live reservations.
out=$(r next ABC-V0 110 --base main) || fail "next block: $out"
[[ $out == *'next-free ABC-V0-301..ABC-V0-410 at '* ]] || fail "next missed a base/open-ref/reservation obstacle: $out"
out=$(r next ABC-V0 110 --base main --owner lane-b) || fail "owner next: $out"
[[ $out == *'next-free ABC-V0-102..ABC-V0-211 at '* ]] || fail "next did not exclude only its owner: $out"
[[ $(status r next NEW-V1 1 --base main) == 1 ]] && grep -q '^no-free-block NEW-V1 size=1 at ' "$test_root/out" || fail "next ignored a whole-family reservation"
[[ $(status r next ABC-V0 10001 --base main) == 2 ]] || fail "next admitted an oversized block"
# A read-only query must neither create nor change the ledger.
cmp "$test_root/ledger.good" "$ledger" || fail "next changed the reservation ledger"
[[ $(status r next ABC-V0 1 --base absent) == 2 ]] || fail "next admitted a missing base"
printf 'malformed\n' >> "$ledger"
[[ $(status r next ABC-V0 1 --base main) == 2 ]] || fail "next admitted a malformed ledger"
cp "$test_root/ledger.good" "$ledger"
# An unreadable/malformed required base index cannot declare a whole family free.
printf 'not an index\n' > "$repo/docs/specs/REQUIREMENTS.tsv"
g add docs/specs/REQUIREMENTS.tsv && g commit -q -m malformed-base
[[ $(status bash -c 'cd "$1" && script/requirement-ids-free.sh FRESH-V0 main' _ "$repo") == 2 ]] &&
  grep -q requirements-index-unavailable "$test_root/out" || fail "a malformed base declared a prefix free: $(cat "$test_root/out")"
[[ $(status r next FRESH-V0 1 --base main) == 2 ]] &&
  grep -q requirements-index-unavailable "$test_root/out" || fail "next admitted a malformed base: $(cat "$test_root/out")"
printf 'requirement-id-reservations_test.sh: ok\n'
