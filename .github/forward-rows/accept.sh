#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-097: decide whether a pull request may narrow from the trusted verdict.
# The PR's narrowing-verdict job runs this copy from the pull request's base
# commit, with gh and jq only. Environment: GH_TOKEN, REPO, FORWARD_EPOCH,
# RUNNER_TEMP, GITHUB_OUTPUT, GITHUB_STEP_SUMMARY, VERDICT_WORKFLOW (the trusted
# workflow path), NOW (seconds since the epoch; default the clock) and
# KILL_PAGES (default 25). Writes `verdict=MET` only when the newest artifact
# named exactly afp-shadow-bar-epoch-<FORWARD_EPOCH>, by created_at:
#   (a) came from a schedule or workflow_dispatch run of VERDICT_WORKFLOW on
#       refs/heads/main of this repository,
#   (b) says MET with a clear kill switch,
#   (c) was created at most 7 days ago and has not expired, and
#   (d) names the current forward epoch;
# and the newest KILL_PAGES pages of forward rows show no MISS of the epoch and
# no tripped kill switch since, where those pages either end the listing or reach
# back to the verdict's creation and hold the latest 20 rows of the epoch (or its
# start). Anything missing, expired, unparseable, uncovered or not MET writes
# `verdict=FULL` with the reason: the pull request runs full CI.
set -u
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=names.sh
. "$here/names.sh"
dir="$RUNNER_TEMP/afp-shadow-bar-accept"
mkdir -p "$dir"
name="afp-shadow-bar-epoch-$FORWARD_EPOCH"
now=${NOW:-$(date -u +%s)}
decide() { # VERDICT REASON [RUN]
  {
    echo "verdict=$1"
    echo "reason=$2"
    echo "verdictRun=${3:-}"
  } >> "$GITHUB_OUTPUT"
  # AFP-V0-461 (V1-2707): the verdict and its reason also reach the job log, as a line and
  # a notice annotation, so a declined narrowing is visible without the step summary.
  echo "AFP-V0-097 narrowing verdict: $1 ($2)"
  echo "::notice title=AFP-V0-097 narrowing verdict::$1 ($2)"
  [ -z "${GITHUB_STEP_SUMMARY:-}" ] || echo "AFP-V0-097 narrowing verdict: $1 ($2)" >> "$GITHUB_STEP_SUMMARY"
  exit 0
}
printf '%s' "$FORWARD_EPOCH" | grep -Eqx '[0-9]+' || decide FULL "forward epoch $FORWARD_EPOCH is not a number"
# The epoch is digits, so the name is safe inside the filter.
art=$(gh api "repos/$REPO/actions/artifacts?name=$name&per_page=100" --jq \
  '[.artifacts[] | select(.name == "'"$name"'")] | max_by(.created_at) // empty
   | [.id, .expired, .created_at, .workflow_run.id, .workflow_run.head_branch,
      .workflow_run.repository_id, .workflow_run.head_repository_id] | map(tostring) | join(" ")' 2> /dev/null) ||
  decide FULL "the $name listing is unreadable"
[ -n "$art" ] || decide FULL "no $name artifact exists"
# shellcheck disable=SC2086
set -- $art
[ "$#" -eq 7 ] || decide FULL "the newest $name artifact is malformed"
expired=$2 created=$3 run=$4 branch=$5 repo_id=$6 head_repo_id=$7
printf '%s' "$run" | grep -Eqx '[0-9]+' || decide FULL "the newest $name artifact names no run"
if [ "$branch" != main ] || [ "$repo_id" = null ] || [ "$repo_id" != "$head_repo_id" ]; then
  decide FULL "the newest $name artifact (run $run) is not from main of this repository" "$run"
fi
[ "$expired" = false ] || decide FULL "the newest $name artifact (run $run) has expired" "$run"
age=$(jq -n -r --arg created "$created" --arg now "$now" '($now | tonumber) - ($created | fromdateiso8601) | floor' 2> /dev/null) ||
  decide FULL "the creation time $created of the newest $name artifact is unreadable" "$run"
if [ "$age" -lt 0 ] || [ "$age" -gt 604800 ]; then
  decide FULL "the newest $name artifact (run $run) is $age s old, not within 7 days" "$run"
fi
source=$(gh api "repos/$REPO/actions/runs/$run" --jq '[(.path // "" | sub("@.*$"; "")), .event, .head_branch,
    (.head_repository.full_name // ""), (.repository.full_name // "")] | join(" ")' 2> /dev/null) ||
  decide FULL "run $run of the newest $name artifact is unreadable" "$run"
# shellcheck disable=SC2086
set -- $source
if ! { [ "$#" -eq 5 ] && [ "$1" = "$VERDICT_WORKFLOW" ] && [ "$3" = main ] && [ "$4" = "$REPO" ] && [ "$5" = "$REPO" ] &&
  { [ "$2" = schedule ] || [ "$2" = workflow_dispatch ]; }; }; then
  decide FULL "run $run ($source) is not a schedule or workflow_dispatch run of $VERDICT_WORKFLOW on main" "$run"
fi
gh run download "$run" --repo "$REPO" --pattern "$name" --dir "$dir" > /dev/null 2>&1 ||
  decide FULL "the $name artifact of run $run could not be downloaded" "$run"
jq -e --argjson run "$run" --argjson epoch "$FORWARD_EPOCH" --arg repo "$REPO" \
  '.profile == "afp-shadow-bar-verdict/0" and .runId == $run and .repository == $repo
   and (.mainSha | type == "string" and test("^[0-9a-f]{40}$"))' "$dir/$name/verdict.json" > /dev/null 2>&1 ||
  decide FULL "the $name verdict of run $run is unparseable or names another run" "$run"
status=$(jq -r '.status' "$dir/$name/verdict.json")
[ "$status" = MET ] || decide FULL "the verdict of run $run is $status" "$run"
[ "$(jq -r '.epoch' "$dir/$name/verdict.json")" = "$FORWARD_EPOCH" ] ||
  decide FULL "the verdict of run $run names epoch $(jq -r '.epoch' "$dir/$name/verdict.json"), not $FORWARD_EPOCH" "$run"
[ "$(jq -r '.killSwitch' "$dir/$name/verdict.json")" = clear ] ||
  decide FULL "the verdict of run $run has a tripped or unknown kill switch" "$run"
# The verdict may be up to 7 days old: rows since then can trip the kill switch.
pages=${KILL_PAGES:-25}
# CIN-V0-004 (V1-3091): a verdict that names when its listing began (listedAt) and its
# AFP-V0-030 window rows (windowNames) needs only the rows since then: the listing stops an
# hour before listedAt, and the kill switch is re-checked over those rows followed by the
# window rows, a re-run row replacing its older row as in a whole listing. A verdict without
# both, or with either malformed, keeps the listing below, back to the verdict's creation.
listed_at=$(jq -r --arg epoch "$FORWARD_EPOCH" 'if (.listedAt | type == "string" and test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"))
    and (.windowNames | type == "array" and length >= 1 and length <= 20
         and all(.[]; type == "string" and test("^ci-forward-(CLEAN|FLAKY|MISS|INCOMPLETE)-pr[0-9]+-[0-9a-f]{40}-e" + $epoch + "-f[01]$")))
  then .listedAt else "" end' "$dir/$name/verdict.json" 2> /dev/null) || listed_at=""
if [ -n "$listed_at" ]; then
  stop=$(jq -n -r --arg t "$listed_at" '$t | fromdateiso8601 - 3600 | todateiso8601' 2> /dev/null) ||
    decide FULL "the listing time $listed_at of the verdict of run $run is unreadable" "$run"
  list_names "$pages" "$dir/names" "$dir/times" "$stop"
  [ -z "$limit" ] || decide FULL "the newest forward rows are unreadable: $limit" "$run"
  [ "$exhausted" = 1 ] || [ "$reached" = 1 ] ||
    decide FULL "the newest $pages artifact page(s) do not reach $stop, an hour before the verdict's listing, so rows since it cannot be excluded" "$run"
  jq -r '.windowNames[]' "$dir/$name/verdict.json" >> "$dir/names"
  live=$(jq -R -s -c --arg epoch "$FORWARD_EPOCH" --arg exhausted "$exhausted" --arg format json --rawfile reclassified "$here/infra-reclassified.txt" -f "$here/bar.jq" < "$dir/names") ||
    decide FULL "the newest forward rows could not be classified" "$run"
  [ "$(printf '%s' "$live" | jq -r '.covered or .window.rows >= 20' 2> /dev/null)" = true ] ||
    decide FULL "the rows since the verdict and its window hold fewer than 20 rows of epoch $FORWARD_EPOCH, so the AFP-V0-030 window is not covered" "$run"
  if [ "$(printf '%s' "$live" | jq -r '.killSwitch')" != clear ] || [ "$(printf '%s' "$live" | jq -r '.epochRows.miss')" != 0 ]; then
    decide FULL "the newest forward rows show a MISS or a tripped kill switch" "$run"
  fi
  decide MET "verdict run $run of $created, epoch $FORWARD_EPOCH; $pages_read page(s) back to $stop" "$run"
fi
list_names "$pages" "$dir/names" "$dir/times"
[ -z "$limit" ] || decide FULL "the newest forward rows are unreadable: $limit" "$run"
live=$(jq -R -s -c --arg epoch "$FORWARD_EPOCH" --arg exhausted "$exhausted" --arg format json --rawfile reclassified "$here/infra-reclassified.txt" -f "$here/bar.jq" < "$dir/names") ||
  decide FULL "the newest forward rows could not be classified" "$run"
# A listing that stopped at the page budget proves nothing about the rows it did not read. It
# must reach back to the verdict's creation (the verdict artifact itself is a main artifact),
# so every row since the verdict was read, and hold the whole AFP-V0-030 window: 20 rows of
# the epoch, or the epoch's start. Otherwise a MISS beyond it cannot be excluded.
if [ "$exhausted" != 1 ]; then
  reached=$(jq -R -s -r --arg created "$created" '($created | fromdateiso8601) as $c
    | any(split("\n")[] | select(length > 0) | split(" ")[0] | (try fromdateiso8601 catch null); . != null and . <= $c)' \
    < "$dir/times" 2> /dev/null) || reached=""
  [ "$reached" = true ] ||
    decide FULL "the newest $pages artifact page(s) do not reach the verdict of $created, so rows since it cannot be excluded" "$run"
  [ "$(printf '%s' "$live" | jq -r '.covered or .window.rows >= 20' 2> /dev/null)" = true ] ||
    decide FULL "the newest $pages artifact page(s) hold fewer than 20 rows of epoch $FORWARD_EPOCH and not its start, so the AFP-V0-030 window is not covered" "$run"
fi
if [ "$(printf '%s' "$live" | jq -r '.killSwitch')" != clear ] || [ "$(printf '%s' "$live" | jq -r '.epochRows.miss')" != 0 ]; then
  decide FULL "the newest forward rows show a MISS or a tripped kill switch" "$run"
fi
decide MET "verdict run $run of $created, epoch $FORWARD_EPOCH" "$run"
