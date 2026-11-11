#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-983 (V1-3750): link one failed full push run of main back to the narrowed
# pull requests merged since the last green full run whose selection skipped a
# failed package. The ci-control-plane main-miss job runs this default-branch copy
# with gh and jq only; it downloads small artifacts and runs no tested code.
# Environment: GH_TOKEN, REPO, RUN_ID, HEAD_SHA, FORWARD_EPOCH, RUNNER_TEMP,
# GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes $RUNNER_TEMP/main-miss/miss.json and
# the step output `name` only when the run names a miss; otherwise it says why not. A pull
# request whose run or narrowing records cannot be read could have skipped a failed package, so
# any such gap abstains: no row, with every unavailable input named.
# CIN-V0-7922 (V1-5198): the row is a suspicion. The main-miss workflow confirms it by reruns
# (mainmiss-confirm.sh), so each miss also names the pull request's merge commit on main and the
# forward epoch its pull-request run stamped. A pull request narrowed under an earlier epoch than
# FORWARD_EPOCH is no miss of this epoch (CIN-V0-001): it is named in the summary and in
# `earlierEpoch`, and is left out of the row.
set -u
here=$(cd "$(dirname "$0")" && pwd)
dir="$RUNNER_TEMP/main-miss"
mkdir -p "$dir/failed" "$dir/prs" "$dir/sel"
say() { printf '%s\n' "$1" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"; }
fetch() { gh run download "$1" --repo "$REPO" --pattern "$2" --dir "$3" > /dev/null 2>&1; }
names() { gh api "repos/$REPO/actions/runs/$1/artifacts?per_page=100" --jq '.artifacts[] | select(.expired | not) | .name'; }
unavailable=""
gap() {
  unavailable="$unavailable${unavailable:+; }$1"
}
hex40() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
digits() { printf '%s' "$1" | grep -Eqx '[0-9]+'; }
MAX_COMMITS=100
MAX_PRS=20

if ! digits "$RUN_ID" || ! hex40 "$HEAD_SHA" || ! digits "$FORWARD_EPOCH"; then
  say "main miss: run $RUN_ID at $HEAD_SHA is not a well-formed run; no row"
  exit 0
fi
if ! fetch "$RUN_ID" 'ci-failed-push-*' "$dir/failed"; then
  say "main miss: run $RUN_ID retained no ci-failed-push artifact, so its failed packages are unknown; no row"
  exit 0
fi
find "$dir/failed" -name failed.json -type f -exec cat {} + > "$dir/failed.jsonl" 2> /dev/null || : > "$dir/failed.jsonl"
if ! jq -e -s '[.[] | .reasons[]?] | length > 0' "$dir/failed.jsonl" > /dev/null 2>&1; then
  say "main miss: run $RUN_ID names no failed package; no row"
  exit 0
fi
# The newest earlier successful push run of ci.yml on main is the last green full run.
green=$(gh api "repos/$REPO/actions/workflows/ci.yml/runs?branch=main&event=push&status=success&per_page=50" \
  --jq "[.workflow_runs[] | select(.id < $RUN_ID) | .head_sha] | first // empty" 2> /dev/null) || green=""
if ! hex40 "$green"; then
  say "main miss: no earlier green full run of main is listed; no row"
  exit 0
fi
commits=$(gh api "repos/$REPO/compare/$green...$HEAD_SHA" \
  --jq "if .status != \"ahead\" or (.total_commits // 0) > $MAX_COMMITS then \"unbounded\" else .commits[].sha end" 2> /dev/null) || commits=""
case $commits in
"" | unbounded)
  say "main miss: the commits from $green to $HEAD_SHA are unreadable, not ahead or more than $MAX_COMMITS; no row"
  exit 0
  ;;
esac
: > "$dir/pulls"
for c in $commits; do
  hex40 "$c" || continue
  # CIN-V0-7922: the merge commit of each pull request goes along, so the confirmation can rerun
  # the skipped package at that commit and its first parent; one that names none is "-".
  gh api "repos/$REPO/commits/$c/pulls" --jq '.[] | select(.merged_at != null and .base.ref == "main") | "\(.number) \(.head.sha) \(.merge_commit_sha // "-")"' \
    >> "$dir/pulls" 2> /dev/null || gap "the pull requests of commit $c are unreadable"
done
sort -u "$dir/pulls" > "$dir/pulls.sorted"
if [ "$(wc -l < "$dir/pulls.sorted" | tr -d ' ')" -gt "$MAX_PRS" ]; then
  say "main miss: more than $MAX_PRS pull requests merged since $green; no row"
  exit 0
fi
: > "$dir/prs.jsonl"
while read -r pr head merge; do
  digits "$pr" && hex40 "$head" || continue
  hex40 "$merge" || merge=""
  run=$(gh api "repos/$REPO/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$head&per_page=20" \
    --jq '[.workflow_runs[] | .id] | max // empty' 2> /dev/null) || run=""
  if ! digits "$run"; then
    gap "no pull_request run of #$pr at $head is readable"
    continue
  fi
  if ! run_names=$(names "$run" 2> /dev/null); then
    gap "the artifacts of run $run of #$pr are unreadable"
    continue
  fi
  # A run that kept no narrowing record ran every shard in full: it skipped nothing.
  printf '%s\n' "$run_names" | grep -q '^ci-narrowing-' || continue
  records="$dir/prs/$pr"
  mkdir -p "$records"
  if ! fetch "$run" 'ci-narrowing-*' "$records"; then
    gap "the ci-narrowing records of run $run of #$pr could not be downloaded"
    continue
  fi
  # CIN-V0-7922: the forward epoch that run stamped on its selection documents, read as collect.sh
  # reads it (CIN-V0-001): one non-negative integer across all of them, or unknown. A run whose
  # selection documents are gone leaves the epoch unknown; that is not a gap.
  epoch=""
  if fetch "$run" 'corvint-test-shadow-*' "$dir/sel/$pr"; then
    epoch=$(find "$dir/sel/$pr" -name selection.json -type f -exec cat {} + 2> /dev/null | jq -er -s '
      if length > 0 then (map(.forwardEpoch) | unique) as $e
        | if ($e | length) == 1 and ($e[0] | type) == "number" and $e[0] >= 0 and $e[0] == ($e[0] | floor) then $e[0] else empty end
      else empty end' 2> /dev/null) || epoch=""
  fi
  digits "$epoch" || epoch=""
  find "$records" -name narrowing.json -type f -exec cat {} + 2> /dev/null |
    jq -s -c --arg pr "$pr" --arg head "$head" --arg merge "$merge" --arg epoch "$epoch" \
      '{pr: ($pr | tonumber), head: $head, merge: (if $merge == "" then null else $merge end),
        epoch: (if $epoch == "" then null else ($epoch | tonumber) end), records: .}' >> "$dir/prs.jsonl" 2> /dev/null ||
    gap "the ci-narrowing records of run $run of #$pr are unreadable"
done < "$dir/pulls.sorted"
if [ -n "$unavailable" ]; then
  say "main miss: abstained for run $RUN_ID, so no row: $unavailable"
  exit 0
fi
if ! jq -n -c --slurpfile failed "$dir/failed.jsonl" --slurpfile prs "$dir/prs.jsonl" \
  --arg runId "$RUN_ID" --arg sha "$HEAD_SHA" --arg epoch "$FORWARD_EPOCH" -f "$here/mainmiss.jq" > "$dir/miss.json" 2> "$dir/miss.err"; then
  say "main miss: the miss classification failed: $(head -c 200 "$dir/miss.err"); no row"
  exit 0
fi
jq -r '.epoch as $e | .earlierEpoch[]? | "main miss: #\(.pr) skipped \(.packages | join(", ")) under forward epoch \(.epoch), earlier than \($e); it is no miss of this epoch (CIN-V0-001)"' "$dir/miss.json" |
  while read -r line; do say "$line"; done
name=$(jq -r '.name // empty' "$dir/miss.json")
if [ -z "$name" ]; then
  if jq -e '(.earlierEpoch | length) > 0' "$dir/miss.json" > /dev/null 2>&1; then
    say "main miss: run $RUN_ID failed $(jq -r '.failed | join(", ")' "$dir/miss.json"); no narrowed pull request of epoch $FORWARD_EPOCH merged since $green skipped them; no row"
  else
    say "main miss: run $RUN_ID failed $(jq -r '.failed | join(", ")' "$dir/miss.json"); no narrowed pull request merged since $green skipped them; no row"
  fi
  exit 0
fi
jq -r '"main miss `\(.name)`: " + ([.misses[] | "#\(.pr) skipped \(.packages | join(", "))"] | join("; "))' "$dir/miss.json" |
  while read -r line; do say "$line"; done
echo "name=$name" >> "$GITHUB_OUTPUT"
