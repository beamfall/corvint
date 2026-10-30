#!/usr/bin/env bash
# usage: dogfood-batch-check.sh BASE
# Corvint's own wrapper over `corvint dogfood batch-check` (DCW-V0-039): it
# keeps the Corvint-only VERSION identity, builds the current tree into a
# temporary directory outside the repository, and runs the read-only check of
# a merge batch of sealed changes inside that binary.
set -uo pipefail
export LC_ALL=C

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}
go_cache=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}

base_arg=${1:?usage: dogfood-batch-check.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
source "$repo/script/dogfood-build-corvint.sh" || exit 2
expected_version=$(cat "$repo/VERSION")
if [[ -z $expected_version ]]; then
  printf 'dogfood-batch-check: REFUSE project-version-unavailable\n' >&2
  exit 2
fi
if [[ -z $corvint_bin_set ]]; then
  build_dir=$(mktemp -d "${TMPDIR:-/tmp}/corvint-batch-check.XXXXXX") || exit 2
  trap 'rm -rf "$build_dir"' EXIT
  corvint_bin="$build_dir/corvint"
  if ! dogfood_build_tree_corvint "$corvint_bin"; then
    printf 'dogfood-batch-check: REFUSE current-tree-corvint-build-failed\n' >&2
    dogfood_build_fix dogfood-batch-check tree
    exit 2
  fi
else
  # A PATH-only name resolves once, before the version probe, so every later verifier receives the executable
  # the probe ran. The result is absolute so a later cd cannot change its meaning.
  [[ $corvint_bin == */* ]] || corvint_bin=$(type -P -- "$corvint_bin") || {
    printf 'dogfood-batch-check: REFUSE corvint-bin-unresolved\n' >&2
    exit 2
  }
  [[ $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 ]] || ! dogfood_version_matches "$version_output" "$expected_version"; then
    printf 'dogfood-batch-check: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
fi
cd "$repo" || exit 2
"$corvint_bin" dogfood batch-check --verifier "$corvint_bin" "$base_arg"
