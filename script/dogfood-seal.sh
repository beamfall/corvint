#!/usr/bin/env bash
# usage: dogfood-seal.sh BASE
# Checks the bound change, then moves its CEM out of the one shared tracked path
# in a rename-only commit, so concurrent branches never edit the same file
# (docs/DOGFOOD.md §4, DOGFOOD-013, decision 0319).
set -uo pipefail
export LC_ALL=C

base_arg=${1:?usage: dogfood-seal.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
"$repo/script/dogfood-check.sh" "$base_arg" || exit
bind=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
if ! git -C "$repo" cat-file -e "$bind:.corvint/change.cem.json" 2>/dev/null; then
  printf 'dogfood-seal: REFUSE nothing-to-seal\n' >&2
  exit 2
fi
# V1-0137: never drop a CEM BASE tracks that this change replaced and no
# .corvint/changes/ file keeps.
base_cem=$(git -C "$repo" rev-parse --verify -q "$base_arg^{commit}:.corvint/change.cem.json" 2>/dev/null)
if [[ -n $base_cem && $base_cem != "$(git -C "$repo" rev-parse "$bind:.corvint/change.cem.json")" ]] &&
  ! grep -Fq " $base_cem"$'\t' <<< "$(git -C "$repo" ls-tree -r "$bind" -- .corvint/changes)"; then
  replaced=$(git -C "$repo" log -1 --format=%H "$base_arg^{commit}" -- .corvint/change.cem.json)
  printf 'dogfood-seal: REFUSE unarchived-base-cem\n' >&2
  printf '  BASE tracks .corvint/change.cem.json (bound at %s) that this change replaced and no .corvint/changes/ file keeps; archive it in a commit on the base branch, then restart this change on that commit\n' "$replaced" >&2
  exit 2
fi
sealed=".corvint/changes/$bind.cem.json"
mkdir -p "$repo/.corvint/changes"
git -C "$repo" mv .corvint/change.cem.json "$sealed" || exit 2
git -C "$repo" commit -q -m "chore: seal change evidence" || exit 2
printf 'dogfood-seal: PASS sealed=%s\n' "$sealed"
