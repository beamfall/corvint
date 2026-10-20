#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-098: decide whether this pull-request shard narrows, from the pinned
# trusted tools, and write the corvint-pr-narrowing/0 record (narrow.jq). The
# go-product-shard job runs this copy from the CORVINT_PR_TOOL_SOURCE checkout,
# in the tested checkout, only after the narrowing-verdict job said MET
# (AFP-V0-097). Environment: GH_TOKEN, REPO, BASE_REF, PR_BASE, PR_HEAD,
# PR_NUMBER, TESTED_MERGE, SHARD, SHARDS, TOOL_SOURCE, VERDICT_RUN, PR_BIN (the
# pinned corvint, selector and driver), SHADOW (the pinned shadow corvint),
# CI_SHARDS, CONFINE, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, RUNNER_TEMP,
# GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes
# $RUNNER_TEMP/corvint-narrowing/narrowing.json, the executed package list
# beside it, and the step output `mode` (NARROWED or FULL). Every failure here
# is FULL; this script never fails the shard.
set -u
here=$(cd "$(dirname "$0")" && pwd)
out="$RUNNER_TEMP/corvint-narrowing"
mkdir -p "$out/driver"
# --no-renames lists both sides of a rename, so a file moved out of .github/ counts.
if changed=$(git diff --no-renames --name-only "$PR_BASE" "$TESTED_MERGE" 2> /dev/null); then
  if printf '%s\n' "$changed" | grep -q '^\.github/'; then github=1; else github=0; fi
else
  github=unreadable
fi
case $(gh api "repos/$REPO/rules/branches/main" --jq 'any(.[]; .type == "merge_queue")' 2> /dev/null) in
true) queue=1 ;;
false) queue=0 ;;
*) queue=unreadable ;;
esac
"$PR_BIN/driver" --mode plan --root "$PWD" --base "$PR_BASE" --head "$PR_HEAD" --target "$TESTED_MERGE" \
  --source "$TOOL_SOURCE" --planner "$PR_BIN/corvint" --selector "$PR_BIN/selector" --out "$out/driver" \
  --runtime "$out/runtime" --shard "$SHARD" --shards "$SHARDS" < /dev/null > "$out/driver.log" 2>&1 || true
[ -f "$out/driver/selection.json" ] || : > "$out/driver/selection.json"
shadow_exit=0
"$SHADOW" --root "$PWD" test --base "$PR_BASE" --profile ci --confine "$CONFINE" --placement universe --dry-run \
  > "$out/shadow.json" 2> "$out/shadow.log" < /dev/null || shadow_exit=$?
go list ./... > "$out/universe" 2> /dev/null || : > "$out/universe"
"$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" < "$out/universe" > "$out/packages" 2> /dev/null || : > "$out/packages"
"$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" --slices < "$out/universe" > "$out/slices" 2> /dev/null || : > "$out/slices"
[ -s "$out/universe" ] || : > "$out/driver/selection.json"
if ! jq -n -c --slurpfile driver "$out/driver/selection.json" --slurpfile shadow "$out/shadow.json" \
  --arg shadowExit "$shadow_exit" --rawfile packages "$out/packages" --rawfile slices "$out/slices" \
  --arg github "$github" --arg mergeQueue "$queue" --arg baseRef "$BASE_REF" --arg base "$PR_BASE" \
  --arg head "$PR_HEAD" --arg target "$TESTED_MERGE" --arg pr "$PR_NUMBER" --arg runId "$GITHUB_RUN_ID" \
  --arg runAttempt "$GITHUB_RUN_ATTEMPT" --arg shard "$SHARD" --arg shards "$SHARDS" \
  --arg toolSource "$TOOL_SOURCE" --arg verdictRun "$VERDICT_RUN" \
  -f "$here/narrow.jq" > "$out/narrowing.json" 2> "$out/narrow.err"; then
  echo "AFP-V0-098 shard $SHARD: FULL (the narrowing decision failed: $(head -c 200 "$out/narrow.err"))" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
  /bin/rm -f "$out/narrowing.json"
  echo "mode=FULL" >> "$GITHUB_OUTPUT"
  exit 0
fi
mode=$(jq -r '.mode' "$out/narrowing.json")
jq -r '.executed[]' "$out/narrowing.json" > "$out/executed"
jq -r --arg shard "$SHARD" '"AFP-V0-098 shard \($shard): \(.mode), \(.executed | length) of \(.shardPackages) package(s); driver \(.driverSelected), shadow \(.shadowSelected)\(if (.reasons | length) > 0 then "; " + (.reasons | join("; ")) else "" end)"' \
  "$out/narrowing.json" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
echo "mode=$mode" >> "$GITHUB_OUTPUT"
