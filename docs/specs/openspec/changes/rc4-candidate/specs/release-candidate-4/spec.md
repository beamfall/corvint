## ADDED Requirements

### Requirement: RCF-V1-001 rc.4 is the M5a qualification candidate

The M5a qualification candidate tag MUST be `v1.0.0-rc.4`, and its commit SHA SHALL be the seed of
the held-out draw (REL-V1-002). It MUST be published only as a GitHub prerelease, and only after
the owner approves the protected publish job (REL-V1-005 to REL-V1-007).

#### Scenario: rc.4 seeds the draw

- **GIVEN** the readiness record for commit C has every row `PASS` or `WAIVED`
- **WHEN** the owner pushes `v1.0.0-rc.4` at C
- **THEN** the held-out draw is seeded from C and recorded with the tag
- **AND** the release is marked as a prerelease, never as latest stable

### Requirement: RCF-V1-002 Candidate numbering after rc.4

A qualification rebuild for a blocker MUST take the next unused `rc` number and SHALL cite its
blocker ticket in the release notes. The M8 release candidate MUST take the next unused number
after the last qualification rebuild. A number SHALL never be reused or moved to another commit.

#### Scenario: Blocker rebuild

- **GIVEN** `v1.0.0-rc.4` is published and qualification finds blocker ticket B
- **WHEN** the fix is merged and a rebuild is tagged
- **THEN** the tag is `v1.0.0-rc.5` and its release notes cite B
- **AND** the held-out draw seeded by rc.4 is not redrawn

### Requirement: RCF-V1-003 Readiness record before the tag

Before `v1.0.0-rc.4` is pushed, a readiness record for the candidate commit MUST list each item
below with `PASS`, `WAIVED` (quoting the owner's words and date) or `NOT_RUN`. The tag MUST NOT be
pushed while any row is `NOT_RUN` or `FAIL`. Items: M4 merged (H7); removals done; flow links signed
(H6); held-out pools sealed (PRR-V1-004); release gates passing on the commit (V1-3177, V1-3055,
V1-4850); tag workflow build job (REL-V1-005); three-target `binaries.json` (REL-V1-008); core-set
archives installable by `corvint-update` (V1-5473); owner decision RCF-D1 (keep `corvint-mcp`)
applied, with V1-5545 retired; the candidate-byte linux/amd64 qualification workflow (V1-0374);
commit authorship on `origin/main` fixed or waived by the owner (V1-5621); RCF-V1-006 to
RCF-V1-011 delivered or explicitly deferred by the owner.

#### Scenario: Unready item holds the tag

- **GIVEN** a readiness record whose release-gates row is `FAIL`
- **WHEN** the tag is proposed
- **THEN** the tag is not pushed and the record names the failing gate and its ticket

### Requirement: RCF-V1-004 Untouched-repository validation on published bytes

V1-0019 MUST run on the archives published for `v1.0.0-rc.4`, verified against `SHA256SUMS`, and
its result SHALL be recorded in the rc.4 release notes as `PASS`, `FAIL` or `NOT_RUN` with a reason.
A `FAIL` or `NOT_RUN` MUST NOT be reported as a pass and SHALL block stable.

#### Scenario: Validation not run

- **GIVEN** rc.4 is published and V1-0019 has not run
- **WHEN** the release notes are read
- **THEN** they report V1-0019 as `NOT_RUN` with the reason, and stable stays blocked

### Requirement: RCF-V1-005 Release notes name promotions and previews

The rc.4 release notes MUST list every surface whose stability or delivery status changed since
rc.3, and SHALL list the docs surfaces that stay `experimental` or `preview`.

#### Scenario: Promotion listed

- **GIVEN** RCF-V1-011 has landed before the rc.4 tag
- **WHEN** the rc.4 release notes are generated
- **THEN** they list the three promoted tools, the four CLI verbs and the corpus format as changed
- **AND** they list generation, flows and claims as still `experimental`

### Requirement: RCF-V1-006 Docs search promotion scope

The promotion MUST cover exactly the CLI verbs `corvint-docs docs corpus build`, `search`, `get`
and `info`; the MCP tools `corvint.docs_search`, `corvint.docs_get` and `corvint.docs_info` on
`corvint-docs mcp`; and the artifact format `corvint-evidence-corpus/2`. Every other docs verb,
tool and format SHALL keep its current delivery and stability label.

#### Scenario: Out-of-scope tool stays experimental

- **GIVEN** the promotion commit has landed
- **WHEN** the delivery status of `corvint.docs_claims` is read
- **THEN** it is unchanged from before the promotion

### Requirement: RCF-V1-007 Frozen qualification query set

Before the first qualification run, a query set MUST be committed with its SHA-256 digest: 25
queries for each of Corvint, Beamfall and one public repository from the pinned proof set, each
pinned to a commit. Queries SHALL be file lookups, symbol lookups and prose questions. The set MUST
NOT change after the first run; a change starts a new set and a new run.

#### Scenario: Query edited after a run

- **GIVEN** a qualification run recorded against query-set digest D
- **WHEN** one query is edited
- **THEN** the digest changes, and evidence recorded against D does not count for the new set

### Requirement: RCF-V1-008 Verified, consistent and read-only qualification

For every query in the frozen set, the qualification MUST run the CLI and the MCP tool against the
same corpus artifact and SHALL require all of the following:

- every returned passage verifies against its pinned blob;
- the CLI and MCP records are byte-equal after canonical JSON encoding (PROD-DOCS-010);
- output stays within the documented bounds;
- `git status --porcelain` of each repository is unchanged.

One failure in any query MUST fail the run. Beamfall evidence SHALL keep only digests and counts.

#### Scenario: One unverified passage

- **GIVEN** 75 queries where one returned passage does not match its pinned blob
- **WHEN** the qualification completes
- **THEN** the run is `FAIL`, names the query and passage, and the promotion is blocked

#### Scenario: Surfaces disagree

- **GIVEN** a query where `docs corpus search` and `corvint.docs_search` return different records
- **WHEN** the records are compared
- **THEN** the run is `FAIL` and names the query

### Requirement: RCF-V1-009 Ranking evaluation floor

The DCS-V1-404 ranking evaluation MUST pass on the promotion commit in two separate runs, and the
two runs SHALL report identical ranks.

#### Scenario: Unstable ranks

- **GIVEN** two DCS-V1-404 runs on one commit
- **WHEN** their ranks differ for any query
- **THEN** the promotion is blocked and the difference is recorded

### Requirement: RCF-V1-010 Rerun on change

A change to ranking, corpus build, the corpus format, or any promoted verb or tool after a passing
qualification MUST rerun RCF-V1-008 and RCF-V1-009 before the promotion lands or before the next
candidate tag. This includes changes to pagination and the construction of result citations.

#### Scenario: Late ranking fix

- **GIVEN** a passing qualification at commit A
- **WHEN** a ranking change lands at commit B
- **THEN** evidence from A does not count for B until both runs pass at B

### Requirement: RCF-V1-011 Status and stability updates in one commit

When RCF-V1-008 and RCF-V1-009 pass, one commit MUST set the promoted scope's delivery status to
`validated` naming the qualification build-log entry; change the `README.md` row for the promoted
scope; set PROD-DOCS-009's state; and mark the three tools' input and output schemas and the
corpus format `stable` in `tools/compatibility-table/surfaces.json` with contract tests
(CMP-V1-002). Result order SHALL stay `preview`. Independent external utility MUST be reported
`EXTERNAL_DEPENDENT` and `NOT_RUN`, never as passed.

#### Scenario: Promotion lands

- **GIVEN** the qualification and both ranking runs pass
- **WHEN** the promotion commit lands
- **THEN** delivery status is `validated` and the three schemas and the format are `stable`
- **AND** result order is still `preview` in `docs/COMPATIBILITY.md`

### Requirement: RCF-V1-012 Rollback

Before rc.4 is published, reverting the promotion commit MUST restore every changed label with no
data migration. After publication, removing or narrowing a promoted `stable` surface SHALL follow
the CMP-V1-004 notice rule.

#### Scenario: Revert before publish

- **GIVEN** the promotion commit has landed and rc.4 is not published
- **WHEN** the commit is reverted
- **THEN** `docs/COMPATIBILITY.md` and the corpus specs show their earlier labels and checks pass

### Requirement: RCF-V1-013 Release-candidate qualification currency

The release-candidate currency check MUST derive trigger patterns from the owner entries in
`.corvint/test-selection.json` naming `cmd/corvint-docs/rcf_qualification_test.go` at both the
qualified and candidate commits. Those owners MUST cover every non-test Go source file in the
repository packages serving the promoted CLI verbs and MCP tools, including their transitive
implementation dependencies. Use package-wide patterns so a newly added file is covered.
When adding a package dependency, extend the owners to cover it. The gate MUST use the
existing owner-pattern dialect and include deleted and renamed paths. Missing or invalid
selection or qualification pins MUST fail closed.
The committed `docs/specs/rcf-qualification.json` MUST pin the last qualified commit and report
SHA-256. With no trigger diff and unchanged selection, the check MAY reuse that pinned evidence.
Otherwise it MUST require `CORVINT_RCF_QUALIFY`, run `TestRCFV1008Qualification` uncached at the
clean candidate checkout, observe that named test pass, and validate a fresh PASS report against
the candidate's frozen query-set digest. It MUST also observe two uncached passes of
`TestDCSV1404SearchRankingEvalFixture`, which checks identical ranks across evaluations.
Missing configuration, a skipped test, failed execution, missing or stale report, or a changed
checkout MUST produce a named nonzero failure, never PASS. The gate's retained output MUST bind
the report digest to the candidate commit; the existing qualification report format is unchanged.
Only a dispatched release-candidate gate SHALL invoke the check, never ordinary main PRs.

#### Scenario: Stale qualification holds the candidate

- GIVEN a ranking or pagination change after the pinned qualification
- WHEN the release-candidate currency check has no live configuration or fresh passing report
- THEN it fails with the missing evidence named, even if ordinary package tests passed

#### Scenario: Unchanged promoted scope

- GIVEN a valid qualification pin and selection at both immutable commits
- WHEN no trigger path or selection declaration changed
- THEN the check passes and names the reused commit and report digest
