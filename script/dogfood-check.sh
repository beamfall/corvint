#!/usr/bin/env bash
# usage: dogfood-check.sh BASE
# Corvint's own wrapper over `corvint dogfood check` (DCW-V0-020): it keeps the
# Corvint-only VERSION identity and the base and current-tree verifier builds
# (DCW-V0-022), then runs the check inside the tree verifier.
set -uo pipefail
export LC_ALL=C

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}

base_arg=${1:?usage: dogfood-check.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
source "$repo/script/dogfood-build-corvint.sh" || exit 2
dogfood_require_full_history dogfood-check || exit 2
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
evidence="$git_dir/corvint"
override=()
expected_version=$(cat "$repo/VERSION")
if [[ -z $expected_version ]]; then
  printf 'dogfood-check: REFUSE project-version-unavailable\n' >&2
  exit 2
fi
if [[ -n $corvint_bin_set ]]; then
  # A PATH-only name resolves once, before the version probe, so every later verifier receives the executable
  # the probe ran. The result is absolute so a later cd cannot change its meaning.
  [[ $corvint_bin == */* ]] || corvint_bin=$(type -P -- "$corvint_bin") || {
    printf 'dogfood-check: REFUSE corvint-bin-unresolved\n' >&2
    exit 2
  }
  [[ $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
fi
if [[ -n $corvint_bin_set ]]; then
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 ]] || ! dogfood_version_matches "$version_output" "$expected_version"; then
    printf 'dogfood-check: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
  override=(--override-verifier "$corvint_bin")
fi
# ASD-V0-070 (V1-3233): an empty environment (env -i) names the missing HOME instead of aborting.
[[ -n ${CORVINT_GOCACHE:-}${GOCACHE:-}${XDG_CACHE_HOME:-}${HOME:-} ]] ||
  { printf 'dogfood-check: REFUSE home-unset: HOME, XDG_CACHE_HOME, GOCACHE and CORVINT_GOCACHE are all unset; set HOME and rerun\n' >&2; exit 2; }
go_cache=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}
tree_bin="$evidence/corvint"
base_bin="$evidence/corvint-base"
mkdir -p "$evidence" || exit 2
if ! dogfood_build_tree_corvint "$tree_bin"; then
  printf 'dogfood-check: REFUSE current-tree-corvint-build-failed\n' >&2
  dogfood_build_fix dogfood-check tree
  exit 2
fi
base_status=0
dogfood_build_base_corvint "$base_bin" "$base" || base_status=$?
if ((base_status == 2)); then
  printf 'dogfood-check: REFUSE base-verifier-source-unavailable\n' >&2
  exit 2
elif ((base_status != 0)); then
  printf 'dogfood-check: REFUSE base-corvint-build-failed\n' >&2
  dogfood_build_fix dogfood-check base
  exit 2
fi
cd "$repo" || exit 2
exec "$tree_bin" dogfood check --base-verifier "$base_bin" --tree-verifier "$tree_bin" ${override[@]+"${override[@]}"} "$base_arg"
