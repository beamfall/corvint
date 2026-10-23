# Corpus delta worklist and span-hash carry-forward

Date: 2026-10-23. Lane: claude/corpus-delta-gh1585 (base 0b7d59ba6). GitHub #1585.
Requirements `DCP-V1-360..369` in `docs/specs/documentation-corpus-v1.md` are accepted under the
owner's standing auto-accept (2026-10-11).

## Problem

Records from an external provider were written against one revision of the repository. When the
repository moved on, the only check was a full rebuild. That rebuild refused or marked every anchor
whose blob had changed. It did not say which records still held, which had only moved, which needed
review, or which new source lines no record covered. The caller also had no way to feed the records
that still held into the next build.

## Decision

- `docs corpus delta` classifies each previous records-provider subject, claim and relation against
  the current revision. Each anchor is re-validated:
  - the blob is read from Git and its SHA-256 checked;
  - the span is hashed with the build's own span digest;
  - the outcome is `carry-forward` (same lines), `moved` (one other window, or symbol
    re-resolution through the `DCP-V1-140..142` resolver to a span with the same hash), `changed`
    (with an estimated mapped span and bounded Myers diff hunks), `removed` or `unresolved`.

  A record takes its worst anchor's class. Claims and relations whose subject or endpoint is not
  confirmed are demoted until the set is stable. Changed source lines that no records-provider claim
  covers become `uncovered` regions. The worklist is sorted with the actionable classes first and
  paged.
- `docs corpus carry-forward --provider ID` emits a `corvint-corpus-provider/1` record of that
  provider's confirmed records:
  - anchors are re-pinned to the current revision with the same span hash;
  - the compiler's review demotion is undone, so importing the record reproduces the previous
    trust, reported trust and limitations;
  - records already in the current corpus are skipped.

  The caller commits and declares the record. Nothing is written by Corvint.
- The current side is either an artifact (`Open`, rederived) or a manifest built in memory
  (`BuildAdmitted`). The previous side is read canonically: every value used from it is checked
  again against Git.

## Rejected

- Fuzzy or semantic re-anchoring. A span that does not hash to its recorded value is never carried.
- Carrying native compiler records. The next build re-derives them.
- An MCP tool. It is outside the issue, and the CLI receipt is the contract.

## Evidence

- `GOTOOLCHAIN=local go test -count=1 -run DCPV136 ./internal/doccorpus/`: ok.
  - `TestDCPV1360DeltaWorklistClasses`: every class, symbol move across paths (TS), dependency
    demotion, uncovered regions, paging, determinism, read-only and refusals.
  - `TestDCPV1366CarryForwardFeedsNextBuild`: the carried record builds, the evidence matches, and
    a second carry is empty.
  - `TestDCPV1363LineDiff`.
- `GOTOOLCHAIN=local go test -count=1 ./cmd/corvint -run 'TestDCPV1360DeltaCLI|TestDocsCorpus|HelpTopic|TestDCPV1174|TestDCPV1241'`: ok. Both current-side forms agree, the worktree is unchanged, and the refusals hold.

## Limits

- Anchors in additional repositories, and journeys, observations and supplied gaps, are not
  carried or classified (stated as limitations).
- Cross-path symbol re-resolution covers the languages the symbol resolver normalizes (Ruby,
  TypeScript/JavaScript). Other symbol anchors are placed by span hash within their own path.
- Diffs past 1024 edits, or past the 8 MiB per-blob or 64 MiB total read bounds, degrade to a coarse
  region or an `unresolved` anchor with its reason.

## Review repair (PR #1589)

Independent review raised six findings and one request; all were confirmed and fixed with
regression tests in `TestDCPV1360DeltaRepairs`:

- Span numbering: the delta numbered lines without the empty line after a final newline, while
  `spanDigest` and the compiler's `span` keep it, so a valid end-of-file anchor panicked once its
  blob changed. Placement and the previous span now use the build's numbering (`DCP-V1-362`).
- Provider closure: classification demotes across all providers, so a provider-scoped
  carry-forward could emit a claim whose subject only another provider carries. Decision: refuse,
  naming each dependency and its carrying provider, rather than silently dropping records or
  widening the record to another provider's IDs; the caller carries and builds that provider first
  (`DCP-V1-365`, `DCP-V1-366`).
- Uncovered regions: covered lines are now subtracted and each uncovered sub-range is its own
  region; current subjects that no claim names are `uncovered` subject entries (`DCP-V1-364`).
  Regions sort before subjects within `uncovered`.
- Bounds: the aggregate read bound is passed as the header-checked per-blob limit, so no body is
  read past it, and every span digest is charged to the hashing bound before it is computed,
  including the unchanged-blob path (`DCP-V1-367`).
- A symbol anchor the resolver reports changed now carries `changed_lines` (`DCP-V1-363`).
