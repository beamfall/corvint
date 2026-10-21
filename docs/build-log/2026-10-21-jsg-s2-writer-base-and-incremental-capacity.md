# JSG S2: writer base, incremental capacity and carried-value verification

Date: 2026-10-21. Tickets: V1-0887, V1-1512. Spec: `docs/specs/corvint-tasks-journal-segments-v0.md`
(`JSG-V0-003`, `JSG-V0-004`, and part of `JSG-V0-014`). Authority: decision 0549 (accepted by the
orchestrator under decision 0548's delegation).

## Change

- `journal`: the writer checkpoint is now `taskman-writer-checkpoint/3`, an active delta that binds
  the derived base `taskman-writer-base/0` (`<checkpoint>.base`) by SHA-256. The base holds sorted
  entry records with path text, request records and notes, and a request or path is looked up in
  it without decoding it. A writer audit keeps the base while the head is fewer than 512 receipts
  past it. Otherwise, and for every complete audit, the checkpoint is base-free and `Encode`
  renders a new base.
- `journal`: `Result.VerifyWriterCheckpoint` and `MutationAudit.VerifyWriterCheckpoint` compare
  every value a retained checkpoint carries with a settled complete audit from receipt 1. That
  covers each entry's sequence, digest and size at or before the checkpoint's sequence, the entry
  and request counts, each request afterimage, the receipt bytes and the archive cost.
- `transaction`: a summarized inventory carries an incremental `importCost`, so a capacity fork
  measures only the entries the plan changes. A summarized fork returns no `Final` inventory.
  `AbortCapacity` stages its directory on a rolled-back clone.
- `store`: the complete mutation route, the complete lease route and the scheduled refresh run the
  verification under the writer lock. When it finds a difference they invalidate the checkpoint.

## Forks decided within the design

1. Anchor freeze: a retired-prefix anchor keeps the `/1` layout in `anchor.go`. It is journal state
   an operator retains, not a derived file, so a seal written by an earlier binary still decodes.
2. `receipt audit` stays read-only (invariant 4) and never reads the derived checkpoint.
   `JSG-V0-003` now says so; the three complete writer routes verify instead. Fail-closed behaviour
   is unchanged, because no writer resumes from a checkpoint the next complete route refutes.
3. Retain after invalidation: verification returns the new invalidation token only when its own
   invalidation was the only change since the caller's token. The complete audit's own derived
   checkpoint then replaces the refuted one. Any other change still retains nothing. Without this,
   a foreign checkpoint was removed but not replaced, and every following write took the complete
   route (`TestCALV0115_WriterCheckpointFallsBackToCompleteAudit/foreign`).
4. Base rewrite at 512 receipts before segmentation; S3 moves the rewrite to the seal.
5. Base before delta: a new base lands before the delta that binds it. A stop between the two
   leaves a delta bound to an absent base, which the next writer declines.
6. `ProfileWriterBase` joins the CLI's live formats.
7. `LastPosted` accessor: a ticket's last posted sequence is read through the base and the delta
   together, never from the delta alone.
8. A summarized capacity fork returns a nil `Final`: the shared base inventory is never handed out
   for later mutation.

## Evidence

- Tests: listed in the spec's Traceability rows for `JSG-V0-003`, `JSG-V0-004` and `JSG-V0-014`.
  The focused journal, store (writer checkpoint, refresh, liveness, lease, capacity), cli and
  transaction runs pass.
- Mutation checks, run and reverted:
  - Removing the capacity fork's rollback fails with "receipt already linked".
  - Disabling the summarized incremental cost gives measurement counts 161, 377, 80 and 80. These
    are not size-independent, so the count test fails.
  - Dropping the size comparison fails the forged entry-size case.
  - Dropping the request afterimage comparison fails the forged afterimage case.
  - A store helper that never invalidates fails the forged mutation-route case.
- Spot measurement, not qualifying: `TestCALV0118_WriterHoldProfile` at 9,500 tickets and 40,000
  receipts, 5 reps. The 1-minute load fell from 102 to 40 on 12 cores.
  - Heartbeat median: 114.2 ms.
    - observe 69.0 ms, mostly the `receipts/` listing, which S3 replaces.
    - commit 29.5 ms.
    - model 2.5 ms.
  - Renew median: 92.7 ms.
    - observe 16.4 ms.
    - commit 43.6 ms.
  - Claim, mutate and release: about 3 to 3.9 s.
    - model about 1.9 s, of which ticket decode is 1.19 s.
    - observe about 0.5 s.
    - bind about 0.35 s.
    - capacity about 0.25 s.

## Rollback

Revert the S2 commit. Both writer checkpoint files are derived. An S1 binary refuses `/3` as
unsupported, ignores the base file, and retains its own `/2` from the complete route.
