# 2026-10-21 gh-issues lane: RSpec denial advisory, #1358 fixture, nightly #1116 evidence

Tickets: V1-2124 (GitHub #1366), V1-2125 (GitHub #1358), V1-2121 and V1-2122 (GitHub #1116),
V1-2111. V1-2191 (GitHub #1405, bun runner) was not started in this lane.

## Problem

- #1366: an RSpec request example that asserts only a 401/403 refusal status of a mutating request
  proves the caller saw a refusal, not that nothing changed; a handler that writes before it
  denies stays green.
- #1358: the issue reports that a block handed to a helper that never yields passes validity.
- #1116: the nightly Release gates failed on runs 39121848489, 39258757168 and 39416257984.

## Decisions

- `TSA-V0-564` and `TSA-V0-565` are added to `docs/specs/test-source-analysis-v0.md` (IDs offset
  +100 above the next free ID on origin/main, 464). They are accepted under the owner's standing
  auto-accept of lane requirements (2026-10-11).
- `RB-DENIAL-WITHOUT-NO-EFFECT` is advisory only and changes no axis, as the issue asks. The
  abstentions `RB-DENIAL-VERB-UNRESOLVED` (no literal request call, or a let/subject shadowing the
  request helper) and `RB-DENIAL-MATCHER-CUSTOM` (a refusal or negated block matcher the reader
  does not know) are advisory too: an `abstained` severity would turn ELIGIBLE hygiene into
  ABSTAINED, which the issue rules out ("never makes the example INELIGIBLE") and which would
  regress every unrelated denial example.
- The request is read in the example body, the subject and lets it (or a hook) calls by name, and
  the before(:each)/let!/subject! hooks of its group chain; `get`/`head` denials are never flagged.
- #1358 is already delivered by V1-1866 (`b4cbca6f8`, `4aaef55de`; TSA-V0-084). The lane adds the
  issue's exact shape as `TestRSpecIssue1358NonYieldingHelper`: the multi-line commented helper in
  the spec file and in an included shared_context is `RB-BLOCK-NEVER-YIELDED` at the helper
  definition; the rewrite and the yield, `block_given?` and `&block` helpers stay clean; a helper
  in a support module abstains with `RB-HELPER-UNRESOLVED` (pinned, not flagged).
- #1116: every failure in the three failing runs is already fixed on main, by root cause, without
  skips: `TestATRV0004` group-kill race (`00c904e78`, V1-2103), `TestTQDV0021_RealProbes` helper fd
  and the no-Python conformance exemption (`7f46e42dc`, V1-1780, V1-1970),
  `TestMCPV0038IndexDeadlineHasDistinctCode` caller deadline (`f8c33fba7`, V1-1771), and the other
  groupreap callers (`becfcd18d`, V1-2111). The macOS `internal/mcp/bridge` package passed in run
  39416257984.

## Evidence

- `TestRSpecDenialWithoutNoEffect`, `TestRSpecDenialRequestOnChain`: pass; both fail with the
  `denialFindings` call removed from `exampleFindings`.
- `TestRSpecIssue1358NonYieldingHelper`: passes; fails when `neverYields` is forced false.
- `./internal/testsource` full package: pass.
- Local repeats on this host at the lane base: `TestATRV0004*` and `TestTQDV0021_RealProbes` at
  `-count=15`, `TestMCPV0038IndexDeadlineHasDistinctCode` at `-count=30`: all pass.

## NOT_RUN

- A passing scheduled nightly Release-gates run after these fixes is not yet observed; the
  hosted-runner timing classes are covered by local repeats only.
- Real-suite qualification of the denial reading against an adopter's Rails suite.
- V1-2191 (bun): not started; no bun toolchain installed, no live qualification.

## Rollback

Remove `internal/testsource/validity_rspec_denial.go`, its test and the `denialFindings` call, the
three rule constants, the help-text sentence in `cmd/corvint/tests.go`, the TSA-V0-564/565 section
and the `TestRSpecIssue1358NonYieldingHelper` file and traceability mention. Nothing is persisted.
