#!/usr/bin/env bash
# usage: script/dogfood-closeout-repin.sh BASE CITATIONS
#
# The repin step of make dogfood-closeout (ASD-V0-568, V1-4101). It runs the mechanical repins
# (script/batch-regen.sh --mechanical: host package versions, moved line citations, use-case
# receipts) before batch-precheck, so a citation that only moved because main moved does not fail
# the seal. When the repin changes tracked files on a clean tracked tree, it commits exactly those
# files and refreshes the citation plan with `corvint dogfood propose-citations --refresh`, so the
# bind sees rows for the repin's hunks; the plan it replaced stays beside it as CITATIONS.pre-repin.
# A failed repin, or one on a tree with uncommitted tracked changes, exits 1 with the repin's output
# and the recovery; nothing is committed. A repin that changes nothing exits 0 and changes nothing.
# DOGFOOD_CLOSEOUT_REGEN overrides the repin command (tests); empty skips the step.
set -uo pipefail
base=${1-} citations=${2-}
[[ -n $base ]] || { printf 'usage: script/dogfood-closeout-repin.sh BASE CITATIONS\n' >&2; exit 2; }
regen=${DOGFOOD_CLOSEOUT_REGEN-script/batch-regen.sh --mechanical}
if [[ -z $regen ]]; then
  printf 'dogfood-closeout-repin: skipped (DOGFOOD_CLOSEOUT_REGEN is empty)\n' >&2
  exit 0
fi
tracked_clean() { git diff --quiet HEAD -- && git diff --cached --quiet HEAD --; }
if ! tracked_clean; then
  printf 'dogfood-closeout-repin: skipped: tracked changes are uncommitted; batch-precheck runs the repin and names them\n' >&2
  exit 0
fi
log=$(mktemp "${TMPDIR:-/tmp}/closeout-repin.XXXXXX") || exit 2
trap '/bin/rm -f "$log"' EXIT
# shellcheck disable=SC2086 # the override is a command line
if ! $regen > "$log" 2>&1 < /dev/null; then
  printf 'dogfood-closeout-repin: FAIL the mechanical repin failed; nothing committed:\n' >&2
  tail -n 40 "$log" | sed 's/^/  | /' >&2
  printf 'fix: run script/batch-regen.sh --mechanical, fix what it names, commit, then rerun make dogfood-closeout with the same inputs\n' >&2
  git checkout -q -- . 2> /dev/null
  exit 1
fi
changed=$(git diff --name-only HEAD --)
if [[ -z $changed ]]; then
  printf 'dogfood-closeout-repin: nothing to repin\n' >&2
  exit 0
fi
printf '%s\n' "$changed" | sed 's/^/dogfood-closeout-repin: repinned /' >&2
printf '%s\n' "$changed" | git add --pathspec-from-file=- || exit 1
git commit -q -m 'chore(repin): mechanical repin before the closeout (V1-4101)' || {
  printf 'dogfood-closeout-repin: FAIL could not commit the repin; it is staged\n' >&2
  exit 1
}
printf 'dogfood-closeout-repin: committed %s\n' "$(git rev-parse --short HEAD)" >&2
if [[ -z $citations || ! -s $citations ]]; then
  exit 0
fi
bin=${CORVINT_BIN:-$(git rev-parse --absolute-git-dir)/corvint/corvint}
[[ -x $bin ]] || bin=corvint
if ! "$bin" dogfood propose-citations --refresh "$citations" "$base" > "$log"; then
  printf 'dogfood-closeout-repin: FAIL the plan refresh failed; the repin is committed\n' >&2
  printf 'fix: corvint dogfood propose-citations --refresh %s %s > plan.new, review it, install it as CITATIONS and rerun make dogfood-closeout\n' "$citations" "$base" >&2
  exit 1
fi
/bin/cp -f "$citations" "$citations.pre-repin" && /bin/cp -f "$log" "$citations" || exit 1
printf 'dogfood-closeout-repin: refreshed %s for the repin hunks (previous plan: %s.pre-repin)\n' "$citations" "$citations" >&2
