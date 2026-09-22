# Proof-Preserving Abstraction V0

Owner: Russell Lewis
Frozen: 2026-08-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/ARCHITECTURE.md`, `docs/PRODUCT.md`,
`docs/specs/proof-carrying-context-optimization-v0.md`

## Agent digest
- Claim: An isolated Go library projects proof state into smaller views without strengthening claims or authority.
- Status: proposed/experimental
- Exists: isolated `internal/proofabstraction` library and non-strengthening certificate tests.
- Blocked on: large hostile corpus, sealed task trial, and independent replication remain `NOT_RUN`.
- Read next: Verified current state; Requirements; Traceability.

## User and measurable job

A Corvint adapter should be able to project a frozen proof state into a smaller audience- or
budget-specific view while an independent local verifier establishes that the view did not add a
claim, widen a claim, increase authority, erase critical uncertainty or conflict, or silently omit
evidence. The certificate is relative to the supplied source envelope. It does not establish that
the source claims are true, that a model read the view, or that rewritten prose is semantically
equivalent.

P0 is deliberately narrower than semantic summarization. It supports only exact retention,
canonical demotion to `UNKNOWN`, and explicitly frontier-accounted omission. It is a pure native-Go
library and conformance experiment. It is not selectable by `corvint`, query, harness, CEM, HDC,
or Proof-Carrying Context Optimization.

## Verified current state

- Corvint can bind evidence and uncertainty in CEM, OCM, and HDC artifacts, but none of those
  contracts defines a smaller proof-state projection with an independently checkable
  non-strengthening certificate.
- Proof-Carrying Context Optimization V0 is proposed and not started. It does not select or govern
  this experimental profile.
- No existing CLI, harness, trace, index, documentation, or repository path consumes the profiles
  defined here. The implementation is an isolated library experiment.

## Frozen profiles and bounds

- source and destination: `corvint-proof-state/0-experimental`;
- request: `corvint-proof-preserving-abstraction-request/0-experimental`;
- certificate: `corvint-proof-preserving-abstraction-certificate/0-experimental`;
- algorithm: `corvint-exact-proof-projection/0`;
- certificate claim: `RELATIVE_NON_STRENGTHENING`;
- maximum encoded artifact: 4 MiB;
- maximum claims: 2,048;
- maximum evidence records: 4,096;
- maximum frontier records: 4,096;
- maximum combined supporting and counter-evidence references per claim: 64;
- maximum token: 256 ASCII bytes.

All objects are closed. Arrays are sorted and duplicate-free. Canonical JSON is UTF-8,
deterministic JSON with no insignificant whitespace. Identity preimages have no terminal newline.

## Proof-state envelope

An envelope contains:

- one repository tuple `objectFormat`, full immutable `revision`, and full immutable `tree`;
- exact `policySha256` and `purposeSha256` digests;
- completeness `COMPLETE|PARTIAL|UNKNOWN`;
- claims sorted by `id`;
- evidence sorted by `id`;
- frontier records sorted by `id`; and
- a domain-separated `envelopeId`.

A claim contains content-derived `id`, `statementSha256`, `scopeSha256`, `critical`, proposition
verdict `PROVED|REFUTED|CONFLICTED|UNKNOWN`, authority class, sorted evidence classes, sorted
supporting evidence, sorted counter-evidence, and sorted unknown reasons. The statement and scope
are opaque digests in P0: the compiler cannot rewrite or reason about either one.

Evidence is an opaque wrapper around one source profile, external identity, and canonical content
digest. Its presence in a source envelope is not verification of the wrapped source. A source
adapter must separately qualify the evidence before constructing a proof state.

Frontier kinds are `SOURCE_UNKNOWN`, `SOURCE_CONFLICT`, `SOURCE_EXCLUSION`, and
`ABSTRACTED_CLAIM`. Source frontier records bind a present claim and recover through
`SOURCE_VERIFIER`. An abstraction frontier binds an omitted claim, the exact source envelope, and
recovers through `LOAD_SOURCE_ENVELOPE`.

IDs are:

```text
proof-evidence:sha256:SHA-256("corvint-proof-evidence/0-experimental" || 0x00 || canonical record without id)
proof-claim:sha256:SHA-256("corvint-proof-claim/0-experimental" || 0x00 || canonical {scopeSha256,statementSha256})
proof-frontier:sha256:SHA-256("corvint-proof-frontier/0-experimental" || 0x00 || canonical record without id)
proof-state:sha256:SHA-256("corvint-proof-state/0-experimental" || 0x00 || canonical envelope without envelopeId)
```

## Request and deterministic algorithm

A request binds the exact source envelope, destination-purpose digest, and exactly one sorted action
per source claim. `RETAIN` has no reason. `DEMOTE_TO_UNKNOWN` and `OMIT` require reason
`AUDIENCE|BUDGET`. The certificate binds the complete canonical request through:

```text
requestSha256 = SHA-256(
  "corvint-proof-preserving-abstraction-request/0-experimental" || 0x00 || canonical request
)
```

Compilation:

1. validates the source and request;
2. applies actions in source claim-ID order;
3. copies every source frontier record;
4. adds one deterministic `ABSTRACTED_CLAIM` frontier for every omission;
5. derives destination evidence as the exact union referenced by retained claims;
6. weakens `COMPLETE` to `PARTIAL` after any demotion or omission;
7. content-addresses the destination;
8. accounts for every source claim, evidence record, and frontier record;
9. content-addresses the certificate; and
10. invokes the independent verifier with the exact source, request, destination, and certificate
    before returning the result.

`DEMOTE_TO_UNKNOWN` preserves claim identity, statement, scope, and criticality; sets verdict to
`UNKNOWN` and authority to `NONE`; removes evidence, counter-evidence, and evidence classes; and sets
the sole unknown reason `ABSTRACTION_WEAKENED`.

## Certificate

The certificate binds source and destination envelope identities, the canonical request digest,
repository tuple, policy and purpose digests, completeness relation, sorted
`IDENTICAL|DEMOTED_TO_UNKNOWN` claim relations, sorted omitted-claim records, one
`PRESERVED|OMITTED` accounting row per source evidence record, and the exact preserved and added
frontier IDs. Verification requires all four inputs: source envelope, exact request, destination
envelope, and certificate. Its identity is:

```text
ppa-certificate:sha256:SHA-256(
  "corvint-proof-preserving-abstraction-certificate/0-experimental" || 0x00 ||
  canonical certificate without certificateId
)
```

## Requirements

- `PPA-V0-001`: A certificate MUST claim only relative non-strengthening between two supplied
  envelopes. It MUST NOT claim source truth, semantic equivalence, model use, task correctness, or
  global completeness.
- `PPA-V0-002`: Source and destination MUST bind the same repository object format, revision, tree,
  and policy digest. P0 MUST reject cross-revision, dirty-state, relocation, and policy transfer.
- `PPA-V0-003`: Every wire object MUST be closed, bounded, canonical, content-addressed, and reject
  unknown fields, duplicates, malformed UTF-8, noncanonical ordering, and invalid identities.
- `PPA-V0-004`: Claim identity MUST bind the exact statement and scope digests. P0 MUST reject
  statement rewriting, scope rewriting, merging, splitting, or synthetic claims.
- `PPA-V0-005`: A request MUST account for every source claim exactly once. The certificate MUST
  contain exactly one matching relation or omission for each requested action and no surplus entry.
  Action, relation, omission, and reason MUST match exactly. Missing, duplicate, or foreign actions,
  relations, or omissions MUST fail rather than imply omission.
- `PPA-V0-006`: P0 MUST permit only exact `RETAIN`, canonical `DEMOTE_TO_UNKNOWN`, and
  frontier-accounted `OMIT`. No transformation may add evidence, evidence class, authority, or a
  stronger completeness state.
- `PPA-V0-007`: Critical, `CONFLICTED`, `UNKNOWN`, and source-frontier-bound claims MUST be retained
  exactly. They cannot be demoted or omitted.
- `PPA-V0-008`: A demoted or omitted claim MUST be noncritical, frontier-free, and originally
  `PROVED` or `REFUTED`.
- `PPA-V0-009`: Every source frontier MUST survive byte-equivalently. Every omission MUST create one
  deterministic `ABSTRACTED_CLAIM` frontier and make a previously complete destination `PARTIAL`.
- `PPA-V0-010`: Destination evidence MUST equal the union referenced by exact retained claims.
  Every source evidence record MUST be reported exactly once as `PRESERVED` or `OMITTED`.
- `PPA-V0-011`: Compilation MUST be byte-deterministic across process and input construction order.
  The compiler MUST invoke a verifier with the exact source, request, destination, and certificate.
  The verifier MUST independently recompute the request digest, check the non-strengthening
  relation, reject unrequested or reason-mismatched certificate entries, and reconstruct every
  certificate accounting field.
- `PPA-V0-012`: The package MUST remain local, read-only, model-free, dependency-free, and native Go.
  It MUST perform no Git, filesystem, network, database, daemon, or process operation.
- `PPA-V0-013`: P0 MUST remain library-only and experimental. No runtime, CLI, query, compaction,
  documentation, CEM, OCM, or harness path may select it.
- `PPA-V0-014`: Source qualification is an adapter responsibility. A CEM relation, documentation
  `SUPPORTED` label, retrieval rank, or caller assertion MUST NOT automatically become `PROVED`.
- `PPA-V0-015`: Promotion requires 100% rejection of at least 10,000 hostile strengthening mutants,
  100% acceptance of at least 1,000 valid generated projections, byte-identical permutation results,
  zero treatment-only critical misses on 30 preregistered tasks, non-inferior sealed correctness,
  at least 40% lower median evidence bytes, at least 25% lower complete cache-aware cost per solved
  task, and independent replication.

## Failure modes and rollback

P0 rejects snapshot or policy drift, added or mutated claims, changed statement or scope identities,
authority promotion, critical-state weakening, hidden conflict or unknown, added or mutated evidence,
removed frontier, unaccounted omission, completeness strengthening, and certificate mismatch. A
certificate replayed without its exact source, request, and destination envelopes is unusable.

Delete `internal/proofabstraction` and this specification to roll back. No repository, session,
trace, index, or generated-document state is mutated.

## Simpler baseline, compatibility, and non-goals

The simpler baseline is to retain and transmit the exact source proof-state envelope without any
projection. It saves no bytes but cannot strengthen, omit, or demote a claim. V0 adds only new,
experimental, library-local profiles; it changes no existing wire, command, repository artifact,
adapter, or selection policy. A change to canonical encoding, identity preimages, action semantics,
limits, or certificate relations requires a new profile or an amendment to this proposed spec with
regenerated hostile vectors.

V0 does not generate or rewrite prose, infer semantic equivalence, rank evidence, qualify source
truth, merge or split claims, persist projections, publish artifacts, or select a projection for an
agent. Unresolved decisions include source-adapter qualification, real audience policies, cache and
transport integration, and whether measured task outcomes justify selection. Requirement
`PPA-V0-015` remains the promotion and kill gate; until it passes, this profile is experimental and
unselectable.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `PPA-V0-001..012` | `internal/proofabstraction` | focused unit, hostile mutation, determinism, and benchmark tests |
| `PPA-V0-013..014` | package boundary; no integrations | repository diff and compile graph |
| `PPA-V0-015` | experimental only | large hostile corpus, sealed task trial, and independent replication `NOT_RUN` |
