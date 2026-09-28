## 2026-09-27 CAL-V0-001..003: the §5.2 writer accepts a non-fixture queue (S1)

The writer refused every queue whose `fixture` was false, both in the INIT digest and in
`validateInput`, so no durable non-fixture store could exist (V1-0398). S1 drops the fixture
condition from both checks and keeps the rest: `validateInput` still refuses a queue that names an
import map or an execution cutover `MALFORMED`, and the INIT digest now reports its identity
mismatch as `INIT queue identity`. CAL-V0-001 presumes a non-fixture store exists to write to, and
nothing else creates one, so amendment A14 records that `init` now accepts that queue shape.

Claims stay closed on such a queue. S7 owns the execution cutover and its `QUALIFICATION` receipt
(CAL-V0-020), and until then `executionCutover` is refused on every queue. So `claim` and
`claim --next` refuse `BLOCKED` `CUTOVER_MISSING` for the missing execution cutover whenever the
queue is not a fixture, and
`plan preview` reports the same blocker for each ticket, after `PAUSED` and before
`BUDGET_UNKNOWN`, in the order `planClaim` checks them.

Kept fixture-only, because CAL-V0-001 does not name them: staging an observation, release
mutations and release reconciliation. Ticket `reconcile` (KEEP_JOURNAL, ADOPT_FILE) and the S2
writer `cutover` have no fixture check and now reach a non-fixture store. Beamfall's queue uses releases for its milestones, so moving that queue
onto a non-fixture store also needs release writes; that is outside this slice and filed as
V1-0449.

Evidence:
- `TestCALV0001_NonFixtureQueueTakesEveryWrite` initializes a non-fixture `ROADMAP` store. It then
  commits a ticket create and edit, an import, pause, unpause, a policy update and the S2 writer
  cutover, and audits `CONSISTENT`.
- `TestCALV0001_ImportMappedQueueStaysRefused` checks that init refuses a non-fixture queue that
  names an import map or an execution cutover. It passes with or without the change.
- `TestCALV0002_NonFixtureQueueRefusesClaims` checks that claim and claim-next refuse and write
  nothing.
- `TestCALV0002_PlanPreviewBlocksANonFixtureQueue` covers plan preview.
- With the model change reverted, `TestCALV0001_NonFixtureQueueTakesEveryWrite` and the claim and
  plan tests fail.
- The fixture tests under `internal/tasks` pass unchanged (CAL-V0-003).

A writer killed between its first staged artifact and its head leaves staging slots that every
later command refuses. This was already true of fixture queues; S7 adds the recovery, and the spec
records the failure mode.

Rollback: restore `!fixture` in both model checks and remove the `CUTOVER_MISSING` checks. A
non-fixture store initialized in the meantime then refuses every write until S1 is reapplied.
