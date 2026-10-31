# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: the AFP-V0-029 bar and AFP-V0-030 kill-switch counts from
# forward-row artifact names alone (V1-0811 AC4); nothing is downloaded.
# Usage: jq -R -s -r --arg epoch E [--arg exhausted 1] [--arg format json] -f bar.jq < NAMES
# (newest first, one per line; exhausted=1 when the listing read every artifact;
# format json prints one object for the AFP-V0-097 verdict instead of markdown).
# A row counts once per (pull request, commit): a re-run row of the same pair
# replaces the older one. An INFRA row (AFP-V0-052, a hosted set-up failure) is
# neither complete nor INCOMPLETE and is reported on its own.
# Two separate counts (owner, 2026-10-20; V1-0812): the AFP-V0-029 bar counts
# every row of the epoch, the AFP-V0-030 kill switch only its latest 20 rows,
# where an INFRA row takes no place. "No MISS anywhere in the epoch" holds only
# for a listing that reached the epoch's start: one that was exhausted or names
# a row of an earlier epoch. Otherwise the bar is NOT ESTABLISHED, never MET.
# AFP-V0-099: a NARROWED row (a pull-request run that executed only its selection)
# measures nothing about the shadow selector, so it is excluded from both the bar
# and the kill-switch window and only reported; its queue run's row still counts.
# AFP-V0-463: an INCOMPLETE row listed in infra-reclassified.txt counts as INFRA.
# AFP-V0-484: the additive shadowCost aggregate reads ci-forward-cost-* names only after the status
# and the kill switch are decided, and neither reads it.
def counts($rows):
  ([$rows[] | select(.class != "INCOMPLETE" and .class != "INFRA")]) as $complete
  | {rows: ([$rows[] | select(.class != "INFRA")] | length), complete: ($complete | length),
     failing: ([$complete[] | select(.f == "1")] | length),
     miss: ([$rows[] | select(.class == "MISS")] | length),
     flaky: ([$rows[] | select(.class == "FLAKY")] | length),
     incomplete: ([$rows[] | select(.class == "INCOMPLETE")] | length),
     infra: ([$rows[] | select(.class == "INFRA")] | length)};
[split("\n") | to_entries[] | .key as $i | .value
 | capture("^ci-forward-(?<class>CLEAN|FLAKY|MISS|INCOMPLETE|INFRA|NARROWED)-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-f(?<f>[01])$")
 | .i = $i] as $parsed
# AFP-V0-463: an INCOMPLETE row named in the owner's reviewed list (--rawfile reclassified
# infra-reclassified.txt, one pr<N>-<sha> key per line) counts as INFRA.
| ([($ARGS.named.reclassified // "") | split("\n")[] | select(test("^pr[0-9]+-[0-9a-f]{40}( |$)"))
    | {key: split(" ")[0], value: true}] | from_entries) as $infraKeys
| [$parsed[] | if .class == "INCOMPLETE" and $infraKeys["pr" + .pr + "-" + .sha] then .class = "INFRA" else . end] as $parsed
| [$parsed[] | select(.epoch == $epoch and .class != "NARROWED")] as $all
| ([$parsed[] | select(.epoch == $epoch and .class == "NARROWED")] | length) as $narrowed
| (($ARGS.named.exhausted // "") == "1"
   or any($parsed[]; (.epoch | tonumber) < ($epoch | tonumber))) as $covered
| (reduce $all[] as $r ({seen: {}, rows: []};
    ($r.pr + "-" + $r.sha) as $k
    | if .seen[$k] then . else .seen[$k] = true | .rows += [$r] end) | .rows) as $unique
| (reduce $unique[] as $r ({rows: [], infra: 0};
    if (.rows | length) >= 20 then . elif $r.class == "INFRA" then .infra += 1 else .rows += [$r] end)) as $window
| counts($unique) as $e
| counts($window.rows) as $w
| (if ($e.complete >= 8 and $e.failing >= 1 and $e.miss == 0) | not then "NOT MET"
   elif $covered then "MET"
   else "NOT ESTABLISHED" end) as $status
| (if $w.miss >= 2 or $w.incomplete >= 3 then "TRIPPED" else "clear" end) as $kill
# AFP-V0-484: the advisory shadow cost of the same rows, from the ci-forward-cost-* names (AFP-V0-483)
# of the epoch, joined to the rows above that are complete. A cost name binds to a row only when it
# repeats the row's class and failing flag and is listed before it (uploaded after it, by the job
# that uploaded the row); the nearest such name wins, and a row without one is unpriced, so an
# older run's cost never prices a re-run's row. It is computed after the status and the kill switch
# and never feeds either, nor any narrowing.
| [split("\n") | to_entries[] | .key as $i | .value
   | capture("^ci-forward-cost-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-(?<class>CLEAN|FLAKY|MISS)-f(?<f>[01])-s(?<s>[0-9]{4}|na)-b(?<b>[0-9]{4}|na)-c(?<c>[0-9]{4}|na)-d(?<d>[0-9]{4}|na)-m(?<m>[01n]{4})$")
   | select(.epoch == $epoch) | .i = $i] as $costNames
| (reduce $costNames[] as $c ({}; .[$c.pr + "-" + $c.sha] += [$c])) as $costsOf
| def costOf($r): [($costsOf[$r.pr + "-" + $r.sha] // [])[] | select(.class == $r.class and .f == $r.f and .i < $r.i)]
    | max_by(.i) // null;
  def costs($rows):
    [$rows[] | select(.class == "CLEAN" or .class == "FLAKY" or .class == "MISS")] as $complete
    | [$complete[] | costOf(.) | select(. != null)] as $cs
    | {complete: ($complete | length), priced: ($cs | length)}
      + (reduce (["selection", "s", 0], ["changed", "b", 1], ["hop1", "c", 2], ["hop2", "d", 3]) as [$name, $slot, $i] ({};
          [$cs[] | .[$slot] | select(. != "na") | sub("^0+(?=[0-9])"; "") | tonumber] as $pm
          | [$cs[] | .m[$i:$i + 1] | select(. != "n")] as $flags
          | .[$name] = {measured: ($pm | length),
                        medianShare: (if $pm == [] then null else ($pm | sort)[($pm | length) / 2 | floor] / 1000 end),
                        meanShare: (if $pm == [] then null else (($pm | add) / ($pm | length) | round) / 1000 end),
                        missMeasured: ($flags | length),
                        missRows: ([$flags[] | select(. == "1")] | length)}));
  {advisory: "never feeds the AFP-V0-029 status, the AFP-V0-030 kill switch or narrowing",
   epoch: costs($unique), window: costs($window.rows)} as $shadowCost
| if ($ARGS.named.format // "") == "json" then
    {status: $status, epoch: ($epoch | tonumber), covered: $covered, epochRows: $e,
     window: $w, windowInfra: $window.infra, killSwitch: $kill, narrowed: $narrowed, shadowCost: $shadowCost,
     # CIN-V0-004: the window's row names, newest first, so a pull request re-checks the kill
     # switch over the rows since the verdict plus these instead of listing back to the window.
     windowNames: [$window.rows[] | "ci-forward-\(.class)-pr\(.pr)-\(.sha)-e\(.epoch)-f\(.f)"]}
  else
  "### AFP-V0-029 forward rows, epoch \($epoch)",
  "",
  "Every listed row of the epoch (AFP-V0-029 bar): \($e.complete) complete, \($e.failing) complete with failing packages, \($e.flaky) FLAKY, \($e.miss) MISS, \($e.incomplete) INCOMPLETE, \($e.infra) INFRA.",
  "",
  "Latest \($w.rows) row(s) of the epoch (AFP-V0-030 window): \($w.complete) complete, \($w.failing) complete with failing packages, \($w.flaky) FLAKY, \($w.miss) MISS, \($w.incomplete) INCOMPLETE.",
  "",
  "Hosted infrastructure failures (AFP-V0-052): \($window.infra) INFRA row(s) between them, outside the window and neither complete nor INCOMPLETE.",
  "",
  "Narrowed pull-request runs (AFP-V0-099): \($narrowed) NARROWED row(s), outside the bar and the window.",
  "",
  "Shadow bar (8 complete rows of the epoch, at least 1 with failing packages, no non-FLAKY miss in the epoch): "
    + $status
    + " (\($e.complete)/8 complete, \($e.failing)/1 failing)"
    + (if $covered then "." else "; the listing did not reach the start of epoch \($epoch), so an older MISS row cannot be excluded." end),
  "",
  "Kill switch (AFP-V0-030: 2 MISS or 3 INCOMPLETE in the latest 20): " + $kill + ".",
  "",
  (["epoch", "window"][] as $scope | $shadowCost[$scope] as $c
   | "Shadow cost, \($scope) (AFP-V0-484, advisory; never the bar or the kill switch): \($c.priced) of \($c.complete) complete row(s) priced; share of Full as median/mean, then rows with a miss: "
     + ([["S", "selection"], ["B", "changed"], ["C", "hop1"], ["D", "hop2"]]
        | map(.[0] as $l | $c[.[1]] | "\($l) \(.medianShare // "-")/\(.meanShare // "-") over \(.measured), \(.missRows) of \(.missMeasured) missed")
        | join("; ")) + "."),
  "",
  "This count is advisory: only the trusted AFP-V0-097 verdict artifact lets a pull request narrow."
  end
