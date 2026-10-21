# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-098: one shard's corvint-pr-narrowing/0 decision. A pull-request shard
# narrows (mode NARROWED) only when every condition holds; otherwise it runs the
# full shard (mode FULL) and the record names every reason:
#   the base branch is main, the change touches no .github/ path, main has a
#   merge-queue rule, the deciding driver's selection (corvint-pr-tests --mode
#   plan) is complete and names this base, head and target, the shadow
#   `corvint test --dry-run` of the same (base, head) over the whole universe is
#   complete, and the driver's selection is a superset of the shadow's.
# Usage: jq -n -c --slurpfile driver SELECTION --slurpfile shadow DRY_RUN
#   --arg shadowExit --rawfile packages SHARD_PACKAGES --rawfile slices SHARD_SLICES
#   --arg github (0|1|other) --arg mergeQueue (1|0|other) --arg baseRef --arg base
#   --arg head --arg target --arg pr --arg runId --arg runAttempt --arg shard
#   --arg shards --arg toolSource --arg verdictRun -f narrow.jq
# SHARD_PACKAGES is this shard's whole packages and SHARD_SLICES its
# "FLAG PATTERN PACKAGE" slice lines, from the same ci-shards call as the full
# branch. `executed` is what the shard runs and `skipped` what it narrows out;
# the queue row of AFP-V0-099 reads `skipped`.
def num: if type == "string" and test("^[0-9]+$") then tonumber else null end;
def hex40: type == "string" and test("^[0-9a-f]{40}$");
def limit_only: test("^[a-z][a-z0-9_+-]*: no qualified adapter and ") and (startswith("go: ") | not);
($driver[0] // null) as $d
| ($shadow[0] // null) as $s
| ([$packages | split("\n")[] | select(length > 0)] | unique) as $whole
| ([$slices | split("\n")[] | select(length > 0) | split(" ") | select(length == 3) | .[2]] | unique) as $sliced
| ($whole + $sliced | unique) as $shardSet
| (if ($d | type) == "object" and ($d.Packages | type) == "array" then [$d.Packages[] | select(type == "string")] | unique else null end) as $D
| (if ($s | type) == "object" then
     [$s.invocations[]?.units[]? | select(.adapter == "go") | .unit
      | select(type == "string" and startswith("go:")) | ltrimstr("go:")] | unique
   else null end) as $S
| (if $D != null and $S != null then $S - $D else [] end) as $omitted
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
    (if ($omitted | length) > 0 then
       "the driver selection is not a superset of the shadow selection: it omits \($omitted | length) package(s) (\($omitted[0:5] | join(", ")))"
     else empty end)
  ] as $reasons
| (if ($reasons | length) == 0 then "NARROWED" else "FULL" end) as $mode
| (if $mode == "NARROWED" then [$shardSet[] | select(IN($D[]))] else $shardSet end) as $executed
| {profile: "corvint-pr-narrowing/0",
   mode: $mode,
   reasons: $reasons,
   pr: ($pr | num), base: $base, head: $head, target: $target,
   runId: ($runId | num), runAttempt: ($runAttempt | num),
   shard: ($shard | num), shards: ($shards | num),
   toolSource: $toolSource, verdictRunId: ($verdictRun | num),
   driverSelected: ($D // [] | length), shadowSelected: ($S // [] | length),
   shadowScope: ($s.scope? // null),
   omitted: $omitted[0:50],
   shardPackages: ($shardSet | length),
   executed: $executed,
   skipped: ($shardSet - $executed)}
