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
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 || ! $version_output =~ ^"Corvint $expected_version (build "(0|[1-9][0-9]*)")"$ ]]; then
    printf 'dogfood-check: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
  [[ $corvint_bin != */* || $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
  override=(--override-verifier "$corvint_bin")
fi
tree_bin="$evidence/corvint"
base_bin="$evidence/corvint-base"
mkdir -p "$evidence" || exit 2
if ! GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local \
  go build -o "$tree_bin" -trimpath ./cmd/corvint 2> /dev/null; then
  printf 'dogfood-check: REFUSE current-tree-corvint-build-failed\n' >&2
  exit 2
fi
base_tree=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-check.XXXXXX") || exit 2
trap 'rm -rf "$base_tree"' EXIT
if ! git -C "$repo" archive --format=tar "$base" | tar -xf - -C "$base_tree"; then
  printf 'dogfood-check: REFUSE base-verifier-source-unavailable\n' >&2
  exit 2
fi
if ! (cd "$base_tree" && GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local \
  go build -o "$base_bin" -trimpath ./cmd/corvint 2> /dev/null); then
  printf 'dogfood-check: REFUSE base-corvint-build-failed\n' >&2
  exit 2
fi
rm -rf "$base_tree"
trap - EXIT
cd "$repo" || exit 2
exec "$tree_bin" dogfood check --base-verifier "$base_bin" --tree-verifier "$tree_bin" ${override[@]+"${override[@]}"} "$base_arg"
