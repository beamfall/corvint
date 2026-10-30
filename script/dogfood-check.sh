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

base_arg=${1:?usage: dogfood-check.sh BASE|--bound-base}
repo=$(cd "$(dirname "$0")/.." && pwd)
# --bound-base (make dogfood-check without BASE) takes the baseRevision of the CEM committed at
# HEAD, never a default branch; with no such CEM it refuses naming BASE (ASD-V0-085, V1-3172).
# A string-aware scan reads only the top-level object's members: a baseRevision nested in another
# member is not the CEM's, and a missing, repeated or non-40-hex top-level member fails closed.
if [[ $base_arg == --bound-base ]]; then
  bound=$(git -C "$repo" show "HEAD:.corvint/change.cem.json" 2> /dev/null | LC_ALL=C awk '
    { text = text $0 "\n" }
    END {
      depth = 0; instr = 0; esc = 0; token = ""; key = ""; want = 0; found = 0; value = ""
      for (i = 1; i <= length(text); i++) {
        c = substr(text, i, 1)
        if (instr) {
          if (esc) { esc = 0; token = token c; continue }
          if (c == "\\") { esc = 1; token = token c; continue }
          if (c != "\"") { token = token c; continue }
          instr = 0
          if (depth == 1 && want) { value = token; want = 0; found++ }
          else if (depth == 1) key = token
          continue
        }
        if (c == "\"") { instr = 1; token = ""; continue }
        if (c == "{" || c == "[") { depth++; want = 0; continue }
        if (c == "}" || c == "]") { depth--; continue }
        if (depth == 1 && c == ":") { want = (key == "baseRevision"); key = ""; continue }
        if (depth == 1 && c == ",") { want = 0; key = ""; continue }
        if (depth == 1 && c !~ /[ \t\r\n]/ && want) { want = 0; found = 99 }
      }
      if (found == 1 && depth == 0 && length(value) == 40 && value !~ /[^0-9a-f]/) print value
    }' || :)
  if [[ -z $bound ]] || ! git -C "$repo" merge-base --is-ancestor "$bound" HEAD 2> /dev/null; then
    printf 'BASE is required: pass BASE=<full origin/main sha>; no CEM bound at HEAD records a base (HEAD:.corvint/change.cem.json is absent or has no single baseRevision that HEAD contains); the Makefile default (main) is not used by dogfood targets\n' >&2
    printf '  fix: make dogfood-check BASE=<the full sha passed to dogfood-change>, or commit the bound CEM first (docs/DOGFOOD.md §4); on a seal commit check its bind commit: git checkout --detach HEAD^\n' >&2
    exit 2
  fi
  base_arg=$bound
  printf 'dogfood-check: BASE=%s from the CEM bound at HEAD (.corvint/change.cem.json baseRevision)\n' "$base_arg" >&2
fi
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
source "$repo/script/dogfood-build-corvint.sh" || exit 2
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
