# Release candidate 4 V1

Owner: Russell Lewis
Date: 2026-11-18
Requirement prefix: `RCF-V1`
Intent status: accepted by the owner on 2026-11-18
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/rc4-candidate/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/release-candidate-4/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-DOCS-001, PROD-DOCS-009, PROD-DOCS-010, PROD-INT-012 to PROD-INT-014),
`docs/specs/release-process-v1.md` (REL-V1) and the change folder
`docs/specs/openspec/changes/proof-repositories/` (PRR-V1, the pinned proof set).

## Agent digest
- Claim: rc.4 is the M5a candidate; docs corpus search leaves experimental after a frozen run.
- Status: accepted by the owner on 2026-11-18 / partial
- Exists: the frozen query set and its digest, the qualification runner, its rerun trigger.
- Blocked on: the qualification run (task 2.3), the promotion commit, the readiness record.
- Read next: Requirements; Delivery and traceability; Rollback.

## User and boundary

The owner wants the next release candidate named and bounded, and the documentation corpus search
read path out of `experimental` (request of 2026-11-18). This spec is the canonical home of
RCF-V1-001 to RCF-V1-012, copied from the change's spec delta. Decisions, failure modes and risks
stay in the change's `design.md`, which owns them. Ticket: V1-5679.

Non-goals (from the change): no promotion of generation, flows, claims, drafts, navigation,
journeys or any `docs_*` tool other than `corvint.docs_search`, `corvint.docs_get` and
`corvint.docs_info`; no ranking-order promise, so result order stays `preview`; no new release
mechanics; no publishing by anyone but the owner; no claim of independent validation, market
usefulness or adoption from owner-run evidence.

Trust boundary: the qualification is owner-run and measures correctness (verified, bounded,
consistent), not usefulness. Independent external utility stays an `EXTERNAL_DEPENDENT` gate,
reported `NOT_RUN`. Beamfall evidence keeps only digests and counts, never passage text.

## Requirements

Requirement IDs are stable. Each requirement keeps the change's text.

- `RCF-V1-001`: (accepted by the owner, 2026-11-18) rc.4 is the M5a qualification candidate

The M5a qualification candidate tag MUST be `v1.0.0-rc.4`, and its commit SHA SHALL be the seed of
the held-out draw (REL-V1-002). It MUST be published only as a GitHub prerelease, and only after
the owner approves the protected publish job (REL-V1-005 to REL-V1-007).

**Scenario: rc.4 seeds the draw**

- GIVEN the readiness record for commit C has every row `PASS` or `WAIVED`
- WHEN the owner pushes `v1.0.0-rc.4` at C
- THEN the held-out draw is seeded from C and recorded with the tag
- AND the release is marked as a prerelease, never as latest stable

- `RCF-V1-002`: (accepted by the owner, 2026-11-18) Candidate numbering after rc.4

A qualification rebuild for a blocker MUST take the next unused `rc` number and SHALL cite its
blocker ticket in the release notes. The M8 release candidate MUST take the next unused number
after the last qualification rebuild. A number SHALL never be reused or moved to another commit.

**Scenario: Blocker rebuild**

- GIVEN `v1.0.0-rc.4` is published and qualification finds blocker ticket B
- WHEN the fix is merged and a rebuild is tagged
- THEN the tag is `v1.0.0-rc.5` and its release notes cite B
- AND the held-out draw seeded by rc.4 is not redrawn

- `RCF-V1-003`: (accepted by the owner, 2026-11-18) Readiness record before the tag

Before `v1.0.0-rc.4` is pushed, a readiness record for the candidate commit MUST list each item
below with `PASS`, `WAIVED` (quoting the owner's words and date) or `NOT_RUN`. The tag MUST NOT be
pushed while any row is `NOT_RUN` or `FAIL`. Items: M4 merged (H7); removals done; flow links signed
(H6); held-out pools sealed (PRR-V1-004); release gates passing on the commit (V1-3177, V1-3055,
V1-4850); tag workflow build job (REL-V1-005); three-target `binaries.json` (REL-V1-008); core-set
archives installable by `corvint-update` (V1-5473); owner decision RCF-D1 (keep `corvint-mcp`)
applied, with V1-5545 retired; the candidate-byte linux/amd64 qualification workflow (V1-0374);
commit authorship on `origin/main` fixed or waived by the owner (V1-5621); RCF-V1-006 to
RCF-V1-011 delivered or explicitly deferred by the owner.

**Scenario: Unready item holds the tag**

- GIVEN a readiness record whose release-gates row is `FAIL`
- WHEN the tag is proposed
- THEN the tag is not pushed and the record names the failing gate and its ticket

- `RCF-V1-004`: (accepted by the owner, 2026-11-18) Untouched-repository validation on published
  bytes

V1-0019 MUST run on the archives published for `v1.0.0-rc.4`, verified against `SHA256SUMS`, and
its result SHALL be recorded in the rc.4 release notes as `PASS`, `FAIL` or `NOT_RUN` with a reason.
A `FAIL` or `NOT_RUN` MUST NOT be reported as a pass and SHALL block stable.

**Scenario: Validation not run**

- GIVEN rc.4 is published and V1-0019 has not run
- WHEN the release notes are read
- THEN they report V1-0019 as `NOT_RUN` with the reason, and stable stays blocked

- `RCF-V1-005`: (accepted by the owner, 2026-11-18) Release notes name promotions and previews

The rc.4 release notes MUST list every surface whose stability or delivery status changed since
rc.3, and SHALL list the docs surfaces that stay `experimental` or `preview`.

**Scenario: Promotion listed**

- GIVEN RCF-V1-011 has landed before the rc.4 tag
- WHEN the rc.4 release notes are generated
- THEN they list the three promoted tools, the four CLI verbs and the corpus format as changed
- AND they list generation, flows and claims as still `experimental`

- `RCF-V1-006`: (accepted by the owner, 2026-11-18) Docs search promotion scope

The promotion MUST cover exactly the CLI verbs `corvint-docs docs corpus build`, `search`, `get`
and `info`; the MCP tools `corvint.docs_search`, `corvint.docs_get` and `corvint.docs_info` on
`corvint-docs mcp`; and the artifact format `corvint-evidence-corpus/2`. Every other docs verb,
tool and format SHALL keep its current delivery and stability label.

**Scenario: Out-of-scope tool stays experimental**

- GIVEN the promotion commit has landed
- WHEN the delivery status of `corvint.docs_claims` is read
- THEN it is unchanged from before the promotion

- `RCF-V1-007`: (accepted by the owner, 2026-11-18) Frozen qualification query set

Before the first qualification run, a query set MUST be committed with its SHA-256 digest: 25
queries for each of Corvint, Beamfall and one public repository from the pinned proof set, each
pinned to a commit. Queries SHALL be file lookups, symbol lookups and prose questions. The set MUST
NOT change after the first run; a change starts a new set and a new run.

**Scenario: Query edited after a run**

- GIVEN a qualification run recorded against query-set digest D
- WHEN one query is edited
- THEN the digest changes, and evidence recorded against D does not count for the new set

- `RCF-V1-008`: (accepted by the owner, 2026-11-18) Verified, consistent and read-only
  qualification

For every query in the frozen set, the qualification MUST run the CLI and the MCP tool against the
same corpus artifact and SHALL require all of the following:

- every returned passage verifies against its pinned blob;
- the CLI and MCP records are byte-equal after canonical JSON encoding (PROD-DOCS-010);
- output stays within the documented bounds;
- `git status --porcelain` of each repository is unchanged.

One failure in any query MUST fail the run. Beamfall evidence SHALL keep only digests and counts.

**Scenario: One unverified passage**

- GIVEN 75 queries where one returned passage does not match its pinned blob
- WHEN the qualification completes
- THEN the run is `FAIL`, names the query and passage, and the promotion is blocked

**Scenario: Surfaces disagree**

- GIVEN a query where `docs corpus search` and `corvint.docs_search` return different records
- WHEN the records are compared
- THEN the run is `FAIL` and names the query

- `RCF-V1-009`: (accepted by the owner, 2026-11-18) Ranking evaluation floor

The DCS-V1-404 ranking evaluation MUST pass on the promotion commit in two separate runs, and the
two runs SHALL report identical ranks.

**Scenario: Unstable ranks**

- GIVEN two DCS-V1-404 runs on one commit
- WHEN their ranks differ for any query
- THEN the promotion is blocked and the difference is recorded

- `RCF-V1-010`: (accepted by the owner, 2026-11-18) Rerun on change

A change to ranking, corpus build, the corpus format, or any promoted verb or tool after a passing
qualification MUST rerun RCF-V1-008 and RCF-V1-009 before the promotion lands or before the next
candidate tag.

**Scenario: Late ranking fix**

- GIVEN a passing qualification at commit A
- WHEN a ranking change lands at commit B
- THEN evidence from A does not count for B until both runs pass at B

- `RCF-V1-011`: (accepted by the owner, 2026-11-18) Status and stability updates in one commit

When RCF-V1-008 and RCF-V1-009 pass, one commit MUST set the promoted scope's delivery status to
`validated` naming the qualification build-log entry; change the `README.md` row for the promoted
scope; set PROD-DOCS-009's state; and mark the three tools' input and output schemas and the
corpus format `stable` in `tools/compatibility-table/surfaces.json` with contract tests
(CMP-V1-002). Result order SHALL stay `preview`. Independent external utility MUST be reported
`EXTERNAL_DEPENDENT` and `NOT_RUN`, never as passed.

**Scenario: Promotion lands**

- GIVEN the qualification and both ranking runs pass
- WHEN the promotion commit lands
- THEN delivery status is `validated` and the three schemas and the format are `stable`
- AND result order is still `preview` in `docs/COMPATIBILITY.md`

- `RCF-V1-012`: (accepted by the owner, 2026-11-18) Rollback

Before rc.4 is published, reverting the promotion commit MUST restore every changed label with no
data migration. After publication, removing or narrowing a promoted `stable` surface SHALL follow
the CMP-V1-004 notice rule.

**Scenario: Revert before publish**

- GIVEN the promotion commit has landed and rc.4 is not published
- WHEN the commit is reverted
- THEN `docs/COMPATIBILITY.md` and the corpus specs show their earlier labels and checks pass

## Delivery and traceability

Each entry names the task in `tasks.md`, then the requirements, implementation and evidence.

- Tasks 4.3 and 4.4 (RCF-V1-001, RCF-V1-002): NOT_PRODUCED until the owner tags rc.4.
- Task 4.1 (RCF-V1-003): done (RCF-D1 applied, V1-5545 COMPLETED).
- Task 4.2 (RCF-V1-003): first-pass record `docs/build-log/2026-11-18-rc4-readiness.md` is HOLD:
  rows M4, removals, flow links, held-out pools and V1-0374 are `NOT_RUN`, and release gates are
  `FAIL`. The task completes only when every row is `PASS` or `WAIVED` on the candidate commit.
- Task 4.4 (RCF-V1-004, RCF-V1-005): draft section "1.0.0-rc.4 release candidate" in
  `docs/RELEASE-NOTES.md`, with V1-0019 `NOT_RUN`. It is final once the tag is published.
- Task 3.2 (RCF-V1-006): delivery `validated` for exactly the promoted scope: the four CLI verbs,
  the three MCP tools and `corvint-evidence-corpus/2`. Every other docs verb, tool and format keeps
  its label (`docs/CAPABILITIES.md`, `docs/COMPATIBILITY.md`). The corpus spec headers are frozen
  and their specs still hold `standing` requirements (SRG-V0-010), so the scoped status is
  recorded here and in PROD-DOCS-009, not in those headers.
- Task 2.1 (RCF-V1-007): `cmd/corvint-docs/testdata/rcf-v1-qualification/queries.json`
  (Corvint, `beamfall/core` and `vuejs/core`, 25 queries each, each repository pinned
  to a commit) and its digest `queries.sha256`. Evidence: `TestRCFV1007FrozenQuerySet`.
- Task 2.2 (RCF-V1-008): `cmd/corvint-docs/rcf_qualification_test.go` (`rcfQualify`, and the
  opt-in run `TestRCFV1008Qualification` under `CORVINT_RCF_QUALIFY`). Evidence:
  `TestRCFV1008QualificationPassesOnAFixture`, `TestRCFV1008UnverifiedPassageFailsTheRun`,
  `TestRCFV1008SurfaceMismatchFailsTheRun`, `TestRCFV1008DigestOnlyReport`. The run itself
  (task 2.3) passed twice on `1f35e9f28` with byte-identical reports
  (`docs/build-log/2026-11-18-rc4-docs-qualification.md`).
- Task 2.3 (RCF-V1-009): two runs of `TestDCSV1404SearchRankingEvalFixture`, both PASS with equal
  ranks (`docs/build-log/2026-11-18-rc4-docs-qualification.md`).
- Task 2.4 (RCF-V1-010): owner rows in `.corvint/test-selection.json` select the runner and the
  DCS-V1-404 evaluation for ranking, corpus build, corpus format and promoted-surface paths.
  Evidence: `corvint affected` on a ranking diff names both test packages.
- Task 3.1 (RCF-V1-011, RCF-D2): `TestRCFV1011DocsToolSchemas`,
  `TestRCFV1011DocsToolOutputs` and `TestRCFV1011CorpusFormat` in
  `internal/mcp/corpusbridge/parity_test.go`, extending the existing typed schema and compact
  receipt fixtures. Field contracts live in `internal/mcp/corpusbridge/testdata/rcf-contract/`;
  they pin input/output fields and nested manifest/artifact types without ranking order.
  Schema comparison rejects unpinned constraints; runtime checks validate each member against
  closed record variants. Comparator regressions are `TestRCFV1011SchemaComparatorRejectsRestrictions`,
  `TestRCFV1011SchemaComparatorClosedForms` and `TestRCFV1011OutputComparatorChecksEveryMember`
  in `internal/mcp/corpusbridge/rcf_contract_test.go`.
  The corpus guard also pins scalar underlying kinds and checks representative JSON encodings.
  `TestRCFV1011ScalarContractClosedForms` and `TestRCFV1011AttemptStateJSON` in
  `internal/mcp/corpusbridge/rcf_scalar_test.go` reject named scalar retyping, unknown scalar
  forms and custom scalar encoders. Dynamic receipt results and raw JSON contents remain
  outside this static scalar check; the output fixtures above cover their exercised forms.
- Task 3.2 (RCF-V1-011): the promotion commit. `tools/compatibility-table/surfaces.json` rows
  `docs-search-mcp-schemas` and `evidence-corpus-format` are `stable` with the task 3.1 contract
  tests; `mcp-schemas` stays `preview` for every other tool and for result order (CMP-V1-003
  item 6 and CMP-V1-005 amended). `README.md` gains a `stable` Documentation search row;
  PROD-DOCS-009 is `validated`, `DELIVERED`. Independent external utility is
  `EXTERNAL_DEPENDENT` and `NOT_RUN`. Evidence: `make compatibility-check`,
  `make capabilities-check`, `make openspec-check`.
- Task 3.3 (RCF-V1-012): reverting the two promotion commits restores the pre-promotion tree
  byte for byte, and the compatibility, capabilities, openspec and spec-requirements checks pass
  on it (`docs/build-log/2026-11-18-rc4-docs-qualification.md`).

## Rollback

Before rc.4 is published: revert the promotion commit; the three surfaces return to `experimental`
and `preview`, and no data migration is needed. After publication: the surfaces are `stable`, so a
removal follows the CMP-V1-004 notice rule. Reverting the readiness record or the rc.4 numbering
changes no shipped bytes. Reverting the runner commit removes the query set, the runner and its
owner rows; the RCF-V1 IDs are then retired, not reused.
