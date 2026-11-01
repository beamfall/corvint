#!/usr/bin/env bash
# usage: dogfood-change.sh BASE
# Local outcome recording needs DOGFOOD_OUTCOME plus one verification source:
# DOGFOOD_VERIFY holds one recorder command per line (blank lines skipped), or
# DOGFOOD_VERIFY_FILE names a file of the same shape and takes precedence.
# DOGFOOD_OCM_LINKS optionally names the author's explicit OCM link plan.
# Corvint's own wrapper over `corvint dogfood change` (DCW-V0-020): it keeps the
# Corvint-only VERSION identity and current-tree build (DCW-V0-022), then runs
# the flow inside that binary.
set -uo pipefail
export LC_ALL=C

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}
# ASD-V0-070 (V1-3233): an empty environment (env -i) names the missing HOME instead of aborting.
[[ -n ${CORVINT_GOCACHE:-}${GOCACHE:-}${XDG_CACHE_HOME:-}${HOME:-} ]] ||
  { printf 'dogfood-change: REFUSE home-unset: HOME, XDG_CACHE_HOME, GOCACHE and CORVINT_GOCACHE are all unset; set HOME and rerun\n' >&2; exit 2; }
go_cache=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}

base_arg=${1:?usage: dogfood-change.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
source "$repo/script/dogfood-build-corvint.sh" || exit 2
dogfood_require_full_history dogfood-change || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
evidence="$git_dir/corvint"
mkdir -p "$evidence" || exit 2
expected_version=$(cat "$repo/VERSION")
if [[ -z $expected_version ]]; then
  printf 'dogfood-change: REFUSE project-version-unavailable\n' >&2
  exit 2
fi
if [[ -z $corvint_bin_set ]]; then
  corvint_bin="$evidence/corvint"
  if ! dogfood_build_tree_corvint "$corvint_bin"; then
    printf 'dogfood-change: REFUSE current-tree-corvint-build-failed\n' >&2
    dogfood_build_fix dogfood-change tree
    exit 2
  fi
else
  # A PATH-only name resolves once, before the version probe, so every later verifier receives the executable
  # the probe ran. The result is absolute so a later cd cannot change its meaning.
  [[ $corvint_bin == */* ]] || corvint_bin=$(type -P -- "$corvint_bin") || {
    printf 'dogfood-change: REFUSE corvint-bin-unresolved\n' >&2
    exit 2
  }
  [[ $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 ]] || ! dogfood_version_matches "$version_output" "$expected_version"; then
    printf 'dogfood-change: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
fi
# The flow's root is the working directory, so run it from the repository;
# relative executable and input paths keep meaning the ones the caller named.
if [[ $PWD != "$repo" ]]; then
  for name in DOGFOOD_INTENTS_FILE DOGFOOD_CITATIONS DOGFOOD_VERIFY_FILE DOGFOOD_OCM_LINKS; do
    value=${!name-}
    [[ -z $value || $value == /* ]] || export "$name=$PWD/$value"
  done
  cd "$repo" || exit 2
fi
# IDX-SNAP-V0-891 (V1-3574): name an analyzer input change and its check at change start, before
# any focused test selection can leave TestAnalyzerSchemaOutput out.
if [[ -f internal/contextindex/analyzer_schema_test.go && -x script/analyzer-inputs.sh ]] &&
  analyzer_inputs=$(script/analyzer-inputs.sh "$base_arg" 2> /dev/null) && [[ -n $analyzer_inputs ]]; then
  printf 'dogfood-change: NOTE analyzer-schema: %s analyzer input(s) changed since %s, first %s: script/analyzer-schema.sh check --output decides whether a bump is needed, and dogfood-finish runs it before the bind\n' \
    "$(grep -c . <<< "$analyzer_inputs")" "$base_arg" "${analyzer_inputs%%$'\n'*}"
fi
exec "$corvint_bin" dogfood change --corvint-bin "$corvint_bin" "$base_arg"
