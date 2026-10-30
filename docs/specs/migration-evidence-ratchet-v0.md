# Migration Evidence Ratchet V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: accepted (GitHub issue #46)
Delivery status: experimental
Authoritative inputs: GitHub issue #46; `../SPEC-DRIVEN-DEVELOPMENT.md`; `documentation-corpus-v1.md` issue-40 and issue-42 evidence boundaries; repository invariants 1, 2, 3, 7 and 8.

## Agent digest
- Status: accepted (GitHub issue #46) / experimental
- Claim: A revision-bound baseline/candidate receipt prevents migration evidence debt from growing without claiming adequacy or completeness.
- Exists: `internal/migrationratchet`; `corvint migration-ratchet --profile FILE`, `schema`, `seal` and `span-digest`; synthetic focused tests.
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
  migration rule binds the exact baseline and candidate artifacts and names the permitted dimension,
  or, for stale and partial only, both snapshots carry the same digest-verified policy that accepts
  that dimension (`MER-V0-015`). Duplicate or contradictory identities additionally require an exact
  selected record digest.
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
- `MER-V0-013`: Every self-digest MUST be the lowercase hex SHA-256 of the documented canonical
  encoding (Go `encoding/json` field order and escaping, own digest member set to the empty
  string), published verbatim by `schema` so a non-Go producer can reproduce it. A digest that
  does not match MUST refuse with the side, the exact field path (`policy.sha256`,
  `records[N].sha256 (KIND ID)`, `exceptions[N].sha256 (ID)` or `artifact_sha256`) and the
  expected digest; a record mismatch MUST be named before the artifact mismatch it causes.
  Status: experimental under the standing lane auto-accept (GitHub issue #1898, V1-2801).
- `MER-V0-014`: `corvint migration-ratchet seal (--profile FILE | --snapshot FILE)` MUST print the
  input with every self-digest filled and links, records and exceptions in canonical order. It MUST
  NOT change content digests, states, links, exception scopes or delta digests, MUST NOT re-bind a
  migration rule's reviewed baseline/candidate artifact digests, MUST NOT write its input, and its
  output MUST still pass full `Compare` validation. Status: experimental under the standing lane
  auto-accept (GitHub issue #1898, V1-2801).
- `MER-V0-015`: A policy MAY carry sorted, unique `accept_incomparable` values from {`partial`,
  `stale`}. It applies only when baseline and candidate policies have the same ID and verified
  digest; then the accepted dimension needs no per-comparison migration rule. The receipt MUST
  carry a `policy-acceptance` identity delta naming the policy and dimensions and a limitation
  stating that records absent from an incomplete or stale inventory are not evidence of removal or
  retention. Every other dimension, one-sided acceptance and invalid values MUST still refuse. The
  member is omitted when absent, so existing policy digests are unchanged. Status: experimental
  under the standing lane auto-accept (GitHub issue #1898, V1-2801).
- `MER-V0-016`: `corvint migration-ratchet schema` MUST print a
  `corvint-migration-evidence-ratchet-schema/1` description derived from the decoder's own types:
  every object's accepted members, types and optionality, record kinds, comparable and
  policy-acceptable dimensions, exception delta kinds, limits, verdict/exit meanings, the canonical
  encoding, every exit-2 refusal pattern, and a sealed example profile that compares as `pass`.
  Status: experimental under the standing lane auto-accept (GitHub issue #1898, V1-2801).
- `MER-V0-017`: `corvint migration-ratchet span-digest --revision FULL_COMMIT --path PATH --lines
  START:END` MUST print the Git blob and the SHA-256 of the newline-joined physical lines of one
  1-based inclusive span of a regular UTF-8 blob at that immutable commit, the same digest
  breakage anchors use, so a producer can bind a sub-file `test-execution` `content_sha256`.
  Mutable revisions, unsafe paths and out-of-blob spans MUST refuse. A record without a `source`
  keeps its digest as producer-supplied evidence; `MER-V0-018` adds compare-time verification for
  records that name one. Status: experimental under the standing lane auto-accept (GitHub issue
  #1898, V1-2801).
- `MER-V0-018`: A record MAY carry `source` `{commit, path, start, end}` naming the 1-based
  inclusive physical line span its `content_sha256` digests. The member is omitted when absent, so
  existing record digests are unchanged. `commit` MUST equal the snapshot's repository revision,
  `path` MUST be a clean relative path and `1 <= start <= end <= 200000`; otherwise the snapshot
  refuses. When any record names a source, `--profile` MUST resolve the repository root and re-read
  each span from Git at that commit before comparing, and MUST refuse with exit 2 when the root is
  unavailable (`source-root-unavailable`), the path or span is unreadable, or the span digest differs
  from `content_sha256`, naming the side and `records[N].source (KIND ID)`. A passing receipt states
  how many spans were re-read. A profile without sources never reads Git. Status: experimental under
  the standing lane auto-accept (GitHub issue #1898, V1-2813).
- `MER-V0-098`: (standing lane auto-accept, 2026-10-28; V1-2915) `migration-ratchet seal` MUST apply the policy validation a compare applies and refuse with exit 2 (`policy accept_incomparable refused`) a policy whose `accept_incomparable` holds a value outside `partial` and `stale`, so sealed output never fails compare on its policy.
- `MER-V0-099`: (standing lane auto-accept, 2026-10-28; V1-2915) A profile with record `source` spans run without `--root` outside any Git checkout MUST refuse with `source-root-unavailable`, not a generic Git-input error.

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

A foreign encoder that escapes differently fails closed with the named field and expected digest;
`seal` is the repair, and it cannot launder evidence because it never changes content digests,
states, links or reviewed rule bindings. Policy acceptance of partial or stale inputs can hide a
removed record, so it is digest-bound in both snapshots and always disclosed in the receipt.
`span-digest` reads only the named blob at a full commit; it cannot prove the span is the whole test
body or that the test ran. A record `source` proves only that the cited span's bytes at the snapshot
commit match `content_sha256`; it cannot prove the producer chose the right span. A missing checkout,
an absent path, an out-of-blob span or a digest mismatch refuses the comparison rather than
downgrading the record to an unverified digest. Records without a source stay producer-asserted, and
the receipt says so whenever any span was verified.

## Non-goals and simpler baseline

The baseline remains a repository-specific script comparing one reviewed JSON snapshot with one
candidate. V0 does not discover tests, infer stable identities, generate contracts, execute tests,
authenticate reviewers, replace a completeness gate, repair stale evidence, publish artifacts or
grant corpus authority. It adds no service, database, daemon or network dependency.
The helpers do not generate snapshots, infer test identities or spans, verify digests of records
that name no `source`, read any commit other than the snapshot revision, or accept schema,
repository, policy, provider, duplicate or contradictory changes by policy.

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
| Foreign encoding named refusal, seal repair and reviewed rule binding | `TestSealRepairsForeignEncodingAndNamesTheField`, `TestSealLeavesRuleArtifactBindingsReviewed` |
| Policy acceptance across comparisons and its refusals | `TestPolicyAcceptsPartialAcrossComparisons` |
| Schema description, passing example and refusal coverage | `TestDescribeCoversDecoderAndRefusals` |
| CLI schema, seal, span-digest and usage refusals | `TestMigrationRatchetCLIHelperForms` |
| Record source spans re-read from Git; mismatch, unreadable, missing reader and shape refusals | `TestRecordSourceSpanIsReverifiedAgainstGit`, `TestMigrationRatchetCLIVerifiesRecordSourceSpans` |

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
| MER-V0-013..014 | `internal/migrationratchet/compare.go`, `internal/migrationratchet/seal.go` | `TestSealRepairsForeignEncodingAndNamesTheField`, `TestSealLeavesRuleArtifactBindingsReviewed`, `TestMigrationRatchetCLIHelperForms` |
| MER-V0-015 | `internal/migrationratchet/compare.go` | `TestPolicyAcceptsPartialAcrossComparisons` |
| MER-V0-016 | `internal/migrationratchet/schema.go` | `TestDescribeCoversDecoderAndRefusals`, `TestMigrationRatchetCLIHelperForms` |
| MER-V0-017 | `cmd/corvint/migration_ratchet.go`, `internal/breakagemap/build.go` | `TestMigrationRatchetCLIHelperForms`, `TestReadSpanDigestsPhysicalLines` |
| MER-V0-018 | `internal/migrationratchet/compare.go`, `cmd/corvint/migration_ratchet.go`, `internal/breakagemap/build.go` | `TestRecordSourceSpanIsReverifiedAgainstGit`, `TestMigrationRatchetCLIVerifiesRecordSourceSpans` |
| MER-V0-098 | `internal/migrationratchet/seal.go`, `cmd/corvint/migration_ratchet.go` | `TestMER098SealRefusesBadAcceptIncomparable` | experimental |
| MER-V0-099 | `cmd/corvint/migration_ratchet.go` | `TestMER099SourceRootUnavailableOutsideGit` | experimental |

### Owned emitted error codes

| Code | Emitted condition | Site |
|---|---|---|
| `broken-reverse-link` | A valid forward evidence link lacks its required reverse link. | `internal/migrationratchet/compare.go:633@7ea1221e` |
| `content-changed` | A paired record's content digest changes. | `internal/migrationratchet/compare.go:570@e09e30cf` |
| `evidence-not-renewed` | Evidence remains byte-identical after its linked target changes. | `internal/migrationratchet/compare.go:641@18d18ed9` |
| `profile-unavailable` | The bounded migration profile cannot be read. | `cmd/corvint/migration_ratchet.go:118@85139ca6` |
| `receipt-write-failed` | The encoded comparison receipt cannot be written to stdout. | `cmd/corvint/migration_ratchet.go:170@3f9aa3f3` |
| `output-encoding-failed` | A schema, seal or span-digest result cannot be encoded. | `cmd/corvint/migration_ratchet.go:276@26ccc007` |
| `output-write-failed` | A schema, seal or span-digest result cannot be written to stdout. | `cmd/corvint/migration_ratchet.go:280@b73cf58d` |
| `rename-masquerade` | A removal and addition of one kind retain the same content digest under different identities. | `internal/migrationratchet/compare.go:613@cfba470a` |
| `seal-input-unavailable` | The bounded profile or snapshot to seal cannot be read. | `cmd/corvint/migration_ratchet.go:205@6256bf2e` |
| `source-root-unavailable` | A profile names record source spans but the repository root cannot be resolved. | `cmd/corvint/migration_ratchet.go:134@3f58cb72` |
| `span-root-unavailable` | The repository root for span-digest cannot be resolved. | `cmd/corvint/migration_ratchet.go:239@80d0f010` |
| `stale-link` | A link target is missing or its content digest no longer matches. | `internal/migrationratchet/compare.go:629@6bdd70f8` |
| `state-uncomparable` | A state transition names a state absent from the reviewed order. | `internal/migrationratchet/compare.go:587@42731da5` |

## Unresolved decisions and promotion criteria

Promotion requires a real repository baseline/candidate pair, repository-owner review of its policy,
an independent integrated review, the full clean-tree gate and dogfood completion. Corpus storage is
an attributed consumer of the receipt, not a new authority source. Authenticated reviewer identity or
automatic snapshot discovery requires a separate accepted contract.
