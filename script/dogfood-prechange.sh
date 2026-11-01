#!/usr/bin/env bash
# usage: dogfood-prechange.sh BASE [PATH...]
# Corvint's own wrapper over `corvint dogfood prechange` (DCW-V0-880): it builds
# the current tree's binary as dogfood-change.sh does and records the agent
# pre-change receipts and their binding before the first edit. DOGFOOD_TASK is
# the task text.
set -uo pipefail
export LC_ALL=C

[[ -n ${CORVINT_GOCACHE:-}${GOCACHE:-}${XDG_CACHE_HOME:-}${HOME:-} ]] ||
  { printf 'dogfood-prechange: REFUSE home-unset: HOME, XDG_CACHE_HOME, GOCACHE and CORVINT_GOCACHE are all unset; set HOME and rerun\n' >&2; exit 2; }
go_cache=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}

base_arg=${1:?usage: dogfood-prechange.sh BASE [PATH...]}
shift
repo=$(cd "$(dirname "$0")/.." && pwd)
source "$repo/script/dogfood-build-corvint.sh" || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
evidence="$git_dir/corvint"
mkdir -p "$evidence" || exit 2
corvint_bin="$evidence/corvint"
if ! dogfood_build_tree_corvint "$corvint_bin"; then
  printf 'dogfood-prechange: REFUSE current-tree-corvint-build-failed\n' >&2
  dogfood_build_fix dogfood-prechange tree
  exit 2
fi
cd "$repo" || exit 2
exec "$corvint_bin" dogfood prechange "$base_arg" "$@"
