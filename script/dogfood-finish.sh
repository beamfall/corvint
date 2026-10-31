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
  # ASD-V0-045 (V1-3119): a missing or unreadable plan names how to draft and pre-check one; this
  # refusal ran no build, prepare or bind, so it does not use up a closeout try.
  if [[ " ${missing[*]} " == *' CITATIONS '* || " ${missing[*]} " == *' CITATIONS:unreadable '* ]]; then
    printf 'fix: draft the plan with corvint dogfood propose-citations %s > plan.tsv, then check it with make dogfood-inputs-check BASE=%s CITATIONS=plan.tsv (the same INTENTS, VERIFY and OUTCOME)\n' \
      "${base_arg:-<full sha>}" "${base_arg:-<full sha>}" >&2
  fi
  printf 'dogfood-finish-lane: nothing was built, prepared or bound\n' >&2
  exit 2
fi
repo=$(cd "$(dirname "$0")/.." && pwd)
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
# DCW-V0-090: a change that touches docs/specs/ must commit the regenerated spec indexes. The
# generator reads a private index of HEAD, so staged or worktree edits cannot hide a stale commit.
if [[ -n $(git -C "$repo" diff --name-only "$base" HEAD -- docs/specs/) ]]; then
  spec_tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-check.XXXXXX") || exit 2
  trap '/bin/rm -rf "$spec_tmp"' EXIT
  mkdir "$spec_tmp/out" || exit 2
  stale=()
  if GIT_INDEX_FILE="$spec_tmp/index" git -C "$repo" read-tree HEAD &&
    (cd "$repo" && GIT_INDEX_FILE="$spec_tmp/index" script/gen-spec-requirements.sh --all "$spec_tmp/out"); then
    for file in REQUIREMENTS.tsv INDEX.json README.md; do
      git -C "$repo" cat-file blob "HEAD:docs/specs/$file" 2> /dev/null | cmp -s - "$spec_tmp/out/$file" ||
        stale+=("docs/specs/$file")
    done
  else
    stale+=(generation-failed)
  fi
  if ((${#stale[@]} > 0)); then
    printf 'dogfood-finish-lane: REFUSE spec-requirements-stale %s\n' "${stale[*]}" >&2
    printf 'fix: stage the changed spec, run make spec-requirements, commit the regenerated files, then rerun\n' >&2
    exit 2
  fi
  /bin/rm -rf "$spec_tmp"
  trap - EXIT
fi
# ASD-V0-087 (V1-3154): a CORVINT_BIN built before this change cannot run the change's own dogfood
# tooling, and a matching VERSION does not prove otherwise. When the change touches that tooling, only
# the lane-built tree binary may run the steps, so an override is refused before any build.
if [[ -n $corvint_bin_set && $corvint_bin != "$git_dir/corvint/corvint" ]]; then
  tooling=$(git -C "$repo" diff --name-only "$base" -- internal/dogfoodflow/ internal/cem/ 'cmd/corvint/dogfood_*.go') || exit 2
  if [[ -n $tooling ]]; then
    printf 'dogfood-finish-lane: REFUSE corvint-bin-predates-tree-tooling CORVINT_BIN=%s changed=%s (%d paths)\n' \
      "$corvint_bin" "${tooling%%$'\n'*}" "$(printf '%s\n' "$tooling" | wc -l | tr -d ' ')" >&2
    printf 'fix: unset CORVINT_BIN and rerun the same make target (dogfood-closeout, dogfood-finish or dogfood-inputs-check) with the same inputs; the lane-built tree binary %s then runs every step (nothing was built, prepared or bound)\n' \
      "$git_dir/corvint/corvint" >&2
    exit 2
  fi
fi
source "$repo/script/dogfood-build-corvint.sh" || exit 2
evidence="$git_dir/corvint"
override=()
expected_version=$(cat "$repo/VERSION")
if [[ -z $expected_version ]]; then
  printf 'dogfood-finish-lane: REFUSE project-version-unavailable\n' >&2
  exit 2
fi
if [[ -n $corvint_bin_set ]]; then
  # A PATH-only name resolves once, before the version probe, so every later verifier receives the executable
  # the probe ran. The result is absolute so a later cd cannot change its meaning.
  [[ $corvint_bin == */* ]] || corvint_bin=$(type -P -- "$corvint_bin") || {
    printf 'dogfood-finish-lane: REFUSE corvint-bin-unresolved\n' >&2
    exit 2
  }
  [[ $corvint_bin == /* ]] || corvint_bin="$PWD/$corvint_bin"
  version_output=$("$corvint_bin" --version 2> /dev/null)
  return_code=$?
  if [[ $return_code -ne 0 ]] || ! dogfood_version_matches "$version_output" "$expected_version"; then
    printf 'dogfood-finish-lane: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
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
go_cache=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}}
tree_bin="$evidence/corvint"
base_bin="$evidence/corvint-base"
mkdir -p "$evidence" || exit 2
# DCW-V0-151: an interrupt during either build names the step and the NOT_PRODUCED block, as finish-lane
# does once it runs (DCW-V0-067); nothing is prepared, bound or committed before it starts. Bash runs the
# trap when the build returns: a terminal interrupt stops the whole foreground group at once.
build_step=build-tree
dogfood_finish_interrupted() {
  printf 'dogfood-finish-lane: %s INTERRUPTED\ndogfood-finish-lane: NOT_PRODUCED rows=0\n' "$build_step"
  printf 'dogfood-finish-lane: INTERRUPTED %s\n  fix: rerun make dogfood-finish BASE=%s with the same inputs; nothing was prepared, bound or committed\n' \
    "$build_step" "$base" >&2
  exit "$1"
}
trap 'dogfood_finish_interrupted 130' INT
trap 'dogfood_finish_interrupted 143' TERM
if ! dogfood_build_tree_corvint "$tree_bin"; then
  printf 'dogfood-finish-lane: REFUSE current-tree-corvint-build-failed\n' >&2
  dogfood_build_fix dogfood-finish-lane tree
  exit 2
fi
steps_bin=$tree_bin
[[ -z $corvint_bin_set ]] || steps_bin=$corvint_bin
# DCW-V0-380: the input pre-flight runs before the base verifier is built and before any bind: the
# verify file, intents manifest and citation plan are checked against a freshly prepared map that is
# then removed. DOGFOOD_FINISH_PREFLIGHT=1 (`make dogfood-inputs-check`) stops after it.
build_step=inputs-check
if [[ ${DOGFOOD_FINISH_PREFLIGHT-} == 1 ]]; then
  trap - INT TERM
  cd "$repo" || exit 2
  exec "$tree_bin" dogfood inputs-check --base "$base_arg" "${inputs[@]}" --corvint-bin "$steps_bin"
fi
(cd "$repo" && exec "$tree_bin" dogfood inputs-check --base "$base_arg" "${inputs[@]}" --corvint-bin "$steps_bin") || exit $?
base_status=0
build_step=build-base
dogfood_build_base_corvint "$base_bin" "$base" || base_status=$?
if ((base_status == 2)); then
  printf 'dogfood-finish-lane: REFUSE base-verifier-source-unavailable\n' >&2
  exit 2
elif ((base_status != 0)); then
  printf 'dogfood-finish-lane: REFUSE base-corvint-build-failed\n' >&2
  dogfood_build_fix dogfood-finish-lane base
  exit 2
fi
trap - INT TERM
cd "$repo" || exit 2
exec "$tree_bin" dogfood finish-lane --base "$base_arg" "${inputs[@]}" --corvint-bin "$steps_bin" \
  --base-verifier "$base_bin" --tree-verifier "$tree_bin" ${override[@]+"${override[@]}"}
