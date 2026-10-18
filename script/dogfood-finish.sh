#!/usr/bin/env bash
# usage: dogfood-finish.sh BASE CITATIONS [INTENTS [VERIFY [OUTCOME]]]
# Corvint's own wrapper over `corvint dogfood finish-lane` (DCW-V0-060): it keeps
# the Corvint-only VERSION identity and builds the base and current-tree
# verifiers as dogfood-check.sh does (DCW-V0-022), then runs the whole bind loop
# inside the tree verifier. CORVINT_BIN, when set, runs the change steps and is
# the check's override verifier, as dogfood-change.sh and dogfood-check.sh use it.
set -uo pipefail
export LC_ALL=C

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}

base_arg=${1-}
citations=${2-}
intents=${3-}
verify=${4-}
outcome=${5-}
# Every missing or unusable input is named in one refusal before any build runs (ASD-V0-015).
# DOGFOOD_FINISH_LANE=1 (`make dogfood-finish LANE=1`) also requires INTENTS, VERIFY and OUTCOME,
# the inputs a lane must supply up front, and BASE as a full commit ID (docs/LANE-ORCHESTRATION.md).
missing=()
[[ -n $base_arg ]] || missing+=(BASE)
[[ -n $citations ]] || missing+=(CITATIONS)
if [[ ${DOGFOOD_FINISH_LANE-} == 1 ]]; then
  [[ -z $base_arg || $base_arg =~ ^[0-9a-f]{40}$ ]] || missing+=(BASE:not-full-sha)
  [[ -n $intents ]] || missing+=(INTENTS)
  [[ -n $verify ]] || missing+=(VERIFY)
  [[ -n $outcome ]] || missing+=(OUTCOME)
fi
for pair in "CITATIONS=$citations" "INTENTS=$intents" "VERIFY=$verify"; do
  [[ -z ${pair#*=} || -f ${pair#*=} ]] || missing+=("${pair%%=*}:unreadable")
done
case $outcome in '' | passed | failed | blocked) ;; *) missing+=(OUTCOME:invalid) ;; esac
if ((${#missing[@]} > 0)); then
  printf 'dogfood-finish-lane: REFUSE missing-inputs %s\n' "${missing[*]}" >&2
  printf 'fix: make dogfood-finish BASE=<full sha> CITATIONS=<plan.tsv> INTENTS=<intents.txt> VERIFY=<verify.txt> OUTCOME=passed|failed|blocked\n' >&2
  exit 2
fi
repo=$(cd "$(dirname "$0")/.." && pwd)
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
evidence="$git_dir/corvint"
override=()
expected_version=$(cat "$repo/VERSION")
if [[ -z $expected_version ]]; then
  printf 'dogfood-finish-lane: REFUSE project-version-unavailable\n' >&2
  exit 2
fi
if [[ -n $corvint_bin_set ]]; then
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 || ! $version_output =~ ^"Corvint $expected_version (build "(0|[1-9][0-9]*)")"$ ]]; then
    printf 'dogfood-finish-lane: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
  [[ $corvint_bin != */* || $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
  override=(--override-verifier "$corvint_bin")
fi
# Relative input paths keep meaning the ones the caller named.
inputs=()
for pair in "--citations=$citations" "--intents=$intents" "--verify=$verify" "--outcome=$outcome"; do
  name=${pair%%=*}
  value=${pair#*=}
  [[ -n $value ]] || continue
  [[ $name == --outcome || $value == /* ]] || value="$PWD/$value"
  inputs+=("$name" "$value")
done
go_cache=${CORVINT_GOCACHE:-${GOCACHE:-/tmp/corvint-go-build-cache}}
tree_bin="$evidence/corvint"
base_bin="$evidence/corvint-base"
mkdir -p "$evidence" || exit 2
if ! (cd "$repo" && GOCACHE="$go_cache" GOTOOLCHAIN=local \
  go build -o "$tree_bin" -trimpath ./cmd/corvint 2> /dev/null); then
  printf 'dogfood-finish-lane: REFUSE current-tree-corvint-build-failed\n' >&2
  exit 2
fi
base_tree=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-finish.XXXXXX") || exit 2
trap 'rm -rf "$base_tree"' EXIT
if ! git -C "$repo" archive --format=tar "$base" | tar -xf - -C "$base_tree"; then
  printf 'dogfood-finish-lane: REFUSE base-verifier-source-unavailable\n' >&2
  exit 2
fi
if ! (cd "$base_tree" && GOCACHE="$go_cache" GOTOOLCHAIN=local \
  go build -o "$base_bin" -trimpath ./cmd/corvint 2> /dev/null); then
  printf 'dogfood-finish-lane: REFUSE base-corvint-build-failed\n' >&2
  exit 2
fi
rm -rf "$base_tree"
trap - EXIT
steps_bin=$tree_bin
[[ -z $corvint_bin_set ]] || steps_bin=$corvint_bin
cd "$repo" || exit 2
exec "$tree_bin" dogfood finish-lane --base "$base_arg" "${inputs[@]}" --corvint-bin "$steps_bin" \
  --base-verifier "$base_bin" --tree-verifier "$tree_bin" ${override[@]+"${override[@]}"}
