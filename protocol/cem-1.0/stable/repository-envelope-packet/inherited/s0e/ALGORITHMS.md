# Proposed stable CEM 1.0 algorithms

Status: PROPOSED / EXPERIMENTAL, contract draft only. Owner intent is accepted; this
technical contract is not accepted or implemented. Apache-2.0. No native AGPL
implementation or native test body was copied into this packet.

`stable.schema.json`, `OPERATION.json` and `RESULT.schema.json` form one proposed
contract. Schema validation alone is insufficient: strict JSON, byte limits,
identities, relations, Git objects and rooted filesystem reads need the algorithms
below. New requirement numbers CEM-V1-017..028 are proposed additions; existing
CEM-V1-001..016 and the experimental candidate keep their original meanings.

## CEM-V1-017: exact wire and absent evidence

The exact profile is `cem/1.0`, with ten required root keys and no others: `spec`,
`baseRevision`, `patchSha256`, `excludedPath`, `evidence`, `hunks`,
`criterionBindings`, `runnerReceipts`, `criterionLinks`, `artifacts`. This is a
separate closed decoder, not a label substituted into a 0.3 or candidate document.
Legacy and candidate operations continue to admit only their existing profiles.

Read one private original byte copy, at most 4 MiB. Require strict UTF-8 and one
complete JSON object, reject duplicate keys at every depth, trailing values,
unpaired surrogates, null substitutes and unknown fields. Maximum nesting depth
is 64, counting the root object as depth one. Each integer token uses unsigned
canonical decimal syntax (zero, or nonzero digit followed by digits), with no
sign, leading zero, fraction or exponent, and value <=9007199254740991. Booleans
are not integers. Validate scalar type before hashing or conversion. No decoder
may drop unrecognized members and then call the document valid.

Bounds: 4096 evidence records; 1..2048 hunks; 0..256 criterion bindings;
0..64 runner references; 0..1024 links; 0..1024 artifact records. Each link has
1..32 unique IDs in each of its three reference arrays. Criteria, runners, links
and artifacts are either all empty or all nonempty and completely linked. Empty
arrays mean no task/runner evidence was supplied, not that no obligations exist.
Every declared criterion and runner must be used; a map can leave some hunks or
criteria in the external ticket unlinked without claiming them satisfied.

Wire-field validation precedes cross-record reference validation, which precedes
independent authority argument checks and Git access. JSON failure precedes
profile admission; an unsupported profile precedes root-key shape checks;
missing required keys precede unknown keys. Within objects, validate in the order
listed in the schema's required array; optional witness objects follow required
hunk members. Within arrays, input order governs first failure. Graph precedence
is criterion identity/uniqueness/coherence, runner uniqueness, link resolution,
link/basis membership, then artifact role closure. OPERATION defines stages.

## CEM-V1-018: unchanged change semantics, with full 0.3 vocabulary

The public corrected `inherited/ALGORITHMS.md` supplies the canonical JSON identity (with its complete spelling in
`inherited/cem-0.1-algorithms.md`), canonical patch, hunk replay, evidence and drift algorithms and the four
structural predicates. Its profile/command/candidate exclusions, draft-stage
qualification notes and prototype implementation status do not define this stable
operation. Stable admission and result semantics are exclusively defined here.
Reusing pure validation algorithms is permitted; rewriting the original `spec`
into another profile to obtain a verifier result is forbidden.

The hunk vocabulary includes supported/evidence-backed, three unknown reasons,
whitespace-only, line-ending-only, rename, move, import-reorder and formatter-only.
Every evidence is referenced by at least one basis. Preserve stable/relocated/
stale/ambiguous/deleted drift, including `baseBlobOid`, nullable target blob and
span, and deterministic evidence-ID order. Stale, ambiguous or deleted evidence
cannot yield an accepted canonical change.

Structural proofs require existing regular base bytes and actual modifications.
They parse Go syntax irrespective of the filename suffix. Rename, import-reorder
and formatter-only use the individual hunk image; move uses the whole file group.
The corrected rename predicate includes local receiver names and statement labels;
method/selector/field/directive exclusions stay intact. Rename is file-local and
can leave sibling-file references uncompilable. Move preserves var/init ordering.
Formatter-only is exact go/format-image equality. None proves package semantics,
build success, test adequacy or behavioral equivalence. Unrelated sibling hunks
cannot prove the individual claim; group-level move intentionally inspects the
whole group. No runtime fallback weakens these predicates.

Coverage and discrimination objects are the complete 0.3 objects, unchanged in
meaning and limits. Coverage retains profileSha256, caller testRun, mode, state
and exact ordered ranges; covered/uncovered/absence remain different. A validated
range may name context lines: no execution or coverprofile replay is implied.
Discrimination retains treeRevision, selectionSha256, mutants, killed, survived,
all survivor descriptions/operators/lines, bounds, state and detail. Count checks
retain uncompiled residuals (mutants may exceed killed+survived); never turn those
residuals into kills. Caller treeRevision is not automatically the authenticated
CEM target. Discriminates/survived/not-run/absence remain distinct.

Text bounds are measured in UTF-8 bytes. testRun is 1..256 bytes, operator 1..64,
description 1..512 and detail 0..512; reject C0 and DEL, not all Unicode characters
classified as controls or nonprinting. U+0085/U+2028 remain valid. Structural and
witness checks must retain the corrected public 0.3 corpus, not merely its happy
path. Unknown/survived/uncovered claims can coexist with valid change integrity.

## CEM-V1-019: lossless typed documents and immutable input

Native `StableMap`, `ParseStable`, `EncodeStable` are distinct from legacy Map and
CandidateMap. A tagged Document can dispatch exact profiles explicitly. A verified
change view is read-only and carries the full original profile and raw digest; it
cannot be passed to a writer that serializes only 0.x fields. Legacy ParseMap,
Canonical and ParseCandidate do not silently widen to stable.

Encoding an edited stable document preserves all validated values, array order,
optional-field absence, reference arrays and witness objects. The proposed stable
encoder uses canonical JSON (UTF-8, sorted object keys, minimal separators,
canonical escaping and decimal integers), followed by one LF. Encoding is not
byte-preserving with respect to arbitrary input formatting. Retain OriginalBytes
separately for mapSha256 and exact sidecar comparison. A read/report route must not
rewrite committed input through the encoder. Mutations create new bytes and a new
binding; dangling links or invalidated references refuse instead of being dropped.
No receipt, capture or plan artifact is parsed and reserialized just to copy it.

## CEM-V1-020: criterion, runner and association records

Criterion binding fields retain the closed public reference structure and add
required `snapshotHeadArtifactSha256`, the SHA256 of retained raw snapshot-head
artifact bytes. The original `snapshotHeadReceiptSha256` remains a distinct native
historical receipt identity; the two digests are not interchangeable. ticketId is
canonical `ticket:<authority>:<queue>:<local>`, <=128 ASCII bytes, with alphanumeric
initial characters and alphanumeric/period/underscore/hyphen continuations; local
is <=64 bytes. acceptanceRevision is a positive canonical decimal string <=16
digits and <=9007199254740991. criterionIndex is an integer 0..255. All digests are
64 lowercase hex digits.

criterionSha256 denotes the exact UTF-8 criterion text digest. acceptanceSha256
denotes SHA256 of the canonical JSON ordered array of all criterion digests,
without LF. The low-level reference verifier cannot recalculate these from opaque
native artifacts: it validates the declared identities, not their native truth.
The binding ID is `criterion:sha256:` + SHA256(canonical JSON of all its members
except id). This stable ID includes every capture/verification/claim/snapshot digest, including
the separate raw snapshot artifact digest; candidate criterion identities do not
change.
IDs and (ticketId, acceptanceRevision, criterionIndex) tuples are unique. Bindings
for one ticket/revision share acceptance, capture, verification, claim, native snapshot identity and raw snapshot artifact
digests. Omitted criteria remain unreferenced.

Runner records retain the exact reference fields and fixed limits from the public
candidate shape: raw receipt SHA256, exact `corvint-test-runner-receipt/0` profile,
raw plan-byte SHA256, sourceGitBinding=NOT_OBSERVED,
executionAuthority=CALLER_OBSERVED, dependencyClosure=NOT_OBSERVED and
authentication=NOT_OBSERVED. Receipt digests are unique. The raw plan SHA256 is
not the native plan struct-serialization identity; neither can substitute for the
other. CALLER_OBSERVED is a declared reference limitation, not proof of execution.

Each link names one unique criterion ID and nonempty unique hunk/evidence/runner
arrays. All resolve within this exact document. Each linked evidence ID occurs in
the basis of at least one linked hunk. Every criterion and runner has a link.
Links declare associations only. They do not prove any test selected or executed,
coverage, mutation kill, criterion satisfaction, Tasks authority or currentness.
Raw receipts retain failures, skips, flaky attempts, interruption, timeout, build,
collection and infrastructure states even when reference integrity succeeds.

## CEM-V1-021: complete declared artifact-role closure

Stable proposes six artifact roles; the first five retain candidate names:

| Reference member | Required artifact kind |
| --- | --- |
| criterion.captureSha256 | tasks-capture |
| criterion.verificationSha256 | tasks-verification |
| criterion.claimTicketSha256 | tasks-claimed-ticket |
| criterion.snapshotHeadArtifactSha256 | tasks-snapshot-head-receipt |
| runner.sha256 | runner-receipt |
| runner.planSha256 | runner-plan |

The sixth role is a deliberate proposed stable addition: the candidate only
retains the snapshot-head digest as a historical reference. Gate A must accept
requiring those raw bytes and the separate raw-artifact digest for stable closure;
candidate fixtures/behavior do not change. The Tasks native identity is not assumed
to hash a raw receipt file. Verifying that the supplied raw snapshot corresponds to
that native identity belongs to the separately admitted Tasks verifier, remains
NOT_OBSERVED here, and needs producer/verifier evidence before integration. Multiple references may share one matching artifact record. Paths and
SHA256 digests are each globally unique across artifact records. The multiset of
required roles reduces to an exact set of (kind,digest) pairs; the declared set
must equal it. No missing, duplicate, wrong-role or unused artifact is allowed.
A digest required with two different roles is a role conflict and refuses.

Paths are 1..512 UTF-8 bytes, relative, with no empty/dot/dot-dot segment,
case-insensitive .git segment, backslash, C0 or DEL. An artifact cannot use the
fixed sidecar path. Use the explicit artifact root only, refuse links in every
component and nonregular leaves, and enforce rooted no-follow reads. At most
4 MiB per artifact and 16 MiB total, under the operation deadline. The map's
artifact SHA256 hashes exact original bytes; no JSON normalization occurs.

After canonical change verification, read all declared artifacts in map order
into bounded private byte copies and verify hashes. Reopen/re-read the full set
through the same rooted discipline; require bytes/digests equal the first pass
and declarations before emitting any complete artifactChecks. Missing/unsupported
reads and exhausted resources are UNSUPPORTED, while observed digest mismatch or
changed bytes are REJECT. Both passes are required even for shared references.
No declared path or artifact is executable. No OS sandbox or immunity to hostile
same-user mutation is implied; observed changes refuse, not a global race-proof claim.

“Complete closure” here means the entire declared outer six-role graph. It does
not mean all transitive native task history, reports, source inventory, tool files
or dependencies inside opaque artifacts have been supplied or verified. Those
obligations stay explicit in the authority/runner layers. A malformed or fabricated
opaque record can have valid referenced bytes. Such a packet cannot pass a native
semantic gate solely because low-level stable reference integrity succeeded.

## CEM-V1-022: result authority axes

OPERATION and RESULT.schema define one closed result. Success is canonical Git
change integrity plus complete declared reference-byte integrity. It is never
Tasks completion, criterion adequacy or release approval. changeIntegrity and
referenceIntegrity are separate; an empty graph produces EMPTY_REFERENCE_SET,
not a claimed observed execution. Structural/witness axes state only the checks
actually performed. All authority, history/currentness, source/execution,
receipt-semantics, authentication, dependency, adequacy/discrimination and external
interoperability axes remain NOT_OBSERVED in this operation, including on success.

Companions may emit separate versioned native verification results bound to the
exact original stable map, expected base, target, artifact inventory and their
own live authority context. A retained caller-authored Tasks verification artifact
is not that check. Historical validity and fresh live applicability are distinct.
Native runner semantics must use the shared runner contract without changing its
bytes/identity, retain every admitted observation and abstain on contradictions.
No boolean from the stable result may replace these layered checks.

## CEM-V1-023..024: operation and committed sidecar

The additive portable operation is `verify-stable` with five explicit options as
specified in OPERATION.json. The first native vertical slice uses an explicit
`corvint cem verify-stable` route with equivalent inputs; generic historical
commands/default output remain unchanged. No --patch, implicit artifact discovery,
execution flag, authority upgrade, version alias or Git-only fallback is admitted.

Authenticate independent full expected-base and target commits, require the map
base equal expected-base, derive the canonical whole-repository patch excluding
only `.corvint/change.cem.json`, and validate exact changed-tree inventory/replay.
Canonical Git recipe and object hash rules are inherited from the corrected public
0.3 packet. Empty-tree identity follows repository object format, never a fixed
SHA1 constant transplanted into SHA256. Generic read failures remain git-read-failed;
canonical diff failures are git-diff-failed; timeout/resource remain distinct.

A base sidecar, if present, must be a regular100644 blob; its profile need not be
stable. A target sidecar, if present, must be regular100644 and byte-identical to
the original stable input. Absence is allowed for preparation, explicitly ABSENT
in results; gate-facing committed-sidecar workflows must require EXACT. Never
normalize before comparison. There is no self-referential target OID in the map.

Prepare content and any committed artifact bytes, compute the map against that
immutable pre-sidecar target, then add the exact sidecar bytes in a new commit and
verify that actual commit. The canonical patch is unchanged by the sidecar-only
commit. Do not rewrite an existing commit to manufacture a match. Committed
artifacts outside the exclusion participate in the content patch normally;
external-root artifact integrity alone does not claim a source Git attestation.
Bundle relocation preserves internal paths and raw bytes under a new explicit root.

## CEM-V1-025..028: migration, admission, qualification and rollback

MIGRATION.json records precise native/portable and all19 consumer-family decisions
plus two later additions. Implement the smallest wire -> canonical/reference verify
-> explicit CLI vertical first, then mutators and dependent consumers. Gate A and
independent review of this contract precede code. No current consumer is promoted
by this draft or by recognizing a string token. A stable writer must own the full
document; a projection-only reader retains the full input digest and all unknowns.
Gate-facing routes must require full artifact verification and their separate
native authority checks, not a convenient Git-only view.

The portable source-build floor remains Go1.24. Structural parser/scanner/gofmt
runtime support is a separate exact-tuple qualification; current go1.27.1 prototype
is experimental. The P0 primary-clean-config-bounded/1 prototype refuses ordinary
clone metadata and linked worktrees; no false native-wide claim is inherited.
Broader repository admission, safe process descendants, supported OS/architectures,
SHA256 and resource ceilings need explicit evidence before support claims.

The literal fixtures in fixtures/manifest.json are independently constructed
proposed conformance inputs and expectations. Their Git objects/digests may be
materialized and checked without a stable implementation. Native stable, portable
stable and CLI observations remain NOT_RUN until implementations exist. They are
Corvint-authored, not independent external adoption or actual runner execution.
All supported runner/platform and negative lifecycle matrices, external consumer
conformance, matched external outcome evaluation and governing release gates
remain mandatory OPEN. The six-role and result-axis decisions need technical
acceptance before implementation. Rollback removes/disables only new opt-in stable
routes, preserves old defaults, candidate/0.x bytes and native Tasks history, and
retains failed evidence. Stable release/default cutover is a separate explicit act.

## R1 deterministic result transitions and fixture transport

OPERATION.json axisTransitions and coreRootPolicy are normative. Early wire failures
leave all five check axes NOT_CHECKED; reference failure retains completed witness
wire validation. No unchecked structural suffix may yield a proven aggregate.
Opaque runner receipt bytes retain FAILED/SKIPPED facts without decoding them into
the low-level result. Decoded runner semantics require the later companion stage.

FIXTURES.pack.json transports the original fixtures losslessly. Paths cited above
refer to its reconstructed tree, never to additional tracked files. See PACKING.md.

## Expanded native repository envelope

The optional experimental native `canonical-repository-bounded/1` admission is
defined in [REPOSITORY-ENVELOPE.md](REPOSITORY-ENVELOPE.md). It supersedes only the
literal-config restriction when that envelope is selected. The original strict
portable envelope and all authority limits remain preserved. Full results and
stage-specific expectations are in `REPOSITORY-ENVELOPE.cells.json`; independent
portable expanded-envelope and Linux execution qualification remain open.
