#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/tasks-reconcile.sh   (make tasks-reconcile)
#
# Post-merge closeout reconcile (TCD-V0-872, V1-3496). It exports merged PRs with gh, runs
# `corvint-tasks ticket closeout-debt --ref REF --prs EXPORT`, then exports the CI runs of each
# proposal's merge commit and PR head with `gh run list --commit SHA` (TCD-V0-2401, V1-4449: a
# latest-N window misses an older merge's green run), and retains the exports and the report
# under <git-common-dir>/corvint/reconcile/<refCommit>/. It prints each batch-close proposal
# (its command with the retained exports filled in) and the tickets left for review, and exits with
# the closeout-debt status: 0 no debt, 1 debt or proposals, 2 refusal. A failed export, a missing
# binary or an unparsable report exits 2 with the reason on stderr; it never skips silently.
#
# Inputs (environment):
#   RECONCILE_REF    integration ref (default origin/main)
#   RECONCILE_PRS    merged PRs to export (default 100)
#   RECONCILE_APPLY  unset (default): apply nothing. dry-run: run each proposal's batch-close
#                    with --dry-run --manual. manual: with --manual. gated: the gated default.
#                    Any failing batch-close makes the exit non-zero.
#   CORVINT_TASKS    corvint-tasks executable (default corvint-tasks on PATH); GH likewise for gh.
set -euo pipefail

ref=${RECONCILE_REF:-origin/main}
limit=${RECONCILE_PRS:-100}
apply=${RECONCILE_APPLY:-}
tasks=${CORVINT_TASKS:-corvint-tasks}
gh=${GH:-gh}

die() { echo "tasks-reconcile: $*" >&2; exit 2; }
case $apply in '' | dry-run | manual | gated) ;; *) die "RECONCILE_APPLY must be dry-run, manual or gated, not $apply" ;; esac
command -v "$tasks" > /dev/null 2>&1 || die "corvint-tasks not found ($tasks)"
command -v "$gh" > /dev/null 2>&1 || die "gh not found ($gh)"
command -v jq > /dev/null 2>&1 || die "jq not found"
commit=$(git rev-parse --verify --quiet "$ref^{commit}") || die "ref $ref does not name a commit"
common=$(git rev-parse --path-format=absolute --git-common-dir) || die "not in a Git repository"
dir=$common/corvint/reconcile/$commit
mkdir -p "$dir"

"$gh" pr list --state merged --limit "$limit" \
  --json number,state,title,body,headRefName,headRefOid,mergeCommit,labels,mergedAt,url > "$dir/prs.json" ||
  die "gh pr list failed; no reconcile for $commit"

set +e
"$tasks" ticket closeout-debt --ref "$ref" --prs "$dir/prs.json" < /dev/null > "$dir/report.json"
status=$?
set -e
jq -e '.items[0].profile == "taskman-closeout-debt/0"' "$dir/report.json" > /dev/null 2>&1 ||
  die "closeout-debt exit $status without a taskman-closeout-debt/0 report ($dir/report.json)"

echo "tasks-reconcile $commit: report $dir/report.json"
jq -r '.items[0].human[]' "$dir/report.json"

# The runs of each proposal's merge commit and PR head, one entry per run, so batch-close finds a
# green run however old the merge is (TCD-V0-2401). The fields let a gated close derive CI gate
# evidence (TCD-V0-770). A failed or unreadable export exits 2; it never becomes an empty file.
runs_dir=$(mktemp -d "$dir/runs.XXXXXX") || die "cannot create a runs directory in $dir"
trap '/bin/rm -rf "$runs_dir"' EXIT
runs_files=()
shas=$(jq -r --slurpfile prs "$dir/prs.json" '.items[0].proposals[]? | .pr as $pr |
  .mergeCommit, ($prs[0][]? | select((.number | tostring) == ($pr | tostring)) | .headRefOid // empty)' "$dir/report.json" | sort -u) ||
  die "cannot read the proposal commits from $dir/report.json; no runs export for $commit"
for sha in $shas; do
  [[ $sha =~ ^[0-9a-f]{40}$ ]] || die "proposal commit $sha is not a full commit id; no runs export for $commit"
  "$gh" run list --commit "$sha" --limit 100 --json databaseId,headSha,conclusion,url,workflowName,event,updatedAt > "$runs_dir/$sha.json" ||
    die "gh run list --commit $sha failed; no runs export for $commit"
  runs_files+=("$runs_dir/$sha.json")
done
if ((${#runs_files[@]})); then
  jq -cs 'add | unique_by(.databaseId)' "${runs_files[@]}" > "$dir/runs.json" ||
    die "gh run list --commit printed no JSON run array; no runs export for $commit"
else
  echo '[]' > "$dir/runs.json"
fi

failed=0
if [[ -n $apply ]]; then
  # Each proposal's batch-close over the retained exports (the command's RUNS placeholder).
  for pr in $(jq -r '.items[0].proposals[]?.pr' "$dir/report.json"); do
    # A proposal is advisory until its immutable delivery evidence is on origin/main.
    evidence=$(jq -r --arg pr "$pr" '.items[0].proposals[] | select(.pr == $pr) | .mergeCommit' "$dir/report.json")
    if ! git merge-base --is-ancestor "$evidence" refs/remotes/origin/main; then
      echo "tasks-reconcile: proposal #$pr evidence $evidence is not on origin/main; nothing closed for this proposal" >&2
      failed=1
      continue
    fi
    argv=("$tasks" ticket batch-close --pr "$pr" --prs "$dir/prs.json" --runs "$dir/runs.json")
    case $apply in
      dry-run) argv+=(--dry-run --manual) ;;
      manual) argv+=(--manual) ;;
    esac
    echo "apply: ${argv[*]}"
    "${argv[@]}" < /dev/null > "$dir/apply-$pr.json" || failed=1
  done
fi
[[ $failed -eq 0 ]] || { echo "tasks-reconcile: a batch-close failed; outputs in $dir" >&2; exit 1; }
if [[ -n $apply && $apply != dry-run ]]; then
  # Retain the original proposal report and report remaining debt after successful applications.
  set +e
  "$tasks" ticket closeout-debt --ref "$ref" --prs "$dir/prs.json" < /dev/null > "$dir/after.json"
  status=$?
  set -e
  jq -e '.items[0].profile == "taskman-closeout-debt/0"' "$dir/after.json" > /dev/null 2>&1 ||
    die "post-apply closeout-debt exit $status without a report ($dir/after.json)"
fi
exit "$status"
