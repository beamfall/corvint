#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-098: decide whether this pull-request shard narrows, from the pinned
# trusted tools, and write the corvint-pr-narrowing/0 record (narrow.jq). The
# go-product-shard job runs this copy from the CORVINT_PR_TOOL_SOURCE checkout,
# in the tested checkout, only after the narrowing-verdict job said MET
# (AFP-V0-097). Environment: GH_TOKEN, REPO, BASE_REF, PR_BASE, PR_HEAD,
# PR_NUMBER, TESTED_MERGE, SHARD, SHARDS, TOOL_SOURCE, VERDICT_RUN, PR_BIN (the
# pinned corvint, selector and driver), SHADOW (the pinned shadow corvint),
# SELECTION (optional, AFP-V0-731), CI_SHARDS, CONFINE, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, RUNNER_TEMP,
# GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes
# $RUNNER_TEMP/corvint-narrowing/narrowing.json, the executed package list
# beside it, and the step output `mode` (NARROWED or FULL). A NARROWED shard
# runs its part of the AFP-V0-590 balanced placement of the selection. Every
# failure here is FULL; this script never fails the shard.
set -u
here=$(cd "$(dirname "$0")" && pwd)
out="$RUNNER_TEMP/corvint-narrowing"
mkdir -p "$out/driver"
case $(gh api "repos/$REPO/rules/branches/main" --jq 'any(.[]; .type == "merge_queue")' 2> /dev/null) in
true) queue=1 ;;
false) queue=0 ;;
*) queue=unreadable ;;
esac
# AFP-V0-731: a merge-queue entry's mq-narrow.sh hands its own selection of the queue range in
# SELECTION; a pull request asks the pinned deciding driver.
if [ -n "${SELECTION:-}" ]; then
  cp "$SELECTION" "$out/driver/selection.json" 2> /dev/null || : > "$out/driver/selection.json"
else
  "$PR_BIN/driver" --mode plan --root "$PWD" --base "$PR_BASE" --head "$PR_HEAD" --target "$TESTED_MERGE" \
    --source "$TOOL_SOURCE" --planner "$PR_BIN/corvint" --selector "$PR_BIN/selector" --out "$out/driver" \
    --runtime "$out/runtime" --shard "$SHARD" --shards "$SHARDS" < /dev/null > "$out/driver.log" 2>&1 || true
fi
[ -f "$out/driver/selection.json" ] || : > "$out/driver/selection.json"
# AFP-V0-800/801: the driver proves the event ancestry before choosing parent 0.
# Queue selections retain their whole queue base (AFP-V0-731).
event_base=$PR_BASE
effective_base=$PR_BASE
if [ -z "${SELECTION:-}" ]; then
  effective_base=$(jq -r --arg event "$event_base" --arg head "$PR_HEAD" --arg target "$TESTED_MERGE" \
    'select(.Reason == "" and .EventBase == $event and .Head == $head and .Target == $target) | .Base' \
    "$out/driver/selection.json" 2> /dev/null) || effective_base=""
fi
# --no-renames includes both sides of a rename out of .github/. The exit status, not --name-only
# text, decides (AFP-V0-933, V1-3109): git quotes a non-ASCII path, so ".github/\303\251.yml" would
# not match ^\.github/. 0 is no change, 1 a change, anything else unreadable.
github=unreadable
if [ -n "$effective_base" ]; then
  rc=0
  git diff --quiet --no-renames "$effective_base" "$TESTED_MERGE" -- .github/ > /dev/null 2>&1 || rc=$?
  case $rc in
  0) github=0 ;;
  1) github=1 ;;
  esac
fi
shadow_exit=0
"$SHADOW" --root "$PWD" test --base "$effective_base" --profile ci --confine "$CONFINE" --placement universe --dry-run \
  > "$out/shadow.json" 2> "$out/shadow.log" < /dev/null || shadow_exit=$?
go list ./... > "$out/universe" 2> /dev/null || : > "$out/universe"
"$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" < "$out/universe" > "$out/packages" 2> /dev/null || : > "$out/packages"
"$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" --slices < "$out/universe" > "$out/slices" 2> /dev/null || : > "$out/slices"
[ -s "$out/universe" ] || : > "$out/driver/selection.json"
# AFP-V0-590: the driver's selection within the universe, placed by measured cost across
# all the shards; this shard's part of that placement is what a NARROWED shard runs. Each
# stage is checked on its own (dash has no pipefail): grep exits 1 for no match and 2 or more
# for an error, and any failure leaves selectedOk unset so narrow.jq records FULL. narrow.jq
# also requires the result to equal the driver selection within the universe.
selected_ok=0
if jq -r '.Packages[]? | select(type == "string")' "$out/driver/selection.json" > "$out/driver-packages" 2> /dev/null; then
  grep_exit=0
  grep -Fx -f "$out/universe" "$out/driver-packages" > "$out/selected-unsorted" 2> /dev/null || grep_exit=$?
  if [ "$grep_exit" -le 1 ] && sort -u "$out/selected-unsorted" > "$out/selected" 2> /dev/null; then
    selected_ok=1
  fi
fi
[ "$selected_ok" = 1 ] || : > "$out/selected"
balanced=0
if "$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" --select "$out/selected" < "$out/universe" > "$out/balanced-packages" 2> /dev/null &&
  "$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" --select "$out/selected" --slices < "$out/universe" > "$out/balanced-slices" 2> /dev/null; then
  balanced=1
else
  : > "$out/balanced-packages"
  : > "$out/balanced-slices"
fi
# full REASON: drop the record and its sidecar, so the shard runs whole.
full() {
  echo "AFP-V0-098 shard $SHARD: FULL ($1)" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
  /bin/rm -f "$out/narrowing.json" "$out/executed"
  echo "mode=FULL" >> "$GITHUB_OUTPUT"
  exit 0
}
if ! jq -n -c --slurpfile driver "$out/driver/selection.json" --slurpfile shadow "$out/shadow.json" \
  --arg shadowExit "$shadow_exit" --rawfile packages "$out/packages" --rawfile slices "$out/slices" \
  --rawfile selected "$out/selected" --rawfile balancedPackages "$out/balanced-packages" \
  --rawfile balancedSlices "$out/balanced-slices" --arg balanced "$balanced" \
  --rawfile universe "$out/universe" --arg selectedOk "$selected_ok" \
  --arg github "$github" --arg mergeQueue "$queue" --arg baseRef "$BASE_REF" --arg base "$effective_base" --arg eventBase "$event_base" \
  --arg head "$PR_HEAD" --arg target "$TESTED_MERGE" --arg pr "$PR_NUMBER" --arg runId "$GITHUB_RUN_ID" \
  --arg runAttempt "$GITHUB_RUN_ATTEMPT" --arg shard "$SHARD" --arg shards "$SHARDS" \
  --arg toolSource "$TOOL_SOURCE" --arg verdictRun "$VERDICT_RUN" \
  -f "$here/narrow.jq" > "$out/narrowing.json" 2> "$out/narrow.err"; then
  full "the narrowing decision failed: $(head -c 200 "$out/narrow.err")"
fi
mode=$(jq -r '.mode' "$out/narrowing.json" 2> /dev/null) || full "the narrowing record is unreadable"
# The tests step narrows only when this sidecar equals the record's `executed` line for line.
jq -r '.executed[]' "$out/narrowing.json" > "$out/executed" 2> /dev/null ||
  full "the executed list could not be written beside the record"
[ "$(jq -r '.executed | join("\n")' "$out/narrowing.json" 2> /dev/null)" = "$(cat "$out/executed" 2> /dev/null)" ] ||
  full "the executed list beside the record disagrees with it"
jq -r --arg shard "$SHARD" '"AFP-V0-098 shard \($shard): \(.mode) (\(.placement)), \(.executed | length) package(s), shard partition \(.shardPackages); driver \(.driverSelected), shadow \(.shadowSelected)\(if ((.reused // []) | length) > 0 then ", reused \(.reused | length) from run \(.reuse.recordRunId) (AFP-V0-814)" elif (.reuse | type) == "object" and .reuse.applied != true then "; no reuse (AFP-V0-816): \(.reuse.reason)" else "" end)\(if (.reasons | length) > 0 then "; " + (.reasons | join("; ")) else "" end)"' \
  "$out/narrowing.json" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
echo "mode=$mode" >> "$GITHUB_OUTPUT"
