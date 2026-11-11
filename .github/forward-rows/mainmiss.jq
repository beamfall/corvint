# SPDX-License-Identifier: Apache-2.0
# AFP-V0-983 (V1-3750): the narrowed pull requests a failed full run of main
# links back to. Usage:
#   jq -n -c --slurpfile failed FAILED --slurpfile prs PRS --arg runId R --arg sha S
#     --arg epoch E -f mainmiss.jq
# FAILED holds failed.jq documents of the main run's failed shards; PRS holds one
# {pr, head, merge, epoch, records: [corvint-pr-narrowing/0 ...]} per pull request merged since
# the last green full run. A pull request is a miss when its NARROWED records'
# `skipped` packages hold a failed package: the full run failed a package that
# pull request's selection left out. `name` is the artifact name when there is a
# miss, else null. CIN-V0-7922: each miss also carries `merge`, the pull request's merge commit on
# main when the API named one, for the confirmation reruns, and `epoch`, the forward epoch the
# pull request's run stamped on its selection documents (null when unknown). A miss of a known
# epoch earlier than E is listed in `earlierEpoch` and is not in `misses` or `name`.
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
      runId: ($nar[0].runId // null), epoch: (if ($p.epoch | type) == "number" then $p.epoch else null end),
      packages: $missed}]
| sort_by(.pr) as $all
| ($epoch | tonumber) as $E
# CIN-V0-7922: a pull request narrowed under a known earlier epoch is no miss of epoch E
# (CIN-V0-001). An unknown epoch (null) stays a miss, so a lost selection document hides nothing.
| [$all[] | select(.epoch != null and .epoch < $E)] as $earlier
| [$all[] | select((.epoch != null and .epoch < $E) | not)] as $misses
| {profile: "corvint-main-miss/0", runId: ($runId | tonumber), sha: $sha, epoch: $E,
   failed: $F, misses: $misses, earlierEpoch: $earlier,
   name: (if ($misses | length) == 0 then null
          else "ci-forward-MAINMISS-r\($runId)-\($sha)-e\($epoch)-n\($misses | length)-pr\([$misses[:10][] | .pr | tostring] | join("_"))" end)}
