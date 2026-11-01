#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/tasks-reconcile.sh   (make tasks-reconcile)
#
# Post-merge closeout reconcile (TCD-V0-872, V1-3496). It exports merged PRs and CI runs with gh,
# runs `corvint-tasks ticket closeout-debt --ref REF --prs EXPORT` and retains the exports and the
# report under <git-common-dir>/corvint/reconcile/<refCommit>/. It prints each batch-close proposal
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
"$gh" run list --limit "$((limit * 4))" --json databaseId,headSha,conclusion,url > "$dir/runs.json" ||
  die "gh run list failed; no reconcile for $commit"

set +e
"$tasks" ticket closeout-debt --ref "$ref" --prs "$dir/prs.json" < /dev/null > "$dir/report.json"
status=$?
set -e
jq -e '.items[0].profile == "taskman-closeout-debt/0"' "$dir/report.json" > /dev/null 2>&1 ||
  die "closeout-debt exit $status without a taskman-closeout-debt/0 report ($dir/report.json)"

echo "tasks-reconcile $commit: report $dir/report.json"
jq -r '.items[0].human[]' "$dir/report.json"

failed=0
if [[ -n $apply ]]; then
  # Each proposal's batch-close over the retained exports (the command's RUNS placeholder).
  for pr in $(jq -r '.items[0].proposals[]?.pr' "$dir/report.json"); do
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
exit "$status"
