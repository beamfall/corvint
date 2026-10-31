#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-097: compute the whole-epoch AFP-V0-029 bar and AFP-V0-030 kill switch
# once, in the trusted ci-forward-bar workflow on main, and write the one
# afp-shadow-bar-verdict/0 document a pull request's narrowing reads. gh and jq
# only; nothing of a tested commit runs. Environment: GH_TOKEN, REPO,
# FORWARD_EPOCH, GITHUB_REF, GITHUB_SHA, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT,
# GITHUB_WORKFLOW_REF, RUNNER_TEMP, GITHUB_STEP_SUMMARY and VERDICT_PAGES
# (default 400 pages of 100), and FORWARD_EPOCH_SINCE (CIN-V0-003: a UTC time
# YYYY-MM-DDTHH:MM:SSZ no later than the merge that advanced main to the epoch; no row of
# the epoch can be older, so a listing that reaches it has reached the epoch's start).
# Writes $RUNNER_TEMP/afp-shadow-bar/verdict.json.
# The status is MET only when bar.jq says MET for a listing that reached the
# epoch's start; anything else, including an unreadable page, is NOT MET or NOT
# ESTABLISHED, which a pull request reads as full CI.
set -u
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=names.sh
. "$here/names.sh"
[ "${GITHUB_REF:-}" = refs/heads/main ] || {
  echo "verdict.sh runs only on refs/heads/main, not ${GITHUB_REF:-an unnamed ref}" >&2
  exit 1
}
printf '%s' "$FORWARD_EPOCH" | grep -Eqx '[0-9]+' || {
  echo "FORWARD_EPOCH $FORWARD_EPOCH is not a number" >&2
  exit 1
}
since=${FORWARD_EPOCH_SINCE:-}
[ -z "$since" ] || printf '%s' "$since" | grep -Eqx '[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z' || {
  echo "FORWARD_EPOCH_SINCE $since is not a UTC time YYYY-MM-DDTHH:MM:SSZ" >&2
  exit 1
}
dir="$RUNNER_TEMP/afp-shadow-bar"
mkdir -p "$dir"
# CIN-V0-004: when the listing began; a pull request lists back to it and no further.
listed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
list_names "${VERDICT_PAGES:-400}" "$dir/names" "$dir/times" "$since"
# A listing that reached FORWARD_EPOCH_SINCE holds every row of the epoch, as an exhausted one does.
covered=$exhausted
[ -z "$reached" ] || covered=1
bar=$(jq -R -s -c --arg epoch "$FORWARD_EPOCH" --arg exhausted "$covered" --arg format json --rawfile reclassified "$here/infra-reclassified.txt" -f "$here/bar.jq" < "$dir/names") ||
  bar='{"status":"NOT ESTABLISHED","error":"the artifact names could not be classified"}'
# The epoch's start as listed: the creation time of its oldest listed forward row.
start=$(grep -E " ci-forward-[A-Z]+-pr[0-9]+-[0-9a-f]{40}-e$FORWARD_EPOCH-f[01]\$" "$dir/times" | cut -d' ' -f1 | grep -v '^-$' | sort | head -n 1)
jq -n -c --argjson bar "$bar" --arg epoch "$FORWARD_EPOCH" --arg start "$start" --arg pages "$pages_read" \
  --arg exhausted "$exhausted" --arg limit "$limit" --arg runId "${GITHUB_RUN_ID:-}" \
  --arg runAttempt "${GITHUB_RUN_ATTEMPT:-}" --arg mainSha "${GITHUB_SHA:-}" --arg repository "$REPO" \
  --arg since "$since" --arg reached "$reached" --arg listedAt "$listed_at" \
  --arg workflowRef "${GITHUB_WORKFLOW_REF:-}" --arg computedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  'def num: if test("^[0-9]+$") then tonumber else null end;
   {profile: "afp-shadow-bar-verdict/0",
    status: (if $bar.status == "MET" or $bar.status == "NOT MET" then $bar.status else "NOT ESTABLISHED" end),
    epoch: ($epoch | tonumber),
    epochStart: (if $start == "" then null else $start end),
    covered: ($bar.covered // false),
    counts: ($bar.epochRows // null),
    window: ($bar.window // null),
    killSwitch: ($bar.killSwitch // null),
    narrowed: ($bar.narrowed // null),
    shadowCost: ($bar.shadowCost // null),
    pages: ($pages | num),
    exhausted: ($exhausted == "1"),
    epochSince: (if $since == "" then null else $since end),
    reachedEpochSince: ($reached == "1"),
    listedAt: $listedAt,
    windowNames: ($bar.windowNames // null),
    limit: (if $limit == "" then null else $limit end),
    error: ($bar.error // null),
    repository: $repository,
    workflowRef: $workflowRef,
    runId: ($runId | num),
    runAttempt: ($runAttempt | num),
    mainSha: $mainSha,
    computedAt: $computedAt}' > "$dir/verdict.json"
jq -r '"### AFP-V0-097 shadow-bar verdict, epoch \(.epoch)\n\n\(.status): \(.counts.complete // 0)/20 complete, \(.counts.failing // 0)/5 failing, \(.counts.miss // 0) MISS in the epoch; kill switch \(.killSwitch // "unknown"); \(.pages) page(s)\(if .exhausted then ", every artifact" elif .reachedEpochSince then ", back to \(.epochSince)" else "" end)\(if .limit then "; " + .limit else "" end)."' \
  "$dir/verdict.json" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
