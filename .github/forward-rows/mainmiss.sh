#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-983 (V1-3750): link one failed full push run of main back to the narrowed
# pull requests merged since the last green full run whose selection skipped a
# failed package. The ci-control-plane main-miss job runs this default-branch copy
# with gh and jq only; it downloads small artifacts and runs no tested code.
# Environment: GH_TOKEN, REPO, RUN_ID, HEAD_SHA, FORWARD_EPOCH, RUNNER_TEMP,
# GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes $RUNNER_TEMP/main-miss/miss.json and
# the step output `name` only when the run names a miss; otherwise it says why not.
set -u
here=$(cd "$(dirname "$0")" && pwd)
dir="$RUNNER_TEMP/main-miss"
mkdir -p "$dir/failed" "$dir/prs"
say() { printf '%s\n' "$1" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"; }
fetch() { gh run download "$1" --repo "$REPO" --pattern "$2" --dir "$3" > /dev/null 2>&1; }
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
  gh api "repos/$REPO/commits/$c/pulls" --jq '.[] | select(.merged_at != null and .base.ref == "main") | "\(.number) \(.head.sha)"' \
    >> "$dir/pulls" 2> /dev/null || say "main miss: the pull requests of commit $c are unreadable"
done
sort -u "$dir/pulls" > "$dir/pulls.sorted"
if [ "$(wc -l < "$dir/pulls.sorted" | tr -d ' ')" -gt "$MAX_PRS" ]; then
  say "main miss: more than $MAX_PRS pull requests merged since $green; no row"
  exit 0
fi
: > "$dir/prs.jsonl"
while read -r pr head; do
  digits "$pr" && hex40 "$head" || continue
  run=$(gh api "repos/$REPO/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$head&per_page=20" \
    --jq '[.workflow_runs[] | .id] | max // empty' 2> /dev/null) || run=""
  records="$dir/prs/$pr"
  mkdir -p "$records"
  if digits "$run" && fetch "$run" 'ci-narrowing-*' "$records"; then
    find "$records" -name narrowing.json -type f -exec cat {} + 2> /dev/null |
      jq -s -c --arg pr "$pr" --arg head "$head" '{pr: ($pr | tonumber), head: $head, records: .}' >> "$dir/prs.jsonl" 2> /dev/null ||
      say "main miss: the ci-narrowing records of pull request #$pr are unreadable"
  fi
done < "$dir/pulls.sorted"
if ! jq -n -c --slurpfile failed "$dir/failed.jsonl" --slurpfile prs "$dir/prs.jsonl" \
  --arg runId "$RUN_ID" --arg sha "$HEAD_SHA" --arg epoch "$FORWARD_EPOCH" -f "$here/mainmiss.jq" > "$dir/miss.json" 2> "$dir/miss.err"; then
  say "main miss: the miss classification failed: $(head -c 200 "$dir/miss.err"); no row"
  exit 0
fi
name=$(jq -r '.name // empty' "$dir/miss.json")
if [ -z "$name" ]; then
  say "main miss: run $RUN_ID failed $(jq -r '.failed | join(", ")' "$dir/miss.json"); no narrowed pull request merged since $green skipped them; no row"
  exit 0
fi
jq -r '"main miss `\(.name)`: " + ([.misses[] | "#\(.pr) skipped \(.packages | join(", "))"] | join("; "))' "$dir/miss.json" |
  while read -r line; do say "$line"; done
echo "name=$name" >> "$GITHUB_OUTPUT"
