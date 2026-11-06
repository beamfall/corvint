# Darwin shard cost-read failure (V1-4732)

Independent review found a CIN-V0-4625 coverage hole: a cost-file I/O failure
on one matrix leg selected round-robin ownership while other legs retained
cost-based ownership. For FULL inventory a,b,c,d with costs 1,2,3,4, shard
zero's fallback executes a instead of d, omitting d while all legs succeed.

Decision: require readable cost history at the shard entry point and propagate
I/O failures before writing execution artifacts or invoking tests. The existing
optional cost-reader helper retains its fallback contract for other callers;
invalid or empty cost content still produces deterministic round-robin
placement. No selection record, workflow interface or existing assertion changes.
CIN-V0-4625 now distinguishes invalid cost content from runner-local I/O
failure. This is the review's permitted fail-the-leg repair, rather than a new
partition distribution mechanism.

The regression injects FileNotFoundError and PermissionError separately into
each of four FULL matrix legs with the review's unequal costs. It requires
that leg to raise before execution or artifact creation and verifies every
other leg retains its original ownership without duplication. All eight
scenarios failed before the repair; all six shard tests and seven selector
tests pass afterward, with existing assertions intact. Raw logs remain under
`~/lanes/macosci-evidence/`. Pre-fix regression log SHA-256:
`e3add981c267ce3466d7c1c106309dde00b4927be4388afadefbf7f0e2c8ed47`.
Post-fix shard-suite log SHA-256:
`c13b3900e36eedb7c76af141f74ef2ca8809f433577d50a36322072c391de60e`.

Self-development routes used: original scoped code/spec/test evidence and
the existing pre-change capture; `affected --base` selected the whole
ci-reuse-plan and ci-forward-row packages. Its plan is advisory with UNKNOWN
scope and self-binary-stale degradation. Focused verification and final
CEM/check results belong in `~/lanes/macosci.handback.md`. Retrieval/learning
evaluations are not applicable to this CI-only repair. Hosted Darwin execution
and runtime remain NOT_OBSERVED; independent review and broad host verification
remain the orchestrator's work. No publication, Tasks writes or seal.

Rollback: revert this repair's sharder, regression and spec clarification
together, restoring the known asymmetric-read omission. The capability's
original workflow-extraction rollback remains available.
