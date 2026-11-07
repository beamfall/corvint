#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-2724 (V1-4934): size the go-product-shard matrix of a pull request from the trusted
# narrowing decision. The shard-plan job runs this copy from the CORVINT_PR_TOOL_SOURCE checkout,
# in the tested checkout, before any shard starts. It runs narrow.sh beside it once, as shard 0
# of FULL_SHARDS with every input a shard's gate gets, so the two decisions agree whenever their
# inputs do. On a NARROWED decision it sizes the balanced placement of the recorded selection
# with ci-shards (AFP-V0-2723: --size-budget-ms WALL_BUDGET_MS, --min-shards MIN_SHARDS, the
# shards' --parallel SHARD_TEST_P) and writes the step outputs `count` and `matrix` (the JSON
# list of shard indexes). A FULL decision, a missing or unreadable record or selection, a sizing
# failure and any count outside 1..FULL_SHARDS keep FULL_SHARDS. The shards still decide for
# themselves: a shard whose own gate records FULL runs its complete-universe partition over the
# sized matrix (AFP-V0-022), so no package is lost; only the job count changes. Environment:
# narrow.sh's (GH_TOKEN, REPO, BASE_REF, PR_BASE, PR_HEAD, PR_NUMBER, TESTED_MERGE, TOOL_SOURCE,
# VERDICT_RUN, PR_BIN, SHADOW, CI_SHARDS, CONFINE, CORVINT_PR_SAMPLE_K, CORVINT_PR_SAMPLE_SEED,
# GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, RUNNER_TEMP, GITHUB_OUTPUT, GITHUB_STEP_SUMMARY) plus
# FULL_SHARDS, MIN_SHARDS, WALL_BUDGET_MS and SHARD_TEST_P. SHARD and SHARDS are set here. This
# script never fails the job.
set -u
here=$(cd "$(dirname "$0")" && pwd)
full=${FULL_SHARDS:-16}
min=${MIN_SHARDS:-2}
budget=${WALL_BUDGET_MS:-400000}
digits() { printf '%s' "$1" | grep -Eqx '[1-9][0-9]*'; }
matrix() { # N: the JSON list [0,...,N-1]
  i=0
  out=""
  while [ "$i" -lt "$1" ]; do
    out="$out${out:+,}$i"
    i=$((i + 1))
  done
  printf '[%s]' "$out"
}
emit() { # N REASON
  echo "count=$1" >> "$GITHUB_OUTPUT"
  echo "matrix=$(matrix "$1")" >> "$GITHUB_OUTPUT"
  echo "AFP-V0-2724 shard plan: $1 of $full shard(s); $2" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
  exit 0
}
digits "$full" && [ "$full" -le 64 ] || full=16
digits "$min" && [ "$min" -le "$full" ] || emit "$full" "MIN_SHARDS '$min' is not a count up to $full"
digits "$budget" || emit "$full" "WALL_BUDGET_MS '$budget' is not a positive number"
out="$RUNNER_TEMP/corvint-narrowing"
/bin/rm -f "$out/narrowing.json"
SHARD=0 SHARDS=$full sh "$here/narrow.sh" || emit "$full" "narrow.sh exited $?"
mode=$(jq -r '.mode' "$out/narrowing.json" 2> /dev/null) || mode=""
[ "$mode" = NARROWED ] || emit "$full" "the trusted decision is ${mode:-missing}, so every shard runs its partition"
jq -r '.selected[]? | select(type == "string")' "$out/narrowing.json" > "$out/plan-selected" 2> /dev/null ||
  emit "$full" "the recorded selection is unreadable"
[ -s "$out/universe" ] || emit "$full" "the package universe is empty"
count=$("$CI_SHARDS" --shards "$full" --min-shards "$min" --size-budget-ms "$budget" --parallel "${SHARD_TEST_P:-1}" \
  --select "$out/plan-selected" < "$out/universe" 2> "$out/size.err") || emit "$full" "ci-shards could not size the selection: $(head -c 200 "$out/size.err")"
digits "$count" && [ "$count" -le "$full" ] || emit "$full" "ci-shards sized '$count', not a count up to $full"
emit "$count" "the balanced placement of $(wc -l < "$out/plan-selected" | tr -d ' ') selected package(s) fits $count shard(s) within ${budget} ms of predicted wall time at -p ${SHARD_TEST_P:-1} (AFP-V0-2723)"
