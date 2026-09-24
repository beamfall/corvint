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

base_arg=${1:?usage: dogfood-change.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
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
  if ! GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local \
    go build -o "$corvint_bin" ./cmd/corvint 2> /dev/null; then
    printf 'dogfood-change: REFUSE current-tree-corvint-build-failed\n' >&2
    exit 2
  fi
else
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 || ! $version_output =~ ^"Corvint $expected_version (build "(0|[1-9][0-9]*)")"$ ]]; then
    printf 'dogfood-change: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
fi
# The flow's root is the working directory, so run it from the repository; a
# relative executable path keeps meaning the one resolved here.
if [[ $PWD != "$repo" ]]; then
  [[ $corvint_bin != */* || $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
  cd "$repo" || exit 2
fi
exec "$corvint_bin" dogfood change --corvint-bin "$corvint_bin" "$base_arg"
