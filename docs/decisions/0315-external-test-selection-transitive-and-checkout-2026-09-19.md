# Decision 0315 — Test selection walks obligations transitively and reads bound checkouts

Date: 2026-09-19. Status: accepted (delegated call on the owner's feature request
Beamfall/corvint#12; owner review of the intent is welcome and changes only the digest line).

## Decision

`advice.test_selection` gains the two extensions specified in
`docs/specs/external-test-selection-v1.md` (`ETS-V1`). Both can only widen.

- **Transitive obligations.** A breadth-first walk over the record's own entity-to-entity
  dependency relations replaces the one-hop widening. It is bounded at 4 hops and 256 entities per
  record. A cut blocks the entities at its edge and reports an unknown. The bounds are fixed
  constants, not a flag: a cut only costs a narrowing, so a caller has no safe reason to lower
  them, and raising them needs measured evidence.
- **Checkout worktree reads.** `corvint affected` reads each resolved `--repository` checkout's
  dirty paths once, with the root's bounded Git status capture. Any dirty path blocks every side
  that checkout binds (`checkout-worktree-dirty`). An unreadable status blocks the same way
  (`checkout-worktree-unreadable`).
- **Wire.** The schema stays `external-test-selection/0`: the codes and the cut `unknowns` row
  are additive, and an inspected row drops the `checkout-worktree-not-inspected` limitation.

## Alternatives weighed

- *A `--obligation-depth` flag*: the only effect of a lower depth is weaker advice. A constant
  is simpler, and the adopter evidence that would justify a higher bound does not exist yet.
- *Per-path checkout rule, as the root uses*: a root dirty path is itself an obligation, but a
  checkout's dirty paths are not, so a dirty helper a test imports would go unseen. Judging the
  checkout as a whole is the fail-closed reading.
- *Walk across records*: entity ids are provider-local. Joining them across records would infer
  relations no provider declared.

## Rollback

Restore `widen` and `limitations` in `selection.go`, remove the `CheckoutStatus` field and its
caller in `cmd/corvint/affected.go`, the new fixtures and tests, the ETS-V1 spec and its index
rows, and this record, and restore the two ETS-V0 non-goals.
