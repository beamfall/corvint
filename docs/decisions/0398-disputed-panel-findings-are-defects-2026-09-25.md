# Decision 0398 — the pre-1.0 panel's disputed findings are defects

Date: 2026-09-25. Status: accepted. Authority: repository owner instruction "treat all disputed
findings as defects and fix them" (2026-09-25), following "if we know of bugs we should address them
before a next release. no point leaving issues" (2026-09-25).

The pre-1.0 expert panel review split on fifteen main-report findings (D1-D15) and seven
Beamfall/corvint-tasks addendum findings. In each, both verifiers confirmed the code facts and
disagreed only on whether the behaviour is a defect or an accepted, documented choice; several
refutations cite a spec clause that requires the behaviour.

## Decision

1. Every disputed finding is a defect to be fixed before the next release. Tickets: D1-D7, D9,
   D11, D12, D14 and D15 are V1-0334 to V1-0345; D8 is fixed with V1-0283 (B3), D10 with V1-0286
   (B8), and D13 with V1-0319; the addendum items are V1-0319 to V1-0326.
2. Where an accepted clause, profile or decision requires the disputed behaviour, the fix amends
   that clause in the same change and marks the amendment "(proposed, decision 0398)"; the owner
   accepts the amended wording as with any clause. This decision does not itself rewrite any clause.
3. A frozen wire profile is changed only by that profile's own versioning rule. A fix that would
   change frozen bytes adds a new profile version or a documented carve-out, never an in-place edit.
4. Promotion gates stay binding. D13 and the addendum's orientation item are fixed by running the
   `context --task` paired-trial gate (V1-0319) and routing to `context` only after it passes; a
   failing trial is recorded and the routing defect stays open.

Rollback: supersede this decision; each fix remains individually revertible.
