#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-441 (V1-2682): decides whether a failed go-product-shard passed its one
# AFP-V0-047 rerun, and only then labels the shard FLAKY.
#
#   flaky.sh DIR STREAM
#
# DIR is the outcomes directory outcomes.sh wrote after `first` and `rerun`
# (`status`, `first.json`, `rerun-lines`, `rerun.json`, `outcomes.json`), plus
# `rerun-status`, the exit status of the bounded rerun (124 when its own timeout
# stopped it), and `whole-failed`, present when the whole-package invocation of
# the first attempt failed. STREAM is the first attempt's go test -json stream.
#
# It exits 0 (FLAKY) only when the first attempt failed, the rerun RAN at least
# one invocation and exited 0, every failed or missing package of the first
# attempt was rerun, each of its package-level fail events has its own rerun
# invocation, every rerun outcome is `pass`, and a failed whole-package
# invocation named at least one whole package to rerun. It then writes
# DIR/flaky.json (corvint-ci-flaky/0), appends a FLAKY section to
# GITHUB_STEP_SUMMARY and prints a ::warning annotation naming the first
# attempt's failed tests. Anything else, including an error here, exits nonzero
# and the shard fails.
set -eu
dir=$1 stream=$2
refuse() {
  echo "AFP-V0-441: not FLAKY: $1; the shard fails"
  exit 1
}
[ "$(cat "$dir/status" 2> /dev/null || true)" != 0 ] || refuse "the first attempt did not fail"
[ -s "$dir/rerun-lines" ] || refuse "no failed invocation was named for the rerun"
state=$(jq -r '.rerun.state' "$dir/outcomes.json" 2> /dev/null || true)
[ "$state" = RAN ] || refuse "the rerun state is ${state:-unreadable}, not RAN"
rerun_status=$(cat "$dir/rerun-status" 2> /dev/null || true)
[ "$rerun_status" = 0 ] || refuse "the rerun exited ${rerun_status:-without a status}"
jq -e 'length > 0 and all(.[]; . == "pass")' "$dir/rerun.json" > /dev/null 2>&1 ||
  refuse "a rerun invocation did not pass"
awk '{ print $NF }' "$dir/rerun-lines" | sort -u > "$dir/flaky-rerun-packages"
jq -r 'to_entries[] | select(.value == "fail" or .value == "missing") | .key' "$dir/first.json" | sort -u > "$dir/flaky-failed-packages"
[ -s "$dir/flaky-failed-packages" ] || refuse "the first attempt names no failed or missing package"
uncovered=$(comm -23 "$dir/flaky-failed-packages" "$dir/flaky-rerun-packages")
[ -z "$uncovered" ] || refuse "failed package(s) not rerun: $(printf '%s\n' "$uncovered" | paste -sd ' ' -)"
# Invocation-level coverage from the stream itself, so a lost failed-slices or whole-failed
# record cannot pass: each package-level fail event is one failed invocation of that package,
# and each needs its own rerun invocation.
jq -R -r 'fromjson? | objects | select(.Action == "fail" and (.Package | type) == "string" and .Test == null) | .Package' \
  < "$stream" | sort | uniq -c > "$dir/flaky-first-fails"
awk '{ print $NF }' "$dir/rerun-lines" | sort | uniq -c > "$dir/flaky-rerun-counts"
short=$(awk 'NR == FNR { n[$2] = $1; next } $1 > n[$2] + 0 { print $2 }' "$dir/flaky-rerun-counts" "$dir/flaky-first-fails")
[ -z "$short" ] || refuse "failed invocation(s) not all rerun: $(printf '%s\n' "$short" | paste -sd ' ' -)"
if [ -f "$dir/whole-failed" ] && ! awk 'NF == 1 { found = 1 } END { exit !found }' "$dir/rerun-lines"; then
  refuse "the whole-package invocation failed with no whole package named for the rerun"
fi
jq -R -n -c '[inputs | fromjson? | objects
  | select(.Action == "fail" and (.Package | type) == "string" and (.Test | type) == "string")
  | {package: .Package, test: .Test}] | unique' < "$stream" > "$dir/flaky-tests.json"
jq -n -c --arg event "${GITHUB_EVENT_NAME:-}" --arg runId "${GITHUB_RUN_ID:-0}" --arg attempt "${GITHUB_RUN_ATTEMPT:-1}" \
  --arg target "${TESTED_MERGE:-}" --arg shard "$SHARD" --arg shards "$SHARDS" --arg status "$(cat "$dir/status")" \
  --rawfile packages "$dir/flaky-failed-packages" --slurpfile tests "$dir/flaky-tests.json" \
  --arg invocations "$(wc -l < "$dir/rerun-lines" | tr -d ' ')" '
  {profile: "corvint-ci-flaky/0", verdict: "FLAKY", event: $event, runId: ($runId | tonumber),
   runAttempt: ($attempt | tonumber), target: $target, shard: ($shard | tonumber),
   shards: ($shards | tonumber), firstStatus: ($status | tonumber),
   rerunInvocations: ($invocations | tonumber),
   failedPackages: [$packages | split("\n")[] | select(length > 0)], failedTests: $tests[0]}' > "$dir/flaky.json.tmp"
mv -f "$dir/flaky.json.tmp" "$dir/flaky.json"
names=$(jq -r '(.failedTests | map("\(.package).\(.test)")) as $t
  | if ($t | length) > 0 then $t else .failedPackages end | .[]' "$dir/flaky.json")
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo "### FLAKY: go-product-shard $SHARD of $SHARDS passed only on its rerun"
    echo
    echo "The first attempt exited $(cat "$dir/status"); its $(wc -l < "$dir/rerun-lines" | tr -d ' ') failed invocation(s) passed one rerun (AFP-V0-441, V1-2682)."
    echo "First-attempt failures:"
    echo
    # shellcheck disable=SC2016 # the backquotes are Markdown code spans
    printf '%s\n' "$names" | sed 's/^/- `/; s/$/`/'
  } >> "$GITHUB_STEP_SUMMARY"
fi
list=$(printf '%s\n' "$names" | head -n 20 | paste -sd ',' - | sed 's/,/, /g')
more=$(($(printf '%s\n' "$names" | wc -l) - 20))
[ "$more" -le 0 ] || list="$list and $more more"
list=$(printf '%s' "$list" | sed 's/%/%25/g')
echo "::warning title=FLAKY go-product-shard $SHARD::shard $SHARD failed its first attempt and passed its one rerun; first-attempt failures: $list"
