# Corpus delta carries adoption records forward

Date: 2026-10-23. Lane: opencode/delta-adoption-gh1611 (base 77a49dfa2). GitHub #1611, a follow-up
to #1599. Requirements `DCP-V1-440..445` in `docs/specs/documentation-corpus-v1.md` are accepted
under the owner's standing auto-accept (2026-10-11). They amend `DCP-V1-361`, `DCP-V1-365` and
`DCP-V1-366`.

## Problem

Reported in #1611: a self-delta (`--previous` equal to `--artifact`) still listed 19,071
`unresolved` entries: 18,939 subjects (159 flows and about 18,780 paragraphs) and 132 relations.
Every anchor of each was `carry-forward`. The cause was in `classifyRecords`: a confirmed record
of an adoption-only subject kind or relation type was demoted to `unresolved` because
`carry-forward` emitted only `corvint-corpus-provider/1`, which cannot hold those kinds or their
typed details.

## Decision

- Adoption kinds are classified like generic records. `carry-forward`, `moved`, `changed` and the
  per-anchor states follow `DCP-V1-362`, `DCP-V1-363` and `DCP-V1-420` unchanged.
- `carry-forward` emits a `corvint-corpus-adoption-provider/1` record when the provider imported
  as an adoption provider (a `typed_detail` import-parity row) or its carried records include an
  adoption kind or a typed detail. The record holds each carried record's
  previous typed details unchanged under `details`, and the per-kind bound is the adoption
  provider's 100,000 records. It is the adoption record as-is, not a generic record plus side
  fields, so the next build's own adoption admission and join validation govern it. Only a `/2`
  manifest imports that schema, so against a `/1` current manifest the operation is refused with
  the schema named, instead of emitting a record the build would refuse later.
- The `DCP-V1-365` closure now includes typed-detail joins: flow paragraphs, variations and
  entrypoints; a paragraph's flow and `retired_to`; coverage denominators; intent comparison
  lists; and supplied status supporting records. Flows and paragraphs reference each other, so the
  demotion became a worklist. A demoted ID re-queues the records that depend on it. This replaces
  the earlier fixed subject, then claim, then relation pass, and gives the same result for those
  records. `carry-forward`'s own closure check names typed-detail references carried only by
  another provider.
- An `exact` test join stays carried only while its file is among the anchors its target test has
  in the next build. Otherwise a moved test would emit a join the build refuses.
- Missing evidence stays a gap: an adoption kind that requires typed details but has none in the
  previous corpus is `unresolved` with that reason (`DCP-V1-441`).
- `validateAdoption`'s "requires typed details" predicates were extracted as
  `typedSubjectNeedsDetails` and `typedRelationNeedsDetails`, so the delta and the build share one
  rule.

## Evidence

| Check | Result |
|---|---|
| `TestDCPV1440DeltaAdoptionRecords` before the fix (original `delta.go`) | FAIL: the self-delta subtests list `docs:flow`, `docs:p0..p2` and `docs:retired` as `unresolved` with "record kind is adoption-only" |
| `TestDCPV1440DeltaAdoptionRecords` after the fix | PASS: self-delta over the additional-repository and multi-revision fixtures has 0 unresolved (paged and jsonl); a partly changed flow is `changed`; its paragraphs are demoted with the typed-detail reason; carried adoption records rebuild with identical `details`; `/1` refused; missing details stay `unresolved` |
| `TestDCPV1442DeltaExactTestJoinNeedsItsFile` | PASS |
| `go test ./internal/doccorpus/` (whole package) | PASS |
| Independent read-only review (subagent) | No HIGH findings. MED: a moved test subject could leave a carried exact test join the build refuses (fixed, demoted with reason). LOW: a demoted record the current corpus holds re-queued its dependents (fixed); an adoption provider whose carried records were all generic got the generic 4,096 bound (fixed by keying on the import-parity row); existence-only join checks against current records (documented in `DCP-V1-442`). |
| `TestCAPV00*`, `TestMCPV0100*`, `TestDocsCorpus*`, `TestDCPV1360DeltaCLI`, `TestDCPV142*` (focused) | PASS |

## Review repair (PR #1615)

- MEDIUM: unchanged anchors each charged the shared 64 MiB hashing bound, so a self-delta with
  1,025 records sharing one 64 KiB span listed the last one `unresolved` even though the corpora
  are identical, breaking `DCP-V1-444`. The delta now verifies each distinct (repository, path,
  blob, lines) span once and reuses the digest from a cache bounded at 65,536 spans. Only
  computed digests are cached and each anchor's span hash is still compared, so an unverified
  span never passes. Spans past the cache bound are hashed and charged each time as before.
  `TestDCPV1444SelfDeltaSharedSpanChargedOnce` failed on the previous code (`unresolved:1`) and
  passes after the fix. It also checks that a forged span hash over a cached span is not carried.
- LOW: `DCP-V1-440..445` now carry the leading standing-acceptance parenthetical, so
  `REQUIREMENTS.tsv` records them as accepted instead of `unrecorded`.

## Limits

- The fix was not re-measured on the reporter's 109,289-entry corpus. The fixtures exercise the
  same code paths, not the same scale.
- Journeys, observations and supplied gaps are still not classified or carried. Restricted
  findings and import parity are not carried either, because they are not records.
- Typed-detail joins are checked for existence only, apart from exact test joins. A carried
  record that joins current typed records can still fail the next build's own join validation,
  which then refuses it.
- A partly changed flow is listed `changed` and is not partly carried. Paragraphs whose flow
  changed are demoted rather than re-attached.
- The verified-span cache covers the unchanged-blob and previous-blob confirmations. Placement
  scans over a changed blob still charge every window they hash, so more than 64 MiB of distinct
  window hashing still ends in `unresolved` with the bound reason.
