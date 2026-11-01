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
# ASD-V0-902 (V1-3481): every up-front refusal names the entry the operator ran, the pre-flight
# (dogfood-inputs-check, also the closeout's first step) or the finish, and its fix line names that
# make target.
prefix=dogfood-finish-lane
entry=dogfood-finish
if [[ ${DOGFOOD_FINISH_PREFLIGHT-} == 1 ]]; then
  prefix=dogfood-inputs-check
  entry=dogfood-inputs-check
fi
[[ ${DOGFOOD_FINISH_ENTRY-} != dogfood-closeout ]] || entry=dogfood-closeout
intents=${3-}
verify=${4-}
outcome=${5-}
# Every missing or unusable input is named in one refusal before any build runs (ASD-V0-015).
# DOGFOOD_FINISH_LANE=1 (`make dogfood-finish LANE=1`) also requires INTENTS, VERIFY and OUTCOME,
# the inputs a lane must supply up front, and BASE as a full commit ID (docs/LANE-ORCHESTRATION.md);
# the pre-flight binds nothing, so it does not require OUTCOME (DCW-V0-848).
missing=()
[[ -n $base_arg ]] || missing+=(BASE)
[[ -n $citations ]] || missing+=(CITATIONS)
if [[ ${DOGFOOD_FINISH_LANE-} == 1 ]]; then
  [[ -z $base_arg || $base_arg =~ ^[0-9a-f]{40}$ ]] || missing+=(BASE:not-full-sha)
  [[ -n $intents ]] || missing+=(INTENTS)
  [[ -n $verify ]] || missing+=(VERIFY)
  # dogfood-closeout runs this pre-flight first and still refuses a missing OUTCOME before any
  # build (DCW-V0-848).
  [[ -n $outcome || (${DOGFOOD_FINISH_PREFLIGHT-} == 1 && ${DOGFOOD_FINISH_ENTRY-} != dogfood-closeout) ]] || missing+=(OUTCOME)
fi
for pair in "CITATIONS=$citations" "INTENTS=$intents" "VERIFY=$verify"; do
  [[ -z ${pair#*=} || (-f ${pair#*=} && -r ${pair#*=}) ]] || missing+=("${pair%%=*}:unreadable")
done
# ASD-V0-903 (V1-3482): a plan with no row is refused here, exit 2, not by the later cem-cite precheck.
if [[ -n $citations && -f $citations && -r $citations ]] && ! grep -q -v -E '^[[:space:]]*(#|$)' -- "$citations"; then
  missing+=(CITATIONS:empty)
fi
case $outcome in '' | passed | failed | blocked) ;; *) missing+=(OUTCOME:invalid) ;; esac
if ((${#missing[@]} > 0)); then
  printf '%s: REFUSE missing-inputs %s\n' "$prefix" "${missing[*]}" >&2
  printf 'fix: make %s BASE=<full sha> CITATIONS=<plan.tsv> INTENTS=<intents.txt> VERIFY=<verify.txt> OUTCOME=passed|failed|blocked\n' "$entry" >&2
  # ASD-V0-045 (V1-3119): a missing or unreadable plan names how to draft and pre-check one; this
  # refusal ran no build, prepare or bind, so it does not use up a closeout try.
  if [[ " ${missing[*]} " == *' CITATIONS '* || " ${missing[*]} " == *' CITATIONS:unreadable '* || " ${missing[*]} " == *' CITATIONS:empty '* ]]; then
    printf 'fix: draft the plan with corvint dogfood propose-citations %s > plan.tsv, then check it with make dogfood-inputs-check BASE=%s CITATIONS=plan.tsv (the same INTENTS, VERIFY and OUTCOME)\n' \
      "${base_arg:-<full sha>}" "${base_arg:-<full sha>}" >&2
    # ASD-V0-961 (V1-3394): a record-only change (a build-log entry alone) has its own recipe.
    printf 'fix: a record-only change (only docs/build-log/ or .agent-evidence/ paths): put the entry path in the INTENTS file, and cite each hunk to the BASE lines of the decision or spec the entry records with relation decision, one row per hunk, for example 1<TAB>docs/specs/SPEC.md<TAB>START:END<TAB>decision; one span may serve every hunk (DCW-V0-057, DCW-V0-058; docs/DOGFOOD.md "Record-only closeout")\n' >&2
  fi
  printf '%s: nothing was built, prepared or bound\n' "$prefix" >&2
  exit 2
fi
# ICL-V0-030 (V1-1629): the Makefile runs this refusal alone first, before the host admission, so a
# missing input is named before the admitted run queues on host load.
[[ ${DOGFOOD_FINISH_INPUTS_ONLY-} != 1 ]] || exit 0
repo=$(cd "$(dirname "$0")/.." && pwd)
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
# DCW-V0-815 (V1-3594): the cheap static doc checks that batch-precheck would otherwise fail minutes
# in run here, before any build: every requirement definition, and, when the change touches a test
# file or the declared read scopes, the test read scopes. A fixture without the checks skips them.
if [[ -x $repo/script/check-requirement-definitions.sh ]] &&
  ! static_out=$(cd "$repo" && script/check-requirement-definitions.sh 2>&1); then
  printf '%s: REFUSE requirement-definitions-invalid\n%s\n' "$prefix" "$(head -n 5 <<< "$static_out")" >&2
  printf 'fix: correct the definitions make requirement-definitions-check names, commit, then rerun make %s (nothing was built, prepared or bound)\n' "$entry" >&2
  exit 2
fi
if [[ -d $repo/tools/test-read-scopes-check ]] &&
  [[ -n $(git -C "$repo" diff --name-only "$base" -- '*_test.go' .corvint/test-read-scopes.json) ]] &&
  ! static_out=$(cd "$repo" && GOCACHE=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}} \
    GOTOOLCHAIN=local go run ./tools/test-read-scopes-check 2>&1 > /dev/null); then
  printf '%s: REFUSE test-read-scopes-invalid\n%s\n' "$prefix" "$(head -n 5 <<< "$static_out")" >&2
  printf 'fix: declare the read in .corvint/test-read-scopes.json or mark a fixture path //corvint:fixture-path, as make test-read-scopes-check names, commit, then rerun make %s (nothing was built, prepared or bound)\n' "$entry" >&2
  exit 2
fi
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
    printf '%s: REFUSE spec-requirements-stale %s\n' "$prefix" "${stale[*]}" >&2
    printf 'fix: stage the changed spec, run make spec-requirements, commit the regenerated files, then rerun\n' >&2
    exit 2
  fi
  /bin/rm -rf "$spec_tmp"
  trap - EXIT
fi
# IDX-SNAP-V0-891 (V1-2976): an analyzer input change whose golden output drifted from its pins is
# refused here, before any build or bind, instead of at batch-precheck after the seal. The static
# check notes a changed input; only then does the golden output test run.
if [[ -f $repo/internal/contextindex/analyzer_schema_test.go && -x $repo/script/analyzer-schema.sh ]]; then
  schema_note=$(cd "$repo" && script/analyzer-schema.sh check --base "$base" 2>&1)
  schema_rc=$?
  if ((schema_rc != 0)) || [[ $schema_note == *'analyzer schema: NOTE '* ]]; then
    if ! schema_out=$(cd "$repo" && script/analyzer-schema.sh check --output --base "$base" 2>&1); then
      printf 'dogfood-finish-lane: REFUSE analyzer-schema-stale
%s
' "$(grep '^analyzer schema:' <<< "$schema_out" | head -n 3)" >&2
      printf 'fix: run script/analyzer-schema.sh bump --base %s, commit the rewritten pins, then rerun (nothing was built, prepared or bound)
' "$base" >&2
      exit 2
    fi
  fi
fi
# ASD-V0-087 (V1-3154): a CORVINT_BIN built before this change cannot run the change's own dogfood
# tooling, and a matching VERSION does not prove otherwise. When the change touches that tooling, only
# the lane-built tree binary may run the steps, so an override is refused before any build.
# ASD-V0-901 (V1-3431): test files build into no binary, and a binary whose Go build stamp is HEAD,
# unmodified, with no tooling path changed since HEAD, was built from this change.
tooling_paths=(internal/dogfoodflow/ internal/cem/ 'cmd/corvint/dogfood_*.go' ':(exclude,glob)**/*_test.go')
tooling_admitted() {
  local stamp revision modified
  [[ -z $(git -C "$repo" diff --name-only HEAD -- "${tooling_paths[@]}") ]] || return 1
  stamp=$(GOTOOLCHAIN=local go version -m "$corvint_bin" 2> /dev/null) || return 1
  revision=$(awk '$1 == "build" && $2 ~ /^vcs\.revision=/ { sub(/^vcs\.revision=/, "", $2); print $2 }' <<< "$stamp")
  modified=$(awk '$1 == "build" && $2 ~ /^vcs\.modified=/ { sub(/^vcs\.modified=/, "", $2); print $2 }' <<< "$stamp")
  [[ -n $revision && $revision == "$(git -C "$repo" rev-parse HEAD)" && $modified == false ]]
}
if [[ -n $corvint_bin_set && $corvint_bin != "$git_dir/corvint/corvint" ]]; then
  tooling=$(git -C "$repo" diff --name-only "$base" -- "${tooling_paths[@]}") || exit 2
  if [[ -n $tooling ]] && ! tooling_admitted; then
    printf '%s: REFUSE corvint-bin-predates-tree-tooling CORVINT_BIN=%s changed=%s (%d paths)\n' "$prefix" \
      "$corvint_bin" "${tooling%%$'\n'*}" "$(printf '%s\n' "$tooling" | wc -l | tr -d ' ')" >&2
    printf 'fix: unset CORVINT_BIN and rerun make %s with the same inputs, or rebuild CORVINT_BIN from HEAD with no uncommitted change (go build -o PATH ./cmd/corvint); the lane-built tree binary %s then runs every step (nothing was built, prepared or bound)\n' \
      "$entry" "$git_dir/corvint/corvint" >&2
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
  exec "$tree_bin" dogfood inputs-check --base "$base_arg" ${inputs[@]+"${inputs[@]}"} --corvint-bin "$steps_bin"
fi
(cd "$repo" && exec "$tree_bin" dogfood inputs-check --base "$base_arg" ${inputs[@]+"${inputs[@]}"} --corvint-bin "$steps_bin") || exit $?
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
exec "$tree_bin" dogfood finish-lane --base "$base_arg" ${inputs[@]+"${inputs[@]}"} --corvint-bin "$steps_bin" \
  --base-verifier "$base_bin" --tree-verifier "$tree_bin" ${override[@]+"${override[@]}"}
