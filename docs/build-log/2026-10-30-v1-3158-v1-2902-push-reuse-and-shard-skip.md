# Push reuse after a narrowed queue run rejected; the job-level shard skip — V1-3158, V1-2902

## V1-3158 (CIN-V0-005)

Fork: keep the full `main` push after a narrowed merge-queue run, or reuse that run. The first
cut (c9ff36e, a98fec9) reused it while `CORVINT_MQ_NARROWING` was `on`. Decision (orchestrator,
on the Codex P1 finding against c827217): rejected. A `main` push reuses only a `merge_group`
record that ran FULL on the identical tree; a narrowed queue record is refused for a push, with
or without `--base`, and the narrowed-record job stays pull-request only. A narrowed merge keeps
its full `main` push as the backstop, and that push still produces forward rows. Both commits
are reverted (baf9476, 6347df8); the ticket's push saving is not delivered by this lane.

## V1-2902 (CIN-V0-007)

On a passed DOCS or REUSE decision no shard job starts, and `go-product` accepts the skip only for
that decision. Remainder, out of this lane's scope: merge-queue tree reuse of `go-static`,
`go-interop` and `cce-live` needs a pull-request record job and changes the AFP-V0-670 and
AFP-V0-080 gated-job structure; building the shadow planner once per run touches the pin steps PR
#11 changes and needs a trusted artifact handoff. `doc-gates` merge-queue reuse already exists
(AFP-V0-670, AFP-V0-671), and V1-1642 was delivered by AFP-V0-080.

Evidence: `TestCINV0005PushNeverReusesANarrowedQueueRecord` (a valid narrowed queue record of the
pushed commit gives FULL with and without `--base`; adding the full record gives REUSE of it),
`TestAFPV0734NarrowedRecordJob`, `TestCINV0007ShardsSkipAtJobLevelOnlyForADocsOrReuseDecision`
(`tools/ci-reuse-plan`, PASS; whole package PASS).
