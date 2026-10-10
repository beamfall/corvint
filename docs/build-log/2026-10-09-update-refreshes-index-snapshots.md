# Core updates refresh index snapshots (V1-1078)

Date: 2026-10-09

## Decision

An index snapshot is keyed by the Core executable's digest. Every binary switch therefore left each
checkout without a matching snapshot, and the Claude Code and Codex hooks degraded with
`dogfood-event-index-snapshot-stale` until someone ran `corvint index --if-stale` by hand.

The refresh now runs at the binary-switch boundary rather than in the hook. `IDX-SNAP-V0-012` forbids
a hook from leaving a detached index child, and invariants 4 and 7 rule out a read-path write or a
background refresher. The new requirement `UPD-V0-008` (proposed) works as follows:

- After a successful Core `apply` that activates a new binary, and after a Core `rollback`,
  `corvint-update` runs the switched binary's `index --if-stale` in the foreground.
- It refreshes each `--refresh-index ROOT`, plus the enclosing checkout when that checkout is already
  indexed.
- Each run is an owned process group bounded at two minutes, run while the updater still holds the
  binary-directory lock.
- `indexRefresh` reports each outcome.
- A failed refresh never fails or reverses the switch.

Reusing an old-engine snapshot when the encoding is unchanged was rejected. The executable key is
the guarantee that a rebuilt extractor never reads an older table.

## Evidence

- `TestUPDV0008CoreSwitchRefreshesIndexedCheckouts` passes. It covers apply and rollback, the
  `failed`, `built`, `fresh` and `skipped` outcomes, deduplication of the same root, the exact argv,
  and no refresh after an unchanged apply or a successful Tasks rollback. A mutation that drops the
  Core-only guard fails the test.
- Live check: the installed Core (build 404) was called through `refreshIndexes` on a scratch
  indexed repository. It reported `built`, then `fresh`, and wrote the snapshot to the shared
  store.
- `corvint affected --base 684cca5f` selected 83 units. The selected Go tests were run; results are
  below.
- Doc gates and use-case receipt gates passed.

## Independent review

A Codex review (gpt-6-astra, read-only) of the staged change made two P2 findings, both confirmed and
fixed:

- The refresh ran after `run` released the update locks, so a concurrent updater could replace the
  binary between refreshes. It now runs inside `run`, under the lock.
- The Tasks exclusion assertion never performed a successful Tasks switch. It now performs a real
  Tasks rollback and also checks an unchanged apply.

## Limits

A Core binary copied into place by hand (a local build) refreshes nothing. Checkouts the operator
neither names nor runs from keep their stale snapshot until their own explicit `index --if-stale`.
