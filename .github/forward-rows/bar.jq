# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-047: the AFP-V0-029 bar and AFP-V0-030 kill-switch counts from
# forward-row artifact names alone (V1-0811 AC4); nothing is downloaded.
# Usage: jq -R -s -r --arg epoch E [--arg exhausted 1] -f bar.jq < NAMES
# (newest first, one per line; exhausted=1 when the listing read every artifact).
# A row counts once per (pull request, commit): a re-run row of the same pair
# replaces the older one. An INFRA row (AFP-V0-052, a hosted set-up failure) is
# neither complete nor INCOMPLETE and is reported on its own.
# Two separate counts (owner, 2026-10-20; V1-0812): the AFP-V0-029 bar counts
# every row of the epoch, the AFP-V0-030 kill switch only its latest 20 rows,
# where an INFRA row takes no place. "No MISS anywhere in the epoch" holds only
# for a listing that reached the epoch's start: one that was exhausted or names
# a row of an earlier epoch. Otherwise the bar is NOT ESTABLISHED, never MET.
def counts($rows):
  ([$rows[] | select(.class != "INCOMPLETE" and .class != "INFRA")]) as $complete
  | {rows: ([$rows[] | select(.class != "INFRA")] | length), complete: ($complete | length),
     failing: ([$complete[] | select(.f == "1")] | length),
     miss: ([$rows[] | select(.class == "MISS")] | length),
     flaky: ([$rows[] | select(.class == "FLAKY")] | length),
     incomplete: ([$rows[] | select(.class == "INCOMPLETE")] | length),
     infra: ([$rows[] | select(.class == "INFRA")] | length)};
[split("\n")[]
 | capture("^ci-forward-(?<class>CLEAN|FLAKY|MISS|INCOMPLETE|INFRA)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-f(?<f>[01])$")] as $parsed
| [$parsed[] | select(.epoch == $epoch)] as $all
| (($ARGS.named.exhausted // "") == "1"
   or any($parsed[]; (.epoch | tonumber) < ($epoch | tonumber))) as $covered
| (reduce $all[] as $r ({seen: {}, rows: []};
    ($r.pr + "-" + $r.sha) as $k
    | if .seen[$k] then . else .seen[$k] = true | .rows += [$r] end) | .rows) as $unique
| (reduce $unique[] as $r ({rows: [], infra: 0};
    if (.rows | length) >= 20 then . elif $r.class == "INFRA" then .infra += 1 else .rows += [$r] end)) as $window
| counts($unique) as $e
| counts($window.rows) as $w
| "### AFP-V0-029 forward rows, epoch \($epoch)",
  "",
  "Every listed row of the epoch (AFP-V0-029 bar): \($e.complete) complete, \($e.failing) complete with failing packages, \($e.flaky) FLAKY, \($e.miss) MISS, \($e.incomplete) INCOMPLETE, \($e.infra) INFRA.",
  "",
  "Latest \($w.rows) row(s) of the epoch (AFP-V0-030 window): \($w.complete) complete, \($w.failing) complete with failing packages, \($w.flaky) FLAKY, \($w.miss) MISS, \($w.incomplete) INCOMPLETE.",
  "",
  "Hosted infrastructure failures (AFP-V0-052): \($window.infra) INFRA row(s) between them, outside the window and neither complete nor INCOMPLETE.",
  "",
  "Shadow bar (20 complete rows of the epoch, at least 5 with failing packages, no non-FLAKY miss in the epoch): "
    + (if ($e.complete >= 20 and $e.failing >= 5 and $e.miss == 0) | not then "NOT MET"
       elif $covered then "MET"
       else "NOT ESTABLISHED" end)
    + " (\($e.complete)/20 complete, \($e.failing)/5 failing)"
    + (if $covered then "." else "; the listing did not reach the start of epoch \($epoch), so an older MISS row cannot be excluded." end),
  "",
  "Kill switch (AFP-V0-030: 2 MISS or 3 INCOMPLETE in the latest 20): "
    + (if $w.miss >= 2 or $w.incomplete >= 3 then "TRIPPED" else "clear" end) + ".",
  "",
  "This count is advisory: nothing narrows on it before V1-0812."
