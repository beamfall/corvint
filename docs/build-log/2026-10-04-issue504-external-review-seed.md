# Issue 504 external review seed

Human-owned intent: GitHub issue 504, native V1-0701. The proposed contract is
`docs/specs/corvint-tasks-external-reviews-v0.md` (ERG-V0-001..010). Its intent stays `proposed`
and its delivery `experimental` because no owner acceptance is recorded; agent Gate A, source
review and the F1 repair review are evidence for the proposal, not acceptance.

Decision: deliver only the preparation plan first slice (ER504-007), four new files with no edit to
existing source: closed request/event/reference codecs in `internal/tasks/snapshot` and a pure
transition, current-view and recovery-check helper in `internal/tasks/transaction`. The files are
byte-identical to the independently reviewed F1-repaired preparation leaf (base 211916336638);
they were not rewritten. They were rebased by copy onto origin/main 4b10a02144fa, where they
compile and pass unchanged. Issue 501 is needed only for the native derived-event slot used by the
future locked writer, so this seed stands on origin/main rather than stacking on issue 501.

F1 (frozen source review MED): a stale RESUBMIT head previously left no route, because a fresh
RETURN required a second RESUBMIT whose prior RETURN head no longer existed. The reviewed repair lets
a fresh authorized RECORD open generation+1 after a stale PASS or stale RESUBMIT; a stale RETURN still
requires an explicit RESUBMIT. Policy changes alone never make a review stale.

Evidence at the committed bytes: focused snapshot/transaction tests (all eight Issue504 tests plus
the GateResult, stage-codec and replay baselines), the race run of the Issue504 tests, `go vet` of
both packages and `gofmt -l` passed. The executable GateResult decoder rejects the external event,
so it carries no completion authority. Actor authentication and reviewer independence are always
`NOT_OBSERVED`.

NOT_RUN or NOT_PRODUCED: the native writer, policy/ticket optional-field wiring, CLI and history
reads, the workState adapter and any live qualification (nothing calls the seed yet); the
exhaustive `make gate` (not requested); owner acceptance. The preparation controller, jobs,
admission records and session key were history and were not rerun or reused.

Rollback: revert this change. Nothing references the new files, the closed profiles are not
written by any command, and no stored byte changes.
