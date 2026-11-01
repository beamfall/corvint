# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: one corvint-pr-forward-row/0 from the bundle classify.sh assembles.
# Input (jq -n): JSON lines {kind: selection|outcomes|reuse, name, doc} or
# {kind, name, unreadable: true}. Arguments: kind (queue|pr), pr, sha, head,
# runId, selectionRunId, outcomesRunId, shards, epoch, collect (newline-
# separated reasons the collector already found) and infra (newline-separated
# shard numbers whose go-product-shard job of the outcomes run failed at "Set
# up job"). Every missing, unreadable or inconsistent input is an incomplete
# reason; a row with any reason is INCOMPLETE and never CLEAN. AFP-V0-052: a
# row is INFRA instead when its only reasons are those hosted set-up failures
# and the coverage gap of exactly their shards, and it has no non-FLAKY miss.
# AFP-V0-099: {kind: narrowing} lines are the selection run's corvint-pr-narrowing/0
# records. A pull-request row of a run that narrowed any shard (or whose record is
# unreadable) is NARROWED. A queue row whose selection run narrowed counts every
# package that run narrowed out as unselected, so a queue failure of one is a MISS.
def num: if type == "string" and test("^[0-9]+$") then tonumber else null end;
def str: if type == "string" and length > 0 then . else null end;
def shortr: if length > 300 then .[0:300] + "..." else . end;
def hex40: type == "string" and test("^[0-9a-f]{40}$");
def digest: type == "string" and test("^sha256:[0-9a-f]{64}$");
# A non-Go language without an adapter or declared full command is a limit of
# the selection, not of its Go run set; every other gap makes the row incomplete.
def limit_only: test("^[a-z][a-z0-9_+-]*: no qualified adapter and ") and (startswith("go: ") | not);
def coverage($docs; $n; $what):
  ($docs | map(.shard)) as $s
  | if ($s | sort) == [range(0; $n)] then empty
    else "\($what) cover shards \($s | sort | map(tostring) | join(",")) of \($n), want each of 0..\($n - 1) once" end;
def covers($docs; $n; $gone): ($docs | map(.shard) | sort) == ([range(0; $n)] - $gone);

[inputs] as $in
| ($shards | num) as $n
| ($pr | num) as $prn
| [$in[] | select(.kind == "selection")] as $selA
| [$in[] | select(.kind == "outcomes")] as $outA
| [$in[] | select(.kind == "reuse")] as $reuseA
| [$in[] | select(.kind == "narrowing")] as $narA
| [$narA[] | select(.unreadable | not) | .doc
   | select(type == "object" and .profile == "corvint-pr-narrowing/0" and .shards == $n
            and (.mode == "NARROWED" or .mode == "FULL")
            and (.shard | type) == "number" and .shard >= 0 and .shard < $n
            and (.runId | type) == "number" and (.executed | type) == "array" and (.skipped | type) == "array")] as $nar
| ([$narA[] | select(.unreadable)] | length > 0 or ($narA | length) != ($nar | length)
   or any($nar[]; .mode == "NARROWED")) as $narrowedRun
| ([$nar[] | select(.mode == "NARROWED") | .skipped[] | select(type == "string")] | unique) as $narrowedOut
| ($reuseA | map(select(.unreadable | not) | .doc) | first // null) as $reuse
| ($kind == "queue" and $reuse != null and $reuse.profile == "corvint-ci-reuse/0" and $reuse.mode == "REUSE") as $reused
| [$selA[] | select(.unreadable | not) | .doc
   | select(type == "object" and .profile == "corvint-pr-selection/0" and .shards == $n
            and (.shard | type) == "number" and .shard >= 0 and .shard < $n
            and (.toolSource | hex40) and (.toolDigest | digest) and (.head | hex40) and (.target | hex40)
            and (.runId | type) == "number" and (.exit | type) == "number" and (.scope | type) == "string"
            and (.manifestDigest | digest) and (.planDigest | digest) and (.partitionDigest | digest)
            and (.selected | type) == "array" and ((.incomplete // []) | type) == "array")] as $sel
| [$outA[] | select(.unreadable | not) | .doc
   | select(type == "object" and .profile == "corvint-ci-outcomes/0" and .shards == $n
            and (.shard | type) == "number" and .shard >= 0 and .shard < $n
            and (.event == "pull_request" or .event == "merge_group") and (.target | hex40)
            and (.runId | type) == "number" and (.status | type) == "number"
            and (.outcomes | type) == "object" and (.rerun | type) == "object"
            and all(.outcomes[]; . == "pass" or . == "fail" or . == "skip" or . == "missing"))] as $out
| ([$out[].target] | unique) as $targets
| (if $kind == "pr" then (if ($targets | length) == 1 then $targets[0] else null end) else ($sha | str) end) as $rowSha
| ([$sel[] | (.incomplete // [])[] | select(type == "string")] | unique) as $gaps
| ([$infra | split("\n")[] | select(test("^[0-9]+$")) | tonumber] | unique) as $infraShards
| [$infraShards[] | "infrastructure: go-product-shard (\(.)) of run \($outcomesRunId) failed at Set up job"] as $infrastructure
| (if $n == null then [] else [coverage($sel; $n; "selection documents"), coverage($out; $n; "outcomes documents")] end) as $coverage
# The hosted failures explain the coverage gap only when the outcomes miss exactly their shards
# and the selection is whole, or, for a row whose selection is the same run, misses them too.
| ($n != null and ($infraShards | length) > 0 and all($infraShards[]; . < $n)
   and covers($out; $n; $infraShards)
   and (covers($sel; $n; []) or ($selectionRunId == $outcomesRunId and covers($sel; $n; $infraShards)))) as $explained
| [ ($collect | split("\n")[] | select(length > 0)),
    (if $n == null then "shard count \($shards) is not a number" else empty end),
    (if $kind != "queue" and $kind != "pr" then "row kind \($kind) is unknown" else empty end),
    (if $prn == null then "pull request number \($pr) is not a number" else empty end),
    (if $rowSha == null then "the tested commit is unknown (outcome targets \($targets | join(",")))" else empty end),
    ($selA[] | select(.unreadable) | "selection artifact \(.name) is unreadable"),
    ($outA[] | select(.unreadable) | "outcomes artifact \(.name) is unreadable"),
    ($reuseA[] | select(.unreadable) | "reuse decision \(.name) is unreadable"),
    ($narA[] | select(.unreadable) | "narrowing record \(.name) is unreadable"),
    (if ($narA | length) != ($nar | length) then "\(($narA | length) - ($nar | length)) narrowing record(s) fail the corvint-pr-narrowing/0 shape or shard count" else empty end),
    ($nar[] | select(.runId != ($selectionRunId | num)) | "narrowing shard \(.shard) is run \(.runId), want \($selectionRunId)"),
    (if ([$nar[].shard] | unique | length) != ($nar | length) then "narrowing records name a shard twice" else empty end),
    (if ($selA | length) != ($sel | length) then "\(($selA | length) - ($sel | length)) selection document(s) fail the corvint-pr-selection/0 shape or shard count" else empty end),
    (if ($outA | length) != ($out | length) then "\(($outA | length) - ($out | length)) outcomes document(s) fail the corvint-ci-outcomes/0 shape or shard count" else empty end),
    $coverage[],
    ($sel[] | select(.runId != ($selectionRunId | num)) | "selection shard \(.shard) is run \(.runId), want \($selectionRunId)"),
    ($out[] | select(.runId != ($outcomesRunId | num)) | "outcomes shard \(.shard) is run \(.runId), want \($outcomesRunId)"),
    ($sel[] | select(.head != $head) | "selection shard \(.shard) planned head \(.head), want \($head)"),
    ($sel[] | select(.shardIndex != .shard or .shardCount != .shards) | "selection shard \(.shard) planned shard \(.shardIndex) of \(.shardCount)"),
    ($sel[] | select(.exit != 0 and .exit != 3) | "selection shard \(.shard): corvint test --dry-run exited \(.exit)"),
    ($sel[] | select(.scope != "NARROWED" and .scope != "FULL" and .scope != "NOTHING") | "selection shard \(.shard) has scope \(.scope)"),
    (if ([$sel[].manifestDigest] | unique | length) > 1 then "selection shards disagree on the manifest digest" else empty end),
    ($gaps[] | select(limit_only | not) | "selection incomplete: \(. | shortr)"),
    ($out[] | select(.status != 0 and ([.outcomes[] | select(. == "fail" or . == "missing")] | length) == 0)
      | "outcomes shard \(.shard) exited \(.status) with no failed or missing package"),
    (if $kind == "pr" then
       ($out[] | select(.event != "pull_request" or .head != $head) | "outcomes shard \(.shard) is \(.event) of head \(.head), want pull_request of \($head)")
     elif $reused then
       (if $reuse.head != $head or ($reuse.runId | type) != "number" or ($reuse.records | length) != $n
        then "reuse decision does not name a complete reused run of head \($head)" else empty end),
       ($out[] | select(.runId != $reuse.runId or .tree != $reuse.tree) | "outcomes shard \(.shard) is not run \($reuse.runId) of tree \($reuse.tree)")
     else
       ($out[] | select(.event != "merge_group" or .target != $sha) | "outcomes shard \(.shard) is \(.event) of \(.target), want merge_group of \($sha)")
     end),
    $infrastructure[]
  ] as $incomplete
| ($incomplete - $infrastructure - (if $explained then $coverage else [] end)) as $unexplained
| (([$sel[].selected[] | select(type == "string")] | unique) - $narrowedOut) as $S
| ([$out[] | .outcomes | to_entries[] | select(.value == "fail" or .value == "missing") | .key] | unique) as $F
| ($F - $S) as $missed
| ($F - $missed) as $both
| [$missed[] as $p
   # A package the selection run narrowed out is never FLAKY: it ran nowhere before the
   # queue, so any queue failure of it is a MISS (AFP-V0-099).
   | select(IN($narrowedOut[]; $p) | not)
   | select([$out[] | select(.outcomes[$p] == "fail" or .outcomes[$p] == "missing")]
            | all(.rerun.state == "RAN" and .rerun.outcomes[$p] == "pass"))
   | $p] as $flaky
# AFP-V0-984 (V1-3751): a pull-request run whose every shard record was sampled ran the full suite
# in place of the driver's selection; a failed package outside that selection, FLAKY ones aside,
# is a miss of the selection. Any other run carries no sample.
| [$nar[] | select((.sample | type) == "object" and (.sample.k | type) == "number" and (.sample.selected | type) == "array")] as $sampledRecs
| (if $kind == "pr" and ($nar | length) == $n and ($sampledRecs | length) == $n then
     ([$sampledRecs[].sample.selected[] | select(type == "string")] | unique) as $T
     | {k: $sampledRecs[0].sample.k, selected: ($T | length), failingOutside: (($F - $flaky) - $T)}
   else null end) as $sample
| (if $kind == "pr" and $narrowedRun then "NARROWED"
   elif ($incomplete | length) > 0 then
     (if $explained and ($unexplained | length) == 0 and ($missed - $flaky | length) == 0 then "INFRA" else "INCOMPLETE" end)
   elif ($missed - $flaky | length) > 0 then "MISS"
   elif ($flaky | length) > 0 then "FLAKY"
   else "CLEAN" end) as $class
# AFP-V0-481/482: the shadow cost of S, of the AFP-V0-480 tiers B, C and D, and of Full (every
# package with an outcome), priced by package-costs.json (--slurpfile costs; a package it does not
# price takes its median and is counted as unpriced), and each selector's miss: a failing package
# outside it that is not FLAKY. A measurement only: nothing here reaches $class or the bar status.
| ([$out[].outcomes | keys[]] | unique) as $full
| ($full | map({key: ., value: true}) | from_entries) as $inFull
| (($ARGS.named.costs // [])[0] // null) as $costDoc
| (if ($costDoc | type) == "object" and $costDoc.profile == "corvint-ci-package-costs/0"
      and ($costDoc.milliseconds | type) == "object" and ($costDoc.milliseconds | length) > 0
      and all($costDoc.milliseconds[]; type == "number" and . > 0)
   then $costDoc.milliseconds else null end) as $price
| (if $price == null then null else ([$price[]] | sort) as $v | $v[($v | length) / 2 | floor] end) as $median
| def msOf($set): if $price == null then null else [$set[] | $price[.] // $median] | add // 0 end;
  msOf($full) as $fullMs
| [$F[] as $p
   | select(IN($narrowedOut[]; $p) | not)
   | select([$out[] | select(.outcomes[$p] == "fail" or .outcomes[$p] == "missing")]
            | all(.rerun.state == "RAN" and .rerun.outcomes[$p] == "pass"))
   | $p] as $flakyAny
| ($F - $flakyAny) as $realFail
| def measure($set; $isFull):
    (if $isFull then $full else [$set[] | select($inFull[.])] | unique end) as $in
    | msOf($in) as $ms
    | (if $isFull then [] else $realFail - $set end) as $miss
    | {full: $isFull, packages: ($in | length), ms: $ms,
       permille: (if $ms == null or ($fullMs // 0) == 0 then null else ($ms * 1000 / $fullMs | round) end),
       missed: ($miss | length), missedPackages: $miss};
  [$sel[] | select(has("tiers") or has("tierIncomplete"))] as $tsel
| ([$tsel[] | .tiers | select(type == "object") | del(.millis)] | unique) as $tv
| def tierShape: type == "object"
    and all(.changed, .hop1, .hop2; . == "FULL" or (type == "array" and all(.[]; type == "string")));
  [ (if ($tsel | length) == 0 then "no selection document carries tiers (a shadow step before AFP-V0-480)" else empty end),
    ($tsel[] | (.tierIncomplete // []) | if type == "array" then .[] else "tierIncomplete is not a list" end | tostring
     | "tiers: \(.)"),
    (if ($tsel | length) > 0 and ($tv | length) == 0 then "no selection document carries a tier object" else empty end),
    (if ($tv | length) > 1 then "selection shards disagree on the tiers" else empty end),
    (if ($tv | length) == 1 and ($tv[0] | tierShape | not) then "the tiers fail the changed/hop1/hop2 shape" else empty end)
  ] as $tierGaps
| (if ($tierGaps | length) == 0 then $tv[0] else null end) as $T
| def tier($name): if $T == null then null elif $T[$name] == "FULL" then measure([]; true) else measure($T[$name]; false) end;
  {incomplete: ([$tierGaps[0:20][] | shortr]
                + (if $price == null then ["package-costs.json is missing or fails the corvint-ci-package-costs/0 shape, so no cost is priced"] else [] end)
                + (if ($full | length) == 0 then ["no package has an outcome, so Full is empty"] else [] end)),
   medianMs: $median,
   unpriced: (if $price == null then null else [$full[] | select($price[.] == null)] | length end),
   fullReasons: (if $T == null or ($T.full | type) != "array" then [] else $T.full[0:20] | map(tostring | shortr) end),
   tierMillis: ([$tsel[].tiers | objects | .millis | numbers] | max),
   full: {packages: ($full | length), ms: $fullMs},
   selection: measure($S; false),
   changed: tier("changed"), hop1: tier("hop1"), hop2: tier("hop2")} as $cost
| ($sel | map(.manifestDigest) | unique) as $md
| def costAt($k): $cost | .selection.missedPackages |= .[0:$k]
    | reduce ("changed", "hop1", "hop2") as $t (.; if .[$t] == null then . else .[$t].missedPackages |= .[0:$k] end);
  def fits: tojson | utf8bytelength <= 65536;
  def row($k; $shadowCost):
    {profile: "corvint-pr-forward-row/0",
     kind: $kind,
     epoch: ($epoch | num),
     class: $class,
     pr: $prn,
     sha: $rowSha,
     head: ($head | str),
     runId: ($runId | num),
     selectionRunId: ($selectionRunId | num),
     outcomesRunId: ($outcomesRunId | num),
     outcomesSource: (if $kind == "pr" then "pull_request" elif $reused then "reuse" else "merge_group" end),
     shards: $n,
     selection: {
       toolSource: ([$sel[].toolSource] | unique | if length == 1 then .[0] else null end),
       manifestDigest: (if ($md | length) == 1 then $md[0] else null end),
       planDigest: ([$sel[].planDigest] | unique | if length == 1 then .[0] else null end),
       partitionDigest: ([$sel[].partitionDigest] | unique | if length == 1 then .[0] else null end),
       scope: ([$sel[].scope] | unique | if length == 1 then .[0] else null end),
       selected: ($S | length),
       limits: [$gaps[] | select(limit_only) | shortr],
       notProduced: ([$sel[].notProduced[]?] | unique),
       narrowed: (if ($narA | length) == 0 then null
                  else {shards: [$nar[] | select(.mode == "NARROWED") | .shard] | sort,
                        narrowedOut: ($narrowedOut | length)} end)},
     counts: {failed: ($F | length), missed: ($missed | length), flaky: ($flaky | length),
              selectedFailedInQueue: ($both | length)},
     reruns: ([$out[].rerun.state] | reduce .[] as $s ({}; .[$s] += 1)),
     failed: $F[0:$k],
     missed: $missed[0:$k],
     flaky: $flaky[0:$k],
     selectedFailedInQueue: $both[0:$k],
     incomplete: ($incomplete | map(shortr))[0:$k],
     infrastructure: $infrastructure,
     truncated: ([$F, $missed, $flaky, $both, $incomplete] | any(length > $k)),
     shadowCost: $shadowCost}
    # AFP-V0-984: only a sampled run's row carries the optional sample member.
    + (if $sample == null then {} else {sample: ($sample | .failingOutside |= .[0:$k] | .missed = ($sample.failingOutside | length > 0))} end);
# The row is bounded without its advisory shadowCost first, so the cost never truncates or drops the
# row; the cost then takes the room left, shortened, replaced by a note, or null.
  first((1000000, 200, 50, 10, 0) as $k | select(row($k; null) | fits) | $k) as $k0
  | first((((1000000, 200, 50, 10, 0) | select(. <= $k0)) as $j | row($k0; costAt($j)) | select(fits)),
          (row($k0; {incomplete: ["the shadow cost did not fit the 64 KiB row bound and was dropped"]}) | select(fits)),
          row($k0; null))
