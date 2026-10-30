# Push reuse of a narrowed queue run and the job-level shard skip — V1-3158, V1-2902

## V1-3158 (CIN-V0-005)

Fork: keep the full `main` push after a narrowed merge-queue run, or reuse that run. Decision
(orchestrator, within the ticket): reuse it, only while `CORVINT_MQ_NARROWING` is `on`. The queue
run already proved one narrowed selection of exactly the range `base_sha..head` and passed; the
push of the same commit tests the same tree. The narrowed-record job now also runs on such a
queue run, and `ci-reuse-plan` admits for a push only the record of a successful `merge_group`
run of the pushed commit naming the same tree, target, base, head and shard count; a pull-request
run's narrowed record is never admitted for a push. Owner-visible consequence: a narrowed merge
then has no full `main` backstop and the push makes no forward row; setting the variable off
restores the full push. Live evidence (two narrowed merges with push run IDs and durations)
follows the merge.

## V1-2902 (CIN-V0-007)

On a passed DOCS or REUSE decision no shard job starts, and `go-product` accepts the skip only for
that decision. Remainder, out of this lane's scope: merge-queue tree reuse of `go-static`,
`go-interop` and `cce-live` needs a pull-request record job and changes the AFP-V0-670 and
AFP-V0-080 gated-job structure; building the shadow planner once per run touches the pin steps PR
#11 changes and needs a trusted artifact handoff. `doc-gates` merge-queue reuse already exists
(AFP-V0-670, AFP-V0-671), and V1-1642 was delivered by AFP-V0-080.

Evidence: `TestCINV0005PushReusesTheNarrowedQueueRunOfThePushedCommit`,
`TestCINV0005PushPassesItsFirstParentOnlyWhileQueueNarrowingIsOn`, `TestAFPV0734NarrowedRecordJob`,
`TestCINV0007ShardsSkipAtJobLevelOnlyForADocsOrReuseDecision` (`tools/ci-reuse-plan`, PASS; whole
package PASS).
