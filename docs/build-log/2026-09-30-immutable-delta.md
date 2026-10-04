# Immutable delta source stage — 2026-09-30

Issue: #389; native ticket: V1-0536 revision 2. Intent is proposed and delivery experimental.
Public base: `25971bda1ca1664d8a546d751cb2bb9bbd454daf`.
Owning contract: [immutable-delta-v0.md](../specs/immutable-delta-v0.md), first pinned at
`194509d5`. No dependency on #390's unmerged impact ABI is used.

## Scope and decisions

Generation 73 admits the 29 source paths. The shared dispatcher/help, spec registry, CEM and four
use-case receipt paths are deliberately deferred to a coordinated scope window. No actual top-level
`corvint delta` delivery, integrated documentation registry, CEM/OCM completion, release qualification
or ticket completion is claimed here. Early tracked dogfood-change was not run because it writes an
excluded shared path. The private source enrollment succeeded after the write boundary was verified;
this defers the shared closeout rather than waiving it.

The source compiler reads explicit immutable commits, uses bounded object-header admission and
copies provider inputs once. All eight affected language providers expose an explicit reader.
Reader failures remain sticky even if a provider ignores an optional read error. The complete reached
unit universe is counted before selection filters units without tests; a shortest witness chain alone
cannot enumerate a diamond. Existing affected selection semantics remain unchanged.

Flowdocs currently extracts lexical Ruby/JavaScript/TypeScript/HTML candidates. Delta withholds
scoped HTML before calling Generate/Open/CheckRevision because that existing reader applies its
4 MiB limit after allocation. Native size/exclusion/unparsed/unsupported-language omissions remain
explicit incomplete documentation. This intentionally conservative boundary is not HTML qualification.
Legacy reader investigation is V1-0565, receipt 1490; the separate legacy provider capture race question
remains V1-0562. Neither is fixed by the new bounded adapter.

Path admission and the published JSON Schema now agree: canonical relative UTF-8 paths, no empty or
dot segments, backslashes, C0 controls or DEL. Opaque keys, provider identities, unit/flow IDs and
uncertainty evidence are hashes; source prose never becomes record text. Schema decision vectors
are wire fixtures, not claims of real Git objects or complete workflow outcomes.

## Independent review and observed proof

Gate A passed revised plan 2 (`a14a9e79c7fb483c12438a39a90185871038772f48c824a3a66f4c3bb5d65014`).
One independent Astra/medium reviewer was used because immutable authority, allocation boundaries
and source privacy require careful review. Runtime model controls and billed/cache tokens were not
observed. No panel, nested delegation or full gate was run.

Initial source review found four issues: documentation allocation bypass, omitted lexical inputs,
missing reached-unit denominator members, and ignored prior-generation input for an empty change.
Repair 1 resolves all four; its regression tests pass. R1 found one additional schema/runtime path
mismatch. Repair 2 aligns the path grammar and proves real Git TAB/LF paths are refused. Final bounded re-review PASS is retained at `/tmp/delta-source-review-r2.json`; all five findings
are resolved, with no third repair. Focused vet completed with exit 0 (`/tmp/delta-vet.log`).

Retained local evidence (these paths are locators, not portable attestations):

- `/tmp/delta-reader-tests.log`: all-eight live/immutable graph and selection parity; special modes,
  sticky swallowed errors, immutable head/dirty checkout, size bounds and corrupt-object refusal PASS.
- `/tmp/delta-repair1-tests.log`, `/tmp/delta-review-regressions.log`: real fixture-merge determinism,
  opaque work keys/prose sentinels, prior-generation binding, safe capture, assertion distinction,
  complete changed paths and SHA-1/SHA-256 evidence PASS.
- `/tmp/delta-complete-focused.log`: focused delta/captured/source/Git-session and internal CLI
  checks PASS before the final path-only repair.
- `/tmp/delta-bounded-adversarial.log`: malformed/oversized/type/OID/short headers refuse without
  fallback; cumulative budget and interrupted-reader descendant cleanup PASS.
- `/tmp/delta-path-schema-tests.log`, `/tmp/delta-schema-validation.json`: runtime path corpus and
  actual Draft 2020-12 validation PASS (8 accepted records, 11 rejected paths, extra field rejected).
  The external validator is an isolated temporary test dependency, not a product dependency.

The first joined-fixture test failed because its fixture assumed Go lexical flows. It was corrected
by adding a supported JavaScript flow; unsupported Go remains explicit uncertainty. The original
failure remains in `/tmp/delta-join-tests.log`.

The complete affected-package run failed only `TestSelectionOnTheLiveDirtyWorktree`: its existing
oracle treats `ENCLOSING_PACKAGE` as a direct-owned-source witness for the new conformance README.
All eight provider suites passed. This observed oracle failure is retained as V1-0564, receipt 1489,
with `/tmp/delta-all-provider-regressions.log`; clean-worktree success must not erase that failure.
The affected plan itself is UNKNOWN with language frontiers and unowned paths, so focused proofs do
not establish exhaustive safety. Full repository gate: NOT_RUN under the scoped-work policy.

## Requirement evidence and remaining closeout

| Requirement | Executable source proof |
|---|---|
| DLT-V0-001 | TestDeltaPathsIncludeCEMAndTypeChanges; TestDeltaNoOpAndBadRevision |
| DLT-V0-002 | TestImmutableAllLanguageParity; TestImmutableNonregularBeforeLanguageFilter; TestImmutableSwallowedReadRefusesGraph |
| DLT-V0-003 | TestBoundedBlobFourMiBBoundary; TestBoundedBlobRejectsHeaderWithoutFallback; TestBoundedBlobCumulativeBudgetBeforeBody; TestBoundedBlobCancellationRetiresDescendant |
| DLT-V0-004 | TestDeltaPreviousGeneration; TestDeltaEmptyChangeBindsPrevious; TestDeltaDocumentationExclusionsAndHTML |
| DLT-V0-005 | TestCapturedSelectionParityAndMutation; TestDeltaCaptureBoundAndTransport; TestDeltaCaptureFIFORefusesWithoutWriter |
| DLT-V0-006 | TestCapturedAssertsDistinctFromVerifies; TestDeltaReachedUnitDenominatorIncludesDiamond |
| DLT-V0-007 | TestDeltaFixtureMergeDeterministicSourceFree |
| DLT-V0-008 | TestDeltaRecordRejectsProseAndInvalidEnums; TestDeltaSchemaPathCorpus; TestDeltaRefusesControlCharacterGitPaths |
| DLT-V0-009 | TestDeltaPublishedDecisionVectors; conservative real merge/no-op fixtures |
| DLT-V0-010 | TestDeltaFixtureMergeDeterministicSourceFree; TestDeltaInternalCLIExplicitImmutableNoOp; actual argv qualification remains deferred |

Remaining: source commit and frozen selected checks; release generation73
UNPUBLISHED; acquire shared scope; integrate CLI/help/registry and exact receipt repairs; actual CLI
qualification; final CEM/OCM/reports/check/seal; publish a ready PR after required checks/review;
coordinator integration and native completion. Merge remains separately authorized. Rollback reverts
only owned source commits and preserves all failed/passing evidence and open tickets.
