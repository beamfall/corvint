# Immutable delta current-main integration and bounded reader compatibility

Date: 2026-10-03. Status: source and focused tests reviewed; terminal qualification pending.

This entry records the distinct current-main integration for issue 389 / V1-0536.
The proposed/experimental immutable-delta intent and requirements remain unchanged.
Public integration base: `32aa5a3f2b7f3fe658d208049fb009e2c9e24c13`.
The four-document seed `79bd635d91f0f3944c781882ae6f6d4bb968d58b` is the approved evidence base.
Enrollment `61668a08d33c7865786bc2f8fb8cf982997d7072c78fae7ad8a81d9f92c261cd`
freezes plan `94aa3939437eed96da6955440485e861e8fe7489a9804b937ddef7742d026fea`
with ten checks and three intents. Its initial zero-diff run produced no CEM/OCM;
`git-diff-failed`, missing native prechange registration and absent outcome input remain recorded.

Ordinary merge `655ddcacc4c2247309a046931f67da6aeec11a03` has the seed as first parent
and original PR #473 head `abb23961a70f2687caf3c1f81b8eec298edb7319` as second parent.
Four artifact-only inverse commits preserve original maps in ancestry while removing their
imported current-tree artifacts. Composition checkpoint `e5a3ff22507c411aa3134c604cad804c16d124eb`
preserves all 350 seed CEM archives, the public native store and all unrelated seed paths.
The frontier citation conflict uses actual merged help and current README anchors; the
duplicate delta catalog entry was removed without changing the four seed documents.

## Compatibility decision and evidence

The merged Go provider initially failed compilation because `applyReadScopes` still accepted
a disk root while its caller supplied an immutable Source. `Source.ReadBounded` and
`RevisionFS.OpenBounded` now admit the manifest against the smaller 1 MiB consumer limit before
allocating a Git body, retaining the existing cumulative bound and immutable identity checks.
Ordinary 4 MiB `Source.Read` behavior is unchanged. Unsupported opaque filesystems refuse;
optional missing manifest opens remain nonfatal. Invalid declarations retain the existing
live selection frontier, while immutable capture failures remain sticky graph refusals.

Assertion-bearing tests cover real Git/ambient disagreement, manifest parity/refusal,
exact-limit and oversize admission, growth, regularity, cumulative budgets and memo bypass.
The eight-provider parity fixture explicitly implements bounded admission. Four focused
package suites and vet passed on initial patch `4bfe70a54e80beb20b6a036eda7c8afc5b964042cab84f2f66245691353c3350`.
The source baseline failure and corrected memo-fixture setup failure remain retained.

Independent review found one P2: an oversized read could discard a simultaneous read error.
Regression tests reproduced both a sentinel failure and mixed `fs.ErrNotExist` before repair.
Repair 1 preserves read, limit and close causes together in returned and sticky errors;
plain missing opens remain optional. Five combined cases, the affected package and vet passed.
The same reviewer incrementally passed repaired patch `55eee2b946811f4f10fb962daae127feda3493760c9ac606e92c02e22ade8de7`.
V1-0536 records the finding and repair in receipt 2298 (R7/AC5), with `agent-memory`, `bugs`
and `tests` labels. Code reservation 218 was returned CANCELLED/FENCED in receipt 2299.

## Traceability and frozen test selectors

The docs-only seed's 37 unresolved test references name 30 distinct tests. All 30 definitions
are present after composition; this static resolution is not a new traceability-gate PASS.
No requirement or catalog change was needed to resolve that initial absence.

Later static preparation found five exact test names required by the frozen runner absent.
Four substantive new tests had different names; the DLT-V0-009 observation-limit witness
was missing. Four direct renames preserved their assertions and all spec-referenced names.
The new `TestDeltaUnknownsSurviveObservationLimit_DLT_V0_009` exercises both production display
limits with `MaxObservations+3` distinct observations. It verifies omitted counts, unknowns
before and after the cutoff, conservative flags and canonical round-trip.

All five exact frozen selectors executed and passed without skips, including both new
display-limit cases (41 pass events). The same independent reviewer passed the test-only
increment at patch `84a297838c02feb5b1e46fae104657a34a234b2950f4b0e0ee16f0433d5f969a`.
Production bytes remained unchanged from repair 1. Receipt 2305 records this evidence in
V1-0536 at R8/AC5; reservation 223 was returned CANCELLED/FENCED in receipt 2306.
The frozen manifest, plan and enrollment were not changed. These are focused source/unit
results, not a terminal frozen-check run; earlier receipts retain their original bindings.

## Remaining qualification and rollback

Native replay has not run on this integration. Historical no-op/tests-needed/findings
observations and four wire vectors do not establish current-target runtime qualification,
native docs-only, release quality, external adoption or runtime behavior coverage.
Native docs-only remains deferred to V1-0579 under the recorded owner decision. Original
PR #473's separate enrollment chronology/owner question remains untouched; no cancellation
or reset is implied. Fresh focused-docs and every frozen check, strict semantic CEM/owning OCM,
publication, integration and native completion remain outstanding. V1-0536 stays open.

Rollback uses ordinary reverts of this activity's owned commits/patches, preserving old
PR ancestry, seed archives, native receipts and frozen enrollment evidence. Historical
shared CEM artifacts must not be restored as proof of this target.
