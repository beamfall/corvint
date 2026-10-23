# Corpus delta across additional repositories and no-op worklists

Date: 2026-10-23. Lane: opencode/delta-addrepo-gh1599 (base dd0d09db5). GitHub #1599, and GitHub
#1598 proposal 2. Ticket V1-2472. Requirements `DCP-V1-420..425` in
`docs/specs/documentation-corpus-v1.md` are accepted under the owner's standing auto-accept
(2026-10-11). They amend `DCP-V1-361`, `DCP-V1-364` and `DCP-V1-369`.

## Problem

Measured on main-8360815 (#1599): a corpus whose root holds only exported provider records and
whose source lives in `DCP-V1-175` additional repositories was compared with itself by
`delta --previous A --artifact A`. The result was 0 carry-forward, 105,694 unresolved and 60,331
uncovered. Three causes:

- `DCP-V1-369` excluded every additional-repository anchor;
- each anchorless record was its own `unresolved` entry;
- every uncited subject was an `uncovered` entry whether or not its lines changed.

The builder-upgrade case of #1585 therefore got no help. Separately (#1598), a caller paging the
worklist paid one full re-derivation per 256-entry page.

## Decision

- Anchors in a declared additional repository are classified like primary anchors. The delta opens
  each repository a previous anchor names, read-only from its declared checkout, checks the
  checkout's root commit against the declared identity, and uses the current manifest's inputs at
  that repository's current declared revision. Blobs are batched per repository, and all
  repositories share the read and hashing bounds. A confirmed anchor is re-pinned to that
  repository's revision, so carry-forward emits it and the next build imports it. Symbol anchors
  are confirmed in that repository's own index. Cross-path symbol re-resolution stays primary-only,
  as `DCP-V1-179` already states. Delta entries name the anchor's `repository` only when it is
  not the primary, so primary receipts keep their entry bytes.
- An undeclared repository, or a checkout that cannot be opened or holds another identity, leaves
  its anchors `unresolved` with the reason. The build refuses these cases, but the delta reports
  a gap and still classifies the other records.
- `--uncovered changed|all` (default `all`, so `DCP-V1-364` keeps its default) separates
  subject-level uncovered entries from changed regions. Under `changed`, the omitted subjects are
  counted in `uncovered_subjects_omitted`.
- `--anchorless summary|list` (default `summary`) replaces per-record anchorless entries with one
  `anchorless` row per provider and record kind. `list` restores the old entries. Summarized
  records still take part in dependency demotion and are never carried. This changes the default
  receipt, as #1599 asked.
- `--format jsonl` classifies once and streams a `corvint-corpus-delta-stream/1` header line, then
  every entry, each byte-identical to its paged form. The stream is bounded by the 128 MiB corpus
  byte bound. Paging options are refused with it rather than ignored. `--format` reuses the
  existing shared corpus option field. The delta-specific options are refused on `carry-forward`.

Rejected: changing the `--uncovered` default to `changed`. That is the better no-op view, but it
silently drops accepted `DCP-V1-364` entries from existing callers. A default anchorless summary is
the explicit #1599 request. Listing changed lines of additional repositories as uncovered regions is
left out because the region ID `PATH:START-END` is primary-scoped. It is recorded as a limitation and
a non-goal.

## Evidence

The fixtures follow the #1599 layout: the provider record is in the primary repository and every
anchor is in a separately committed library repository.

| #1599 row | Before | After (test) |
| --- | --- | --- |
| same artifact on both sides: carry-forward | 0 | every anchored record (`TestDCPV1420…/same-input`) |
| same artifact: unresolved | all additional-repository records plus anchorless | 0 listed; anchorless as one summary row |
| same artifact: uncovered | every uncited subject | 0 under `--uncovered changed`; omitted count equals the `all` listing |
| older builder, identical inputs | same as above | worklist identical to the same-artifact case |
| library line moved / changed | unresolved | `moved` re-pinned at the new line / `changed` with hunks and estimated span |
| carry-forward of the moved records | nothing carried | record re-pinned to the library's new revision builds; a second carry-forward is empty |
| undeclared repository or missing checkout | generic unresolved | unresolved naming the repository and cause |
| #1598: page loop | one re-derivation per page | one jsonl stream equal to the concatenated pages |

Tests run (focused, through the host semaphore):
`go test -run 'TestDCPV1420|TestDCPV1360|TestDCPV1366|TestDCPV1363|TestDCPV117' ./internal/doccorpus/`
and
`go test -run 'TestDCPV1360DeltaCLI|TestDCPV1421|TestDocsCorpusOperationFlagMatrix|Help' ./cmd/corvint/`.
Both PASS.

## Limits

- The real 166k-entry corpus from #1599 was not re-measured. The counts above are fixture
  evidence, and the production no-op count stays `NOT_OBSERVED` until the reporter reruns it.
- Changed source lines in additional repositories are not listed as uncovered regions. Symbol
  re-resolution across paths stays primary-only.
- The jsonl stream is built in memory before it is written. It is bounded, but it is not
  incremental.
