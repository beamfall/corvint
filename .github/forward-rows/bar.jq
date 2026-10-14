# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-047: the AFP-V0-029 bar and AFP-V0-030 kill-switch counts from
# forward-row artifact names alone (V1-0811 AC4); nothing is downloaded.
# Usage: jq -R -s -r --arg epoch E -f bar.jq < NAMES (newest first, one per line)
# A row counts once per (pull request, commit): a re-run row of the same pair
# replaces the older one. Only the latest 20 rows of the epoch count.
[split("\n")[]
 | capture("^ci-forward-(?<class>CLEAN|FLAKY|MISS|INCOMPLETE)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-f(?<f>[01])$")
 | select(.epoch == $epoch)] as $all
| (reduce $all[] as $r ({seen: {}, rows: []};
    ($r.pr + "-" + $r.sha) as $k
    | if .seen[$k] then . else .seen[$k] = true | .rows += [$r] end) | .rows) as $unique
| $unique[0:20] as $w
| ([$w[] | select(.class != "INCOMPLETE")]) as $complete
| {rows: ($w | length), complete: ($complete | length),
   failing: ([$complete[] | select(.f == "1")] | length),
   miss: ([$w[] | select(.class == "MISS")] | length),
   flaky: ([$w[] | select(.class == "FLAKY")] | length),
   incomplete: ([$w[] | select(.class == "INCOMPLETE")] | length)} as $c
| "### AFP-V0-029 forward rows, epoch \($epoch)",
  "",
  "Latest \($c.rows) row(s) of the epoch: \($c.complete) complete, \($c.failing) complete with failing packages, \($c.flaky) FLAKY, \($c.miss) MISS, \($c.incomplete) INCOMPLETE.",
  "",
  "Shadow bar (20 complete rows, at least 5 with failing packages, no non-FLAKY miss): "
    + (if $c.complete >= 20 and $c.failing >= 5 and $c.miss == 0 then "MET" else "NOT MET" end)
    + " (\($c.complete)/20 complete, \($c.failing)/5 failing).",
  "",
  "Kill switch (AFP-V0-030: 2 MISS or 3 INCOMPLETE in the latest 20): "
    + (if $c.miss >= 2 or $c.incomplete >= 3 then "TRIPPED" else "clear" end) + ".",
  "",
  "This count is advisory: nothing narrows on it before V1-0812."
