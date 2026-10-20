# 2026-10-20 V1-2039, V1-2040: example receipt join and caller-asserted requirements

Tickets: V1-2039 (GitHub #1291) and V1-2040 (GitHub #1292).

## Decision

`TSA-V0-410..419` are added to `docs/specs/test-source-analysis-v0.md`, accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). The block sits above the highest TSA-V0 ID
on origin/main and in the open PRs checked at lane start.

- V1-2039: `corvint tests example-join --receipt FILE` prints `corvint-example-join/0`, one record
  per example of one Ruby or JS provider receipt. Each record carries a line-free identity
  (`rspec-scoped-id`: spec path plus RSpec scoped id; `js-title-path`: file > project > full
  title), the run outcome, the static hygiene and strength verdict, and the revision plus the
  committed files and blob IDs the verdict read.
  - The join runs at the one revision the receipt retained before and after its run. A missing
    revision refuses `tests-join-revision-unbound`; a run across two revisions, another `--rev`, or
    a bound file (Ruby spec or helper blob, JS SHA-256) whose committed content differs refuses
    `tests-join-revision-mismatch`.
  - A shared-group example is keyed by its including spec and read through its own inclusion, so
    the record cites both files. A dirty receipt joins only through files it bound; an including
    spec it did not bind abstains `rspec-inclusion-source-unbound`.
- V1-2040: `--select SELECTOR=REQ[,REQ...]` (repeatable) and `--requirements FILE` (one pair per
  line) take PATH, PATH:LINE or PATH[ID] selectors. The record echoes the IDs as
  `requirements{basis: "caller-asserted"}`; they are never inferred and never change a verdict or
  outcome. A malformed pair refuses `tests-join-selector-malformed`, a selector matching no example
  refuses `tests-join-selector-unknown`.
- The provider receipt formats are unchanged, to stay additive beside #1311 and V1-2045. The echo
  lives in the join record, not inside the provider document.

## Evidence

- Focused tests: `TestExampleJoinRubyShiftedLines`, `TestExampleJoinSharedGroup`,
  `TestExampleJoinJS`, `TestExampleJoinRevisionRefusals`, `TestExampleJoinRequirements`
  (`./internal/testsource`, whole package passes) and `TestTestsExampleJoinCLI` (`./cmd/corvint`).
- Base fails: on the lane base the new tests do not build (`undefined: ExampleJoin`).

## Review fixes

One independent Codex round found three P2 defects; each is fixed with a regression test in
`TestExampleJoinSupportFiles`:

- A clean receipt whose helper set was unbound (for example a committed helper symlink,
  `helper-not-regular-file`) still resolved helpers from the other committed support files. Helper
  resolution now stays unbound with the receipt's reason whether the receipt is clean or dirty, and
  no support file is read (`TSA-V0-414`).
- Support files were read under the validity command's own budget, so 300 support files passed the
  256-file bound and an oversized one degraded to `rspec-support-unreadable`. They are now read
  through the join's file cache, so both refuse `tests-bound-exceeded` (`TSA-V0-416`).
- A verdict decided by a support helper did not cite it. Every support file the verdict was read
  with is now recorded with its blob (`TSA-V0-412`); a shared example's record now also lists the
  support set after the including spec and the shared file.

## Limits

- Live qualification against a real RSpec or Playwright run: `NOT_RUN`; the tests use constructed
  receipts bound to committed fixture repositories.
- A JS test file outside the repository root, or a receipt whose digest keys do not identify the
  anchor file uniquely, abstains `example-identity-unbound`.

## Rollback

Revert the change: the verb, its two files and the `readOneBlob` split are additive, and no stored
format changes.
