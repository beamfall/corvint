# V1-1945, V1-1946 and V1-1949: obligation ledger gate hardening

Date: 2026-10-19
Tickets: V1-1945, V1-1946 and V1-1949 (GitHub issue beamfall/corvint#1202).
Requirements: TOL-V0-100..109 (`docs/specs/corvint-tasks-obligation-ledger-v0.md`).
They are accepted under the owner's standing auto-accept of lane requirements (2026-10-11). No
accepted requirement is amended.

## Decisions

- **Resolve at the reviewed commit (V1-1945 a).** `reviewedSubject` reads the reviewed file at the
  review's own commit in the repository the review read, never at `HEAD`. When that commit is gone
  it names `REVIEWED_COMMIT_UNAVAILABLE`; a missing, oversized or different blob has its own
  reason. It is additive output and never changes a verdict, item or exclusion.
- **Undeclared cite labels refused only when opted in (V1-1945 b).** TOL-V0-079 says a label is not
  checked at write, and a non-goal names that verification. Refusing by default would amend an
  accepted requirement, so `obligations.requireDeclaredCites` (default false) refuses with
  `OBLIGATION_CITE_REPOSITORY_UNDECLARED:`, following the TOL-V0-082 precedent for an opt-in
  stricter write. The default stays unchecked; that is a recorded limit, not a delivered default.
- **Classifier made configurable (V1-1945 c).** `obligations.testFilePatterns` extends the closed
  naming set; the closed set is still tried first, so no existing classification changes.
- **Unborn test-repository HEAD (V1-1946).** An unborn `HEAD` now reads as
  `TEST_REPOSITORY_UNRESOLVED` with the credit withheld instead of `stale:true`. The store's own
  repository keeps its existing behaviour; that case is a residual.
- **Plan witnesses (V1-1949 a, b).** `plan --witnesses` names every ledger witness with its test
  type, refuses a stale reviewed witness as `STALE_REVIEW`, and names on each `UNASSIGNED` row the
  witness considered and why it was insufficient. A `WITNESSED` row was already never `UNASSIGNED`;
  the issue's symptom came from external plan scripts that counted only browser reports. Output
  without the flag keeps its exact bytes. `preflight --plan` does not take the flag
  (`NOT_PRODUCED`, residual).
- **Capability at the environment baseline (V1-1949 c).** `obligations.environmentBaselines` names
  a ref or object id per repository, and `ticket obligations capability` answers only from that
  commit, abstaining `UNKNOWN` when it is undeclared or unresolved. It never fetches and never
  falls back to `HEAD`.

## TOL-V0-079/080/084 traceability (V1-1945 d)

PR #1203's dogfood-check reported two requirements without linked tests but did not name them, and
the original output was not retained. A scan of the TOL-V0-079..088 tests found TOL-V0-079 and
TOL-V0-084 only inside range comments and TOL-V0-080 with no anchor at all. Exact-ID tests now
exist for all three (`TestTOLV0079_CitesBoundsReplaceAndDefault`, `TestTOLV0080_ScopeShape`,
`TestTOLV0084_CandidateShapeAndClear`). Which two of the three the check counted is inferred, not
observed.

## Conformance fix found by the TOL-V0-080 test

TOL-V0-080 accepts a scope decision evidence reference of 1..512 bytes. The implementation bounded
it with the 256-byte witness evidence constant, so a 257..512-byte reference was refused. A new
`MaxObligationScopeEvidenceBytes = 512` now bounds both scope readers; the witness evidence bound is
unchanged. The spec was not amended: the code now matches it. An older binary refuses a ledger
holding such a reference (fail-closed); see the spec's rollback paragraph.

## Mutation evidence

Twelve mutations, each applied alone and reverted, were killed by the named test:

| Mutation | Killed by |
| --- | --- |
| Unborn test-repository `HEAD` treated as a commit | `TestTOLV0100_UnbornTestRepositoryHeadUnresolved` |
| `requireDeclaredCites` check removed | `TestTOLV0102_DeclaredCitesRequired` |
| Declared patterns ignored | `TestTOLV0103_DeclaredTestFilePatterns` |
| `STALE_REVIEW` findings dropped | `TestTOLV0105_PlanStaleReviewedWitness` |
| Capability answered at `HEAD` | `TestTOLV0107_CapabilityAtEnvironmentBaseline` |
| Reviewed subject read at `HEAD` | `TestTOLV0101_ReviewedSubjectAtReviewedCommit` |
| `REVIEW_HELD` reported as `NO_LEDGER_WITNESS` | `TestTOLV0106_UnassignedNamesConsideredWitness` |
| Scope evidence bound 512 back to 256 | `TestTOLV0080_ScopeShape` |
| Declared witness test type lost | `TestTOLV0104_PlanCountsLedgerWitnesses` |
| Baseline ref validation disabled | `TestTOLV0102_103_107_ObligationsPolicyMembers` |
| `truncated` ignores the file and blob bounds | `TestTOLV0108_CapabilityBoundsAndReadOnly` |
| `SEARCH_BOUND_EXCEEDED` abstention removed | `TestTOLV0108_CapabilityBoundsAndReadOnly` |

## Independent review (one Codex round)

`codex exec -s read-only` reviewed `d283682dd..23cc352d8` and reported six findings:

1. The capability listing streams the whole tree after 1,024 files. Recorded: memory stays bounded
   (paths beyond the bound are not retained, and the listing is a streamed `ls-tree`), but time
   grows with the tree, as for the boundary inventory. The spec's bound is on files examined.
2. `truncated` ignored the file and blob bounds, so a match beside an unexamined file read as
   complete. Fixed; TOL-V0-108 now names that case and `TestTOLV0108_CapabilityBoundsAndReadOnly`
   covers it.
3. A tree-listing or blob-read failure kept the resolved baseline commit, against TOL-V0-109.
   Fixed: `commit` is null on that abstention. No fixture damages a tree object (`NOT_RUN`).
4. The store's own unborn `HEAD` still reads a reviewed witness as stale (and `REVIEW_STALE_AT_HEAD`
   in `plan --witnesses`). Recorded as a residual: accepted TOL-V0-077 defines staleness for the
   store worktree as the path being absent at `HEAD`, and credit is withheld either way. Changing
   it would amend TOL-V0-077.
5. Count bounds on `testFilePatterns` and `environmentBaselines` refuse `LIMIT_EXCEEDED`, while the
   new requirement text said `MALFORMED`. Fixed in the new requirement text to match the existing
   `externalReviews` convention; the policy test now asserts each code.
6. Missing coverage for the capability bounds and read-only claim: added
   `TestTOLV0108_CapabilityBoundsAndReadOnly`. The three damaged-object-store subject reasons stay
   without fixtures and are listed `NOT_RUN` in the acceptance row.

## Process deviation

The lane-start `make dogfood-change` was skipped to keep the two-try dogfood budget for the bound
change; `corvint affected --base 714393264114757adf858187dc0d5913e9830c57` ran at lane start
(rc 0). Integrated evidence (a live deployment ref, a non-browser adopter plan) is `NOT_RUN`.
