#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-047: collect one completed CI run's forward-row inputs and classify them.
# The ci-control-plane forward-rows job runs this default-branch copy with gh and
# jq only; it downloads small artifacts and never runs code of the tested commit.
# Environment: GH_TOKEN, REPO, RUN_ID, EVENT (merge_group|pull_request), HEAD_SHA,
# HEAD_BRANCH, PR_NUMBER (may be empty), FORWARD_SHARDS, FORWARD_EPOCH,
# RUNNER_TEMP, GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes
# $RUNNER_TEMP/forward-row/row.json and the step output `name`; a run that is
# not a forward row (not a queue branch of main, no pull request, or a run that
# executed neither a full shard nor the shadow) writes neither and says why.
set -u
here=$(cd "$(dirname "$0")" && pwd)
dir="$RUNNER_TEMP/forward-row"
mkdir -p "$dir/selection" "$dir/outcomes" "$dir/reuse"
collect=""
note() {
  collect="$collect$1
"
}
say() { printf '%s\n' "$1" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"; }
names() { gh api "repos/$REPO/actions/runs/$1/artifacts?per_page=100" --jq '.artifacts[] | select(.expired | not) | .name'; }
fetch() { gh run download "$1" --repo "$REPO" --pattern "$2" --dir "$3" > /dev/null 2>&1; }
hex40() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
digits() { printf '%s' "$1" | grep -Eqx '[0-9]+'; }

if ! digits "$RUN_ID" || ! hex40 "$HEAD_SHA"; then
  say "forward row: run $RUN_ID at $HEAD_SHA is not a well-formed run; no row"
  exit 0
fi
sha="" head="" pr="" selection_run="" outcomes_run="$RUN_ID" reuse=""
case "$EVENT" in
merge_group)
  case "$HEAD_BRANCH" in
  gh-readonly-queue/main/pr-*) ;;
  *)
    say "forward row: $HEAD_BRANCH is not a merge-queue branch of main; no row"
    exit 0
    ;;
  esac
  pr=${HEAD_BRANCH#gh-readonly-queue/main/pr-}
  pr=${pr%%-*}
  kind=queue sha=$HEAD_SHA
  head=$(gh api "repos/$REPO/commits/$sha" --jq '.parents | if length == 2 then .[1].sha else empty end' 2> /dev/null) || head=""
  hex40 "$head" || {
    note "the second parent of queue commit $sha is unreadable or absent"
    head=""
  }
  if run_names=$(names "$RUN_ID"); then
    if printf '%s\n' "$run_names" | grep -qx ci-reuse; then
      if fetch "$RUN_ID" ci-reuse "$dir/reuse" && [ -f "$dir/reuse/ci-reuse/decision.json" ]; then
        reuse="$dir/reuse/ci-reuse/decision.json"
        if jq -e '.mode == "REUSE"' "$reuse" > /dev/null 2>&1; then
          outcomes_run=$(jq -r '.runId' "$reuse")
          digits "$outcomes_run" || {
            note "the REUSE decision names no reused run"
            outcomes_run=""
          }
        fi
      else
        note "the ci-reuse decision of run $RUN_ID could not be downloaded"
      fi
    fi
  else
    note "the artifacts of run $RUN_ID are unreadable"
  fi
  if [ -n "$head" ]; then
    selection_run=$(gh api "repos/$REPO/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$head&status=success&per_page=20" \
      --jq '[.workflow_runs[]] | max_by(.run_started_at) | .id // empty' 2> /dev/null) || selection_run=""
    digits "$selection_run" || {
      note "no successful pull_request run of head $head is readable, so no selection"
      selection_run=""
    }
  fi
  ;;
pull_request)
  pr=$PR_NUMBER
  digits "$pr" || {
    say "forward row: run $RUN_ID names no pull request of this repository; no row"
    exit 0
  }
  kind=pr head=$HEAD_SHA selection_run=$RUN_ID
  if ! run_names=$(names "$RUN_ID"); then
    note "the artifacts of run $RUN_ID are unreadable"
  elif ! printf '%s\n' "$run_names" | grep -Eq '^(ci-outcomes-pull_request-|corvint-test-shadow-)'; then
    say "forward row: pull_request run $RUN_ID executed no full shard and no shadow plan (DOCS, batched or skipped); no row"
    exit 0
  fi
  ;;
*)
  say "forward row: a $EVENT run is not a forward row"
  exit 0
  ;;
esac
digits "$pr" || {
  say "forward row: no pull request number in $HEAD_BRANCH; no row"
  exit 0
}
if [ -n "$outcomes_run" ]; then
  fetch "$outcomes_run" 'ci-outcomes-*' "$dir/outcomes" || note "run $outcomes_run retained no ci-outcomes artifact"
fi
if [ -n "$selection_run" ]; then
  fetch "$selection_run" 'corvint-test-shadow-*' "$dir/selection" || note "run $selection_run retained no corvint-test-shadow artifact"
fi
class=$(ROW_KIND=$kind ROW_PR=$pr ROW_SHA=$sha ROW_HEAD=$head ROW_RUN=$RUN_ID ROW_SELECTION_RUN=$selection_run \
  ROW_OUTCOMES_RUN=$outcomes_run ROW_SHARDS=$FORWARD_SHARDS ROW_EPOCH=$FORWARD_EPOCH \
  ROW_SELECTION_DIR="$dir/selection" ROW_OUTCOMES_DIR="$dir/outcomes" ROW_REUSE=$reuse ROW_COLLECT=$collect \
  sh "$here/classify.sh" "$dir/row.json")
case "$class" in CLEAN | FLAKY | MISS | INCOMPLETE) ;; *) class=INCOMPLETE ;; esac
row_sha=$(jq -r '.sha // empty' "$dir/row.json" 2> /dev/null) || row_sha=""
hex40 "$row_sha" || row_sha=$HEAD_SHA
failing=$(jq -r 'if (.counts.failed // 0) > 0 then 1 else 0 end' "$dir/row.json" 2> /dev/null) || failing=0
[ "$failing" = 1 ] || failing=0
name="ci-forward-$class-pr$pr-$row_sha-e$FORWARD_EPOCH-f$failing"
echo "name=$name" >> "$GITHUB_OUTPUT"
say "forward row \`$name\` ($kind run $RUN_ID; selection from run ${selection_run:-none}; outcomes from run ${outcomes_run:-none})"
jq -r '"counts: \(.counts // {} | tojson); incomplete: \(.incomplete // [] | .[0:5] | join("; "))"' "$dir/row.json" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}" 2> /dev/null || true
