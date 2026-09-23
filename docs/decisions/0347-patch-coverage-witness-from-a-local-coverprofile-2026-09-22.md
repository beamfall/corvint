# Decision 0347 — Patch coverage witness from a local coverprofile

Date: 2026-09-22. Status: accepted; experimental delivery (ticket V1-0085, `TCQ-V0-051..054`).
Extends decision 0338's `cem/0.3` profile by one optional hunk member; amends no frozen vector.

## Context

A `test-claim` basis on a CEM hunk records that a producer named a test for the change. Nothing
in the map said whether that test reached the changed lines, so a reviewer reading `cem report`
could read a cited test as a tested hunk. Corvint already parses Go coverprofiles inside the
gorunner live-verify path, and the TCQ spec's YAGNI paragraph excluded "coverage ingestion"
without saying what a bounded version would look like.

## Decision

1. `corvint cem cover --map --coverprofile --test-run [--output]` ingests exactly one
   operator-named local Go coverprofile. Corvint never discovers profiles or runs tests here; the
   profile SHA-256 and the `--test-run` text are recorded verbatim and are never verified.
2. The witness is one optional `coverage` hunk member admitted only on `cem/0.3`:
   `{profileSha256, testRun, mode, state, covered}`. `cem/0.1` and `cem/0.2` keep rejecting the
   key as `unknown-field`, so no existing map, fixture, conformance vector, or the
   `interop/cem01-go` consumer changes; `cover` upgrades a canonical map to `cem/0.3` exactly as a
   structural mechanical reason does (CEM-SM-006).
3. Coverage follows diff-cover semantics: the hunk's added lines intersected with lines of
   profile blocks whose count is positive. Every hunk receives a witness; a hunk none reach is
   `state: uncovered` with an empty `covered` array. Absent and uncovered are distinct states,
   and neither is tested.
4. The downgrade is rendering only. `cem report` gains a `## Test claims` section listing each
   hunk with a `test-claim` basis as `tested` or downgraded with reason `no-coverage-witness` or
   `coverage-witness-uncovered`. Dispositions, counts, the worklist, and the `status`/`verify`
   JSON envelopes are unchanged, which keeps oracle parity untouched.
5. The gorunner parser is reused through two exported wrappers (`ParseCoverProfile`,
   `ParseCoverageBlockLine`) with no behaviour change to the live-verify path; there is no second
   coverprofile parser.

## Alternatives weighed

- Admitting `coverage` on `cem/0.2` as well: rejected because the closed-key validator and the
  frozen `cem/0.2` surplus-field vector would have to change, and every 0.2 consumer would need
  updating; 0.3 already carries the "additive on canonical" precedent.
- Changing a downgraded hunk's disposition or counts: rejected because a witness is caller
  evidence, not verification; a coverage gap must be visible without moving a hunk out of
  `supported` and without altering the frozen status envelope.
- Intersecting with the whole `newRange`: rejected because context lines a test reaches would
  count as coverage of the change.

## Consequences

- `cem cover` is dispatched but `cmd/corvint/help.go`'s `cemHelpActions` was outside this
  change's ownership, so `corvint cem cover --help` and the cem help text do not yet list it
  (follow-up).
- `docs/specs/cem-0.3-structural-mechanical.md` still says the profile "adds only" the structural
  reasons; its wording and any 0.3 schema need a follow-up amendment.
- Rollback: remove the `cover` action, the `coverage` validator branch, and the report section;
  a map carrying the member then fails closed as `unknown-field`.
