# Candidate algorithms — cem/1.0-experimental.1

Status: experimental candidate, not stable CEM 1.0. License: Apache-2.0.

## Admission and inherited change binding (CEM-V1-001)

Admit exactly `cem/1.0-experimental.1`. In particular `cem/1.0`, unknown candidate
revisions, and historical profiles are not this interface. Never change a candidate
`spec` into 0.x to obtain verification. The legacy interfaces remain unchanged.

The root has exactly ten required fields: `spec`, `baseRevision`, `patchSha256`,
`excludedPath`, `evidence`, `hunks`, `criterionBindings`, `runnerReceipts`,
`criterionLinks`, `artifacts`. The first six fields retain the CEM 0.2 canonical
binding, evidence, hunk and basis algorithms described by
`docs/CHANGE-EVIDENCE-MAP.md`, `docs/cem-0.2.schema.json` and the historical
`interop/cem-0.1/ALGORITHMS.md`. The excluded path is exactly
`.corvint/change.cem.json`. Coverage/discrimination fields and structural mechanical
reasons from 0.3 are not admitted in this candidate; that limitation blocks stable
promotion. Each evidence ID is used by a hunk basis, as in the inherited contract.

Input is at most 4 MiB, strict UTF-8, one complete JSON object, no duplicate keys
at any depth, no unknown fields, depth at most 64. Integers retain the inherited
unsigned safe-integer domain 0..9007199254740991, encoded as canonical decimal
integer tokens: no sign, fraction or exponent. This explicit candidate lexical
restriction does not alter the historical portable 0.1 integer acceptance. Every scalar must have its declared JSON type before decoding or hashing; null,
booleans, strings, arrays and objects never substitute for integer fields. Null
does not substitute for required strings, arrays or objects.
Record bounds are 4096 evidence, 2048 hunks, 256 criterion bindings, 64 runner
receipts, 1024 links and 1024 artifacts. Each new array is nonempty. A link holds
1..32 unique references in each reference array. All existing identity, span and
range bounds remain in force.

Both independent expected base and target are required full lowercase Git commit
OIDs. Resolve and verify these immutable objects, require the map's base equal the
expected base, and derive the whole-repository base-to-target patch excluding only
the fixed sidecar. A supplied patch is never an input to candidate verification.
A present base sidecar is a regular 100644 blob. A present target sidecar is a
regular 100644 blob byte-for-byte identical to the original candidate input; its
absence is allowed by the inherited canonical contract. Do not canonicalize the
candidate before that comparison. A candidate contains no self-referential target
commit field.

The canonical diff uses an empty attribute source, disabled external diff/textconv,
no renames, Myers algorithm, no indent heuristic, three context lines, zero inter-hunk
context, full index IDs, a/ and b/ prefixes and no color. Ambient/system/global Git
configuration, replace refs, alternates, lazy network fetch, hooks, filters and
attributes cannot supply evidence or alter the derivation. Unsupported repository
conditions refuse. The verifier must establish that the derived patch covers the
immutable changed tree entries and replays verified base bytes into verified target
bytes, and validate all hunk identities, basis references, evidence spans and drift.
A stale/ambiguous/deleted cited span does not produce an accepted integrity result.

A consumer may impose documented narrower operational repository bounds and return
unsupported. The standalone candidate consumer initially admits primary SHA-1 or
SHA-256 repositories with regular directories and regular tracked files; linked
worktrees and tracked symlinks/gitlinks are unsupported. This is a reference
portability limit, not equivalent qualification for every native repository profile.

## Criterion references (CEM-V1-002)

A criterion binding has exactly `id`, `ticketId`, `acceptanceRevision`,
`acceptanceSha256`, `criterionIndex`, `criterionSha256`, `captureSha256`,
`verificationSha256`, `claimTicketSha256`, `snapshotHeadReceiptSha256`.
All named SHA-256 values are 64 lowercase hex digits. `acceptanceRevision` is a
canonical positive decimal string of at most 16 digits and at most 9007199254740991.
`criterionIndex` is a zero-based integer below 256. `ticketId` uses the canonical
native reference spelling `ticket:<authority>:<queue>:<local>`, at most 128 ASCII
bytes; tokens begin with an ASCII letter or digit, continue with letters/digits or
`._-`, and the local token is at most 64 bytes. This checks reference syntax only,
not the native ticket record, its authority or its history.

`criterionSha256` is SHA-256 of the exact criterion UTF-8. `acceptanceSha256` denotes
SHA-256 of the canonical JSON ordered array of all criterion SHA-256 strings,
without a trailing newline. Record `id` is `criterion:sha256:` followed by SHA-256
of its canonical JSON object with only the `id` member omitted: sorted UTF-8 object
keys, minimal separators, inherited canonical string escaping and shortest integer
encoding. IDs and the tuple (ticketId, acceptanceRevision, criterionIndex) are
unique. Bindings for the same ticket/revision share acceptance, capture,
verification, claim-ticket and snapshot digests. They need not enumerate every
criterion: absence means unreferenced, never satisfied.

Capture, verification and claim-ticket digests resolve to declared artifacts with
kinds `tasks-capture`, `tasks-verification`, `tasks-claimed-ticket`, respectively.
The snapshot digest is an immutable historical reference, not a current snapshot
claim. This candidate does not decode or trust a partial copy of native Tasks
records. Without the separately admitted native Tasks verifier, native authority,
historical validity and current applicability are all `NOT_OBSERVED`, even when
artifact byte integrity succeeds. A verifier result is never a Tasks completion
receipt or an execution gate.

## Runner references and links (CEM-V1-003..005)

A runner reference has exactly `sha256`, `profile`, `planSha256`,
`sourceGitBinding`, `executionAuthority`, `dependencyClosure`, `authentication`.
The profile is exactly `corvint-test-runner-receipt/0`; digest fields are SHA-256.
The receipt and plan digests resolve to artifacts of kinds `runner-receipt` and
`runner-plan`. Digests are unique. The four assurance fields are exactly
`NOT_OBSERVED`, `CALLER_OBSERVED`, `NOT_OBSERVED`, `NOT_OBSERVED`, respectively.
The native receipt has no Git-source attestation; do not infer one from its
input digest, source pathname, current HEAD or the CEM target. These fields
represent limits of the reference interface, not an authenticated observation.

A link has exactly `criterionId`, `hunkIds`, `evidenceIds`,
`runnerReceiptSha256s`. Every reference resolves to exactly one corresponding
record in this map. Criterion IDs are unique across links; every declared
criterion and every declared runner receipt is referenced by a link. Each cited
link evidence ID must occur in a basis of one of that link's hunk IDs; proximity
alone creates no evidence relation. Every link is a declared association, never
an assertion that tests ran, covered a line, killed a mutant or met a criterion.
No pass, satisfied, killed, coverage or discrimination property is admitted here.
Original native test states, retries, report hashes and failures remain intact in
the referenced raw receipt; the candidate never fabricates or rewrites them.

## Artifact inventory (CEM-V1-003, CEM-V1-007)

An artifact has exactly `kind`, `path`, `sha256`. Kind is one of the five values
above. Paths and digests are unique across the inventory. Every artifact is
referenced in its matching role; unused, missing, duplicated or wrong-kind
artifacts refuse. A single artifact may be referenced by several criteria.
Each path is 1..512 UTF-8 bytes, relative, with no empty/dot/dot-dot segment or case-insensitive `.git` segment
(for example `.GiT`),
backslash or control byte. Reads use a separately supplied artifact root, refuse
symlinks in every component and nonregular files, and stay within that root.
No referenced path or artifact executes any code. Read at most 4 MiB per artifact
and 16 MiB total, under the verification deadline. Hash original bytes exactly;
changed bytes or a digest mismatch refuse. The filesystem is not an OS sandbox
against hostile same-user concurrent mutation.

Raw artifacts remain opaque reference evidence. Valid hashes do not prove their
native schemas, reported execution, claimed actor, receipt authenticity or semantic
content. Output must say `referenceIntegrity: REFERENCE_INTEGRITY_ONLY` and retain
`nativeAuthority`, `historicalValidity`, `currentApplicability`, `sourceGitBinding`,
`authentication`, `dependencyClosure`, `criterionDiscrimination` and
`externalInteroperability` all `NOT_OBSERVED`. A malformed, failed or fabricated
native record may still have intact referenced bytes; this interface makes no
native outcome judgment about it. Consumers needing native authority must use the
native Tasks/runner verification boundaries and must not treat this outcome as
those boundaries.

## Result and portable command (CEM-V1-007..010)

The standalone additive interface is:

```
verify-candidate --repository DIR --map FILE --expected-base FULL_OID --target FULL_OID --artifacts DIR
```

Each flag appears once. No `--patch` is admitted. The result is one JSON line with
profile `cem-candidate-verification/1`, `spec`, `integrity` (`VERIFIED` or
`NOT_VERIFIED`), `baseRevision`, `targetRevision`, `referenceIntegrity`, `limits`
and `code`. The limitations above are present on success and failure. Exit 0
means only canonical change and referenced-byte integrity verified; exit 1 is
invalid/missing evidence or unsafe drift; exit 2 is invocation/operational refusal.
The historical verify and ci command ABI and behavior are unchanged.

The candidate packet and both implementations are Corvint-authored reference
portability only. Existing frozen manifests remain immutable; candidate vectors
are a new packet. Stable release additionally requires all consumer migrations,
0.3 capability portability, native runtime/source/authority qualification,
independently authored consumer evidence and external outcome evaluation.
Rollback removes the opt-in candidate interface, not old maps, defaults or history.
