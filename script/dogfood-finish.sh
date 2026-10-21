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
  [[ -z ${pair#*=} || (-f ${pair#*=} && -r ${pair#*=}) ]] || missing+=("${pair%%=*}:unreadable")
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
source "$repo/script/dogfood-build-corvint.sh" || exit 2
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
  # A PATH-only name resolves once, here, so every later verifier receives the executable the version check ran.
  [[ $corvint_bin == */* ]] || corvint_bin=$(type -P -- "$corvint_bin") || {
    printf 'dogfood-finish-lane: REFUSE corvint-bin-unresolved\n' >&2
    exit 2
  }
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
if ! dogfood_build_tree_corvint "$tree_bin"; then
  printf 'dogfood-finish-lane: REFUSE current-tree-corvint-build-failed\n' >&2
  exit 2
fi
base_status=0
dogfood_build_base_corvint "$base_bin" "$base" || base_status=$?
if ((base_status == 2)); then
  printf 'dogfood-finish-lane: REFUSE base-verifier-source-unavailable\n' >&2
  exit 2
elif ((base_status != 0)); then
  printf 'dogfood-finish-lane: REFUSE base-corvint-build-failed\n' >&2
  exit 2
fi
steps_bin=$tree_bin
[[ -z $corvint_bin_set ]] || steps_bin=$corvint_bin
cd "$repo" || exit 2
exec "$tree_bin" dogfood finish-lane --base "$base_arg" "${inputs[@]}" --corvint-bin "$steps_bin" \
  --base-verifier "$base_bin" --tree-verifier "$tree_bin" ${override[@]+"${override[@]}"}
