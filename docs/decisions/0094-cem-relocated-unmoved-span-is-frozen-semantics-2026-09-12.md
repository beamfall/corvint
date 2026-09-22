# Decision 0094 — `relocated` for an unmoved span is frozen cem/0.1 semantics

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`internal/cem/verify/verify.go` reports `relocated` for a span in an edited file even when the
recomputed target span equals the recorded span. The question was whether the frozen drift
algorithm allows `stable` there, making Go wrong, or requires blob-level `stable`.

It requires blob-level `stable`. The Go classifier is correct and is not changed:

- `interop/cem-0.1/ALGORITHMS.md:149-153@a4df8285` (Same-path drift) decides it. `stable` is defined only by
  identical blob OID; every other existing path takes the exact base span bytes and searches the
  entire target blob, and exactly one match is `relocated` with that target span. No offset
  comparison exists in the rule, so an unmoved span in a changed blob is `relocated` by definition.
- `CEM-CB-003` (`docs/specs/cem-0.2-canonical-binding.md`) keeps `cem/0.2` drift rules
  byte-for-byte compatible with `cem/0.1`, and `CEM-CB-005` restates the same `stable` and
  `relocated` split for `cite`. Neither profile can report `stable` here without an errata.
- The independent `interop/cem01-go` reference (`cem.go`, same-path drift loop) applies the same
  rule: a changed blob with one match is `relocated`. Go and the reference agree.
- The frozen parity case `cem-status-bound` needs no change, and `GPK-V0-033` gives no ground to
  replace it: there is no divergence to adjudicate.

No new wire disclosure is added. A drift-row member such as the reverted `baseSpan` is a field
addition that `GPK-V0-002` assigns to its owning spec and `GPK-V0-003` forbids as a Go variant
(decision 0091). The limitation is documented instead in the `cem-pilot-kit.md` failure and trust
model: `relocated` asserts that the exact bytes were found once in a changed blob, not that they
moved, and a reader tells an unmoved span apart by comparing the drift row's `targetSpan` with the
map evidence record of the same `evidenceId`. A distinct unmoved status needs a cem/0.1 errata or a
new profile together with a spec-authored parity expectation.

Rollback: revert this decision's commit, which restores the fixes backlog entry and removes the
spec note; no code or wire bytes change.
