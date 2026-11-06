# Pibrow verifier repair 1

Scope: only the three reported verifier failures against
`fb2071e66a248dee560095b883c0516f3ab14284`, base
`39c03198086681fb24f509fbd00b34696d3268c4`.

The frozen context failures differ only in the fixture commit ID. `gitFixture` pinned
its dates but inherited operator `GIT_AUTHOR_*` / `GIT_COMMITTER_*` exports, which override
its `user.name=t` / `user.email=t@x`. Pin that existing fixture identity in the child Git
environment (TCP-V0-024/150); retain the complete byte comparisons and golden unchanged.
The exact reported `Freeze|GPKV0002|Golden` selection passes with the required lane identity
exports. This changes fixture determinism, not the product wire contract.

AHI-020/173 require a version newer than shipped Pi changes. The existing bump command moves
all seven declared locations together from 0.3.12 to 0.3.13, including the Pi tuple under
PWV-V0-004. Historical validation versions remain unchanged. The history-based gate must
run after this bump is committed; a dirty-tree attempt still reports the older committed
version, as expected.

Decision: amend ASD-V0-036 to mirror the exact complete CIN-V0-2304 shell block through
`make ci-batch-cem-check`. `script/ci-batch-cem-check.sh` retains the existing event filter,
trusted PR-base build, explicit verifier, head checkout and failure propagation. The parser
compares the entire block to that script before mapping it; any command drift falls back
to the existing refusal. This avoids modifying the protected CI workflow or exempting its
check. Added shell assertions require the mirror step, reject a changed verifier argument,
and check the non-batch event filter. Existing assertions remain intact.

Verification uses the affected plan, the whole existing cmd/corvint package, the reported
shell suite and doc-gates. Raw outputs and affected plans live outside the repository in
`/home/russell/lanes/pibrow-evidence/repair1/`; final command results and limitations are
retained in the lane handback's Verifier repair 1 section. Independent review belongs to
the orchestrator. No ticket completion, publication, promotion or seal is claimed.

Self-development routes used: dogfood change at repair start and affected selection.
Retrieval/learning evaluation is not applicable; no ranking or learning behavior changes.
The original lane's live-host and missing-prerequisite limitations remain in force.
Rollback restores the test fixture helper and removes the local mirror/parser amendment,
then restores the Pi version only together with the shipped-content change it versions.
It never changes a golden, an assertion or the CI trust boundary.

Observed repair checks: the complete shell suite passes; the normal adapter target passes
on retry (55 OpenCode, 142 Pi, 19 protected Pi; 21 named skips). Its first run retained the
previously recorded process-cleanup ESRCH flake, without changing that assertion. The whole
cmd/corvint package was executed without a filter and failed after 519.195 s. Its retained
log includes synthetic ancestor-worktree/profile failures, executable qualification refusing
`/home/russell/lanes` mode 0775, and a migration candidate-digest mismatch. These broader
failures are not repaired by changing frozen output or source outside this repair's scope;
the host verifier must run the whole command package under its normal filesystem boundary.
The reported frozen/golden selection itself passes. Doc-gates and the final CEM check run
after this implementation commit; their actual outcomes belong to the retained handback.

Batch integration (claude/batch-1105a, 2026-11-05): main had meanwhile landed ASD-V0-2472
`script/batch-precheck.sh`, so this repair's ASD-V0-036 amendment, `ci-batch-cem-check`
target, mirror script and its shell assertions were dropped in favor of main's version, and
ASD-V0-036 keeps its pre-repair text. The Pi host qualification workflow from the original lane
lands in its own control-plane change, separate from this batch.
