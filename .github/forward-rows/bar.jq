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
# AFP-V0-983 (V1-3750): the advisory mainMiss count reads the epoch's ci-forward-MAINMISS-* names
# (one per failed full run of main; a re-run's newer name replaces) the same way.
# CIN-V0-7922 (V1-5198): a suspected main miss is confirmed by reruns (mainmiss-confirm.sh); a
# confirmed one is uploaded as a MISS row, ci-forward-MISS-pr<N>-<merge>-e<E>-f1, which the row
# pattern above counts like any MISS (main-miss.yml uploads it only while the repository variable
# CORVINT_MAIN_MISS_KILL is `on`). The check names, ci-forward-MAINCHECK-r<run>-<sha>-e<E>-
# c<confirmed>-k<flake>-u<unconfirmed>-pr<N>[_<N>...] (one per failed run; a re-run's newer name
# replaces), are reported as mainMiss.checks so a MAINMISS suspicion can be read against its verdict.
# CIN-V0-7921 (V1-5218): a FULL row (ci-forward-FULL-pr<N>-<sha>-e<E>-f<f>-r<skew|abstain|plan>,
# a pull-request run whose every shadow shard ran FULL) measured no narrowing: it is neither
# complete nor INCOMPLETE, takes no place in the window and is counted on its own, one per
# (pull request, commit), by type.
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
# CIN-V0-7921: FULL rows carry a type suffix, so the row pattern above never matches them.
| ([split("\n")[]
    | capture("^ci-forward-FULL-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-f(?<f>[01])-r(?<why>skew|abstain|plan)$")
    | select(.epoch == $epoch)]
   | reduce .[] as $r ({seen: {}, rows: []}; ($r.pr + "-" + $r.sha) as $k | if .seen[$k] then . else .seen[$k] = true | .rows += [$r] end)
   | .rows
   | {rows: length,
      skew: ([.[] | select(.why == "skew")] | length),
      abstain: ([.[] | select(.why == "abstain")] | length),
      plan: ([.[] | select(.why == "plan")] | length),
      pullRequests: ([.[].pr | tonumber] | unique)}) as $full
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
| ([split("\n")[]
    | capture("^ci-forward-MAINMISS-r(?<run>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-n(?<n>[0-9]+)-pr(?<prs>[0-9]+(_[0-9]+)*)$")
    | select(.epoch == $epoch)]
   | reduce .[] as $m ({seen: {}, rows: []}; if .seen[$m.run] then . else .seen[$m.run] = true | .rows += [$m] end)
   | .rows
   | {advisory: "a suspicion; only a confirmed miss, uploaded as a MISS row (CIN-V0-7922), feeds the AFP-V0-029 status and the AFP-V0-030 kill switch",
      runs: length, misses: (map(.n | tonumber) | add // 0),
      pullRequests: ([.[].prs | split("_")[] | tonumber] | unique)}) as $mainMiss
# CIN-V0-7922: the confirmation checks of the epoch's failed runs, one per run.
| ([split("\n")[]
    | capture("^ci-forward-MAINCHECK-r(?<run>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-c(?<c>[0-9]+)-k(?<k>[0-9]+)-u(?<u>[0-9]+)-pr(?<prs>[0-9]+(_[0-9]+)*)$")
    | select(.epoch == $epoch)]
   | reduce .[] as $m ({seen: {}, rows: []}; if .seen[$m.run] then . else .seen[$m.run] = true | .rows += [$m] end)
   | .rows
   | {runs: length, confirmed: (map(.c | tonumber) | add // 0), flake: (map(.k | tonumber) | add // 0),
      unconfirmed: (map(.u | tonumber) | add // 0),
      pullRequests: ([.[].prs | split("_")[] | tonumber] | unique)}) as $mainChecks
| ($mainMiss + {checks: $mainChecks}) as $mainMiss
# AFP-V0-984 (V1-3751): the sampled full runs of narrowing pull requests (one per pull request and
# head; a re-run's newer name replaces), their misses, and the Wilson 95% upper bound of the rate.
| ([split("\n")[]
    | capture("^ci-forward-SAMPLE-pr(?<pr>[0-9]+)-(?<sha>[0-9a-f]{40})-e(?<epoch>[0-9]+)-k(?<k>[0-9]+)-m(?<m>[01])$")
    | select(.epoch == $epoch)]
   | reduce .[] as $r ({seen: {}, rows: []}; ($r.pr + "-" + $r.sha) as $key | if .seen[$key] then . else .seen[$key] = true | .rows += [$r] end)
   | .rows
   | length as $n | ([.[] | select(.m == "1")] | length) as $x
   | {advisory: "never feeds the AFP-V0-029 status, the AFP-V0-030 kill switch or narrowing",
      sampled: $n, misses: $x,
      estimate: (if $n == 0 then null else (($x / $n) * 1000 | round) / 1000 end),
      upper95: (if $n == 0 then null
                else 1.96 as $z | ($x / $n) as $p
                  | (($p + $z * $z / (2 * $n) + $z * ((($p * (1 - $p) / $n) + $z * $z / (4 * $n * $n)) | sqrt)) / (1 + $z * $z / $n))
                  | (. * 1000 | ceil) / 1000 end)}) as $sampledMissRate
| if ($ARGS.named.format // "") == "json" then
    {status: $status, epoch: ($epoch | tonumber), covered: $covered, epochRows: $e,
     window: $w, windowInfra: $window.infra, killSwitch: $kill, narrowed: $narrowed, full: $full, shadowCost: $shadowCost,
     mainMiss: $mainMiss, sampledMissRate: $sampledMissRate,
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
  "FULL shadow runs (CIN-V0-7921): \($full.rows) FULL row(s), outside the bar and the window: \($full.skew) skew (the trusted tool refused the tested tree's affected-graph inputs), \($full.abstain) abstain, \($full.plan) planned whole"
    + (if ($full.pullRequests | length) > 0 then "; " + ($full.pullRequests | map("#\(.)") | join(", ")) else "" end) + ".",
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
  "Main full-run misses (AFP-V0-983, suspected; a miss confirmed by reruns is uploaded as a MISS row, CIN-V0-7922): \($mainMiss.misses) narrowed pull request(s) skipped a package a later full run of main failed, over \($mainMiss.runs) failed run(s)"
    + (if ($mainMiss.pullRequests | length) > 0 then ": " + ($mainMiss.pullRequests | map("#\(.)") | join(", ")) else "" end)
    + "; confirmation checks over \($mainMiss.checks.runs) run(s): \($mainMiss.checks.confirmed) confirmed, \($mainMiss.checks.flake) flake, \($mainMiss.checks.unconfirmed) unconfirmed.",
  "",
  "Sampled full runs (AFP-V0-984, advisory; never the bar or the kill switch): \($sampledMissRate.misses) of \($sampledMissRate.sampled) sampled narrowing pull request(s) failed a package outside the driver's selection"
    + (if $sampledMissRate.sampled == 0 then "; no estimate yet." else "; estimated miss rate \($sampledMissRate.estimate), Wilson 95% upper bound \($sampledMissRate.upper95)." end),
  "",
  "This count is advisory: only the trusted AFP-V0-097 verdict artifact lets a pull request narrow."
  end
