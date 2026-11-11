#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-098: decide whether this pull-request shard narrows, from the pinned
# trusted tools, and write the corvint-pr-narrowing/0 record (narrow.jq). The
# go-product-shard job runs this copy from the CORVINT_PR_TOOL_SOURCE checkout,
# in the tested checkout, only after the narrowing-verdict job said MET
# (AFP-V0-097). Environment: GH_TOKEN, REPO, BASE_REF, PR_BASE, PR_HEAD,
# PR_NUMBER, TESTED_MERGE, SHARD, SHARDS, TOOL_SOURCE, VERDICT_RUN, PR_BIN (the
# pinned corvint, selector and driver), SHADOW (the pinned shadow corvint),
# SELECTION (optional, AFP-V0-731), CORVINT_PR_SAMPLE_K and CORVINT_PR_SAMPLE_SEED (optional,
# AFP-V0-984; a merge-queue SELECTION is never sampled), CI_SHARDS, CONFINE, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, RUNNER_TEMP,
# GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes
# $RUNNER_TEMP/corvint-narrowing/narrowing.json, the executed package list
# beside it, and the step output `mode` (NARROWED or FULL). A NARROWED shard
# runs its part of the AFP-V0-590 balanced placement of the selection. Every
# failure here is FULL; this script never fails the shard. AFP-V0-2724: the
# shard-plan job runs this script too, through shard-plan.sh beside it.
# CIN-V0-8421 (V1-5196): every decision line goes to the job log (stdout) as well as the step
# summary, a FULL one as `AFP-V0-098 shard N: FULL (REASON; ...); reason codes: CODE, ...`, and
# decision.json beside the record (corvint-pr-narrowing-decision/0: mode, reasonCodes, reasons,
# shard, run) is written for every decision, including the FULL fallbacks below that drop the
# record, so full-reasons.sh can tally FULL causes over run history from the uploaded artifact.
# CIN-V0-8423, CIN-V0-8424 (V1-5202): github-guard.sh classifies each changed input as global
# (FULL) or mapped to the test packages that consume it; a range changing only mapped inputs
# narrows when the selection holds every named consumer, else it is FULL naming the omission.
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
  # CIN-V0-2300 (V1-4446): a driver fallback (a non-empty Reason) admits no base above, which left
  # the FULL record with base "", an unreadable .github/ check and no shadow plan. Prove parent 0
  # here as AFP-V0-800 does: the tested merge has exactly two parents, the second is the head, and
  # the event base is an ancestor of the first. Any failed step keeps "" (the record stays FULL and
  # names it); an admitted driver base is never replaced, and a selection without a Reason that
  # fails the identity filter keeps "". The mode is unchanged: a driver Reason is FULL in
  # narrow.jq whatever the base.
  driver_reason=$(jq -r 'if type == "object" then (.Reason // "") | tostring else "" end' "$out/driver/selection.json" 2> /dev/null) || driver_reason=""
  if [ -z "$effective_base" ] && [ -n "$driver_reason" ]; then
    parents=$(git rev-list --parents -n 1 "$TESTED_MERGE" 2> /dev/null) || parents=""
    set -f
    # shellcheck disable=SC2086 # the commit and its parents, checked below
    set -- $parents
    set +f
    if [ "$#" = 3 ] && [ "$1" = "$TESTED_MERGE" ] && [ "$3" = "$PR_HEAD" ] &&
      printf '%s\n' "$2" | grep -Eqx '[0-9a-f]{40}' &&
      git merge-base --is-ancestor "$event_base" "$2" > /dev/null 2>&1; then
      effective_base=$2
    fi
  fi
fi
# AFP-V0-2720 (V1-4930; amends AFP-V0-933): only a change to a trusted narrowing input under
# .github/ forces FULL. github-guard.sh beside this script, from the same trusted checkout, names
# the inputs and reads the range with git alone: 0 no input changed, 1 a global input changed (the
# inputs follow, first five recorded), mapped only mapped inputs changed (CIN-V0-8423: each
# follows as `INPUT CONSUMER...`, checked against the selection below), anything else unreadable.
# A missing guard is unreadable.
github=unreadable
github_inputs=""
github_mapped=""
github_missing=""
: > "$out/github-mapped"
if [ -n "$effective_base" ] && guard=$(sh "$here/github-guard.sh" "$effective_base" "$TESTED_MERGE" 2> /dev/null); then
  case $(printf '%s\n' "$guard" | sed -n 1p) in
  0) github=0 ;;
  1)
    github=1
    github_inputs=$(printf '%s\n' "$guard" | sed -n '2,6p' | tr '\n' ',' | sed 's/,$//; s/,/, /g')
    ;;
  mapped)
    github=mapped
    printf '%s\n' "$guard" | sed -n '2,$p' | sed '/^$/d' > "$out/github-mapped"
    [ -s "$out/github-mapped" ] || github=unreadable
    ;;
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
# CIN-V0-8424 (V1-5202): a range that changes only mapped .github/ inputs narrows only when the
# selection within the universe holds every consumer package the guard names for each input;
# a missing consumer, an unreadable module path or a malformed guard line is FULL.
if [ "$github" = mapped ]; then
  module=$(sed -n 's/^module[[:space:]]\{1,\}\([^[:space:]]\{1,\}\)[[:space:]]*$/\1/p' go.mod 2> /dev/null | head -n 1)
  if ! printf '%s' "$module" | grep -Eqx '[A-Za-z0-9][A-Za-z0-9._~/-]*'; then
    github=unreadable
  else
    while IFS=' ' read -r input consumers; do
      [ -n "$input" ] && [ -n "$consumers" ] || { github_missing="$github_missing, $input: no consumer named"; continue; }
      github_mapped="$github_mapped, $input: $consumers"
      for consumer in $consumers; do
        grep -Fqx "$module/$consumer" "$out/selected" 2> /dev/null || github_missing="$github_missing, $input: $consumer"
      done
    done < "$out/github-mapped"
    github_mapped=${github_mapped#, }
    github_missing=${github_missing#, }
    [ -z "$github_missing" ] || github=consumer
  fi
fi
balanced=0
if "$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" --select "$out/selected" < "$out/universe" > "$out/balanced-packages" 2> /dev/null &&
  "$CI_SHARDS" --shard "$SHARD" --shards "$SHARDS" --select "$out/selected" --slices < "$out/universe" > "$out/balanced-slices" 2> /dev/null; then
  balanced=1
else
  : > "$out/balanced-packages"
  : > "$out/balanced-slices"
fi
sample_k=${CORVINT_PR_SAMPLE_K:-}
[ -z "${SELECTION:-}" ] || sample_k=""
# say LINE: one decision line, to the job log and, when set, the step summary (CIN-V0-8421).
say() {
  echo "$1"
  [ -z "${GITHUB_STEP_SUMMARY:-}" ] || echo "$1" >> "$GITHUB_STEP_SUMMARY"
}
# decision MODE CODES REASONS: write decision.json beside the record (CIN-V0-8421). CODES and
# REASONS are newline-separated and parallel. A failed write leaves no sidecar; the mode output
# and the record are unaffected.
decision() {
  jq -n -c --arg mode "$1" --arg codes "$2" --arg reasons "$3" --arg shard "$SHARD" --arg shards "$SHARDS" \
    --arg runId "$GITHUB_RUN_ID" --arg runAttempt "$GITHUB_RUN_ATTEMPT" --arg pr "$PR_NUMBER" --arg head "$PR_HEAD" --arg target "$TESTED_MERGE" \
    'def num: if type == "string" and test("^[0-9]+$") then tonumber else null end;
     {profile: "corvint-pr-narrowing-decision/0", mode: $mode,
      reasonCodes: [$codes | split("\n")[] | select(length > 0)], reasons: [$reasons | split("\n")[] | select(length > 0)],
      shard: ($shard | num), shards: ($shards | num), runId: ($runId | num), runAttempt: ($runAttempt | num),
      pr: ($pr | num), head: $head, target: $target}' > "$out/decision.json" 2> /dev/null || /bin/rm -f "$out/decision.json"
}
# full CODE REASON: drop the record and its sidecar, so the shard runs whole; the decision
# sidecar and the log line keep the cause (CIN-V0-8421).
full() {
  decision FULL "$1" "$2"
  say "AFP-V0-098 shard $SHARD: FULL ($2); reason codes: $1"
  /bin/rm -f "$out/narrowing.json" "$out/executed"
  echo "mode=FULL" >> "$GITHUB_OUTPUT"
  exit 0
}
if ! jq -n -c --slurpfile driver "$out/driver/selection.json" --slurpfile shadow "$out/shadow.json" \
  --arg shadowExit "$shadow_exit" --rawfile packages "$out/packages" --rawfile slices "$out/slices" \
  --rawfile selected "$out/selected" --rawfile balancedPackages "$out/balanced-packages" \
  --rawfile balancedSlices "$out/balanced-slices" --arg balanced "$balanced" \
  --rawfile universe "$out/universe" --arg selectedOk "$selected_ok" \
  --arg github "$github" --arg githubInputs "$github_inputs" --arg githubMapped "$github_mapped" --arg githubMissing "$github_missing" --arg mergeQueue "$queue" --arg baseRef "$BASE_REF" --arg base "$effective_base" --arg eventBase "$event_base" \
  --arg head "$PR_HEAD" --arg target "$TESTED_MERGE" --arg pr "$PR_NUMBER" --arg runId "$GITHUB_RUN_ID" \
  --arg runAttempt "$GITHUB_RUN_ATTEMPT" --arg shard "$SHARD" --arg shards "$SHARDS" \
  --arg toolSource "$TOOL_SOURCE" --arg verdictRun "$VERDICT_RUN" \
  --arg sampleK "$sample_k" --arg sampleSeed "${CORVINT_PR_SAMPLE_SEED:-}" \
  -f "$here/narrow.jq" > "$out/narrowing.json" 2> "$out/narrow.err"; then
  full decision-failed "the narrowing decision failed: $(head -c 200 "$out/narrow.err")"
fi
mode=$(jq -r '.mode' "$out/narrowing.json" 2> /dev/null) || full record-unreadable "the narrowing record is unreadable"
# The tests step narrows only when this sidecar equals the record's `executed` line for line.
jq -r '.executed[]' "$out/narrowing.json" > "$out/executed" 2> /dev/null ||
  full sidecar-unwritable "the executed list could not be written beside the record"
[ "$(jq -r '.executed | join("\n")' "$out/narrowing.json" 2> /dev/null)" = "$(cat "$out/executed" 2> /dev/null)" ] ||
  full sidecar-mismatch "the executed list beside the record disagrees with it"
# CIN-V0-8421: the FULL line names every reason and its code, in the log and the summary.
codes=$(jq -r '.reasonCodes[]?' "$out/narrowing.json" 2> /dev/null) || codes=""
reasons=$(jq -r '.reasons[]?' "$out/narrowing.json" 2> /dev/null) || reasons=""
decision "$mode" "$codes" "$reasons"
[ "$mode" != FULL ] || say "AFP-V0-098 shard $SHARD: FULL ($(printf '%s\n' "$reasons" | sed '/^$/d' | paste -sd ';' - | sed 's/;/; /g')); reason codes: $(printf '%s\n' "$codes" | sed '/^$/d' | paste -sd ',' - | sed 's/,/, /g')"
say "$(jq -r --arg shard "$SHARD" '"AFP-V0-098 shard \($shard): \(.mode) (\(.placement)), \(.executed | length) package(s), shard partition \(.shardPackages); driver \(.driverSelected), shadow \(.shadowSelected)\(if (.overSelectedCount // 0) > 0 then ", over-selected \(.overSelectedCount) (AFP-V0-9510): \(.overSelected[0:5] | join(", "))" else "" end)\(if ((.reused // []) | length) > 0 then ", reused \(.reused | length) from run \(.reuse.recordRunId) (AFP-V0-814)" elif (.reuse | type) == "object" and .reuse.applied != true then "; no reuse (AFP-V0-816): \(.reuse.reason)" else "" end)\(if ((.githubMapped // []) | length) > 0 then "; mapped .github/ inputs (CIN-V0-8423): " + (.githubMapped | join(", ")) else "" end)\(if (.reasons | length) > 0 then "; " + (.reasons | join("; ")) else "" end)"' \
  "$out/narrowing.json" 2> /dev/null)"
echo "mode=$mode" >> "$GITHUB_OUTPUT"
