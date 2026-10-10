# Affected-plan decision 0431 spec amendments

Date: 2026-10-10. Ticket: V1-0810. Authority: decision 0431, accepted by the owner on
2026-10-05; this change records that accepted intent without workflow or tool changes.

Amended AFP-V0-013/014/016/017/022/026 and the bounded documentation CI exception;
added AFP-V0-028 (queue-backstopped PR narrowing), AFP-V0-029 (forward rows and shadow
admission) and AFP-V0-030 (kill switch and reviewed epoch re-arm), including failure
modes and R1–R3 rollback. Decision 0431 now names the final IDs; AFP-V0-027 remains
V1-0809. Existing status entries, including decision 0488, were preserved by insertion.
README/INDEX status mirrors were updated for spec-index consistency, and REQUIREMENTS.tsv
was regenerated from a scratch Git index containing the edited spec.

Delivery remains experimental. AFP-V0-028..030 are not implemented; traceability names
planned tests, with hosted qualification, queue behavior, cost and timing evidence
`NOT_OBSERVED`. Historical qualification remains optional evidence for this profile.
No hosted run, workflow change, commit, push or ticket-store write was performed.
`make gate` and repository-wide Go tests: `NOT_RUN`, explicitly excluded by the request.
Post-commit CEM binding/sealing: `NOT_RUN`, because commits are explicitly excluded.

Corvint query and affected were used; their bounded context/selection is not behavioral
proof. The index refresh was unavailable because the shared Git directory is outside
sandbox write permissions. The update check could not establish freshness (version
smoke failed: descendant snapshot unavailable); installed Core is 1.0.0-rc.3 build 407,
Tasks is 0.0.0-tcp01-unverified+build.407, and available official versions remain unknown.
No update or ticket filing was attempted beyond this read-only check.

Verification uses GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp and GOMAXPROCS=3.
The shared Git index is not writable here, so staging/checks use a scratch index and
object directory. A scratch mktemp adapter supplies the explicit TMPDIR template for
no-argument calls, since macOS mktemp otherwise uses an unwritable system temp directory.

Independent read-only review found a stale historical prerequisite in the Agent digest;
it was corrected to distinguish current full execution, optional historical evidence and
queue-backed enablement. No other review findings. The first documentation gate run
passed after fixing a prose range that the requirement-definition checker interpreted
as a duplicate clause. The final digest correction is checked again on the final spec.

Final verification: `make -s spec-requirements` succeeded. All requested targets passed:
`spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
`decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
`unbounded-readers-check`, `use-case-receipts-check` and `use-case-receipts-test`.
The separate `sh script/repin-use-case-receipts.sh --check` passed: all 18 receipts current;
the fixture test passed all 5 cases. `git diff --check` and exact preservation of both
existing spec status strings passed. Independent review confirmed its digest finding resolved.
A repeated run initially inherited a scratch index modified by the receipt fixture; the
index was restored from the repository baseline, REQUIREMENTS.tsv regenerated, and the
fixture test rerun without the parent's scratch Git variables. The final gate run passed.
No repository workflow or tool behavior was changed to accommodate the sandbox.
