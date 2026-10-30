# SPDX-License-Identifier: Apache-2.0
# AFP-V0-098: one shard's corvint-pr-narrowing/0 decision. A pull-request shard
# narrows (mode NARROWED) only when every condition holds; otherwise it runs the
# full shard (mode FULL) and the record names every reason:
#   the base branch is main, the change touches no .github/ path, main has a
#   merge-queue rule, the deciding driver's selection (corvint-pr-tests --mode
#   plan) is complete and names this base, head and target, the shadow
#   `corvint test --dry-run` of the same (base, head) over the whole universe is
#   complete, the driver's selection is a superset of the shadow's, and this
#   shard's part of the AFP-V0-590 balanced placement was computed from a
#   recorded selection equal to the driver's selection within the universe.
# Usage: jq -n -c --slurpfile driver SELECTION --slurpfile shadow DRY_RUN
#   --arg shadowExit --rawfile packages SHARD_PACKAGES --rawfile slices SHARD_SLICES
#   --rawfile selected SELECTED --rawfile balancedPackages BALANCED_PACKAGES
#   --rawfile balancedSlices BALANCED_SLICES --arg balanced (1|other)
#   --rawfile universe UNIVERSE --arg selectedOk (1|other)
#   --arg github (0|1|other) --arg mergeQueue (1|0|other) --arg baseRef --arg base
#   --arg eventBase (original PR base; optional for historical callers)
#   --arg head --arg target --arg pr --arg runId --arg runAttempt --arg shard
#   --arg shards --arg toolSource --arg verdictRun -f narrow.jq
# SHARD_PACKAGES is this shard's whole packages and SHARD_SLICES its
# "FLAG PATTERN PACKAGE" slice lines, from the same ci-shards call as the full
# branch. SELECTED is the driver's selection within the universe, one package per
# line, and BALANCED_PACKAGES / BALANCED_SLICES this shard's part of its
# `ci-shards --select` placement (AFP-V0-590). UNIVERSE is `go list ./...`, one
# package per line, and selectedOk is 1 only when every stage that wrote SELECTED
# succeeded. `executed` is what the shard runs
# (placement BALANCED: its part of the selection's placement) and `skipped` the
# packages of its complete-universe partition outside the selection, which run
# nowhere; the queue row of AFP-V0-099 reads `skipped`.
# AFP-V0-814 (V1-3100): a merge-queue selection may also name Reused, packages of the queue range
# whose passed result an admitted narrowed pull-request record already holds, with that record in
# Reuse. They are disjoint from the selection, count toward the shadow superset, run nowhere and
# are recorded as `reused` and `reuse`, never as `skipped`. Reused with no applied Reuse is FULL.
def num: if type == "string" and test("^[0-9]+$") then tonumber else null end;
def hex40: type == "string" and test("^[0-9a-f]{40}$");
def limit_only: test("^[a-z][a-z0-9_+-]*: no qualified adapter and ") and (startswith("go: ") | not);
($driver[0] // null) as $d
| ($shadow[0] // null) as $s
| ([$packages | split("\n")[] | select(length > 0)] | unique) as $whole
| ([$slices | split("\n")[] | select(length > 0) | split(" ") | select(length == 3) | .[2]] | unique) as $sliced
| ($whole + $sliced | unique) as $shardSet
| ([$selected | split("\n")[] | select(length > 0)] | unique) as $sel
| ([$universe | split("\n")[] | select(length > 0)] | unique) as $U
| ([$balancedPackages | split("\n")[] | select(length > 0)]
   + [$balancedSlices | split("\n")[] | select(length > 0) | split(" ") | select(length == 3) | .[2]] | unique) as $placed
| (if ($d | type) == "object" and ($d.Packages | type) == "array" then [$d.Packages[] | select(type == "string")] | unique else null end) as $D
| (if ($s | type) == "object" then
     [$s.invocations[]?.units[]? | select(.adapter == "go") | .unit
      | select(type == "string" and startswith("go:")) | ltrimstr("go:")] | unique
   else null end) as $S
| (if ($d | type) == "object" then $d.Reused // [] else [] end) as $Rraw
| (if ($Rraw | type) == "array" and all($Rraw[]; type == "string") then $Rraw | unique else null end) as $R
| (if $D != null and $S != null then $S - $D - ($R // []) else [] end) as $omitted
| [ (if $baseRef != "main" then "the base branch is \($baseRef), not main" else empty end),
    (if $github == "1" then "the change touches .github/"
     elif $github != "0" then "the change set is unreadable, so a .github/ change cannot be excluded" else empty end),
    (if $mergeQueue == "0" then "main has no merge-queue rule"
     elif $mergeQueue != "1" then "the rules of main are unreadable" else empty end),
    (if ($base | hex40 | not) or ($head | hex40 | not) or ($target | hex40 | not) then "the base, head or target is not a full commit id" else empty end),
    (if $d == null then "the driver wrote no selection"
     elif ($d | type) != "object" then "the driver selection is not an object"
     elif ($d.Reason // "") != "" then "driver: \($d.Reason | .[0:300])"
     elif $D == null or $D == ["./..."] then "the driver selected the whole universe"
     elif $d.Base != $base or $d.Head != $head or $d.Target != $target then "the driver planned another base, head or target"
     else empty end),
    (if $s == null then "the shadow wrote no plan"
     elif ($s | type) != "object" then "the shadow plan is not an object"
     else
       (if $shadowExit != "0" and $shadowExit != "3" then "corvint test --dry-run exited \($shadowExit)" else empty end),
       (if ($s.scope | IN("NARROWED", "FULL", "NOTHING")) | not then "the shadow scope is \($s.scope)" else empty end),
       ([($s.incomplete // [])[] | select(type != "string" or (limit_only | not))] | if length > 0 then "the shadow plan is incomplete: \(map(tostring) | join("; ") | .[0:300])" else empty end)
     end),
    (if $R == null then "the reused list is not a list of packages"
     elif ($R | length) == 0 then empty
     elif ($d.Reuse | type) != "object" or $d.Reuse.applied != true or ($d.Reuse.recordRunId | type) != "number"
       or ($d.Reuse.recordJobId | type) != "number" or ($d.Reuse.recordBase | hex40 | not) or ($d.Reuse.queueBase | hex40 | not)
       or ($d.Reuse.reused != ($R | length)) then "the reused packages name no applied reuse record"
     elif $D != null and ($D - ($D - $R) | length) > 0 then "a reused package is also selected"
     else empty end),
    (if ($omitted | length) > 0 then
       "the driver selection is not a superset of the shadow selection: it omits \($omitted | length) package(s) (\($omitted[0:5] | join(", ")))"
     else empty end),
    (if $selectedOk != "1" then "the selection within the universe could not be computed"
     elif $D == null then empty
     elif $sel != ($D - ($D - $U)) then "the recorded selection is not the driver selection within the universe"
     elif ($D | length) > 0 and ($sel | length) == 0 then "the driver selection names no package of the universe"
     else empty end),
    (if $balanced != "1" then "the balanced placement of the selection failed"
     elif ($placed - $sel | length) > 0 then "the balanced placement names an unselected package" else empty end)
  ] as $reasons
| (if ($reasons | length) == 0 then "NARROWED" else "FULL" end) as $mode
| (if $mode == "NARROWED" then $placed else $shardSet end) as $executed
| {profile: "corvint-pr-narrowing/0",
   mode: $mode,
   placement: (if $mode == "NARROWED" then "BALANCED" else "PARTITION" end),
   reasons: $reasons,
   pr: ($pr | num), base: $base, eventBase: ($ARGS.named.eventBase // $base), effectiveBase: $base, head: $head, target: $target,
   runId: ($runId | num), runAttempt: ($runAttempt | num),
   shard: ($shard | num), shards: ($shards | num),
   toolSource: $toolSource, verdictRunId: ($verdictRun | num),
   driverSelected: ($D // [] | length), shadowSelected: ($S // [] | length),
   shadowScope: ($s.scope? // null),
   omitted: $omitted[0:50],
   shardPackages: ($shardSet | length),
   selected: (if $mode == "NARROWED" then $sel else [] end),
   executed: $executed,
   skipped: (if $mode == "NARROWED" then $shardSet - $D - $R else [] end),
   reused: (if $mode == "NARROWED" then $R else [] end),
   reuse: (if ($d | type) == "object" and ($d.Reuse | type) == "object" then $d.Reuse else null end)}
