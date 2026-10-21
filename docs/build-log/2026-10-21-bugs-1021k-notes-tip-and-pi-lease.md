# bugs-1021k: pinned notes-tip anchor read and the Pi lease comparison

Date: 2026-10-21. Lane: claude/bugs-1021k (base 8def072e0). Tickets: V1-2157, V1-2243 delivered;
V1-0920 and V1-1736 already delivered on main; V1-2233 in flight in #1450. The `CEM-V1-067`
amendment is accepted under the owner's standing auto-accept (2026-10-11); no new requirement ID.

## V1-2157: `--notes-tip` read the anchor note through the mutable ref

Problem: `cem provenance --notes-tip OID` captured the live `refs/notes/corvint` tip, then
`anchorRow` read the note with `git notes list` through the ref. A ref moved A -> B -> A between
the capture and `requireUnmoved` passed that check and reported B's note against tip A. The
verdict ledger had the same shape and was fixed for `CEM-V1-345` (V1-2024).

Decision: under `--notes-tip`, `anchorRowAt` reads the note from the captured live commit's tree by
object ID (`noteBlobAt`), as `readLedgerAt` does; an absent live ref reads no row (the row is
`rollback` anyway). Without `--notes-tip` the read is unchanged. `CEM-V1-067` gains one sentence.

Evidence: `TestCEMV1067PinnedNotesTipReadIgnoresARefMovedAwayAndBack` moves the ref to a malformed
note B and back around the read through the `anchorRead` seam and expects A's `verified` row. With
the read reverted to the ref, it fails with state `malformed`. The `internal/gitnotes` package
passes.

## V1-2243: Pi host tests 6 and 7 compared a read-time clock

Problem: the `LCP PWV` tests deep-compare `attempt show` before and after host transitions to prove
the operator lease is untouched. `attempt show` carries `observedAt`, the read's own clock
(`addHolderObservation`), so any transition that crosses a second fails the comparison.
Reproduced with `CORVINT_PI_HOST_BIN` set to this tree's binary and corvint-tasks build 702.

Decision: compare the attempt without `observedAt`; every lease member, `holderStatus` and
`lastHeartbeatAt` included, still has to be identical. Test-only; no shipped package content
changes.

Evidence: both `LCP PWV` tests failed on `deepStrictEqual` at `observedAt` before the change and
pass after it.

## Already delivered

- V1-0920: `ticket create` reports `ticketId`, `resultingRevision` and `receipt` in `items[0]`
  (observed on this base); `TestCALV0250_DefaultedIssuedAtRetryNamesTheDifference` (d7e59c4cd)
  asserts the ID on create and the same ID on a request-ID replay.
- V1-1736: 2c484b759 declared `.github/cishards` in the package read scope and reports `go list`
  stderr. The failing run 39092839754 (head eebd77878) lacks that commit; the passing run
  39093442555 (head 2c537078c) has it, so the cause was the Landlock read scope.

## NOT_RUN and rollback

NOT_RUN: `make gate`; the Pi tests 1-5 with a candidate Core binary (they passed with the default
build). Rollback: revert the two code commits; the `CEM-V1-067` sentence drops with the first.
