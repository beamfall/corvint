#!/usr/bin/env bash
set -uo pipefail
export LC_ALL=C

# This script asserts on dogfood-report.json with ripgrep (rg); without it every rg call below
# fails as a bare "command not found" deep into the run, which reads as a real defect instead of
# a missing prerequisite. Fail closed here, before any build or Git work starts.
if ! command -v rg >/dev/null 2>&1; then
    printf 'dogfood-check: REFUSE unsupported-environment-missing-rg\n' >&2
    exit 1
fi

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}

base_arg=${1:?usage: dogfood-check.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
base_corvint_bin=
override_corvint_bin=
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
target=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
report="$repo/.corvint/dogfood-report.json"
intent_snapshot="$repo/.corvint/change.ocm-intents"
aggregate_status="$repo/.corvint/change.ocm-status.json"
local_outcome_evidence="$git_dir/corvint/local-outcome.json"
context_abstention_evidence="$git_dir/corvint/prechange-impact-abstention.json"
context_abstention=0
evidence="$git_dir/corvint"
run_tmp=
child_pid=
anchor_state=NOT_OBSERVED
anchor_merge_base=
base_verifier_sha256=
tree_verifier_sha256=
override_verifier_sha256=

resolve_anchor() {
  local configured merges config_result
  configured=$(git -C "$repo" config --local --get-all corvint.dogfood.anchor)
  config_result=$?
  [[ $config_result -eq 1 ]] && return
  if [[ $config_result -ne 0 || -z $configured ]]; then
    printf 'dogfood-check: REFUSE anchor-ref-unavailable\n' >&2
    exit 2
  fi
  if [[ $configured == *$'\n'* ]]; then
    printf 'dogfood-check: REFUSE multiple-anchor-refs\n' >&2
    exit 2
  fi
  merges=$(git -C "$repo" merge-base --all "$configured" "$target") || {
    printf 'dogfood-check: REFUSE anchor-ref-unavailable\n' >&2
    exit 2
  }
  if [[ -z $merges || $merges == *$'\n'* ]]; then
    printf 'dogfood-check: REFUSE anchor-merge-base-ambiguous\n' >&2
    exit 2
  fi
  anchor_state=OBSERVED
  anchor_merge_base=$merges
  if [[ $base != "$anchor_merge_base" ]]; then
    printf 'dogfood-check: REFUSE base-not-anchored\n' >&2
    exit 2
  fi
}

cleanup() {
  if [[ -n ${child_pid:-} ]]; then
    kill -TERM "$child_pid" 2>/dev/null || :
    wait "$child_pid" 2>/dev/null || :
    child_pid=
  fi
  if [[ -n ${run_tmp:-} ]]; then
    rm -rf "$run_tmp"
    run_tmp=
  fi
}

interrupted() {
  local code=$1
  trap - EXIT HUP INT TERM
  cleanup
  exit "$code"
}

trap cleanup EXIT
trap 'interrupted 129' HUP
trap 'interrupted 130' INT
trap 'interrupted 143' TERM

resolve_corvint_bins() {
  local expected_version version_output return_code
  expected_version=$(cat "$repo/VERSION")
  if [[ -z $expected_version ]]; then
    printf 'dogfood-check: REFUSE project-version-unavailable\n' >&2
    exit 2
  fi
  if [[ -n $corvint_bin_set ]]; then
    override_corvint_bin=$corvint_bin
    "$override_corvint_bin" --version > "$run_tmp/corvint-version.stdout" 2> "$run_tmp/corvint-version.stderr" &
    child_pid=$!
    wait "$child_pid"
    return_code=$?
    child_pid=
    version_output=$(cat "$run_tmp/corvint-version.stdout")
    if [[ $return_code -ne 0 || ! $version_output =~ ^"Corvint $expected_version (build "(0|[1-9][0-9]*)")"$ ]]; then
      printf 'dogfood-check: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
      exit 2
    fi
    override_verifier_sha256=$(shasum -a 256 "$override_corvint_bin" | awk '{print $1}')
  fi
  corvint_bin="$evidence/corvint"
  base_corvint_bin="$evidence/corvint-base"
  mkdir -p "$evidence" "$run_tmp/base-tree"
  GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local \
    go build -o "$corvint_bin" -trimpath ./cmd/corvint 2> "$run_tmp/corvint-build.stderr" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -ne 0 ]]; then
    printf 'dogfood-check: REFUSE current-tree-corvint-build-failed\n' >&2
    exit 2
  fi
  if ! git -C "$repo" archive --format=tar "$base" > "$run_tmp/base.tar" ||
    ! tar -xf "$run_tmp/base.tar" -C "$run_tmp/base-tree"; then
    printf 'dogfood-check: REFUSE base-verifier-source-unavailable\n' >&2
    exit 2
  fi
  (
    cd "$run_tmp/base-tree" || exit 2
    GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local \
      go build -o "$base_corvint_bin" -trimpath ./cmd/corvint 2> "$run_tmp/base-corvint-build.stderr"
  ) &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -ne 0 ]]; then
    printf 'dogfood-check: REFUSE base-corvint-build-failed\n' >&2
    exit 2
  fi
  tree_verifier_sha256=$(shasum -a 256 "$corvint_bin" | awk '{print $1}')
  base_verifier_sha256=$(shasum -a 256 "$base_corvint_bin" | awk '{print $1}')
}

run_verifier() {
  local name=$1 binary=$2
  shift 2
  "$binary" --root "$repo" "$@" > "$run_tmp/$name.stdout" 2> "$run_tmp/$name.stderr" &
  child_pid=$!
  wait "$child_pid"
  verifier_status=$?
  child_pid=
}

verifiers_agree() {
  local left=$1 right=$2 left_status=$3 right_status=$4
  [[ $left_status -eq $right_status ]] &&
    cmp -s "$run_tmp/$left.stdout" "$run_tmp/$right.stdout" &&
    cmp -s "$run_tmp/$left.stderr" "$run_tmp/$right.stderr"
}

record_dogfood_check() {
  local outputs_agree=$1 bootstrap=$2 replacement temporary override=null
  if [[ -n $override_verifier_sha256 ]]; then
    override='"sha256:'"$override_verifier_sha256"'"'
  fi
  replacement='  ,"dogfoodCheck": {"baseVerifierSha256": "sha256:'"$base_verifier_sha256"'", "treeVerifierSha256": "sha256:'"$tree_verifier_sha256"'", "overrideVerifierSha256": '"$override"', "bootstrapUnknown": '"$bootstrap"', "maximumUnknownAfterBootstrap": 0, "outputsAgree": '"$outputs_agree"'}'
  temporary="$run_tmp/dogfood-report.json"
  awk -v replacement="$replacement" '
    /^  ,"dogfoodCheck": / {$0 = replacement; replaced++}
    {print}
    END {if (replaced != 1) exit 1}
  ' "$report" > "$temporary" || return 1
  mv "$temporary" "$report"
}

count_bootstrap_unknowns() {
  local path count=0
  while IFS= read -r path; do
    if ! git -C "$repo" cat-file -e "$base:$path" 2>/dev/null; then
      count=$((count + 1))
    fi
  done < "$intent_snapshot"
  printf '%d' "$count"
}

cem_base_revision() {
  local value
  value=$(git -C "$repo" show "$1:.corvint/change.cem.json" 2>/dev/null |
    sed -n 's/^  "baseRevision": "\([0-9a-f]\{40\}\)",$/\1/p') || return 1
  [[ $value =~ ^[0-9a-f]{40}$ ]] || return 1
  git -C "$repo" merge-base --is-ancestor "$value" "$1" || return 1
  printf '%s' "$value"
}

# A seal commit has one parent and only renames that parent's CEM, unchanged, to
# .corvint/changes/<parent>.cem.json (docs/DOGFOOD.md §4, DOGFOOD-013).
is_seal_commit() {
  local parent
  parent=$(git -C "$repo" rev-parse --verify -q "$1^1") || return 1
  git -C "$repo" rev-parse --verify -q "$1^2" >/dev/null && return 1
  [[ $(git -C "$repo" diff-tree -r -M --no-commit-id --name-status "$parent" "$1") == \
    "R100"$'\t'".corvint/change.cem.json"$'\t'".corvint/changes/$parent.cem.json" ]]
}

# The previous binding is the CEM committed at BASE, else the bind commit under
# the newest seal reachable from BASE (DOGFOOD-014).
previous_binding() {
  local seal
  if git -C "$repo" cat-file -e "$base:.corvint/change.cem.json" 2>/dev/null; then
    printf '%s' "$base"
    return
  fi
  while IFS= read -r seal; do
    if is_seal_commit "$seal"; then
      git -C "$repo" rev-parse "$seal^1"
      return
    fi
  done < <(git -C "$repo" rev-list --no-merges --max-count=256 "$base" -- .corvint/changes)
  return 1
}

unbound_not_observed() {
  printf 'dogfood-check: NOTE unbound-commits NOT_OBSERVED %s\n' "$1" >&2
}

# Reports, without failing, non-merge commits after the base's committed CEM
# base that no CEM committed in that window binds, and separately those bound
# only by a retroactive binding commit (docs/DOGFOOD.md §4).
report_unbound_commits() {
  local previous previous_base window sidecar_commit sidecar_base bound covered='' retroactive_pairs='' not_normal unbound retroactive count
  previous=$(previous_binding) || { unbound_not_observed previous-cem-absent; return; }
  previous_base=$(cem_base_revision "$previous") || { unbound_not_observed previous-cem-base-unavailable; return; }
  window=$(git -C "$repo" rev-list --no-merges --max-count=257 "$base" "^$previous_base") ||
    { unbound_not_observed window-unavailable; return; }
  if [[ $(printf '%s' "$window" | grep -c .) -gt 256 ]]; then
    unbound_not_observed window-exceeds-256-commits
    return
  fi
  while IFS= read -r sidecar_commit; do
    if is_seal_commit "$sidecar_commit"; then
      covered+=$sidecar_commit$'\n'
      continue
    fi
    sidecar_base=$(cem_base_revision "$sidecar_commit") || { unbound_not_observed window-cem-base-unavailable; return; }
    bound=$(git -C "$repo" rev-list "$sidecar_commit" "^$sidecar_base" "^$previous_base")
    if [[ $(git -C "$repo" log -1 --format='%(trailers:key=Corvint-Dogfood-Binding,valueonly)' "$sidecar_commit") == retroactive ]]; then
      retroactive_pairs+=$(printf '%s\n' "$bound" | sed "s/\$/ $sidecar_commit/")$'\n'
    else
      covered+=$bound$'\n'
    fi
  done < <(git -C "$repo" rev-list --full-history --no-merges "$base" "^$previous_base" -- .corvint/change.cem.json)
  not_normal=$(printf '%s\n' "$window" | grep -vxF -f <(printf '%s\n' "$covered"))
  [[ -n $not_normal ]] || return
  # A normal binding takes precedence; the newest retroactive binding names a commit.
  unbound=$(awk 'FILENAME == ARGV[1] { retro[$1] = 1; next } $0 != "" && !($1 in retro)' \
    <(printf '%s' "$retroactive_pairs") <(printf '%s\n' "$not_normal"))
  retroactive=$(awk 'FILENAME == ARGV[1] { if (!($1 in retro)) retro[$1] = $2; next } $1 in retro { print $1 " binding=" retro[$1] }' \
    <(printf '%s' "$retroactive_pairs") <(printf '%s\n' "$not_normal"))
  if [[ -n $unbound ]]; then
    count=$(printf '%s\n' "$unbound" | grep -c .)
    printf 'dogfood-check: NOTE unbound-commits count=%d window=%s..%s\n' "$count" "$previous_base" "$base" >&2
    printf '%s\n' "$unbound" | sed 's/^/  unbound /' >&2
  fi
  if [[ -n $retroactive ]]; then
    count=$(printf '%s\n' "$retroactive" | grep -c .)
    printf 'dogfood-check: NOTE retroactive-bound-commits count=%d window=%s..%s\n' "$count" "$previous_base" "$base" >&2
    printf '%s\n' "$retroactive" | sed 's/^/  retroactive /' >&2
  fi
}

resolve_anchor
if is_seal_commit "$target"; then
  printf 'dogfood-check: REFUSE sealed-head\n' >&2
  printf '  HEAD only seals the bound CEM; check its parent: git checkout --detach HEAD^\n' >&2
  exit 2
fi
git -C "$repo" diff --quiet "$base" "$target" -- .
diff_status=$?
if [[ $diff_status -gt 1 ]]; then
  exit 2
fi
worktree_status=$(git -C "$repo" status --porcelain --untracked-files=all) || exit 2
if [[ -n $worktree_status ]]; then
  refusal=dirty-worktree
  if [[ $diff_status -eq 0 ]]; then
    refusal=uncommitted-change-not-in-base-target
  fi
  printf 'dogfood-check: REFUSE %s\n' "$refusal" >&2
  printf '  required order: commit the change; make dogfood-change BASE=<sha>; commit .corvint/change.cem.json; make dogfood-change BASE=<sha>; make dogfood-check BASE=<sha>\n' >&2
  exit 2
fi
if [[ $diff_status -eq 0 ]]; then
  printf 'dogfood-check: PASS no-change-clean-worktree\n'
  exit 0
fi
report_unbound_commits
if [[ -n ${DOGFOOD_EXCEPTION:-} ]]; then
  printf 'dogfood-check: FAIL exception-contract-undefined\n' >&2
  exit 2
fi
if [[ ! -f $report ]]; then
  printf 'dogfood-check: FAIL dogfood-report-missing\n' >&2
  printf '  fix: run make dogfood-change BASE=%s on this HEAD until it reports complete\n' "$base" >&2
  # A reviewer's clone of a bind commit never has the author's private report (DCW-V0-017).
  if git -C "$repo" cat-file -e "$target:.corvint/change.cem.json" 2>/dev/null; then
    printf '  review: verifier agreement is author-only evidence (docs/DOGFOOD.md step 11); verify the bound CEM instead: corvint cem verify --map .corvint/change.cem.json --expected-base %s --target %s\n' "$base" "$target" >&2
  fi
  exit 1
fi
report_base=$(sed -n 's/^  "base": "\([0-9a-f]*\)",$/\1/p' "$report")
report_target=$(sed -n 's/^  "target": "\([0-9a-f]*\)",$/\1/p' "$report")
if [[ $report_base != "$base" || $report_target != "$target" ]]; then
  printf 'dogfood-check: FAIL dogfood-report-drift\n' >&2
  printf '  fix: the report binds another BASE or HEAD; rerun make dogfood-change BASE=%s on this HEAD\n' "$base" >&2
  exit 1
fi
if ! rg -q '"complete": true' "$report"; then
  printf 'dogfood-check: FAIL dogfood-report-drift\n' >&2
  printf '  fix: the report is not complete; resolve the rows make dogfood-change BASE=%s lists, then rerun it\n' "$base" >&2
  exit 1
fi
if [[ $anchor_state == OBSERVED ]]; then
  if ! rg -q '^  ,"anchor": \{"state": "OBSERVED", "mergeBase": "'"$anchor_merge_base"'"\}$' "$report"; then
    printf 'dogfood-check: FAIL dogfood-report-drift\n' >&2
    exit 1
  fi
elif ! rg -q '^  ,"anchor": \{"state": "NOT_OBSERVED", "mergeBase": null\}$' "$report"; then
  printf 'dogfood-check: FAIL dogfood-report-drift\n' >&2
  exit 1
fi
evidence_sha=$(sed -n 's/^  ,"localOutcomeEvidenceSha256": "sha256:\([0-9a-f]\{64\}\)"$/\1/p' "$report")
if [[ -z $evidence_sha || ! -f $local_outcome_evidence ]] ||
  [[ $(shasum -a 256 "$local_outcome_evidence" | awk '{print $1}') != "$evidence_sha" ]]; then
  printf 'dogfood-check: FAIL local-outcome-evidence-drift\n' >&2
  exit 1
fi
context_abstention_sha=$(sed -n 's/^  ,"contextAbstentionEvidenceSha256": "sha256:\([0-9a-f]\{64\}\)"$/\1/p' "$report")
if rg -q '"name": "prechange-impact", "status": "NOT_PRODUCED", "reason": "unsupported-impact-range"' "$report"; then
  context_abstention=1
  if [[ -z $context_abstention_sha || ! -f $context_abstention_evidence ]] ||
    [[ $(shasum -a 256 "$context_abstention_evidence" | awk '{print $1}') != "$context_abstention_sha" ]]; then
    printf 'dogfood-check: FAIL context-abstention-evidence-drift\n' >&2
    exit 1
  fi
  actual_argv=()
  if [[ -f $evidence/prechange-impact.argv ]]; then
    while IFS= read -r -d '' value; do actual_argv+=("$value"); done < "$evidence/prechange-impact.argv"
  fi
  actual_argv_sha=$(shasum -a 256 "$evidence/prechange-impact.argv" 2>/dev/null | awk '{print $1}')
  canonical_argv_sha=$(printf '%s\0' "${actual_argv[@]}" | shasum -a 256 | awk '{print $1}')
  stdout_sha=$(shasum -a 256 "$evidence/prechange-impact.json" 2>/dev/null | awk '{print $1}')
  stderr_sha=$(shasum -a 256 "$evidence/prechange-impact.stderr" 2>/dev/null | awk '{print $1}')
  expected_artifact='{"argvSha256":"sha256:'"$actual_argv_sha"'","base":"'"$base"'","exitStatus":"2","profile":"corvint-dogfood-context-abstention/0","reason":"unsupported-impact-range","status":"NOT_PRODUCED","stderrSha256":"sha256:'"$stderr_sha"'","stdoutSha256":"sha256:'"$stdout_sha"'","step":"prechange-impact","target":"'"$target"'"}'
  if [[ ! -f $evidence/prechange-impact.argv || ! -f $evidence/prechange-impact.json || ! -f $evidence/prechange-impact.stderr ]] ||
    [[ $actual_argv_sha != "$canonical_argv_sha" || ${#actual_argv[@]} -ne 10 || -z ${actual_argv[0]:-} || ${actual_argv[1]:-} != --root || ${actual_argv[2]:-} != "$repo" ||
      ${actual_argv[3]:-} != impact || ${actual_argv[4]:-} != --base || ${actual_argv[5]:-} != "$base" ||
      ${actual_argv[6]:-} != --range-profile || ${actual_argv[7]:-} != expanded-256 ||
      ${actual_argv[8]:-} != --limit || ${actual_argv[9]:-} != 20 ]] ||
    [[ $(wc -l < "$evidence/prechange-impact.stderr") -ne 1 || -s $evidence/prechange-impact.json ]] ||
    ! rg -q '^\{"code": "unsupported-impact-range", "error": "[A-Za-z0-9 ._/:()-]+", "ok": false\}$' "$evidence/prechange-impact.stderr" ||
    [[ $(cat "$context_abstention_evidence") != "$expected_artifact" ]]; then
    printf 'dogfood-check: FAIL context-abstention-evidence-drift\n' >&2
    exit 1
  fi
elif [[ -n $context_abstention_sha || -e $context_abstention_evidence ]]; then
  printf 'dogfood-check: FAIL context-abstention-evidence-drift\n' >&2
  exit 1
elif ! rg -q '^  ,"contextAbstentionEvidenceSha256": null$' "$report"; then
  printf 'dogfood-check: FAIL dogfood-report-drift\n' >&2
  exit 1
fi
run_tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-check.XXXXXX") || exit 2
resolve_corvint_bins
if [[ $context_abstention -eq 1 ]]; then
  impact_arguments=(impact --base "$base" --range-profile expanded-256 --limit 20)
  run_verifier base-impact "$base_corvint_bin" "${impact_arguments[@]}"
  base_status=$verifier_status
  run_verifier tree-impact "$corvint_bin" "${impact_arguments[@]}"
  tree_status=$verifier_status
  if ! verifiers_agree base-impact tree-impact "$base_status" "$tree_status" ||
    [[ $tree_status -ne 2 || -s $run_tmp/tree-impact.stdout ]] ||
    ! cmp -s "$run_tmp/tree-impact.stderr" "$evidence/prechange-impact.stderr"; then
    printf 'dogfood-check: FAIL verifier-disagreement\n' >&2
    exit 1
  fi
  if [[ -n $override_corvint_bin ]]; then
    run_verifier override-impact "$override_corvint_bin" "${impact_arguments[@]}"
    override_status=$verifier_status
    if ! verifiers_agree tree-impact override-impact "$tree_status" "$override_status"; then
      printf 'dogfood-check: FAIL verifier-disagreement\n' >&2
      exit 1
    fi
  fi
fi
if [[ ! -f $repo/.corvint/change.cem.json ]]; then
  printf 'dogfood-check: FAIL cem-map-missing\n' >&2
  sed -n 's/.*"name": "\([^"]*\)", "status": "NOT_PRODUCED", "reason": "\([^"]*\)".*/  missing \1: \2/p' "$report" >&2
  exit 1
fi
if [[ ! -f $intent_snapshot || ! -f $aggregate_status ]]; then
  printf 'dogfood-check: FAIL missing-intent-scope\n' >&2
  exit 1
fi
run_verifier base-ocm "$base_corvint_bin" dogfood-ocm status --expected-base "$base" --target "$target"
base_status=$verifier_status
run_verifier tree-ocm "$corvint_bin" dogfood-ocm status --expected-base "$base" --target "$target"
tree_status=$verifier_status
if ! verifiers_agree base-ocm tree-ocm "$base_status" "$tree_status"; then
  record_dogfood_check false 0 || :
  printf 'dogfood-check: FAIL verifier-disagreement\n' >&2
  exit 1
fi
if [[ -n $override_corvint_bin ]]; then
  run_verifier override-ocm "$override_corvint_bin" dogfood-ocm status --expected-base "$base" --target "$target"
  override_status=$verifier_status
  if ! verifiers_agree tree-ocm override-ocm "$tree_status" "$override_status"; then
    record_dogfood_check false 0 || :
    printf 'dogfood-check: FAIL verifier-disagreement\n' >&2
    exit 1
  fi
fi
if [[ $tree_status -ne 0 ]]; then
  reason=$(sed -n 's/.*"code"[[:space:]]*:[[:space:]]*"\([A-Za-z0-9_-]*\)".*/\1/p' "$run_tmp/tree-ocm.stderr" | head -1)
  record_dogfood_check true 0 || :
  printf 'dogfood-check: FAIL %s\n' "${reason:-ocm-policy}" >&2
  exit 1
fi
if ! cmp -s "$run_tmp/tree-ocm.stdout" "$aggregate_status"; then
  record_dogfood_check true 0 || :
  printf 'dogfood-check: FAIL intent-scope-drift\n' >&2
  printf '  fix: the OCM maps changed after make dogfood-change; rerun make dogfood-change BASE=%s\n' "$base" >&2
  exit 1
fi
bootstrap_unknown=$(count_bootstrap_unknowns)
if ! rg -Fq '  ,"dogfoodPolicy": {"bootstrapUnknown": '"$bootstrap_unknown"', "maximumUnknownAfterBootstrap": 0}' "$report"; then
  record_dogfood_check true "$bootstrap_unknown" || :
  printf 'dogfood-check: FAIL dogfood-report-drift\n' >&2
  exit 1
fi
cem_arguments=(cem status --map .corvint/change.cem.json --expected-base "$base" --target "$target"
  --max-unknown "$bootstrap_unknown" --max-mechanical 0)
run_verifier base-cem "$base_corvint_bin" "${cem_arguments[@]}"
base_status=$verifier_status
run_verifier tree-cem "$corvint_bin" "${cem_arguments[@]}"
tree_status=$verifier_status
if ! verifiers_agree base-cem tree-cem "$base_status" "$tree_status"; then
  record_dogfood_check false "$bootstrap_unknown" || :
  printf 'dogfood-check: FAIL verifier-disagreement\n' >&2
  exit 1
fi
if [[ -n $override_corvint_bin ]]; then
  run_verifier override-cem "$override_corvint_bin" "${cem_arguments[@]}"
  override_status=$verifier_status
  if ! verifiers_agree tree-cem override-cem "$tree_status" "$override_status"; then
    record_dogfood_check false "$bootstrap_unknown" || :
    printf 'dogfood-check: FAIL verifier-disagreement\n' >&2
    exit 1
  fi
fi
record_dogfood_check true "$bootstrap_unknown" || {
  printf 'dogfood-check: FAIL dogfood-report-write\n' >&2
  exit 1
}
sed -n '1p' "$run_tmp/tree-cem.stdout"
if [[ $tree_status -ne 0 ]]; then
  printf 'dogfood-check: FAIL cem-policy\n' >&2
  sed -n 's/.*"name": "\([^"]*\)", "status": "NOT_PRODUCED", "reason": "\([^"]*\)".*/  missing \1: \2/p' "$report" >&2
  exit 1
fi
sed -n '1p' "$run_tmp/tree-ocm.stdout"
printf 'dogfood-check: PASS\n'
