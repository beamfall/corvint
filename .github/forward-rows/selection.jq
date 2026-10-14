# SPDX-License-Identifier: AGPL-3.0-or-later
# AFP-V0-047: one shard's corvint-pr-selection/0 document, the retained S of
# AFP-V0-029, from the shadow `corvint test --dry-run` stdout and manifest.
# Usage: jq -n -c --slurpfile dry DRY_RUN --slurpfile manifest MANIFEST
#   --arg toolSource --arg toolDigest --arg pr --arg head --arg base --arg target
#   --arg tree --arg runId --arg runAttempt --arg shard --arg shards --arg exit
#   -f selection.jq
# `selected` is the shard's Go run set: the packages of the dry-run's go units
# (whole packages and slices alike). What the dry-run cannot provide is named
# in notProduced, never invented.
def num: if type == "string" and test("^[0-9]+$") then tonumber else null end;
def opt: if type == "string" and length > 0 then . else null end;
($dry[0] // {}) as $d
| ($manifest[0] // {}) as $m
| {profile: "corvint-pr-selection/0",
   toolSource: ($toolSource | opt),
   toolDigest: ($toolDigest | opt),
   pr: ($pr | num),
   head: ($head | opt),
   base: ($base | opt),
   target: ($target | opt),
   tree: ($tree | opt),
   runId: ($runId | num),
   runAttempt: ($runAttempt | num),
   shard: ($shard | num),
   shards: ($shards | num),
   exit: ($exit | num),
   scope: ($d.scope // null),
   reason: ($d.reason // null),
   placement: ($d.placement // null),
   shardIndex: ($d.shardIndex // null),
   shardCount: ($d.shardCount // null),
   incomplete: ($d.incomplete // ["the dry-run printed no plan"]),
   selected: ([$d.invocations[]?.units[]? | select(.adapter == "go") | .unit
               | select(type == "string" and startswith("go:")) | ltrimstr("go:")] | unique),
   manifestDigest: ($d.manifestDigest // null),
   manifestUnits: ($m.units // null | if type == "array" then length else null end),
   planDigest: ($m.planDigest // null),
   partitionDigest: ($m.partitionDigest // null),
   sourceDigest: ($m.sourceDigest // null),
   notProduced: ["selectionDigest: the AFP-V0-013 selector/driver selection digest; the driver pins are empty, so no selector runs"]}
