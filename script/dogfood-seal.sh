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
sealed=".corvint/changes/$bind.cem.json"
mkdir -p "$repo/.corvint/changes"
git -C "$repo" mv .corvint/change.cem.json "$sealed" || exit 2
git -C "$repo" commit -q -m "chore: seal change evidence" || exit 2
printf 'dogfood-seal: PASS sealed=%s\n' "$sealed"
