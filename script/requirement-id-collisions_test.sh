#!/usr/bin/env bash
# Covers ASD-V0-950..952 (V1-2909, V1-1009, V1-3614): the 2026-10-27 ASD-V0-032 and ASD-V0-036
# collisions and the batch C CAL-V0 collision replayed on a synthetic repository, each found from
# local branches and worktrees before either side merges, and a lane block check that sees an
# unmerged branch.
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-requirement-id-collisions-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
repo="$test_root/repo"
mkdir -p "$repo/script" "$repo/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$source_root/script/requirement-id-collisions.sh" "$repo/script/"
git -C "$repo" init -q -b main
g() { git -C "$repo" -c user.name=t -c user.email=t@example.invalid "$@"; }

# define SPEC ID TEXT appends one requirement to SPEC, regenerates the index and commits both.
define() {
  # shellcheck disable=SC2016
  printf -- '- `%s`: %s\n' "$2" "$3" >> "$repo/docs/specs/$1.md"
  g add -- "docs/specs/$1.md"
  (cd "$repo" && script/gen-spec-requirements.sh > docs/specs/REQUIREMENTS.tsv)
  g add -- docs/specs/REQUIREMENTS.tsv
  g commit -q -m "define $2"
}
check() { (cd "$repo" && script/requirement-id-collisions.sh --base main "$@" 2>&1); }

g add -- script
define self ASD-V0-031 'the shared base requirement.'
define cal CAL-V0-185 'the shared base calendar requirement.'

# 2026-10-27: claude/batch-1027f (unpushed) took ASD-V0-036 for V1-2900, then the host-tooling lane
# numbered ASD-V0-036 from origin/main; precheck-base and batch-drift both took ASD-V0-032.
g checkout -q -b claude/batch-1027f main
define self ASD-V0-036 'V1-2900 batch clause.'
g checkout -q -b claude/precheck-base main
define self ASD-V0-032 'precheck-base clause.'
g checkout -q -b claude/batch-drift main
define self ASD-V0-032 'batch-drift clause.'
g checkout -q -b claude/host-tooling main
define self ASD-V0-036 'host-tooling clause.'

output=$(check) && fail "the host-tooling lane's ASD-V0-036 collision passed: $output"
case $output in
  *'collision ASD-V0-036 here docs/specs/self.md "host-tooling clause." vs claude/batch-1027f ('*') docs/specs/self.md "V1-2900 batch clause."'*) ;;
  *) fail "the ASD-V0-036 collision was not named with both sides: $output" ;;
esac
case $output in *ASD-V0-032*) fail "a collision between two other refs was charged to this checkout: $output" ;; esac

# The ASD-V0-032 pair: a worktree checked out on batch-drift sees precheck-base's branch.
g worktree add -q "$test_root/drift" claude/batch-drift
output=$( (cd "$test_root/drift" && script/requirement-id-collisions.sh --base main) 2>&1) &&
  fail "the ASD-V0-032 collision passed: $output"
case $output in
  *'collision ASD-V0-032 here docs/specs/self.md "batch-drift clause." vs claude/precheck-base ('*'"precheck-base clause."'*) ;;
  *) fail "the ASD-V0-032 collision was not named: $output" ;;
esac

# Batch C (V1-1009): two lanes each take the next free CAL-V0 ID. The second lane sees it before
# integration, from its staged spec and before it commits.
g checkout -q -b claude/lane-c1 main
define cal CAL-V0-186 'lane c1 clause.'
# The same row on both sides is one change (a batch that merged a lane) and passes.
g checkout -q -b claude/batch-c claude/lane-c1
define self ASD-V0-045 'a batch-only clause.'
output=$(check) || fail "a batch holding the lane's own row was refused: $output"
g checkout -q -b claude/lane-c2 main
# shellcheck disable=SC2016
printf -- '- `CAL-V0-186`: lane c2 clause.\n' >> "$repo/docs/specs/cal.md"
g add -- docs/specs/cal.md
output=$(check) && fail "the batch C CAL-V0-186 collision passed before the commit: $output"
case $output in
  *'collision CAL-V0-186 here docs/specs/cal.md "lane c2 clause." vs claude/lane-c1 ('*'"lane c1 clause."'*) ;;
  *) fail "the staged CAL-V0-186 collision was not named: $output" ;;
esac
g commit -q -m 'lane c2' && (cd "$repo" && script/gen-spec-requirements.sh > docs/specs/REQUIREMENTS.tsv) &&
  g add -- docs/specs/REQUIREMENTS.tsv && g commit -q -m 'lane c2 index'

# Once lane c1 merges into the base its ID is the base's and no longer new here; lane c2's merge
# then fails the generator's SRG-V0-547 duplicate refusal. A lane whose IDs nobody else defines
# passes.
g branch -q -D claude/batch-c
g checkout -q main
g merge -q --no-ff -m 'merge c1' claude/lane-c1
g checkout -q -b claude/lane-free main
define self ASD-V0-040 'a free clause.'
output=$(check) || fail "a lane with free IDs was refused: $output"
case $output in *'no-collision 1 new IDs here'*) ;; *) fail "the clean summary was not printed: $output" ;; esac

# A branch with no history shared with the base is skipped, not compared.
g checkout -q --orphan claude/unrelated
g rm -q -r --cached docs > /dev/null
rm -rf "$repo/docs/specs"/*
define self ASD-V0-040 'an unrelated lineage clause.'
g checkout -q -f claude/lane-free
output=$(check) || fail "an unrelated lineage was compared: $output"

# ASD-V0-952 (V1-3614): a block check sees IDs an open ref defines; a free block passes.
output=$(check --block ASD-V0 030 039) && fail "a block partly defined on open refs passed: $output"
for want in 'used ASD-V0-032 claude/batch-drift (' 'used ASD-V0-032 claude/precheck-base (' 'used ASD-V0-036 claude/batch-1027f ('; do
  case $output in *"$want"*) ;; *) fail "the block check lacks '$want': $output" ;; esac
done
case $output in *ASD-V0-040*) fail "this checkout's own ID was reported as used by another ref: $output" ;; esac
output=$(check --block ASD-V0 050 059) || fail "a free block was refused: $output"
case $output in *'free ASD-V0-050..ASD-V0-059 on'*) ;; *) fail "the free block line is missing: $output" ;; esac

for bad in '--block ASD-V0 9 1' '--block asd 1 2' '--base no-such-ref' '--bogus'; do
  # shellcheck disable=SC2086
  status=0
  (cd "$repo" && script/requirement-id-collisions.sh $bad) > /dev/null 2>&1 || status=$?
  test "$status" = 2 || fail "bad input '$bad' did not exit 2"
done

printf 'requirement-id-collisions_test.sh: ok\n'
