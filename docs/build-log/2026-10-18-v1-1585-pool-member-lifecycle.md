# V1-1585 typed pool member lifecycle operations

Date: 2026-10-18
Ticket: V1-1585 (GitHub #1028)
Spec: `docs/specs/corvint-tasks-pool-safe-reuse-v0.md`, PSR-V0-160..167. The requirements are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). PSR-V0-150..153
are left to the open PR #1089.

## Decisions

- One command, `pool operate --operation OP`, over a new lease verb `POOL_OPERATE`, instead of five
  verbs. It reuses the pool command path end to end: the pending entry commits before execution
  (CAL-V0-078, so nothing after it is retryable), the owned process runner kills the group on
  timeout or interrupt and bounds output at 64 KiB, and the result is settled by the existing
  `POOL_OBSERVE` child request. `pool recover` settles an orphaned operation.
- The typed refusals (`OPERATION_UNDECLARED`, `OPERATION_STATE_ILLEGAL`) are detail tokens on
  existing wire codes, as V1-1586 did for source reasons, so the closed code set and older readers
  are unchanged.
- State table: provision and teardown from FREE, refresh from FREE or QUARANTINED, reset and
  retarget from QUARANTINED. A passed run from FREE frees the member again; a passed run from
  QUARANTINED stays QUARANTINED with `cleanupPassed` false, so this change adds no new path to FREE
  for a quarantined member: cleanup and `pool confirm-safe` stay the authority. Every failure
  quarantines with `<operation> <class>` as the reason prefix.
- Receipts live in pools.json as an optional `operations` list of `taskman-pool-operation/0`
  records, one per member and operation; the pending operation is an optional entry `operation`
  key. Both keys appear only when set, so existing bytes are unchanged. The profile is a live
  format (CAL-V0-130/131) with a format-version row, avoiding the V1-1586 CI regression.
- Operator attestation (OWNER or OPERATOR) is required: the operations act on members outside any
  attempt, like `pool cleanup` and `pool confirm-safe`.
- `retarget` takes `--source REV:TREE`, checked against `allowedSources` when the pool declares
  them, and passes it to the command as `CORVINT_POOL_TARGET_SOURCE`.

## Evidence

Focused tests (Darwin, heavy.sh, one package per command) are listed in the spec's Traceability
(PSR-V0-160..167): store (seven tests including a background child killed at timeout and a
200000-byte output classed `OUTPUT_LIMIT`), transaction (orphan recovery), snapshot (codec,
legacy bytes and eleven rejections) and CLI (operate, replay, status view, undeclared refusal), plus
the CI traps `TestCALV0131_*`, the format-version table, `TestImportDirection` and
`TestIndexCoversSpecsAndHeaders`. Linux runtime NOT_RUN.

## Independent review (Codex, PR #1104 at 2215d54aa)

Three findings, each fixed with a store regression test that failed before its fix:

- P1: a passed operation freed an identity-guarded member without an identity observation, so a
  `refresh` that moved its physical source could leave it FREE against PSR-V0-084. A guarded member
  now stays QUARANTINED (`SOURCE_IDENTITY_UNPROVED`) and `pool confirm-safe`'s probe decides
  (`TestPSRV0163_GuardedPassedOperationNeverFree`, which also shows the drift quarantine).
- P2: a runner refusal before execution (configRef blob, revision moved after admission) left the
  entry pending until recovery recorded `ORPHANED`. It is now settled in the same request as a
  `SPAWN_FAILED` or `SOURCE_CHANGED` receipt and quarantine (`TestPSRV0165_PreExecutionFailureSettled`).
- P2: a replay after a later run of the same operation lost its outcome. Replay now reads the
  request's receipt from the pools.json its own observe transaction posted, so pools.json does not
  grow (`TestPSRV0164_ReplayAfterSupersedingRun`). PSR-V0-163..165 text is tightened, same IDs.

## Residuals (issue 1028 stays open)

- Operations by the holder of an ALLOCATED allocation are not supported.
- No automatic return to FREE after a passed reset or retarget.
- At 256 members with five receipts each, pools.json can reach its 1 MiB bound; the write is then
  refused `LIMIT_EXCEEDED`.
- No dispatcher or sweep use of operations; non-Unix execution is `UNKNOWN`.
