# Selection hardening: test and driver counts stay off the frozen JSON (V1-5401)

Date: 2026-11-11
Ticket: V1-5401
Kind: orchestrator decision under the owner's delegation
Spec: `docs/specs/selection-hardening-v1.md` (SLF-V1-005, SLF-V1-008)

## Decision

The orchestrator settled two questions from the first-round handback of lane `selhard` under the
owner's delegation (2026-11-11).

1. SLF-V1-008 against the frozen profiles (handback friction 5). The affected-plan/0 and
   affected-plan/1 profiles stay frozen for 1.0 (CCF-V1-006), and their core-freeze goldens are
   not regenerated. SLF-V1-008 now requires the `tests=<n> drivers=<m>` counts on the
   `corvint affected` stderr headline. The JSON plan's `tests` and `drivers` members wait for the
   next affected-plan profile version and are recorded as "Not yet built (frozen profile
   CCF-V1-006)". Change task 9 is ticked once the headline proof holds.
2. A top-level `include` in a Vitest config (handback friction 6). The evaluator stays
   conservative: it reads a membership key only under a known parent (`test`, `typecheck` or a
   Jest config object), so a top-level `include` keeps `typescript:executable-config-unresolved`.
   The spec says so in one sentence.

## Evidence

- The headline proof is `cmd/corvint/affected_counts_test.go`:
  `TestAffectedHeadlineCountsTestsAndDriversApart_SLFV1008` (three tests and two drivers print
  `tests=3 drivers=2`, no driver is listed) and
  `TestAffectedHeadlineCountsSurviveTheLineBound_SLFV1008` (a 40-character revision and base plus
  long unknown-reason counts cut the line at 200 bytes, and both counts survive). Task 9 is ticked
  on that proof after the review fix described in
  `docs/build-log/2026-11-11-selection-hardening-review-fixes.md`.
- AFP-V0-3642 now says where the counts sit on the headline's first line.

## Rollback

Revert the commits. The frozen goldens were never changed, so no profile rollback is needed.
