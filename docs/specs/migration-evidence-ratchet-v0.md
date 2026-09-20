# Migration Evidence Ratchet V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: accepted (GitHub issue #46)
Delivery status: experimental
Authoritative inputs: GitHub issue #46; `../SPEC-DRIVEN-DEVELOPMENT.md`; `documentation-corpus-v1.md` issue-40 and issue-42 evidence boundaries; repository invariants 1, 2, 3, 7 and 8.

## Agent digest
- Status: accepted (GitHub issue #46) / experimental
- Claim: A revision-bound baseline/candidate receipt prevents migration evidence debt from growing without claiming adequacy or completeness.
- Exists: `internal/migrationratchet`; `corvint migration-ratchet --profile FILE`; synthetic focused tests.
- Blocked on: real consumer snapshots, clean-tree gate, dogfood closure and independent review.
- Read next: Requirements; Trust boundary, limits, and failure modes; Acceptance and testing matrix.

## User and measurable job

A repository migrating a large legacy test estate needs CI to prevent new evidence debt and loss of
already-reviewed evidence before a repository-wide completeness gate can pass. The smallest useful
result is a byte-stable baseline-to-candidate receipt whose raw denominators and per-identity deltas
can be retained by CI or ingested as corpus evidence.

## Verified current state

Issue 40 added revision-pinned behavior-contract and Playwright execution joins under
`internal/doccorpus/behavior.go`. Issue 42 added separately scoped repeated-run stability evidence in
`internal/doccorpus/stability.go`. Neither profile compares a reviewed migration baseline with a
candidate or enforces a no-growth debt policy. Existing count ratchets cannot detect a weakened
criterion, renamed identity, stale evidence digest or lost reverse relation when totals are unchanged.

## Requirements

- `MER-V0-001`: The profile MUST bind baseline and candidate repository IDs, immutable commit and
  tree object IDs, snapshot schemas, digest-bound repository policies, provider IDs/schemas/digests
  and self-verified snapshot artifact SHA-256 digests. A branch name or malformed object ID MUST be
  refused.
- `MER-V0-002`: A snapshot MUST carry stable `(kind,id)` identities for legacy cases, discovered
  test/project executions, behavior contracts, documented coverage targets, owners, reviewers,
  reviews and immutable evidence manifests. Each record MUST bind its source content digest, state,
  complete relation set and record digest.
- `MER-V0-003`: The receipt MUST expose raw baseline and candidate totals by kind and state, the
  unresolved denominator, per-identity changes, and separate ordered collections for additions,
  removals, content changes, state changes, stale evidence, broken reverse links and unknowns.
- `MER-V0-004`: Repository policy MUST be content-bound and support forbidding new legacy cases,
  requiring a current reviewed contract for new or behaviorally changed test executions, preventing
  terminal-state regression, invalidating retained review/runtime evidence after bound content
  changes, and preventing growth of the unresolved denominator.
- `MER-V0-005`: A grandfathered unresolved identity MAY remain. The candidate unresolved denominator
  MUST NOT grow when policy enables that rule. Every receipt MUST state that a passing ratchet proves
  neither behavioral adequacy nor migration completeness.
- `MER-V0-006`: Every link MUST bind the target kind, identity and content digest plus its required
  reverse relation. A missing target, changed digest, unchanged review/runtime evidence across a
  changed bound source, or absent reverse edge MUST remain separately visible and fail policy unless
  an exact current exception applies. Defect deltas MUST retain the expected and actual content,
  relation and reverse-relation bindings needed to distinguish two defects on one identity.
- `MER-V0-007`: An exception MUST be an immutable digest-verified record with one exact delta,
  record kind and identity scope, a stable owner, one or more stable reviewers, a nonempty reason and
  an ISO date expiry. Its scope MUST bind the canonical delta SHA-256, so one exception cannot cover a
  second defect on the same identity. Mutating an existing exception, widening its scope, or retaining
  it after expiry MUST fail closed.
- `MER-V0-008`: Cross-schema, cross-repository, cross-policy, cross-provider, stale, partial,
  duplicate or contradictory inputs MUST be refused unless one separately digest-verified reviewed
  migration rule binds the exact baseline and candidate artifacts and names the permitted dimension.
  Duplicate or contradictory identities additionally require an exact selected record digest.
- `MER-V0-009`: A reviewed migration rule MAY map one exact baseline identity to one exact candidate
  identity. Without that mapping, same-kind delete/add records with the same content digest MUST be
  reported as an unknown rename masquerade, not accepted as migration progress.
- `MER-V0-010`: The profile decoder MUST reject unknown fields, trailing JSON and inputs over 4 MiB;
  each snapshot is bounded to 4,096 records and 4,096 exceptions. Records and relation lists MUST be
  canonical and every emitted collection MUST have deterministic ordering.
- `MER-V0-011`: `corvint migration-ratchet --profile FILE` MUST write exactly one canonical JSON
  receipt. Exit 0 is `pass`; exit 1 carries a valid `fail` or `unknown` receipt; exit 2 emits a typed
  refusal and no success receipt. The command reads inputs and MUST NOT mutate repository, index,
  trace, observation or task state.
- `MER-V0-012`: Executable negatives MUST cover a renamed test masquerading as delete/add, a weakened
  criterion at unchanged count, a removed reverse link, a changed test body retaining old evidence,
  a new uncontracted test and an expired exception. Deterministic pass and refusal paths MUST also be
  exercised.

## Trust boundary, limits, and failure modes

The profile verifies identity, closure and digest consistency; it does not authenticate provider
honesty, the semantic adequacy of a contract, reviewer identity, runtime execution, current served
content or the completeness of a migration inventory. Repository policy and evidence providers stay
attributed inputs. Review and owner identities are stable records, not authentication claims.

Malformed or oversized JSON, mutable revision names, invalid digests, invalid exception identity,
unresolved duplicate selection and unreviewed comparability changes refuse before comparison. A
well-formed comparison retains policy violations as `fail` and record-level incomparability as
`unknown`. Both are CI failures. Resource use is bounded by the input and record ceilings; the
command performs no network access or provider execution.

## Non-goals and simpler baseline

The baseline remains a repository-specific script comparing one reviewed JSON snapshot with one
candidate. V0 does not discover tests, infer stable identities, generate contracts, execute tests,
authenticate reviewers, replace a completeness gate, repair stale evidence, publish artifacts or
grant corpus authority. It adds no service, database, daemon or network dependency.

## Acceptance and testing matrix

| Concern | Evidence |
| --- | --- |
| Deterministic baseline-to-candidate receipt and raw denominators | `TestRevisionBoundReceiptEndToEnd` |
| Six issue-46 hostile controls | `TestMigrationEvidenceNegativeControls` |
| Cross-domain/stale/partial/duplicate/contradictory refusal | `TestIncomparableInputsRefuseWithoutReviewedRule` |
| Exact reviewed comparability rules, identity mappings, duplicate selection and scoped exception | `TestReviewedRulesAndScopedException` |
| Canonical record/link closure, mapped evidence renewal, exact exception scope and trailing-input refusal | `TestCanonicalRecordClosure`, `TestMappedEvidenceIdentityStillRequiresRenewal`, `TestExceptionBindsOneConcreteDelta`, `TestDecodeRejectsEveryTrailingValue` |
| CI exit contract, deterministic stdout and malformed immutable binding | `TestMigrationRatchetCLIExitAndDeterministicReceipt`, `TestMigrationRatchetCLIRefusesMutableOrMalformedBinding` |
| Repository byte identity | `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` migration-ratchet case |

Focused package and command tests are required for this experimental slice. The repository's full
clean-tree gate, dogfood CEM/OCM closure, independent review and a real consumer snapshot remain the
promotion gate. Synthetic fixtures establish protocol behavior only.

## Rollout, rollback, and maintenance

The command and schemas are additive and experimental. Repositories opt in by constructing a
reviewed profile; no default gate invokes it. Schema or policy changes require an explicit reviewed
migration rule rather than silent reinterpretation. Rollback removes the command, package and this
index entry; stored receipts remain labelled experimental evidence and grant no authority.

## Traceability

| Requirements | Implementation | Evidence |
| --- | --- | --- |
| MER-V0-001..010 | `internal/migrationratchet` | `TestRevisionBoundReceiptEndToEnd`, `TestMigrationEvidenceNegativeControls`, `TestIncomparableInputsRefuseWithoutReviewedRule`, `TestReviewedRulesAndScopedException`, `TestCanonicalRecordClosure`, `TestMappedEvidenceIdentityStillRequiresRenewal`, `TestExceptionBindsOneConcreteDelta`, `TestDecodeRejectsEveryTrailingValue` |
| MER-V0-011..012 | `cmd/corvint/migration_ratchet.go` | `TestMigrationRatchetCLIExitAndDeterministicReceipt`, `TestMigrationRatchetCLIRefusesMutableOrMalformedBinding`, `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` |

## Unresolved decisions and promotion criteria

Promotion requires a real repository baseline/candidate pair, repository-owner review of its policy,
an independent integrated review, the full clean-tree gate and dogfood completion. Corpus storage is
an attributed consumer of the receipt, not a new authority source. Authenticated reviewer identity or
automatic snapshot discovery requires a separate accepted contract.
