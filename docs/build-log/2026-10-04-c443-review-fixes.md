# Issue 443 transport adapter: independent-review fixes

Human-owned intent: GitHub issue443, native V1-0611 and V1-0688, and the owner's 2026-10-04
transport-adapter decision. This entry records the fixes for the independent review of branch
`c443-transport-adapter` at `f8ee6024`. It does not authorize or run the held recovery attempt.

## Findings and dispositions

- HIGH, analyzer audit drift (confirmed): `TestAnalyzerSchemaInputs/IDX-SNAP-V0-017` failed because
  the ALO-V0-017 budgeted branch in `internal/contextindex/git.go` changed the audited input digest
  from `f512402c...` to `f8028dc5...`. Decision: **re-pin, no `analyzerSchemaID` bump**. The
  requirement bumps the schema for an incompatible analyzer or encoding change. This branch is
  neither. When the context carries an operation budget, `gitRaw` runs the same executable,
  arguments, environment and stdin through `gitrun.Run`, and returns the stdout bytes unchanged or
  an error. Extraction, facts and pack encoding are unchanged, so a pack written by either path is
  valid for both. `gitrun` and `cemcode` are process-execution and error-classification
  dependencies, not extraction dependencies, so they do not join the audit. Evidence:
  `TestAggregateGitExecutorSuccessParity` (new) finds byte-identical output from both executors
  for `ls-tree`, `log` and `cat-file --batch` with stdin, over a non-ASCII quoted path.
  `TestAggregateGitExecutorErrorParity` covers the error side.
- MED, unpinned verifier executables (confirmed): admission matched provenance only. The compiled-in
  #443 row now pins each role's adapted binary: BASE
  `sha256:0a68e6fe4cdf1adfe0029852ae1e073a3cc38f211161c7df5f23d5d1df42fb16` and HELD
  `sha256:e308360e3ef3f3dbae8ffde39f6309c427856cba80fab7188d2492175b521d66`. Both were re-hashed
  from the build2 witness binaries and match its `build.out`. If the provenance matches and either
  digest differs, admission refuses `transport-recovery-verifier-not-admitted`. Link-time test rows
  now carry ten fields. The native test pins its built verifiers and adds a `wrong-binary-digest`
  refusal. The running binary embeds its own rows and cannot be pinned, so the native
  `fixed-running-binary` case now refuses at the pin. `TestTransportRecoveryRefusesRunningExecutable`
  covers the running-executable refusal directly. The BASE and HELD patch bytes are unchanged
  (`c610b712...`, `6f58ea18...`, re-hashed).
- LOW, leaked stage (confirmed): `writeAggregateExclusiveContext` left `.aggregate-stage-*` behind
  when cancellation or the operation deadline arrived between the verified close and the install.
  The cleanup is deliberately narrower than a blanket defer. The spec keeps write, close, readback
  and install fault stages "visible and charged", so only that non-fault refusal removes its stage.
  `TestAggregateExclusiveLateCancelRemovesStage` fails without the fix.
- LOW, ALO-V0-023 wording (confirmed): admission also accepts an enrollment with no aggregate
  outcome, which is the native test's own path after the validated legacy refusal. The requirement
  now says the transaction is absent or pending (not COMMITTED, fewer than three issued). The
  earlier entry's "existing pending" wording is superseded.
- LOW, documented only: `gitstatus.EnableOwnedWorker` proves only process-group leadership, not
  that the parent Owner created or will retire the group. The historical patch bytes and pinned
  binaries cannot change this. It is recorded as a known limit in the spec's failure modes.
  Retirement assurance remains the coordinator's pinned Owner and observed RELEASED.

## Limits

The held recovery attempt and any admission against real state remain NOT_PRODUCED, pending the
owner's go. The binary pins are Darwin arm64 go1.27.1 build outputs; any rebuild that changes the
bytes is refused rather than admitted. Linux and Windows lanes remain NOT_PRODUCED.
