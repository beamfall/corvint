# 2026-10-06: V1-0892 read-only `pool status`

## Intent

Issue beamfall/corvint#647 (ticket V1-0892). `corvint-tasks pool status` returned ERROR "unknown pool
verb", and `pool --help` listed only cleanup, confirm-safe, recover and sweep. To learn which member
an attempt holds or why a member is QUARANTINED, operators parsed `queue status`
(`pools[].members[]`), `attempt show`, or a ledger.

## Change

- `PSR-V0-013`: `pool status [--pool ID] [--member ID]` returns one `taskman-pool-status/0` item in
  the existing envelope. For each member it reports state, allocation, holder, stage, attempt,
  generation, changedSeq, a pending command kind and a quarantine reason. `lastHealth` or
  `lastCleanup` comes from the digest-verified pool observation that the current allocation keeps.
- `PSR-V0-014`: the filters accept each flag at most once. An unknown pool or member refuses
  `MALFORMED` and names it. No new refusal code was added, because `attempt show` and
  `CheckPoolExclusions` already use `MALFORMED` for unknown identifiers. The verb is listed in
  `help`, in `pool --help` and in its own help.
- `PSR-V0-015`: the command is a pure TM-V0-008 read through `withStore` plus the shared
  `pools.json` audit, followed by one content-addressed evidence read. It takes no lock, runs no
  probe and writes nothing.
- Docs: `docs/TASKS-EXTERNAL-AGENTS.md` pool section.

## Decisions

- Journal receipts carry sequence numbers but no wall-clock time. The quarantine `since` and the
  outcome `observedAt` are therefore `NOT_OBSERVED`. The journal `changedSeq` is the available
  ordering anchor. It records the member's latest pool-state change, which a sweep phase can
  re-record, so it is not labelled as the first moment of quarantine.
- Pool state keeps one observation for each allocation. The other command kind, a FREE member's
  earlier history and sweep-phase observations (another profile) are `NOT_OBSERVED`. Recovering
  them would require an unbounded scan of the journal history.
- `queue status` `pools[]` is unchanged.

## Evidence

Focused tests `TestPSRV0013_PoolStatusMembers`, `TestPSRV0014_PoolStatusFilters` and
`TestPSRV0015_PoolStatusLockFreeAndWritesNothing` in `internal/tasks/cli/pool_status_test.go`, plus
the existing help tests. The lock test holds both the writer lock and the preparation lock while
the read runs, and compares the state and intent trees byte for byte.
