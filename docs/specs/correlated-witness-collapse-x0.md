# Correlated Witness Collapse X0

Owner: Russell Lewis
Drafted: 2026-08-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/CHANGE-EVIDENCE-MAP.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Declared common causes may collapse duplicate witness obligations only without strengthening authority or hiding uncertainty.
- Status: proposed/experimental
- Exists: the bounded contract and deterministic algorithm; no Corvint implementation exists.
- Blocked on: implementation, conformance, and evidence that declarations are complete enough for use.
- Read next: User and measurable job; Verified current state; Requirements.

## User and measurable job

An author or reviewer must not mistake several evidence records for several independent witnesses
when those records share a generator, parser, oracle, source, or premise. X0 compiles explicitly
declared common causes over one exact Change Evidence Map into a deterministic upper bound on the
number of independent witnesses supporting each hunk.

The wording is deliberate: an undeclared dependency can make the true independent count lower.
X0 may reduce an optimistic count; it never certifies independence, upgrades evidence, or closes a
claim.

## Verified current state

- CEM 0.1 and 0.2 provide immutable evidence identities and typed hunk-basis references, but do not
  represent evidence lineage or common causes.
- The Human Documentation Compiler validates pinned evidence and
  `SUPPORTED|CONFLICTED|UNKNOWN` claim state, but does not assess whether cited records are
  independent.
- Evidence Precedence Tiers V0 is proposed and not implemented. X0 therefore does not rank
  authority, consume a tier, or introduce a competing authority table.
- No Corvint package compiles common-cause evidence components or emits an independence upper bound.

## Scope and definitions

A **witness** is one CEM evidence ID referenced by a hunk basis. A **common cause** is an explicit
declaration that two or more witnesses depend on the same domain-separated source, generator,
parser, oracle, or premise identity. A **membership attestation** is another evidence ID in the same
CEM cited as the basis for that declaration. The attestation MUST differ from the witness it covers;
self-attestation is invalid. Presence of an attestation proves only that the pinned record exists;
X0 does not prove that its prose or semantics establish the dependency.

The **independence upper bound** for a hunk is the number of connected components among its unique
witness evidence IDs after applying all declared common causes. Missing declarations may make the
true number lower. False declarations may make the reported number too low and cause conservative
abstention, but cannot strengthen a conclusion.

## Requirements

- `CWC-X0-001`: X0 MUST consume one exact bounded `cem/0.1` or `cem/0.2` byte document through the
  existing strict CEM parser and bind the report to its raw SHA-256, spec, base revision, and patch
  digest. Structural parsing is not repository or canonical CEM verification.
- `CWC-X0-002`: Common causes MUST use exactly `source`, `generator`, `parser`, `oracle`, or
  `premise`, plus a 64-character lowercase SHA-256 subject identity. Cause IDs MUST be
  domain-separated, content-derived from kind and subject digest, and stable across input order.
- `CWC-X0-003`: Every witness and membership-attestation reference MUST identify evidence present
  in the input CEM, and each membership's attestation evidence ID MUST differ from its witness
  evidence ID. A self-attestation, duplicate cause/witness membership, unknown reference, malformed
  digest, unsupported kind, cause with fewer than two distinct witnesses, or bound violation MUST
  fail closed with no partial report.
- `CWC-X0-004`: Collapse MUST be the deterministic transitive closure of declared common-cause
  memberships. Union-find insertion order MUST NOT select a public identity; component membership,
  cause membership, and content-derived group IDs MUST be derived from sorted values.
- `CWC-X0-005`: Per-hunk raw witness count MUST deduplicate evidence IDs across repeated CEM
  relation types. The adjusted value MUST be named `independence_upper_bound`, MUST never exceed the
  raw count, and MUST NOT be described as a confirmed independent-witness count.
- `CWC-X0-006`: A report MUST preserve the CEM hunk disposition, normalized causes and membership
  attestations, correlation groups, and the exact assurance `declared-common-causes-only`. Unknown
  or mechanical hunks MUST remain unknown or mechanical and cannot be promoted by X0.
- `CWC-X0-007`: Report bytes MUST be deterministic UTF-8 JSON with one terminal LF. Verification
  MUST reconstruct the declared inputs, recompile against the exact raw CEM, and require
  byte-identical canonical output.
- `CWC-X0-008`: X0 MUST be bounded, local, read-only, network-free, and free of filesystem or
  subprocess access. It MUST retain no raw cause-identity preimages, source bodies, authority tiers,
  confidence scores, or claim prose.
- `CWC-X0-009`: X0 MUST accept at most 8,192 causes and 16,384 memberships, in addition to inherited
  CEM bounds. Public errors MUST use stable codes and MUST NOT echo source bodies or raw cause
  preimages.
- `CWC-X0-010`: X0 MUST NOT modify CEM, OCM, documentation claims, repository state, or policy;
  expose a CLI or public wire protocol; infer common causes; rank authority; or enforce an
  independence threshold.
- `CWC-X0-011`: Focused tests MUST cover direct, disconnected, transitive, repeated-relation,
  permutation, unknown/mechanical, hostile-reference, exact-binding, limit, canonical-round-trip,
  and report-tampering cases. Benchmarks MUST exercise a five-to-one case and both disconnected and
  connected production-bound graphs.
- `CWC-X0-012`: Every report MUST remain `experimental` and `UNPROVEN`. Green unit tests, Corvint
  dogfood, or a synthetic five-to-one result MUST NOT be presented as automatic lineage detection,
  real-world superiority, independent validation, patent novelty, or a breakthrough.

## Deterministic algorithm

1. Parse the exact raw CEM and index its evidence IDs.
2. Sort membership declarations by kind, subject digest, witness ID, and attestation ID before
   validation, so input permutation cannot affect output or multi-defect precedence.
3. Derive and sort common causes. Require at least two distinct witnesses per cause.
4. Initialize one disjoint-set node per CEM evidence ID. Apply causes in cause-ID order and members
   in witness-ID order.
5. Materialize final components from sorted members rather than internal disjoint-set roots. Emit
   only non-singleton correlation groups, with IDs derived from their sorted witnesses and causes.
6. For each hunk in CEM order, deduplicate basis evidence IDs, map them to final components, count
   distinct components, and list a group only when at least two of its members occur in that hunk.
7. Sort every set-valued report field and encode the closed struct-only report shape deterministically.

Time is `O(E + A log A + A alpha(E) + H*B log B)` for evidence `E`, memberships `A`, hunks `H`,
and bases per hunk `B`. The inherited bounds are 4,096 evidence records, 2,048 hunks, and 32 bases
per hunk.

## Trust boundary and failure modes

- Self-attestation is invalid because it adds no distinct pinned basis for the lineage declaration.
- A forged or mistaken membership can over-collapse evidence. X0 therefore remains advisory and
  cannot close a policy threshold.
- An omitted membership can under-collapse evidence. This is why the result is only an upper bound.
- Membership evidence identity does not prove the dependency's semantics; human review or a future
  registered lineage producer must establish that separately.
- SHA-256 digests reveal equality and do not anonymize private identity. Reports remain local-only.
- CEM parsing proves structural validity only. Callers requiring repository assurance must first run
  the existing exact or canonical CEM verifier over the same raw bytes.
- Different cause kinds are domain-separated even when their subject digests match.
- Resource limits reject oversized input before graph construction. The package performs no I/O.

### Failure codes

The `internal/witnesscollapse` compiler fails with the kebab-case codes below (decision 0100).
Each row cites the first emitting site and quotes the message returned there, which is the whole
of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `duplicate-membership` | `internal/witnesscollapse/compile.go:77` | "common cause repeats a witness membership" |
| `invalid-cause-kind` | `internal/witnesscollapse/identity.go:23` | "common cause kind is unsupported" |
| `invalid-cause-subject` | `internal/witnesscollapse/identity.go:26` | "common cause subject must be a lowercase SHA-256" |
| `non-common-cause` | `internal/witnesscollapse/compile.go:84` | "common cause must name at least two distinct witnesses" |
| `report-encode-failed` | `internal/witnesscollapse/canonical.go:14` | "report cannot be encoded" |
| `report-mismatch` | `internal/witnesscollapse/canonical.go:44` | "report differs from deterministic recompilation" |
| `too-many-attributions` | `internal/witnesscollapse/compile.go:25` | "memberships exceed the <value>-item bound" |
| `too-many-causes` | `internal/witnesscollapse/compile.go:62` | "common causes exceed the <value>-item bound" |
| `unknown-attestation` | `internal/witnesscollapse/compile.go:71` | "membership attestation is absent from the CEM" |
| `unknown-witness` | `internal/witnesscollapse/compile.go:68` | "membership references evidence absent from the CEM" |

## Simpler baseline and non-goals

The baseline is the raw number of unique CEM evidence IDs per supported hunk. X0 does not add
automatic lineage discovery, source-code analysis, model inference, authority precedence,
signatures, multi-repository evidence, persistence, a daemon, a database, a UI, a CLI, or a public
schema. It does not integrate with the Human Documentation Compiler in this slice.

## Acceptance, benchmarks, and rollback

Experimental delivery requires all `CWC-X0-001..012` focused tests, `go test` and `go vet` for the
package, byte-stable golden output, and benchmarks that report time and allocation cost at the
declared bounds. The initial performance ceiling is 250 ms and 128 MiB allocated for each maximum
compile or verify on the pinned Go 1.27 release runner; a tighter ratchet may be frozen only from a
clean recorded run.

Rollback is deletion of the isolated package and this experimental spec. No existing wire,
repository artifact, command, or consumer changes.

## Breakthrough promotion gate

X0 demonstrates deterministic collapse of already-declared common causes only. A later registered
lineage-producer trial must evaluate 10,000 planted common-cause families and 500 independently
labelled real changes, with zero duplicate-induced proof closures, at least 95% common-mode recall,
under 5% false collapse, causal ablation, hostile missing and forged lineage, a strongest-baseline
comparison, and independent replication. Prior-art claim charts and legal review remain separate.
Until all gates pass, the capability remains `experimental / UNPROVEN`.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `CWC-X0-001..003` | `internal/witnesscollapse/compile.go`, `identity.go` | parser, binding, identity, reference, and hostile-input tests |
| `CWC-X0-004..006` | `internal/witnesscollapse/compile.go`, `types.go` | direct, transitive, disconnected, repeated-relation, and disposition tests |
| `CWC-X0-007` | `internal/witnesscollapse/canonical.go` | permutation golden, round-trip, and tampering tests |
| `CWC-X0-008..010` | isolated package API and fixed limits | static package boundary plus limit and error tests |
| `CWC-X0-011` | `collapse_test.go`, `benchmark_test.go` | focused test and benchmark commands |
| `CWC-X0-012` | fixed report labels | exact-label and no-promotion assertions |
