# SPDX-License-Identifier: Apache-2.0
# CIN-V0-861 (V1-3332): the row-absence input of the AFP-V0-030 kill switch. Since CIN-V0-001 a
# run whose selection documents carry no single epoch of the collector writes no forward row, so
# a shadow outage (no shadow, a dead shadow pin, a failed artifact download) only stops rows and
# never counts INCOMPLETE. The collector names each such run instead:
#   ci-forward-NOROW-pr<N>-<sha>-e<E>-r<nodoc|mixed|target>
#                                                     no selection document, no single integer
#                                                     epoch, or (CIN-V0-2301) documents that name
#                                                     no single tested merge (an outage)
#   ci-forward-EPOCH-pr<N>-<sha>-e<K>                 one earlier epoch K (excluded on purpose)
# Usage: jq -R -s -c --arg epoch E [--arg threshold N] --rawfile reclassified infra-reclassified.txt
#        -f absence.jq < NAMES
# NAMES: artifact names, newest first, one per line (bar.jq's input). The window is the latest 20
# (pull request, commit) entries of the epoch among its rows that take a kill-switch place (not
# INFRA, not NARROWED: bar.jq's window) and its NOROW markers; a re-run's newer entry replaces
# the older. As in bar.jq (AFP-V0-463), an INCOMPLETE row named in the reviewed list counts as
# INFRA and takes no place, so a reclassified row cannot push a NOROW marker out of the window. The switch trips when THRESHOLD (default 3, as for INCOMPLETE) of them are NOROW.
# An EPOCH marker, or a row of an earlier epoch, takes no place, so epoch-1 runs are never counted
# as absence. A malformed epoch or threshold trips. verdict.sh (CIN-V0-861) and accept.sh
# (CIN-V0-2302) run this copy beside bar.jq; the marker's <sha> is the run's head commit, so a
# pull-request NOROW is not replaced by a later row of the same head, which names the tested
# merge: it can only count more absence, never less.
($epoch | test("^[0-9]+$")) as $epochOK
| (($ARGS.named.threshold // "3") | test("^[1-9][0-9]*$")) as $thresholdOK
| if ($epochOK and $thresholdOK) | not then
    {killSwitch: "TRIPPED", error: "the epoch or the threshold is malformed"}
  else
    (($ARGS.named.threshold // "3") | tonumber) as $threshold
    | ([($ARGS.named.reclassified // "") | split("\n")[] | select(test("^pr[0-9]+-[0-9a-f]{40}( |$)"))
        | {key: split(" ")[0], value: true}] | from_entries) as $infraKeys
    | [split("\n")[]
       | (capture("^ci-forward-(?<class>CLEAN|FLAKY|MISS|INCOMPLETE|INFRA|NARROWED)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<e>[0-9]+)-f[01]$")
          // capture("^ci-forward-(?<class>NOROW)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<e>[0-9]+)-r(?<why>nodoc|mixed|target)$")
          // empty)
       | select(.e == $epoch and .class != "NARROWED")
       | if .class == "INCOMPLETE" and $infraKeys["pr" + .pr + "-" + .sha] then .class = "INFRA" else . end] as $entries
    | (reduce $entries[] as $r ({seen: {}, rows: []};
        ($r.pr + "-" + $r.sha) as $k | if .seen[$k] then . else .seen[$k] = true | .rows += [$r] end)
       | [.rows[] | select(.class != "INFRA")][:20]) as $window
    | [$window[] | select(.class == "NOROW")] as $absent
    | {killSwitch: (if ($absent | length) >= $threshold then "TRIPPED" else "clear" end),
       threshold: $threshold, window: ($window | length), absent: ($absent | length),
       absentRows: [$absent[] | "pr\(.pr)-\(.sha) \(.why)"]}
  end
