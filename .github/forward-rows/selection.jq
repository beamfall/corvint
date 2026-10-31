# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: one shard's corvint-pr-selection/0 document, the retained S of
# AFP-V0-029, from the shadow `corvint test --dry-run` stdout and manifest.
# Usage: jq -n -c --slurpfile dry DRY_RUN --slurpfile manifest MANIFEST
#   --arg toolSource --arg toolDigest --arg pr --arg head --arg base --arg target
#   --arg tree --arg runId --arg runAttempt --arg shard --arg shards --arg exit
#   -f selection.jq
# `selected` is the shard's Go run set: the packages of the dry-run's go units
# (whole packages and slices alike). What the dry-run cannot provide is named
# in notProduced, never invented.
# AFP-V0-480: with --slurpfile tiers TIERS (tiers.sh output) the document also carries the
# additive `tiers` (changed, hop1, hop2, full, universe, millis) and `tierIncomplete`
# fields; without it they are absent, as in every older document.
# CIN-V0-001 (Codex P2): with --arg forwardEpoch E the document also carries forwardEpoch, the
# forward-evidence epoch of the ci.yml that produced it (CORVINT_PR_FORWARD_EPOCH). collect.sh
# names the row with this value, never its own; a document without it, as every document
# produced before the field, belongs to no epoch the collector can count.
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
| if $ARGS.named.forwardEpoch == null then . else . + {forwardEpoch: ($ARGS.named.forwardEpoch | num)} end
| if $ARGS.named.tiers == null then .
  else ($ARGS.named.tiers[0] // {}) as $t
    | . + {tiers: (if ($t.tiers | type) == "object" then $t.tiers + {millis: ($t.millis // null)} else null end),
           tierIncomplete: (if ($t.tierIncomplete | type) == "array" then $t.tierIncomplete
                            else ["the tier computation printed no document"] end)}
  end
