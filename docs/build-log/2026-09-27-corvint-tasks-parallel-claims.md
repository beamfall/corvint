## 2026-09-27 Corvint Tasks agent leases S8: parallel claims by scoped, path-overlap collisions

The owner reported that Beamfall can work only one ticket at a time against `beamfall-core` and
asked for Corvint to let many tickets run in parallel. The owner also accepted the S2 amendment
(cutover as the single TCP-00 §5.4 A5 `AUTHORITY_SWITCH`, recorded in
`2026-09-27-corvint-tasks-cutover-authority-switch.md`) on the same day.

Findings, from a read-only survey of Beamfall's `script/roadmap.sh` at the time of writing, not
from running it:

- A claim whose ticket declares no allowed paths collides with every other claim, and the runner
  admits at most two claims per repository by default.
- The survey counted 222 of 300 open core tickets without declared paths. That count was reported
  by a subagent and not recounted.

Owner decisions (2026-09-27), recorded as S8, CAL-V0-021 to 026, landing directly after S3:

- Scope. Every claim carries a scope. A ticket with `QUALIFIED` coverage uses its declared paths.
  Otherwise the claim uses paths the agent requests, or else paths derived in process from Corvint's
  local context index, or else the whole repository. Chosen over declared paths only, which keeps
  most core tickets serialized, and over an advisory derived scope, which lets a wrong prediction
  put two agents on one file.
- Enforcement. `submit` refuses `OUT_OF_SCOPE` for any changed path outside the scope. `widen`
  adds paths only when they collide with no live attempt. The derivation has no measured recall
  (`corvint surprise` measures misses but has no qualified rate), so it is only safe when it is
  enforced.
- Collision. Collisions are path overlap of edit sets under TCP-00 §4.2, with no per-repository
  lane limit beyond `maxActiveAttempts` and no dependency or import-closure expansion. Chosen over
  WQO-V0-041 import-neighbour closure, which prevents some rebase-gate failures at a large cost in
  parallelism. Semantic interference between disjoint scopes is caught by the gates at the exact
  candidate tree before `complete`.
- Lock hold. Each lease command must hold the store lock for work that does not grow with the
  ticket count. A full audit took about 3.2 s at 2,894 items under load (CTS-V0-003 entry), which
  would cap lease throughput far below the goal. The target is a p95 under 500 ms for `claim` and
  `renew` on a 3,000-ticket store, reusing a verified audit cache keyed by the head receipt and the
  intent tree.

Acceptance adds a measurement on the Beamfall fixture store of how many open core tickets could
hold concurrent claims under the new rule, compared with the runner's two-lane rule.

Rollback: remove `widen` and the submit scope check, and every claim reverts to one
`WHOLE_REPOSITORY` resource, which serializes claims as today's runner does.
