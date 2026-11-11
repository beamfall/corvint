# SPDX-License-Identifier: Apache-2.0
# AFP-V0-983 (V1-3750): the narrowed pull requests a failed full run of main
# links back to. Usage:
#   jq -n -c --slurpfile failed FAILED --slurpfile prs PRS --arg runId R --arg sha S
#     --arg epoch E -f mainmiss.jq
# FAILED holds failed.jq documents of the main run's failed shards; PRS holds one
# {pr, head, records: [corvint-pr-narrowing/0 ...]} per pull request merged since
# the last green full run. A pull request is a miss when its NARROWED records'
# `skipped` packages hold a failed package: the full run failed a package that
# pull request's selection left out. `name` is the artifact name when there is a
# miss, else null. CIN-V0-7922: each miss also carries `merge`, the pull request's merge commit on
# main when the API named one, for the confirmation reruns.
def hex40: type == "string" and test("^[0-9a-f]{40}$");
([$failed[] | objects | .reasons[]? | .package | select(type == "string")] | unique) as $F
| [$prs[] | objects | select((.pr | type) == "number" and (.head | hex40))
   | . as $p
   | [($p.records // [])[] | objects
      | select(.profile == "corvint-pr-narrowing/0" and .mode == "NARROWED" and (.skipped | type) == "array")] as $nar
   | ([$nar[] | .skipped[] | select(type == "string")] | unique) as $out
   | ($F - ($F - $out)) as $missed
   | select(($missed | length) > 0)
   | {pr: $p.pr, head: $p.head, merge: (if ($p.merge | hex40) then $p.merge else null end),
      runId: ($nar[0].runId // null), packages: $missed}]
| sort_by(.pr) as $misses
| {profile: "corvint-main-miss/0", runId: ($runId | tonumber), sha: $sha, epoch: ($epoch | tonumber),
   failed: $F, misses: $misses,
   name: (if ($misses | length) == 0 then null
          else "ci-forward-MAINMISS-r\($runId)-\($sha)-e\($epoch)-n\($misses | length)-pr\([$misses[:10][] | .pr | tostring] | join("_"))" end)}
