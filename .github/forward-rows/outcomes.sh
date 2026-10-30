#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: the go-product-shard side of the forward rows. It derives this
# shard's corvint-ci-outcomes/0 document from its captured go test -json stream
# and names the bounded rerun; the workflow runs the rerun itself, and the
# first run's status alone decides the shard.
#
#   outcomes.sh first DIR STATUS STREAM  writes DIR/outcomes.json and DIR/rerun-lines
#   outcomes.sh rerun DIR STREAM         rewrites DIR/outcomes.json with the rerun
#
# DIR holds `expected` (one package per go test invocation), `whole` (the
# shard's whole packages) and `failed-slices` (one "FLAG PATTERN PACKAGE" line
# per failed slice invocation). The environment names the run: GITHUB_EVENT_NAME,
# GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, TESTED_MERGE, PR_HEAD, OUTCOMES_TREE,
# SHARD and SHARDS. A failed run names each failed whole package and failed
# slice for one rerun, at most RERUN_BOUND of them; more records SKIPPED_BOUND
# and reruns none. The document is written before a rerun with state
# INTERRUPTED, so a rerun the job timeout kills still leaves the first outcomes.
set -eu
RERUN_BOUND=8
mode=$1 dir=$2
here=$(cd "$(dirname "$0")" && pwd)
doc() { # STATE RERUN_MAP INVOCATIONS
  jq -n -c --arg event "$GITHUB_EVENT_NAME" --arg runId "$GITHUB_RUN_ID" --arg attempt "${GITHUB_RUN_ATTEMPT:-1}" \
    --arg target "$TESTED_MERGE" --arg head "${PR_HEAD:-}" --arg tree "${OUTCOMES_TREE:-}" \
    --arg shard "$SHARD" --arg shards "$SHARDS" --arg status "$(cat "$dir/status")" \
    --arg state "$1" --slurpfile first "$dir/first.json" --slurpfile rerun "$2" \
    --arg invocations "$3" --arg bound "$RERUN_BOUND" '
    def opt: if . == "" then null else . end;
    {profile: "corvint-ci-outcomes/0", event: $event, runId: ($runId | tonumber),
     runAttempt: ($attempt | tonumber), target: $target, head: ($head | opt), tree: ($tree | opt),
     shard: ($shard | tonumber), shards: ($shards | tonumber), status: ($status | tonumber),
     outcomes: $first[0],
     rerun: {state: $state, bound: ($bound | tonumber), invocations: ($invocations | tonumber),
             outcomes: $rerun[0]}}' > "$dir/outcomes.json.tmp"
  mv -f "$dir/outcomes.json.tmp" "$dir/outcomes.json"
}
case $mode in
first)
  status=$3 stream=$4
  printf '%s\n' "$status" > "$dir/status"
  jq -R -n -c --rawfile expected "$dir/expected" -f "$here/outcomes.jq" < "$stream" > "$dir/first.json"
  echo '{}' > "$dir/none.json"
  : > "$dir/rerun-lines"
  if [ "$status" != 0 ]; then
    jq -r 'to_entries[] | select(.value == "fail" or .value == "missing") | .key' "$dir/first.json" > "$dir/failing"
    if [ -s "$dir/failing" ] && [ -f "$dir/whole" ]; then
      grep -Fx -f "$dir/failing" "$dir/whole" >> "$dir/rerun-lines" || true
    fi
    [ ! -f "$dir/failed-slices" ] || cat "$dir/failed-slices" >> "$dir/rerun-lines"
  fi
  n=$(wc -l < "$dir/rerun-lines" | tr -d ' ')
  if [ "$n" -eq 0 ]; then
    state=NOT_NEEDED
  elif [ "$n" -gt "$RERUN_BOUND" ]; then
    state=SKIPPED_BOUND
    : > "$dir/rerun-lines"
  else
    state=INTERRUPTED
  fi
  doc "$state" "$dir/none.json" "$n"
  ;;
rerun)
  stream=$3
  awk '{ print $NF }' "$dir/rerun-lines" > "$dir/rerun-expected"
  jq -R -n -c --rawfile expected "$dir/rerun-expected" -f "$here/outcomes.jq" < "$stream" > "$dir/rerun.json"
  doc RAN "$dir/rerun.json" "$(wc -l < "$dir/rerun-lines" | tr -d ' ')"
  ;;
*)
  echo "outcomes.sh: unknown mode $mode" >&2
  exit 2
  ;;
esac
