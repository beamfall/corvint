# Immutable delta V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner issue #389; native ticket V1-0536 revision 2; repository
product invariants; docs/SPEC-DRIVEN-DEVELOPMENT.md; docs/DOGFOOD.md.

## Agent digest
- Claim: One deterministic, read-only, source-content-free observation of an explicit immutable change.
- Status: proposed/experimental; public CLI integration is under qualification.
- Exists: reviewed immutable compiler, public CLI, schema and conformance; bounded native no-op, tests-needed and findings witnesses.
- Blocked on: final bound qualification/publication and native completion. Native docs-only is deferred by owner decision 2026-10-01 (V1-0579 follow-up).
- Read next: Requirements; Bounds and failures; Acceptance and traceability.

## User and current baseline

Post-merge consumers need documentation drift, test obligations, assertion gaps and revision
identity in one record. Today they must combine affected, flowdocs and external evidence themselves.
The existing affected providers read the live checkout; delta needs arbitrary verified Git revisions.
The simpler baseline remains the separate commands and their explicit unknowns.
This prototype does not depend on the unmerged non-Go impact ABI from issue #390.

## Requirements

- `DLT-V0-001`: Delta MUST bind full lowercase base/head commit OIDs, head tree, complete
  changed-path digest and build. It MUST include the CEM path and additions, deletions and mode
  changes, verify object identities, and never substitute ambient HEAD or checkout content.
- `DLT-V0-002`: The affected graph MUST read every participating provider's source, config,
  metadata and explicit hidden harness through one immutable source. All eight providers MUST
  support it. Any encountered unexcluded nonregular entry, exhausted bound, integrity failure or
  swallowed reader failure MUST prevent partial graph admission and retain full-suite obligation.
- `DLT-V0-003`: Git blob bodies MUST be admitted before allocation by expected OID/type/size,
  per-source and cumulative budgets, then self-hashed. Refused reads MUST NOT use an unbounded
  fallback or memo bypass. Temporary sessions MUST close and cancellation MUST retire descendants.
- `DLT-V0-004`: Documentation drift MUST reuse flowdocs identity/check rules and report stale,
  retired, unchanged and added lexical observations. Previous generation MUST be bounded and
  bound to base/repository/confined non-dot scope. Missing or invalid baseline MUST remain unknown;
  generated candidates alone MUST NOT imply an existing document is clean.
- `DLT-V0-005`: External records MUST be captured once with bounded safe regular-file reads.
  Selection, assertion projection and digests MUST consume the same immutable bytes without
  filename/command/MCP/network redispatch. Strict selection confidence MUST remain unscored.
  Missing, stale, unavailable, incomplete or unverifiable coverage MUST require run-full-suite.
- `DLT-V0-006`: Only fresh verified explicit path-to-path asserts relations MAY establish a
  declared asserting link. Verifies MUST NOT become asserts. Unit/path relations MUST NOT close
  lexical-flow gaps: current schemas do not provide exact flow witnesses. Touched lexical flows
  and affected units MUST have separate explicit denominators; runtime behaviour remains unknown.
- `DLT-V0-007`: Optional bounded work-key extraction MUST use only self-hashed head commit
  metadata and a bounded Go regular expression. Only bounded opaque ASCII identifiers MAY be
  hashed to domain-separated keys. Commit prose and raw matched identifiers MUST NOT be emitted.
- `DLT-V0-008`: Output MUST be deterministic canonical JSON containing only validated paths,
  OIDs, digests, opaque hashed identities, closed enums, counts and booleans. It MUST exclude
  source excerpts, diff bodies, names, summaries, ticket prose and raw upstream maps. Unknown
  upstream values MUST become closed uncertainty codes, never echoed text.
- `DLT-V0-009`: The complete record MUST expose sorted uncertainty and a downstream decision
  with precedence findings, tests-needed, docs-only, no-op. Unknowns MUST survive display limits.
  No-op requires a complete empty change set; documentation-only requires complete classification.
- `DLT-V0-010`: The command MUST be R0: no network, state, tracing, object/ref/index writes or
  temporary checkout. Identical explicitly bound inputs MUST produce identical bytes. Published
  schema/conformance and two-run fixture equality MUST cover this contract before CLI delivery.

## Bounds and failures

Source blobs: 4 MiB before allocation; existing cumulative Git and affected unit/walk limits apply.
Git operations: one compilation declares 1,024 + 2 x 400,000 (`affected.MaxWalkEntries`) =
801,024 operations within the shared 30-minute wall budget, not the frozen 1,024 default.
Immutable paths resolve through verified parent listings: one operation per first-listed
directory plus one per blob read, with verified sizes reused for Stat. Exhaustion stays fail-closed
as `immutable-graph-unavailable` with the full suite required; it never narrows selection.
Documentation drift admits at most 32 scopes and so runs up to 128 context builds per record.
Unix input capture's no-follow refusal covers only the final path component; parent directories of
an explicit input path are trusted to the caller (known limit).
At most 32 provider records, 1 MiB each and 16 MiB aggregate. Prior generation: 16 MiB.
Work pattern: 1024 bytes; key: 128 ASCII bytes; extraction and output counts are bounded.
Unix capture uses nonblocking, no-follow open and fstat regularity, bounded read and close.
An unqualified platform refuses capture; it never uses an unsafe fallback. Missing optional source
paths alone are not fatal; read-integrity/mode/bound failures are sticky even if a plugin ignores them.
Cutoffs, excluded changed inputs, unsupported syntax, dynamic behaviour and absent provider evidence
remain explicit incomplete scope. Local checkout identity/freshness is an external input dependency.
No raw upstream diagnostic prose may cross the output boundary.

## Closed errors and uncertainty ownership

These emitted codes belong to this experimental slice. Compiler/input refusals return no record;
observation uncertainty stays inside the canonical record and retains its full-suite/gap obligation.
The shared captured-input helper's bounded refusal is mapped to provider capture uncertainty by
delta. No raw upstream diagnostic becomes an output code.

| Code | Meaning and retained boundary |
|---|---|
| `delta-invalid-arguments` | Invalid explicit revision, build, pattern or bounded option input; refuse compilation. |
| `delta-repository-unavailable` | Repository admission unavailable; refuse without ambient fallback. |
| `delta-head-unavailable` | Explicit head identity/tree unavailable; refuse. |
| `delta-change-set-unavailable` | Complete immutable changed-path set unavailable; refuse. |
| `delta-unrepresentable-path` | Changed path cannot cross the closed wire boundary; refuse. |
| `delta-input-unavailable` | Input file cannot be captured under the platform's safe regular-file profile; refuse capture. |
| `delta-record-invalid` | Canonical record violates its closed schema/invariants; refuse encoding. |
| `captured-record-bound` | Shared captured-provider input exceeds MaxRecordBytes; no unbounded copy/fallback. |
| `immutable-graph-unavailable` | Immutable affected graph admission fails; require full suite. |
| `affected-scope-incomplete` | Affected plan retains incomplete scope; require full suite. |
| `provider-coverage-missing` | No provider capture supplied; require full suite. |
| `provider-bound-exceeded` | Provider count bound reached; preserve incomplete coverage/full suite. |
| `provider-capture-unavailable` | Safe bounded capture/decode or total byte admission unavailable; require full suite. |
| `external-coverage-incomplete` | Strict captured selection cannot justify narrowing; require full suite. |
| `external-projection-incomplete` | Selected evidence projection omitted required observations; require full suite. |
| `unit-assertion-witness-unavailable` | Affected-unit assertion gap remains; findings and separate unit denominator. |
| `flow-assertion-witness-unavailable` | Touched lexical flow has no exact admitted asserting witness; findings. |
| `documentation-baseline-unavailable` | Nonempty change has no previous generation; retain baseline uncertainty. |
| `documentation-baseline-invalid` | Previous generation capture, identity, revision or confined scope invalid; retain uncertainty. |
| `documentation-incomplete` | Inventory/source/scope classification or bounded read incomplete; no clean-document claim. |
| `documentation-stale` | Existing generated observation has a changed lexical body; retain stale finding. |
| `observation-bound-exceeded` | Observation count exceeds display/admission bound; retain cutoff uncertainty. |
| `work-key-pattern-unconfigured` | Optional work-key pattern omitted; explicit uncertainty exempt from findings precedence. |
| `work-key-metadata-unavailable` | Verified bounded head commit metadata unavailable; retain uncertainty. |
| `work-key-invalid` | Matched key violates opaque ASCII/length admission; emit no raw key. |
| `runtime-behaviour-unclassified` | Runtime coverage remains unknown; explicit uncertainty exempt from findings precedence. |

## Non-goals and authority

No execution authority, runtime coverage, new provider transport, release qualification, learned
ranking, accepted intent promotion, writable snapshot, daemon or dependency on #390. A declared
assertion link is not proof of behavioural coverage. Existing live APIs preserve their contracts.
Public dispatch qualification is separate from source-stage compilation; shared registry and final evidence remain required.

## Acceptance and traceability

| Requirements | Planned implementation | Required evidence |
|---|---|---|
| DLT-V0-001, DLT-V0-003 | gitauth bounded_blob, revision_fs, delta_paths, delta_metadata | immutable identity, all path types, hostile header, byte bounds, cancellation |
| DLT-V0-002 | affected Source, BuildFS, eight providers | live/immutable graph and selection parity, arbitrary head, hidden config, retained read refusal |
| DLT-V0-004 | delta documentation adapter | missing/stale/wrong baseline, identity relocation, confined scope |
| DLT-V0-005, DLT-V0-006 | extevidence captured selection; delta inputs | stable input parity, mutation trap, FIFO/symlink/growth refusal, verifies/asserts distinction |
| DLT-V0-007, DLT-V0-008 | delta projection and canonical encoding | opaque keys, prose sentinels, closed schema and unknown states |
| DLT-V0-009, DLT-V0-010 | delta compiler, CLI, protocol and conformance | four decisions, full-suite uncertainty, two-run equality, no mutation/network |

Each requirement is anchored by exact-ID Go subtests (`t.Run("DLT-V0-NNN ...")`, V1-0583):

| Requirement | Anchoring tests |
|---|---|
| DLT-V0-001 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree, TestDeltaNoOpAndBadRevision; `internal/cem/gitauth`: TestDeltaPathsIncludeCEMAndTypeChanges |
| DLT-V0-002 | `internal/cem/gitauth`: TestRevisionFSImmutableAndBounds; `internal/delta`: TestDeltaGraphAboveDefaultGitBudget; `internal/liveverify/affected`: TestImmutableAllLanguageParity, TestImmutableNonregularBeforeLanguageFilter, TestImmutableSwallowedReadRefusesGraph, TestSourceReadBoundsAndStickyFailure, TestSourceHiddenWalkRetainsIgnoredFailure |
| DLT-V0-003 | `internal/cem/gitauth`: TestRevisionFSListingCostScalesWithDirectories_DLT_V0_003, TestBoundedBlobRejectsHeaderWithoutFallback, TestBoundedBlobCumulativeBudgetBeforeBody, TestBoundedBlobCancellationRetiresDescendant, TestBoundedBlobIdentityAndNoMemoBypass, TestBoundedBlobFourMiBBoundary |
| DLT-V0-004 | `internal/delta`: TestDeltaPreviousGeneration, TestDeltaEmptyChangeBindsPrevious, TestDeltaDocumentationExclusionsAndHTML |
| DLT-V0-005 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree, TestDeltaIncompleteProviderRequiresFullSuite, TestDeltaCaptureBoundAndTransport, TestDeltaCaptureFIFORefusesWithoutWriter; `internal/extevidence`: TestCapturedSelectionParityAndMutation, TestCapturedFailuresDoNotNarrow |
| DLT-V0-006 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree, TestDeltaReachedUnitDenominatorIncludesDiamond; `internal/extevidence`: TestCapturedAssertsDistinctFromVerifies |
| DLT-V0-007 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree; `internal/cem/gitauth`: TestDeltaMetadataAndBoundedSHA256 |
| DLT-V0-008 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree, TestDeltaRecordRejectsProseAndInvalidEnums, TestDeltaSchemaPathCorpus, TestDeltaRefusesControlCharacterGitPaths, TestDeltaRecordsMatchPublishedSchema |
| DLT-V0-009 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree, TestDeltaNoOpAndBadRevision, TestDeltaPublishedDecisionVectors (all four wire decisions; native docs-only deferred below) |
| DLT-V0-010 | `internal/delta`: TestDeltaFixtureMergeDeterministicSourceFree; `cmd/corvint`: TestDeltaInternalCLIExplicitImmutableNoOp, TestDeltaCLIPreservesClosedRefusalCodes |

Execution evidence is retained in `docs/build-log/2026-09-30-immutable-delta-public-integration.md`.
Final integration uses approved public base `7bd7f5e03ad177e7496ce5dbd1563b8466f591a4`.
Native generation 88 was widened atomically in receipt 1579; ticket effects remain INCOMPLETE.
The earlier source-stage and public-base `01f557` workflows remain historical; the latter was
explicitly cancelled as incomplete before a distinct final-base enrollment. No result or satisfaction
is transferred to the new base. The initial empty-diff/absent-intent refusal remains visible.
The branch was later reintegrated by squash import onto public `cdd2e31fffd732419eb9ade531ccf289259f3243` under owner
decision 2026-10-01; see `docs/build-log/2026-10-01-delta-reintegration.md`. No earlier
receipt transfers to that base either.

**Owner decision 2026-10-01 (docs-only deferred).** The owner accepted, as an interim limit, that a
documentation-only change keeps producing `tests-needed` (or `findings`) under the strict-provider
profile: "keep docs-only as tests-needed for now". The classification is unchanged; the `docs-only`
value stays in the closed schema and published wire vectors, and native `docs-only` is deferred to
V1-0579, which stays open. The other three native classes are witnessed. The history below records
why the class is unreachable today. Actual native `tests-needed` is witnessed
by a changed Node test invocation with a stable documented helper and fresh explicit assertion.
Native `docs-only` has a current-profile structural conflict: strict external coverage must qualify
every changed path through runnable test evidence; the resulting selected test overrides the
`docs-only` decision. Missing or non-narrow provider evidence instead forces `findings`.
This is not permission to weaken requirements, remove unknowns or change classification.
V1-0579 retains the predicate proof and the separately verified positive fixture. Runtime behaviour
remains unknown; `runtime-behaviour-unclassified` is explicitly exempt from findings precedence.

## Rollout, rollback and open decisions

The prototype remains experimental until focused checks, independent review, live CLI conformance,
CEM/OCM and integration complete. Publish no validated/production claim from source-stage evidence.
Rollback reverts only this capability's owned changes under integration policy, preserving receipts.
Any schema, reader bound, provider relation or scope change requires re-evaluation of affected proofs.
Owner acceptance and external outcome validation remain separate promotion gates.
