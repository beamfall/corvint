# Stable CEM 1.0 algorithms

Status: the stable `cem/1.0` wire is implemented and produced on main: the Corvint
producer writes it, and the native verifier (`corvint cem verify-stable`) and the
portable consumer (`interop/cem01-go`) both verify it. The technical contract in
this packet is still proposed. It is not accepted and not promoted to a stable
release, and qualification is bounded (see CEM-V1-025..028 below). Owner scope is
accepted. Apache-2.0. No native AGPL implementation or native test body was copied
into this packet.

`stable.schema.json`, `OPERATION.json` and `RESULT.schema.json` form one contract.
Schema validation alone is insufficient: strict JSON, byte limits, identities,
relations, Git objects and rooted filesystem reads need the algorithms below.
CEM-V1-017..028 were added with this packet; existing CEM-V1-001..016 and the
experimental candidate keep their original meanings. The support tiers and
citation block, the result serialization and verdict digest, and the canonical
diff recipe are stated at the end of this file (CEM-V1-108..110, with the
nested result member order as CEM-V1-113), and `citation-vectors/` pins their
byte-exact outputs (CEM-V1-111).

## CEM-V1-017: exact wire and absent evidence

The exact profile is `cem/1.0`, with ten required root keys, the two optional root
keys `provenance` (CEM-V1-093) and `role` (CEM-V1-095), and no others: `spec`,
`baseRevision`, `patchSha256`, `excludedPath`, `evidence`, `hunks`,
`criterionBindings`, `runnerReceipts`, `criterionLinks`, `artifacts`. This is a
separate closed decoder, not a label substituted into a 0.3 or candidate document.
A missing required key precedes any unknown root key. The optional root members
are validated after every required member and before cross-record reference
validation, `provenance` before `role`.
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

CEM-V1-176 fixes the stage and code where the graph precedence above is easy to
misread. A hunk basis that names no evidence record fails at the wire stage
(`invalid-field`), before any reference check. A repeated hunk ID is
`duplicate-reference`, checked after runner receipt uniqueness and before any
link is resolved. Within each link, every one of its three ID arrays is checked
for repeats before any ID is resolved, so a repeated unknown ID is
`duplicate-reference`, not `unresolved-reference`. A declared
criterion or runner receipt that no link names is `reference-coherence`, checked
after link resolution and basis membership and before artifact role closure.
In the artifact list, a repeated path or digest is `duplicate-reference`, checked
per record before that record's role; a role mismatch or a missing or extra
required artifact is `artifact-role-closure`. All of these, except the wire
failure, are REJECT at the references stage. `reference-precedence/` pins one
case of each for both consumers.

## CEM-V1-018: unchanged change semantics, with full 0.3 vocabulary

The canonical JSON identity, canonical patch, hunk replay, evidence and drift
algorithms and the four structural predicates are restated in cem/1.0 vocabulary
in "CEM-V1-018 restated: 0.1 algorithms" and "CEM-V1-018 restated: 0.3 hunk
vocabulary, structural reasons and witnesses" below; those sections are the
1.0-normative text. The pinned copies `inherited/ALGORITHMS.md` and
`inherited/cem-0.1-algorithms.md` are retained unchanged as historical provenance
for that restatement; where they differ from this file, this file governs. Their
profile/command/candidate exclusions, draft-stage qualification notes and
prototype implementation status do not define this stable operation. Stable
admission and result semantics are exclusively defined here.
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

Discrimination witness bound (CEM-V1-090). The "mutants may exceed killed+survived"
allowance above does not let `mutants` exceed the witness's own `bounds.maxMutants`:
a `discriminates` witness whose `mutants` is greater than `bounds.maxMutants` is
refused at the wire stage with `invalid-field`; `mutants` equal to `maxMutants` is
admitted. A witness whose `survived` differs from the number of `survivors` entries,
or whose survivor `line` lies outside the hunk's `newRange`, is likewise refused
`invalid-field`. Survivors need not be distinct, and no `killed + survived = mutants`
rule applies. These rules reproduce the tamper-matrix verdicts
`discrimination-mutants-over-bound` and `discrimination-survivors-inconsistent`
(wire, `invalid-field`) and the boundary control `discrimination-at-bound` (ACCEPT).

Text bounds are measured in UTF-8 bytes. testRun is 1..256 bytes, operator 1..64,
description 1..512 and detail 0..512; reject C0 and DEL, not all Unicode characters
classified as controls or nonprinting. U+0085/U+2028 remain valid. Structural and
witness checks must retain the corrected public 0.3 corpus, not merely its happy
path. Unknown/survived/uncovered claims can coexist with valid change integrity.

## CEM-V1-018 restated: 0.1 algorithms

This section restates, for the stable `cem/1.0` map, the pure algorithms first
published as `inherited/cem-0.1-algorithms.md`. The requirement IDs of the 0.1
and 0.2 specifications remain their historical identities; the text below is the
1.0-normative statement of the same behaviour. Nothing here admits a 0.x profile.

1. Strict JSON. As CEM-V1-017: at most 4 MiB, strict UTF-8, one object, no
   duplicate keys, integers 0..9007199254740991 in canonical decimal syntax.
2. Paths. A repository path is 1..512 UTF-8 bytes, relative POSIX, with no
   leading `/`, backslash, NUL or other control byte, and no empty, `.` or `..`
   segment. Paths are never case-folded or Unicode-normalized.
3. Canonical identity JSON. Identity inputs are serialized with object keys in
   byte order, no insignificant whitespace, `"` and `\` escaped, C0 controls as
   `\b`, `\f`, `\n`, `\r`, `\t` or lowercase `\u00XX`, all other code points as
   raw UTF-8, and integers in canonical decimal. The digest is lowercase SHA-256
   over those bytes.
4. Evidence identity. `id` is `evidence:sha256:` followed by the digest of the
   canonical object `{"blobOid","path","span":{"end","start"},"spanSha256"}`.
5. Hunk identity. `id` is `hunk:sha256:` followed by the digest of the canonical
   object `{"contentSha256","newPath","newRange":{"count","start"},"oldPath",
   "oldRange":{"count","start"}}`.
6. Canonical patch profile. `patchSha256` is the SHA-256 of the canonical patch
   bytes produced by the CEM-V1-023 diff recipe: at most 8 MiB and at most
   262,144 LF-terminated records. The parser is a `diff --git` state machine.
   File modes are 100644 or 100755 only; an `old mode`/`new mode` change is
   rejected; a created file needs `new file mode` and an old range of `0,0`; a
   deleted file needs `deleted file mode` and a new range of `0,0`. Hunk headers
   follow `@@ -start[,count] +start[,count] @@`, and `\ No newline at end of file`
   applies only to the immediately preceding record.
7. Base simulation. `baseRevision` resolves to a commit whose full lowercase OID
   equals the supplied value byte for byte. Hunks are grouped by
   `(oldPath,newPath)` in patch order and replayed against base blobs read from
   Git objects only, never from the working tree. A create needs the destination
   absent; a delete needs the old path with a matching mode and yields empty
   bytes; a rename needs the old path and an absent destination. Every context
   and removed record is compared byte for byte and every cursor is checked. The
   patch is simulated, never executed.
8. Evidence. An evidence record names a regular blob (mode 100644 or 100755,
   never a symlink or gitlink; 40- or 64-hex OID) at `baseRevision`, a nonempty half-open byte span `[start,end)` within it, and the
   SHA-256 of those span bytes. At most 64 MiB is read per blob and 128 MiB in
   total. Every evidence record is referenced by at least one basis.
9. Dispositions. A `supported` hunk carries reason `evidence-backed` and 1..32
   bases; each basis names one evidence ID and one relation from `specification`,
   `decision`, `test-claim`, `implementation`, `call-site`, `dependency`,
   `incident`. An `unknown` hunk carries an empty basis and one of
   `no-evidence`, `insufficient-evidence`, `conflicting-evidence`. A `mechanical`
   hunk carries an empty basis and one of the mechanical reasons below.
10. Text mechanical reasons. `line-ending-only` holds when replacing CRLF with LF
    in both images yields equal bytes. `whitespace-only` holds when both images
    have equal LF counts and each record is equal after deleting the bytes
    0x09, 0x0b, 0x0c, 0x0d and 0x20.
11. Limits. 4096 evidence records; 2048 hunks; 32 bases per hunk; a reason of at
    most 64 bytes; at most 1024 Git operations, 10 seconds per operation and 30
    minutes in total for one verification.
12. Drift. Each evidence record is checked at its literal same path in the
    target, never following a rename. An identical blob OID is `stable`. A missing
    path, or an entry that is not a 100644/100755 blob, is `deleted`. Otherwise
    the exact base span bytes are searched in the target blob, counting
    overlapping matches: zero is `stale`, one is `relocated` with that span, two
    or more is `ambiguous`. There is no normalization or fuzzy matching. The drift
    row keeps `baseBlobOid`; the target blob and span are null when absent. Rows
    are ordered by evidence ID. Each record costs one target path lookup, plus
    one target blob read only for a 100644/100755 blob whose OID differs; the
    searched bytes are the span the evidence stage authenticated, and no base
    blob is read again. When the operation limit refuses an operation here, the
    result keeps exactly the rows completed before it (CEM-V1-260).

## CEM-V1-018 restated: 0.3 hunk vocabulary, structural reasons and witnesses

This section restates, for the stable map, the 0.3 additions first published as
`inherited/ALGORITHMS.md` and specified as CEM-SM-001..010 in
`docs/specs/cem-0.3-structural-mechanical.md`. Those IDs remain the historical
requirement identities and are cited here as references; the text below is the
1.0-normative statement.

The stable mechanical reasons are `whitespace-only`, `line-ending-only`, `rename`,
`move`, `import-reorder` and `formatter-only` (CEM-SM-001). The four structural
reasons are recomputed by the consumer from Git; the map carries only the reason
token, never a structural witness.

1. Inputs (CEM-SM-002). A structural proof uses only the base blob at
   `baseRevision` and the parsed canonical patch. A missing or non-regular base,
   a created or deleted file, or a hunk whose replayed image equals its base
   refuses the structural claim.
2. Scope (CEM-SM-003, CEM-SM-004). `rename`, `import-reorder` and
   `formatter-only` apply the single hunk in isolation to the base. `move`
   inspects the whole file group. Go syntax is parsed irrespective of the file
   extension.
3. Failure (CEM-SM-005). A parse, scan or format failure makes the claim
   `unproven-mechanical`. No fallback to a weaker predicate exists.
4. Rename (CEM-SM-007, decision 0521). Exactly one identifier pair `from`->`to`
   differs; comments equal their whole-word substitution; both names are
   unexported; `to` is absent from the base and `from` is declared in the file
   (local receiver names and statement labels included). The pair is not a
   selector, composite-literal key, struct field, interface or concrete method or
   package name. `//go:`, `//line ` and `//export ` directives are unchanged.
   Rename is file-local and can leave sibling-file references uncompilable.
5. Move (CEM-SM-008). Tokens outside top-level declarations are unchanged; the
   top-level declarations form the same multiset in a different order; `var`
   declarations and `init` functions keep their relative order.
6. Import reorder (CEM-SM-009). The ordered import specs differ; the multiset of
   name/path pairs and all comments are unchanged; tokens outside the import
   declarations are unchanged.
7. Formatter only (CEM-SM-010). `go/format` renders base and target images to
   identical bytes.
8. Runtime gate. A structural reason is checked only on a sealed structural
   runtime tuple (CEM-V1-050; currently `go1.27.2`); otherwise the consumer
   reports `unsupported-structural-runtime`.

None of these predicates proves package semantics, build success, test adequacy or
behavioural equivalence.

Witnesses. Hunk `coverage` and `discriminates` are optional 0.3 witness objects
carried unchanged from the source document. Coverage has five keys; its ranges
are ascending, nonoverlapping, nonadjacent and inside `newRange`. Discriminates
has nine keys; `killed + survived <= mutants`; state `discriminates` needs
killed >= 1 and survived = 0, `survived` needs survived >= 1, `not-run` needs
mutants = 0; the survivors array length equals `survived`. Text bounds exclude
only C0 and DEL. Result witness copies preserve absence. A consumer never
synthesizes a witness that the source did not carry.

Nesting depth follows CEM-V1-017. Git diff failures report `git-diff-failed`
and Git object read failures report `git-read-failed`.

### 0.3 schema fragments relied on

`stable.schema.json` restates, as identical JSON values, these 13
`$defs` from `inherited/cem-0.3.schema.json`: `basis`, `bounds`, `coverage`,
`coveredRange`, `discriminates`, `evidence`, `gitOid`, `hunk`, `path`, `range`,
`sha256`, `span`, `survivor`. The stable schema adds the reference-record
definitions (`criterionBinding`, `plainCriterionBinding`, `tasksCriterionBinding`,
`externalCriterionBinding`, `criterionId`, `runnerReceipt`, `criterionLink`, `artifact`, `artifactPath`) and
the CEM-V1-093/095 definitions `sha1Oid`, `provenance` and `role`; it does not
reference the inherited schema file at runtime.

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

CEM-V1-083 gives a criterion binding two closed forms. The plain form is a Git
blob span criterion with exactly `id`, `criterionIndex`, `criterionSha256` and
`evidenceId`. The Corvint Tasks fields (`ticketId`, `acceptanceRevision`,
`acceptanceSha256`, `captureSha256`, `verificationSha256`, `claimTicketSha256`,
`snapshotHeadReceiptSha256`, `snapshotHeadArtifactSha256`) are one optional
closed extension. A consumer selects the form by key presence before any value
check: if any Tasks member is present, the record must be the complete Tasks form
below and `evidenceId` is an unknown member; otherwise it must be the complete
plain form. Missing members of the selected form refuse `missing-field` (checked
first), other members `unknown-field`, and bad values `invalid-field`. A plain
`evidenceId` is `evidence:sha256:` plus 64 lowercase hex digits; it must resolve
to an evidence record of this document (`unresolved-reference`) and
`criterionSha256` must equal that record's `spanSha256` (`reference-coherence`),
so the criterion is the exact base blob span that canonical evidence
verification reads from Git. A plain `evidenceId` occurs in at most one binding
(`duplicate-reference`). Plain bindings form one unnamed group whose
criterionIndex values are unique; no Tasks tuple can collide with it. The plain
ID is computed exactly as below over its three non-id members. Plain bindings
require no Tasks artifact role and no `.taskman/` store; their links, runner
records and runner artifact roles are unchanged.

Tasks-form criterion binding fields retain the closed public reference structure and add
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
candidate shape: raw receipt SHA256, a profile of exactly `corvint-test-runner-receipt/0`
or `opaque/0` (receipt labels below), raw plan-byte SHA256, sourceGitBinding=NOT_OBSERVED,
executionAuthority=CALLER_OBSERVED, dependencyClosure=NOT_OBSERVED and an
authentication member (CEM-V1-084 below). Receipt digests are unique. The raw plan SHA256 is
not the native plan struct-serialization identity; neither can substitute for the
other. CALLER_OBSERVED is a declared reference limitation, not proof of execution.

Receipt labels (CEM-V1-310): a producer labels a runner record `corvint-test-runner-receipt/0`
only when it decoded and admitted the receipt bytes as that runner document, and
`opaque/0` when it checked them only by SHA256. A reader accepts both labels with
the same fixed limits and checks; neither label is verified by the reader, and
neither raises any result axis. Maps written before `opaque/0` existed keep their
label and stay valid.

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

The four criterion rows apply only to Tasks-form bindings (CEM-V1-083); a
plain-form binding contributes no required artifact role.
CEM-V1-084 adds a conditional seventh role: `runner.authentication.envelopeSha256`
requires a `receipt-attestation` artifact, only for a `dsse-ed25519` receipt.

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
interoperability axes remain NOT_OBSERVED in this operation, including on success,
except that CEM-V1-084 lets a consumer trusted-keys file move `authentication`.

Companions may emit separate versioned native verification results bound to the
exact original stable map, expected base, target, artifact inventory and their
own live authority context. A retained caller-authored Tasks verification artifact
is not that check. Historical validity and fresh live applicability are distinct.
Native runner semantics must use the shared runner contract without changing its
bytes/identity, retain every admitted observation and abstain on contradictions.
No boolean from the stable result may replace these layered checks.

## CEM-V1-084: runner receipt authentication

A runner record's `authentication` is one of three closed forms: the sealed
legacy string `"NOT_OBSERVED"` (read as method none and echoed unchanged),
`{"method":"none"}`, or `{"method":"dsse-ed25519","keyid":K,"envelopeSha256":E}`.
The method value selects the key set before any value check, so a consumer
refuses `missing-field` first, then `unknown-field`, then `invalid-field`
(an undefined method or a value that is not 64 lowercase hex). K is the
lowercase hex SHA-256 of the raw 32-byte Ed25519 public key. E is the SHA-256
of the exact envelope bytes.

Graph: after the receipt and plan roles, a repeated E across receipts refuses
`duplicate-reference`; E must resolve to a `receipt-attestation` artifact
(`artifact-role-closure`), and a `receipt-attestation` artifact that no receipt
names is unused (`artifact-role-closure`). A map that carries a key file as an
attestation artifact therefore refuses: a map never supplies its own trust.

Envelope: one closed DSSE object `{payload, payloadType, signatures}` with
`payloadType` `application/vnd.in-toto+json`, standard padded base64 payload and
exactly one signature `{keyid, sig}` (sig 64 bytes). The payload is a closed
in-toto Statement v1: `_type` `https://in-toto.io/Statement/v1`, `predicateType`
`https://corvint-context.dev/attestation/cem-receipt/v1`, `predicate` `{}`, and
exactly three subjects in order: `runner-receipt` `{"sha256":R}`, `runner-plan`
`{"sha256":P}` and `target-tree` `{"gitTree":T}`. The signature covers the DSSE
v1 PAE `DSSEv1 28 application/vnd.in-toto+json <len> <payload>`.

After the two artifact passes, for every dsse receipt a consumer checks, in
order: the envelope shape (`attestation-malformed`), signature keyid equal to K
(`attestation-key-mismatch`), R, P and T equal to the receipt digest, the plan
digest and the target commit's tree (`attestation-subject-mismatch`, checked
without any trust decision), and, only when K is trusted, the Ed25519 signature
(`attestation-signature-invalid`). Any of these is a REJECT at stage
`artifacts`, exit 1, with the code in `issueCodes`, `referenceIntegrity` FAILED
and `authentication` FAILED.

Trust comes only from the consumer: `--trusted-keys FILE`, an absolute regular
file of at most 64 KiB holding the closed
`{"profile":"corvint-trusted-keys/1","keys":[{"keyid":K,"ed25519PublicKey":B64}]}`
with 1..64 unique keys whose keyid is the digest of the key. Any other file is
`invalid-arguments` at stage `arguments`, exit 2, before the map is read. On
ACCEPT the `authentication` axis is NOT_OBSERVED without a trusted-keys file;
otherwise UNTRUSTED_KEY when any dsse receipt names a keyid the file does not
list (its signature is then not checked), NOT_OBSERVED when no receipt is dsse
or any receipt is unsigned, and VERIFIED only when every receipt is dsse with a
trusted keyid and a valid signature. A signature never
raises `executionAtCommit` or any other axis. Producing a signed receipt is an
operator step outside this operation.

## CEM-V1-093: map provenance

`provenance` is an optional closed object recording that a stable map was
converted from a 0.x map. A native stable map omits it. It has exactly five
members, all required, validated in this order:

1. `sourceSpec`: one of `cem/0.1`, `cem/0.2`, `cem/0.3`.
2. `sourceBlobOid`: the 40-hex lowercase Git blob ID of the source map.
3. `sourceSha256`: the 64-hex lowercase SHA-256 of the source map bytes.
4. `bindCommit`: the 40-hex lowercase commit that bound the source map.
5. `converter`: matching `^corvint-cem-convert/[0-9]+(\.[0-9]+)*$`.

A non-object value is `invalid-field`. Within the object a missing member is
`missing-field`, then an unknown member is `unknown-field`, then an invalid value
is `invalid-field`. Provenance is never read for authority, ranking, any result
axis or any verdict, and the result never echoes it. It contributes to the map
bytes and therefore to `mapSha256` only.

## CEM-V1-094: Producer-only members

The closed list of producer-only stable map members is:

1. `criterionBindings`
2. `runnerReceipts` (including any runner `authentication` object, which exists
   only inside a receipt)
3. `criterionLinks`
4. `artifacts` (including receipt attestation records)

A producer that holds no task or runner evidence, including the 0.x converter,
emits each of these as an empty array. A map whose four producer-only members are
all empty is valid: both consumers ACCEPT it with `referenceIntegrity`
`EMPTY_REFERENCE_SET` and every non-core axis `NOT_OBSERVED`. There is no "intent
and scope" member and no "structural witness" member: intent stays in the
project's specs and tickets, and structural mechanical reasons are recomputed
from Git (CEM-V1-018). Hunk `coverage` and `discriminates` are not producer-only
members; they are optional source witnesses carried unchanged when present and
omitted when absent.

## CEM-V1-095: role

`role` is an optional closed string enum naming why the change exists:
`change` (the meaning when `role` is absent), `bind-chore` or `seal-chore`. Any
other string, or a non-string, is `invalid-field`. Role never changes a verdict,
tier or result axis, and the result never echoes it. Grouping by role in readers
is a separate reader contract.

## CEM-V1-023..024: operation and committed sidecar

The additive portable operation is `verify-stable` with five explicit options as
specified in OPERATION.json. The first native vertical slice uses an explicit
`corvint cem verify-stable` route with equivalent inputs; generic historical
commands/default output remain unchanged. No --patch, implicit artifact discovery,
execution flag, authority upgrade, version alias or Git-only fallback is admitted.

Authenticate independent full expected-base and target commits, require the map
base equal expected-base, derive the canonical whole-repository patch excluding
only `.corvint/change.cem.json`, and validate exact changed-tree inventory/replay.
The canonical Git recipe is stated in full under "CEM-V1-110: canonical diff
recipe" below. Empty-tree identity follows repository object format, never a fixed
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

MIGRATION.json records the native/portable and all 19 consumer-family decisions
plus two later additions, in the order they were planned: the wire ->
canonical/reference verify -> explicit CLI vertical first, then mutators and
dependent consumers. That order has been carried out on main; every consumer now
reads `cem/1.0` only. Technical acceptance of this contract and independent
review remain open. No consumer is promoted to a stable release by this packet or
by recognizing a string token. A stable writer must own the full
document; a projection-only reader retains the full input digest and all unknowns.
Gate-facing routes must require full artifact verification and their separate
native authority checks, not a convenient Git-only view.

The portable source-build floor remains Go1.24. Structural parser/scanner/gofmt
runtime support is a separate exact-tuple qualification. The verifiers admit
exactly go1.27.1 (the prototype tuple), go1.27.2 (decision 0522) and go1.27.3
(decision 0547), each later tuple sealed in `../structural-tuples/<tuple>`. They
report each as `QUALIFIED_EXACT_TUPLE`: the structural corpus in
`../structural-corpus` qualifies go1.27.1, and the later tuples share its
parser-closure digest (decision 0551). They refuse every other release with
`unsupported-structural-runtime`. The P0 primary-clean-config-bounded/1 prototype refuses ordinary
clone metadata and linked worktrees; no false native-wide claim is inherited.
Broader repository admission, safe process descendants, supported OS/architectures,
SHA256 and resource ceilings need explicit evidence before support claims.

The literal fixtures in fixtures/manifest.json are Corvint-constructed
conformance inputs and expectations from before the implementation existed. The
executed stable suites are `tamper-matrix/`, `citation-vectors/`,
`neutral-criterion/`, `receipt-authentication/` and `repository-envelope-packet/`:
the native verifier and the portable consumer both run them with pinned verdicts.
`R1-EXPECTED.json` is a declarative expectation set that no test reads or
executes; it is kept as construction history and is not a conformance claim.
All of this evidence is Corvint-authored, not independent external adoption or
actual runner execution.
All supported runner/platform and negative lifecycle matrices, external consumer
conformance, matched external outcome evaluation and governing release gates
remain mandatory OPEN. The six-role and result-axis decisions are implemented
and still await technical acceptance. Rollback removes/disables only new opt-in stable
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

## CEM-V1-108: support tiers and the citation block

An ACCEPT result carries `citation`, a block with profile
`cem-citation-support/0` that the consumer derives from the verified map, the
parsed canonical patch and the bound evidence span bytes. A REJECT carries
`citation: null`. No tier, count or anchor ever changes `outcome`, `stage`,
`code`, `issueCodes` or `drift`. Everything below is byte arithmetic on ASCII:
there is no case folding except where stated, no Unicode normalization and no
locale.

Inputs.

- Hunk order. The map's hunks are sorted by `id`, comparing the strings as
  bytes. Every list and every shifted position below uses this order.
- Hunk path. Each map hunk is matched to the canonical patch hunk with the same
  `id`. Its path is that patch file section's post-image path, or its pre-image
  path when the section has no post-image path (a whole-file deletion). For a
  deleted file the path is therefore the path the file had at the base.
- Added bytes. Concatenate, in patch order, the payload of every `+` body line of
  the hunk. A payload is the line after its one-byte `+` prefix, including its
  terminating LF. The LF is kept even when a `\ No newline at end of file` marker
  follows the line. Removed (`-`) and context (` `) lines contribute nothing.
- Span bytes. For each evidence record, the bytes `[start, end)` of its base blob,
  exactly as bound by `spanSha256`. A span may begin or end in the middle of a
  line. All rules below read only these bytes, never the rest of the blob.

Requirement ID tokens. Scan the added bytes left to right. A candidate starts at
a byte `A`..`Z` whose preceding byte, if any, is not a boundary byte; the
boundary bytes are `A`..`Z`, `a`..`z`, `0`..`9`, `_` and `-`. At a candidate,
match the pattern `[A-Z][A-Z0-9-]{2,31}-[0-9]{3}` anchored at that byte, with
the greedy, leftmost-first backtracking semantics of Perl, RE2 or Go `regexp`.
If there is no match, or the byte after the match is a boundary byte, discard
the candidate and continue at the next byte; a shorter match is not retried.
Otherwise record the occurrence and continue after its last byte, so
occurrences never overlap. The hunk's IDs are the distinct occurrence strings.

Defining lines. A span defines ID `X` when some line of the span, after removing
leading spaces and tabs, begins with the three bytes ``- ` `` (hyphen, space, backtick) followed by `X`, a
backtick and a colon, with `X` non-empty and containing no backtick. Lines are
the pieces of the span split on LF, so the first line starts at the first span
byte even when the span begins mid-line, and the last line may lack an LF. A
reader that looks at the whole blob line around the span instead of the span
bytes gives different answers when the span starts mid-line.

Anchors. A hunk with basis evidence IDs `E` has:

- `id` when at least one of its IDs is defined by the span of at least one
  member of `E`;
- `path` when its path is non-empty and the span of at least one member of `E`
  contains the full path as a byte substring, or contains the path's last
  `/`-separated component when that component is at least 6 bytes long.

`anchors` lists the kinds found, `id` before `path`; it is `[]` when none is
found.

Tiers.

1. `mechanical-proved` when the hunk's disposition is `mechanical`.
2. `unknown` when the hunk has no basis.
3. Otherwise the hunk is supported: `anchored` when its own basis set gives at
   least one anchor, else `cited`.

`held` is a reserved count that is always 0. Mechanical and unknown hunks always
have `anchors: []`.

Annotation. A hunk's `annotation` is true when it has at least one `+` line and
every token of its added bytes is neutral. To find the tokens, replace each
requirement ID occurrence with spaces, then take every maximal run of
`A`..`Z`, `a`..`z`, `0`..`9` and `_`. A token is neutral when it is shorter than
3 bytes, or when its ASCII-lowercased bytes are one of these 39 stop terms:

```text
also been build change changed changes claim dist docs document
documentation evidence generated have hunk must only packages report
reports requirement requirements shall should sidecar sources supported
testing tests that their there these they those vendor vendored which
will
```

The list is the `lexical-relevance-floor-v0` stop list without the 21 terms that
are a keyword, reserved word, predeclared identifier or builtin in a common
programming language (`error`, `false`, `from`, `include`, `into`, `package`,
`print`, `return`, `self`, `source`, `string`, `test`, `then`, `this`, `true`,
`unknown`, `value`, `when`, `where`, `while`, `with`), so a short code line such
as `return value - 1` is not an annotation.

Annotation is computed for every hunk, whatever its tier.

Counts.

- `hunksByClass`: the number of hunks in each tier.
- `annotationHunks`: the number of hunks with `annotation: true`.
- `evidenceFanOut`: for each evidence ID in the basis of at least one supported
  hunk, the number of distinct supported hunks whose basis names it. Each entry
  is the two-element array `[evidenceId, count]`. Entries are sorted by count,
  highest first, then by evidence ID in byte order.
- `distinctEvidence`: the number of `evidenceFanOut` entries.
- `maxFanOut`: the largest count, or 0 when there are no entries.
- `samePathCitations`: over the supported hunks, the number of basis entries
  (each `(evidenceId, relation)` pair, not each distinct evidence ID) whose
  evidence record `path` equals the hunk path. A deletion hunk compares against
  its base path.
- `concentratedHunks`: 0 when `maxFanOut` is below 2; otherwise the number of
  supported hunks for which every distinct basis evidence ID has a fan-out count
  equal to `maxFanOut`. Concentration needs an evidence record that two hunks
  share, so a map whose hunks all cite distinct evidence reports 0.

Shift null. Let `S` be the supported hunks in hunk order and `B(h)` the sorted
set of distinct basis evidence IDs of `h`. When `S` has fewer than 2 members,
`invariant` is true and `specificity` is 0. Otherwise:

- `invariant` is true when every `B(h)` equals `B(S[0])`, and false otherwise.
- The shifted anchor count is the number of `i` in `0..|S|-1` for which hunk
  `S[i]` (its own IDs and path) has at least one anchor against the basis set
  `B(S[i-1])`. For `i = 0` use the last member's set, `B(S[|S|-1])`.
- `specificity` is `hunksByClass.anchored` minus the shifted anchor count. It can
  be negative.

## CEM-V1-109: result serialization and the verdict digest

The citation block. Its members appear in this order: `profile`, `hunks`,
`hunksByClass`, `annotationHunks`, `distinctEvidence`, `evidenceFanOut`,
`maxFanOut`, `samePathCitations`, `concentratedHunks`, `invariant`,
`specificity`. Each `hunks` row has the members `hunkId`, `tier`, `anchors`,
`annotation`, in that order. `hunksByClass` has `held`, `mechanical-proved`,
`anchored`, `cited`, `unknown`, in that order. Empty lists are `[]`, never
`null`. Integers are canonical decimal and booleans are `true` and `false`. The
block is written compactly: `,` between members and elements and `:` after each
member name, with no spaces or line breaks. Every string in the block is an
identifier or a fixed token, so no escape occurs in it. `citation-vectors/`
pins these bytes exactly.

The result object. `verify-stable` writes one JSON object followed by one LF to
standard output, compact as above. The top-level members appear in this order
on every outcome: `profile`, `spec`, `verificationMode`, `outcome`, `accept`,
`stage`, `code`, `issueCodes`, `assurance`, `expectedBase`, `targetRevision`,
`mapSha256`, `patchSha256`, `sidecar`, `drift`, `hunks`, `references`,
`artifactChecks`, `citation`, `axes`, `runtime`, `repositoryEnvelope`,
`verdictSha256`. Strings, integers, `true`, `false` and `null` are written as
in the canonical JSON of the verdict digest below, with one difference: `<`,
`>` and `&` are written as `\u003c`, `\u003e` and `\u0026`, and U+2028 and
U+2029 as `\u2028` and `\u2029`. So `/`, U+007F and every other code point
at or above U+0080 are raw UTF-8. The member order inside
nested objects is fixed too (CEM-V1-113, below), so the whole line is
reproducible byte for byte.

The nested member order (CEM-V1-113). Arrays keep their element order. Object
members appear in this order:

- each `drift` entry: `evidenceId`, `path`, `baseBlobOid`, `status`,
  `targetBlobOid` (a string or `null`), `targetSpan` (`null`, or an object
  with `end` first and then `start`);
- each `hunks` entry: in every object of the entry, at any depth, the members
  are sorted by name, comparing the names as bytes, as in the canonical JSON
  below. For a hunk this is `basis`, `coverage`, `discriminates`,
  `disposition`, `id`, `newRange`, `oldRange`, `path`, `reason`, where
  `coverage` and `discriminates` appear only when the map carries them. A range
  is `count`, `start`, and a `basis` entry is `evidenceId`, `relation`. The
  witness objects and their `covered`, `bounds` and `survivors` contents are
  sorted the same way. A member's value is the map's value, re-encoded
  compactly, and an empty array is `[]`;
- `references`: `artifacts`, `criterionBindings`, `criterionLinks`,
  `runnerReceipts`. An `artifacts` entry is `kind`, `path`, `sha256`. A
  `criterionBindings` entry is `id`, `ticketId`, `acceptanceRevision`,
  `criterionIndex`, `acceptanceSha256`, `criterionSha256`, `captureSha256`,
  `verificationSha256`, `claimTicketSha256`, `snapshotHeadReceiptSha256`,
  `snapshotHeadArtifactSha256`, `evidenceId`. Every member except `id`,
  `criterionIndex` and `criterionSha256` is left out when the map has no value
  for it, so the Tasks form carries the eight Tasks members and the plain form
  carries `evidenceId`. A `criterionLinks` entry is `criterionId`, `hunkIds`,
  `evidenceIds`, `runnerReceiptSha256s`. A `runnerReceipts` entry is `sha256`,
  `profile`, `planSha256`, `sourceGitBinding`, `executionAuthority`,
  `dependencyClosure`, `authentication`. The `authentication` value is the
  string `"NOT_OBSERVED"` for the legacy form, the object `method` for
  `{"method":"none"}`, or the object `envelopeSha256`, `keyid`, `method` for
  `dsse-ed25519` (CEM-V1-084);
- each `artifactChecks` entry: `kind`, `path`, `sha256`, `byteLength`;
- `axes`: sorted by name, comparing the names as bytes;
- `runtime`: `goVersion`, `structuralQualification`;
- `citation`: as stated above.

`citation-vectors/results/` and `neutral-criterion/results/accept.json` pin
whole result lines: every citation vector case, and the neutral criterion
ACCEPT with its references and artifact checks. Each file is exactly the line
a consumer writes, including the final LF. Only `runtime.goVersion` names the
toolchain that runs the consumer, and the pins carry `go1.27.2`. To compare
under any other admitted toolchain, substitute its name there first.

The verdict digest. `verdictSha256` is the lowercase hex SHA-256 of the
canonical JSON of the result object with the members `runtime`,
`repositoryEnvelope` and `verdictSha256` removed. Canonical JSON here is:

- object members sorted by name, comparing the names as bytes;
- no whitespace, `,` between members and elements, `:` after names;
- strings with `"` and `\` escaped as `\"` and `\\`, the controls U+0008,
  U+000C, U+000A, U+000D and U+0009 as `\b`, `\f`, `\n`, `\r` and `\t`, every
  other code point below U+0020 as `\u00XX` with lowercase hex, and every other
  code point, including `<`, `>` and `&`, as raw UTF-8;
- integers in canonical decimal (the result carries no other numbers);
- `true`, `false` and `null` as written.

Because the members are sorted, the digest does not depend on the member order
of the written line. The digest is computed for every outcome, ACCEPT or REJECT.

## CEM-V1-110: canonical diff recipe

The canonical patch is the standard output of this Git invocation, run in the
repository under test, where `BASE` and `TARGET` are the full commit object IDs
and `EMPTY` is the empty-tree ID of the repository's object format
(`4b825dc642cb6eb9a060e54bf8d69288fbee4904` for SHA-1,
`6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321` for
SHA-256), and `GITDIR` is the repository's resolved absolute Git directory:

```sh
git --no-optional-locks --git-dir=GITDIR \
  -c core.fsmonitor=false -c core.untrackedCache=false \
  -c core.attributesFile=/dev/null -c credential.helper= \
  -c core.quotePath=false -c diff.suppressBlankEmpty=false \
  -c diff.orderFile=/dev/null -c diff.noprefix=false \
  -c diff.mnemonicPrefix=false -c diff.renames=false \
  -c diff.algorithm=myers -c diff.wsErrorHighlight=none \
  -c diff.srcPrefix=a/ -c diff.dstPrefix=b/ -c diff.external= \
  -c diff.ignoreSubmodules=none -c advice.graftFileDeprecated=false \
  --attr-source=EMPTY diff --full-index --no-color --no-ext-diff \
  --no-textconv --no-renames --no-indent-heuristic --diff-algorithm=myers \
  --unified=3 --inter-hunk-context=0 --src-prefix=a/ --dst-prefix=b/ \
  --ignore-submodules=none --end-of-options BASE TARGET \
  -- . ':(exclude).corvint/change.cem.json'
```

The environment is cleared except for `PATH`, `SystemRoot`, `USERPROFILE` and
the temporary directory variables `TMPDIR`, `TEMP` and `TMP`, and then set to: `LANG=C`, `LC_ALL=C`,
`GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=/dev/null`,
`GIT_CONFIG_SYSTEM=/dev/null`, `GIT_TERMINAL_PROMPT=0`, `GIT_OPTIONAL_LOCKS=0`,
`GIT_NO_LAZY_FETCH=1`, `GIT_NO_REPLACE_OBJECTS=1`, `GIT_GRAFT_FILE=/dev/null`,
`GIT_ASKPASS=`, `GIT_ATTR_NOSYSTEM=1` and `GCM_INTERACTIVE=never`. A consumer
may add settings that only refuse network transports or hooks (the native
verifier adds `GIT_ALLOW_PROTOCOL=` and `core.hooksPath=/dev/null`); none of
them changes the patch bytes. A repository whose `info/attributes` file has an
effective attribute line is refused with the public code
`unsupported-repository-envelope` rather than diffed, because no option above can neutralize that file.

The output is accepted as the canonical patch only after it is checked against
the self-hashed base and target trees and blobs. Its file sections must match
the changed entries one to one and in order, each header line must name the
verified modes and object IDs, a binary section is pinned by its object IDs,
and replaying each textual section against the verified base bytes must give
the verified target bytes. Any other output is refused. The patch is then
parsed as in item 6 of "CEM-V1-018 restated: 0.1 algorithms", and
`patchSha256` is the SHA-256 of its bytes. Limit: the patch bytes are defined by this Git
invocation. This packet does not restate Git's Myers diff, so a consumer that
does not run Git has to reproduce Git's output for these options byte for byte.

## CEM-V1-111: citation vectors

`citation-vectors/` holds a repository recipe, one clean map, seven refused
copies of it and `EXPECTED.json` (profile `cem-stable-citation-vectors/1`).
The expected values were produced by the native verifier and are asserted by
both consumers. The README lists the misreadings each case catches.

## CEM-V1-330..336: external criterion

A third closed criterion form binds a criterion kept in a separate Git
repository. It has exactly `id`, `criterionIndex`, `criterionSha256`,
`externalRepository`, `externalBlobOid` and `externalSpan`. Form selection is
by key presence before any value check: any Tasks member selects the Tasks
form; otherwise any of `externalRepository`, `externalBlobOid` or
`externalSpan` selects the external form; otherwise the plain form applies.
`evidenceId` is then an unknown member. Error order is unchanged.

`externalRepository` is a consumer-side name matching
`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`. It is not a path or URL; the consumer
maps it to a local repository root. `externalBlobOid` is a Git object ID, 40
or 64 lowercase hex digits. `externalSpan` is the closed `span` object
`{start, end}` with `0 <= start < end`. `criterionSha256` is the SHA-256 of
bytes `[start, end)` of that blob.

The ID is `criterion:sha256:` + SHA256(canonical JSON of the five non-id
members), with `externalSpan` written as `{"end":N,"start":M}`. External and
plain criteria share the unnamed group, so `criterionIndex` is unique across
both. The tuple (externalRepository, externalBlobOid, start, end) occurs at most
once (`duplicate-reference`). An external criterion needs no evidence record and
no Tasks artifact role. It is linked like any other criterion.

Verification runs after the declared artifacts and receipt authentication, in
map order. For each external criterion:

1. If the consumer did not supply the named repository, refuse
   `artifact-unavailable` at stage `artifacts`. The outcome is `UNSUPPORTED`
   with `accept: null`, and the criterion stays NOT_OBSERVED.
2. Admit the root under the same envelope as the verified repository. Read the
   blob. Any admission or read failure is also `artifact-unavailable`.
   Cancellation is `verification-timeout`. The native consumer reports an
   unobserved process cleanup as `unsupported-process-containment`.
3. If `end` exceeds the blob length, or the span digest differs from
   `criterionSha256`, reject with `artifact-digest-mismatch`. The issue code
   goes in `issueCodes` and `referenceIntegrity` is `FAILED`.

Only when every external span matches can the verification reach `ACCEPT`.
The result profile is unchanged. The criterion is echoed in `references` as
declared.

An older `cem/1.0` consumer selects the plain form for an external record and
refuses `missing-field` (`evidenceId`) at stage `wire`. It never accepts.
