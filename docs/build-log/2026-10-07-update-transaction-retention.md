# 2026-10-07: V1-0929 corvint-update state retention bound

## Intent

Ticket V1-0929. Every `corvint-update apply` left a whole `transaction-*` directory (download,
unpacked release, smoke home, previous executable, receipt), about 61 MiB each on this machine,
and an apply killed before its receipt left an unusable directory forever. Rollback only ever
needs the latest committed transaction's previous executable and receipt.

## Change

- Proposed `UPD-V0-007` (pending owner acceptance) in `docs/specs/operator-update-v0.md`.
- `apply` and `rollback` take an exclusive lock on the state directory for the whole run and first
  remove every `transaction-*` directory without `receipt.json` (`sweepIncomplete`). Only apply
  creates transactions, and only under that lock, so such a directory is provably abandoned.
- After a successful activation, `retainCommitted` removes the committed transaction's
  `archive.tar.gz`, `smoke-home` and extracted candidate (the installed bytes), keeping the
  receipt, `previous`, checksums, release metadata, qualification evidence and notices, then removes
  every other transaction bound to the same component and destination.
- The result names each removed path in `removed` and each kept-but-unclassifiable entry
  (link, non-directory, unreadable or malformed receipt, failed removal) in `left`.
- `check` is unchanged: no lock, no writes.

## Measured

`TestUPDV0007RetentionBoundInterruptedSweepAndRollback`-shaped fixture with a 4 MiB padded
executable, three successive applies (builds 162, 163, 164), state-directory bytes after each:

| | after apply 1 | after apply 2 | after apply 3 |
|---|---|---|---|
| base 5808c26c | 10,683,533 | 21,367,037 | 32,050,462 |
| this change | 4,195,818 | 4,195,819 | 4,195,819 |

The base grows by about 10.7 MB per apply; with the bound the state holds one previous executable
plus evidence, constant across applies. The base figures came from a scratch copy of the base
`update.go` run against the same fixture, deleted afterwards.

The test also covers an interrupted apply (a receipt-less transaction swept by the next
rollback), rollback after pruning restoring the exact build-163 digest, a second rollback refused,
another component's transaction kept, a malformed receipt left and named, and a held state lock
refusing the run.

## Limits

- Rollback depth is one step. Earlier transactions are removed once superseded, which changes the
  previous behaviour of keeping all of them.
- A transaction whose activation failed after its receipt was written stays until the next
  successful apply for that component and destination.
- `rollback` still refuses while any transaction receipt is malformed (UPD-V0-005, unchanged); the
  retention pass leaves such a directory and names it rather than guessing.
- The state lock also serialises two runs that share a state directory but target different
  destinations.
- Not run: actual macOS lifecycle against published releases, `make dogfood-change`/CEM binding, the
  exhaustive gate.
