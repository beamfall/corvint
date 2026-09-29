# Native release lifecycle integration (issue 357)

Integrated the bounded CAL-V0-027 source and tests previously sealed at
`a2fcefe00e0c154c3dc774a0560010f7ffa1f001` onto public base
`edafa4d152926f3537746af2c73ad52cc032cdea`. Existing pool and supervisor
contracts remain present. The colliding archive requirement is CAL-V0-042;
CAL-V0-027 retains the accepted non-fixture lifecycle contract.

## New integration evidence

Evidence directory: `/tmp/corvint-native-release-357`. The published Tasks
binary (sha256 `9efb3e6ebc5d2d83bf12ffca0a1c9475689206ae8db787538290f65742e86dc7`)
refused release creation on disposable native queues with UNSUPPORTED,
"release mutation is fixture-only". The patched binary (sha256
`099f524636ff9f35b09aa3c84deed5bcaec161862c589cc26aa89364ea90e972`)
passed the existing native release binding and readiness scenarios through a
scratch Go overlay that replaces only the test CLI adapter with an external
executable. Those scenarios exercise create, update, candidate, record-gate,
promote, ordered releases, settled KEEP_JOURNAL reconciliation, and existing
source, CAS, actor/policy, acceptance and gate refusals. Original test bodies
remain unchanged. Both binary outcomes are retained, not relabelled historical runs.

Focused store, snapshot and CLI package tests passed (189.863s, 4.809s and
50.147s); specindex and companionrelease passed (0.288s and 119.990s).
Specification/requirement/traceability/decision/citation checks passed. An initial
stale-TSV check failure is retained; regeneration repaired the locator before review.
Independent review reported no P1/P2 findings on staged diff sha256
`fcc27ec3e03acd056363f8e05e9ff7bf4d9b2b8527225220b4f17ddf6bc268a0`.
The following delta only records these outcomes and updates delivery wording.

Corvint query, tracked-path impact, affected planning and enrolled dogfood were
used with fresh receipts. The initial dirty-tree dogfood pass was incomplete
(map unavailable, OCM drift and outcome/index failure); final immutable binding
is a separate subsequent step. No retrieval/ranking change requires a frozen
retrieval evaluation. Provider, mutation and external adoption routes are not
applicable. The compiled disposable lifecycle is local evidence, not production
queue takeover or independent interoperability qualification. Billed tokens,
cache use and complete cost remain NOT_OBSERVED.

## Remaining delivery boundary

Repository-wide gate is NOT_RUN under scoped issue policy. V1-0449 remains open;
its declared full-gate requirement and canonical completion are coordinator-owned.
No canonical queue, release projection, download or issue completion was changed by
this integration. Artifact publication must bind the final integrated source.
Rollback restores the three admission boundaries while retaining receipts,
release projections and evidence blobs. Shared staging observes supported classes
without cleanup authority; active staging and ADOPT_FILE retain their refusals.
