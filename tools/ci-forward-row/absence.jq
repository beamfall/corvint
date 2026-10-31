# SPDX-License-Identifier: Apache-2.0
# CIN-V0-861 (V1-3332): the row-absence input of the AFP-V0-030 kill switch. Since CIN-V0-001 a
# run whose selection documents carry no single epoch of the collector writes no forward row, so
# a shadow outage (no shadow, a dead shadow pin, a failed artifact download) only stops rows and
# never counts INCOMPLETE. The collector names each such run instead:
#   ci-forward-NOROW-pr<N>-<sha>-e<E>-r<nodoc|mixed>   no selection document, or no single
#                                                     integer epoch (an outage)
#   ci-forward-EPOCH-pr<N>-<sha>-e<K>                 one earlier epoch K (excluded on purpose)
# Usage: jq -R -s -c --arg epoch E [--arg threshold N] -f absence.jq < NAMES
# NAMES: artifact names, newest first, one per line (bar.jq's input). The window is the latest 20
# (pull request, commit) entries of the epoch among its rows that take a kill-switch place (not
# INFRA, not NARROWED: bar.jq's window) and its NOROW markers; a re-run's newer entry replaces
# the older. The switch trips when THRESHOLD (default 3, as for INCOMPLETE) of them are NOROW.
# An EPOCH marker, or a row of an earlier epoch, takes no place, so epoch-1 runs are never counted
# as absence. A malformed epoch or threshold trips.
($epoch | test("^[0-9]+$")) as $epochOK
| (($ARGS.named.threshold // "3") | test("^[1-9][0-9]*$")) as $thresholdOK
| if ($epochOK and $thresholdOK) | not then
    {killSwitch: "TRIPPED", error: "the epoch or the threshold is malformed"}
  else
    (($ARGS.named.threshold // "3") | tonumber) as $threshold
    | [split("\n")[]
       | (capture("^ci-forward-(?<class>CLEAN|FLAKY|MISS|INCOMPLETE)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<e>[0-9]+)-f[01]$")
          // capture("^ci-forward-(?<class>NOROW)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<e>[0-9]+)-r(?<why>nodoc|mixed)$")
          // empty)
       | select(.e == $epoch)] as $entries
    | (reduce $entries[] as $r ({seen: {}, rows: []};
        ($r.pr + "-" + $r.sha) as $k | if .seen[$k] then . else .seen[$k] = true | .rows += [$r] end)
       | .rows[:20]) as $window
    | [$window[] | select(.class == "NOROW")] as $absent
    | {killSwitch: (if ($absent | length) >= $threshold then "TRIPPED" else "clear" end),
       threshold: $threshold, window: ($window | length), absent: ($absent | length),
       absentRows: [$absent[] | "pr\(.pr)-\(.sha) \(.why)"]}
  end
