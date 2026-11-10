# Corvint spec health report, 2026-11-09

Owner: Russell Lewis (repository owner). This report proposes classes. It changes no spec.
Date: 2026-11-09
Intent status: proposed
Delivery status: not-started

## Agent digest
- Claim: Each of the 168 surviving specs gets a proposed class of keep, merge, retire or outside 1.0, with its acceptance and test-citation gaps.
- Status: proposed / not-started
- Exists: this report; no spec was changed, merged, retired or split.
- Blocked on: the owner's decision on each proposed class and on `docs/specs/PRODUCT-PLAN-1.0.md`.
- Read next: `docs/specs/PRODUCT-PLAN-1.0.md` and `docs/decisions/PROPOSED-openspec-workflow.md`.

## What this report is

`docs/specs/PRODUCT-PLAN-1.0.md` proposes 43 product requirements for the 1.0 release. One of
them, PROD-INT-010, says that every surviving spec names the product requirement it serves, or is
retired, or is marked as outside 1.0. This report is the first pass at that requirement. It
proposes one class for each spec file under `docs/specs/` that is not retired.

`docs/specs/INDEX.json` lists 281 spec files at commit `1d9dc4c90`. 112 of them have the delivery
status `retired`. Of the other 169, one is the product plan itself. The remaining 168 are the
subject of this report. This report is itself a file under `docs/specs/` and is not counted.

Everything below is a proposal. No spec was edited, merged, retired or split, and no requirement ID
was renumbered or deleted. A class takes effect only when the owner decides it.

## The four classes

- KEEP. The spec serves at least one product requirement in the plan, and the table names which.
  Where the note says "New pairing", the plan does not name this spec yet; if the owner agrees with
  the pairing, the plan gains the reference. 34 of the 71 KEEP rows are new pairings.
- MERGE. The spec describes the same contract as another file and should move into it. Requirement
  IDs stay as they are: the definition lines move, and the old file is retired with a pointer to the
  new home. By the line counts in `docs/specs/REQUIREMENTS.tsv`, no proposed merge takes its target
  past the limit of 256 requirements for one spec file; the largest merged total is 112 lines.
- RETIRE. The spec describes something that was superseded or cut. Retiring means setting its
  delivery status to `retired` and recording the reason. The body and the requirement IDs stay in
  the file as history, as they do for the 112 specs already retired.
- OUT-OF-1.0. The spec stays as it is and may govern code that keeps running, but no product
  requirement in the plan depends on it, so its gaps do not block 1.0. The owner may instead
  decide that the subject belongs in 1.0; it then needs a product requirement in the plan. The
  groups are: kept command (a command that decision 0594 keeps but the plan does not name), change
  evidence (`corvint cem` and its companions), analyzers, experiment (measurement programmes and
  local ledgers), deferred, history (records of finished migrations) and repository gate (checks
  of this repository's own process).

## How the numbers were produced

- Intent and Delivery come from each spec's header lines as `docs/specs/INDEX.json` records them.
  The Intent column shortens the header to one of these: `accepted` (the header says accepted and
  names a decision, a pull request or an owner statement), `accepted, no decision named` (the header
  says accepted and names nothing), `standing` (the header rests only on the owner's standing
  auto-accept for lane work), `proposed`, `experimental`, `per requirement`, `retired`, `superseded`
  and `rejected`. This report did not open the cited decisions to check that they cover the whole
  file.
- Reqs is the number of requirement lines the spec has in `docs/specs/REQUIREMENTS.tsv`. Owner,
  Standing and None split those lines by the acceptance column of that file: `owner` (the line names
  an acceptance with a decision number or a date), `standing` (the line was written under the
  standing auto-accept and the owner has not read it back) and `unrecorded` (the line carries no
  acceptance note and inherits the header). `docs/SPEC-DRIVEN-DEVELOPMENT.md` defines the three
  modes.
- In tests counts the spec's requirement IDs that appear as text in at least one tracked test file.
  The test files are those whose names match `*_test.go`, `*_test.sh`, `*.test.ts`, `*.test.js`,
  `*.test.mjs`, `*_test.py` or `*_spec.rb`. A match shows that a test names the requirement. It does
  not show that the test checks it, and a test can check a requirement without naming it.
- Untraced counts the requirement IDs that appear in no test file and are also not listed in the
  spec's own section that maps requirements to tests or measured evidence, where the spec has one.
  These are the lines with no recorded evidence of either kind.
- A spec has one delivery status for the whole file. Where a header says `implemented`, this report
  treats every line of the file as claimed, because the header does not say which lines are meant.
- A statement that a command no longer exists comes from a text search for the command name in
  tracked Go files that are not tests. Rows that rest on such a search say "Verify first".
- No Go test was run and no spec body was read in full. The inputs were the index, the digests,
  `docs/specs/REQUIREMENTS.tsv`, the section headings of the largest files and targeted searches.

## Counts

| Class | Group | Specs | Reqs | Owner | Standing | None | In tests | Untraced |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| KEEP | testing | 31 | 1523 | 85 | 912 | 526 | 1097 | 39 |
| KEEP | flows and documentation | 23 | 1276 | 25 | 354 | 897 | 979 | 54 |
| KEEP | integration and release | 17 | 696 | 28 | 442 | 226 | 438 | 118 |
| MERGE | all | 19 | 196 | 4 | 106 | 86 | 119 | 23 |
| RETIRE | superseded | 7 | 127 | 0 | 0 | 127 | 24 | 57 |
| RETIRE | cut surface | 12 | 137 | 6 | 5 | 126 | 4 | 60 |
| OUT-OF-1.0 | kept command | 16 | 713 | 6 | 421 | 286 | 510 | 55 |
| OUT-OF-1.0 | change evidence | 10 | 567 | 157 | 206 | 204 | 178 | 184 |
| OUT-OF-1.0 | analyzers | 8 | 72 | 0 | 0 | 72 | 38 | 29 |
| OUT-OF-1.0 | experiment | 10 | 129 | 6 | 20 | 103 | 75 | 5 |
| OUT-OF-1.0 | deferred | 3 | 201 | 2 | 2 | 197 | 34 | 73 |
| OUT-OF-1.0 | history | 4 | 148 | 25 | 23 | 100 | 73 | 21 |
| OUT-OF-1.0 | repository gate | 8 | 94 | 3 | 29 | 62 | 44 | 3 |
| Total | all | 168 | 5879 | 347 | 2520 | 3012 | 3613 | 721 |

The four classes hold 71 KEEP, 19 MERGE, 19 RETIRE and 59 OUT-OF-1.0 specs. The 168 specs hold 5879
requirement lines. `docs/specs/REQUIREMENTS.tsv` holds 9005 lines in all: 43 belong to the product
plan, 5879 to the 168 surviving specs and 3083 to retired specs.

Of the 5879 lines, 347 record an owner acceptance, 2520 a standing acceptance and 3012 none. 3613
are named in a test file and 721 are untraced. The KEEP specs alone hold 3495 lines, of which 138
record an owner acceptance and 211 are untraced.

## KEEP: testing product

31 specs. They hold 1523 requirement lines, of which 39 are untraced.

| Spec | Serves | Intent | Delivery | Reqs | Owner | Standing | None | In tests | Untraced | Note |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `additive-narrowing-v0.md` | PROD-TEST-001, PROD-TEST-002 | standing | experimental | 15 | 0 | 15 | 0 | 11 | 0 | Narrows Go changes to importers that can be reached. |
| `affected-cli-consistency-v0.md` | PROD-TEST-001 | standing | experimental | 17 | 1 | 15 | 1 | 16 | 0 | One contract for flags, errors and path handling of `corvint affected`. New pairing. |
| `affected-edit-oracle-v0.md` | PROD-TEST-002 | accepted, no decision named | experimental | 8 | 0 | 8 | 0 | 0 | 0 | The measurement of misses and over-selection. No requirement ID appears in a test file. |
| `affected-plan-python-playwright-v0.md` | PROD-TEST-011 | per requirement | experimental | 48 | 0 | 48 | 0 | 45 | 0 | Python and Playwright selection under the affected-plan prefix. New pairing. |
| `affected-plan-reuse-v0.md` | PROD-TEST-012 | standing | experimental | 15 | 0 | 15 | 0 | 12 | 0 | `corvint test reuse` and the RUN or SKIP decision for CI jobs. New pairing. |
| `affected-plan-selection-v0.md` | PROD-TEST-001, PROD-TEST-002 | standing | experimental | 172 | 0 | 127 | 45 | 139 | 1 | Companion file under the same prefix as the affected plan. New pairing. |
| `affected-plan-v0.md` | PROD-TEST-001, PROD-TEST-002, PROD-TEST-012 | accepted | experimental | 256 | 28 | 181 | 47 | 214 | 0 | The plan contract. At the 256-requirement cap; a three-way split is proposed below. |
| `affected-zero-unknown-v0.md` | PROD-TEST-002 | experimental | experimental | 54 | 0 | 4 | 50 | 46 | 0 | Closes UNKNOWN results with sound selections or stated proofs. |
| `ci-narrowing-v0.md` | PROD-TEST-012 | standing | experimental | 51 | 8 | 35 | 8 | 39 | 5 | CI narrowing and its alarm for missing forward rows. |
| `dotnet-affected-selection-v0.md` | PROD-TEST-011 | proposed | experimental | 10 | 0 | 0 | 10 | 0 | 2 | .NET selection. Intent is proposed; no requirement ID appears in a test file. New pairing. |
| `external-evidence-provider-v0.md` | PROD-TEST-004 | accepted | experimental | 35 | 0 | 8 | 27 | 21 | 0 | The provider record format that the selection advice reads. New pairing; uncertain. |
| `external-test-selection-v0.md` | PROD-TEST-004 | accepted | experimental | 14 | 0 | 0 | 14 | 11 | 0 | The `--provider FILE` input that E2E selection reads today. New pairing. |
| `go-line-selection-v0.md` | PROD-TEST-008 | standing | experimental | 29 | 0 | 29 | 0 | 23 | 0 | Line-level narrowing for Go with fallback. |
| `go-live-test-provider-v0.md` | PROD-TEST-006, PROD-TEST-007 | accepted | experimental | 54 | 0 | 0 | 54 | 17 | 2 | The Go test provider binary. Every line is unrecorded for acceptance. New pairing. |
| `java-affected-test-v0.md` | PROD-TEST-011 | accepted, no decision named | experimental | 10 | 0 | 0 | 10 | 0 | 0 | Java selection. No requirement ID appears in a test file. New pairing. |
| `js-live-test-provider-v0.md` | PROD-TEST-006 | proposed | experimental | 0 | 0 | 0 | 0 | 0 | 0 | The JS and TS test provider binary. The file has no numbered requirements. New pairing. |
| `kotlin-jvm-affected-test-v0.md` | PROD-TEST-011 | proposed | experimental | 12 | 0 | 3 | 9 | 4 | 8 | Kotlin on the JVM. Intent is proposed. New pairing. |
| `language-plugins-v0.md` | PROD-TEST-011, PROD-INT-001 | accepted, no decision named | experimental | 27 | 0 | 27 | 0 | 10 | 2 | One manifest per language, read by the selector instead of naming languages. New pairing. |
| `live-proof-carrying-verification-v0.md` | PROD-TEST-007, PROD-TEST-008 | accepted | not-started | 190 | 40 | 92 | 58 | 130 | 1 | The watch contract. Header delivery says not-started although slices are implemented; 190 requirements. |
| `native-shell-affected-test-v0.md` | PROD-TEST-011 | accepted, no decision named | experimental | 17 | 0 | 2 | 15 | 7 | 0 | C-family and shell selection. New pairing. |
| `performance-profiling-and-read-budget-v0.md` | PROD-TEST-007, PROD-DOCS-001 | standing | experimental | 52 | 0 | 39 | 13 | 51 | 1 | Read budgets for commands and MCP servers. New pairing. |
| `process-group-owner-v0.md` | PROD-TEST-006 | proposed | experimental | 20 | 2 | 10 | 8 | 12 | 0 | Clean-up of child processes started by a test run. Intent is a proposed extraction. New pairing. |
| `revision-cache-dirty-worktree-v0.md` | PROD-TEST-007 | proposed | experimental | 22 | 0 | 3 | 19 | 12 | 0 | Reuse of the committed index on a dirty checkout, which a save-time answer needs. New pairing. |
| `ruby-affected-test-adapter-v0.md` | PROD-TEST-011 | proposed | experimental | 26 | 0 | 9 | 17 | 11 | 0 | Ruby selection. Intent is proposed. New pairing. |
| `runner-selection-plugins-v0.md` | PROD-TEST-011 | accepted | experimental | 62 | 2 | 60 | 0 | 39 | 3 | Runner plugins and the rule that an unqualified runner runs in full. |
| `rust-affected-test-understanding-v0.md` | PROD-TEST-011 | proposed | experimental | 10 | 0 | 0 | 10 | 0 | 0 | Rust selection. Intent is proposed; no requirement ID appears in a test file. New pairing. |
| `session-test-runner-v0.md` | PROD-TEST-006, PROD-TEST-012 | accepted | implemented | 51 | 0 | 24 | 27 | 48 | 0 | `corvint test`: plan, run the selection, shard, fall back. |
| `swift-affected-test-understanding-v0.md` | PROD-TEST-011 | proposed | experimental | 13 | 0 | 1 | 12 | 0 | 0 | Swift selection. Intent is proposed; no requirement ID appears in a test file. New pairing. |
| `test-runner-execution-v0.md` | PROD-TEST-006 | proposed | experimental | 75 | 0 | 30 | 45 | 47 | 13 | Overlaps `session-test-runner-v0.md`: two specs govern running tests. Intent is proposed. New pairing. |
| `test-selection-precision-v0.md` | PROD-TEST-001, PROD-TEST-002 | standing | experimental | 50 | 0 | 48 | 2 | 40 | 1 | Precision rules; intent is recorded only under the standing lane auto-accept. |
| `typescript-javascript-affected-adapter-v0.md` | PROD-TEST-011 | proposed | experimental | 108 | 4 | 79 | 25 | 92 | 0 | JavaScript and TypeScript selection. |

## KEEP: flows and documentation product

23 specs. They hold 1276 requirement lines, of which 54 are untraced. The flow model is listed here
although it also serves the testing product.

| Spec | Serves | Intent | Delivery | Reqs | Owner | Standing | None | In tests | Untraced | Note |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `application-flow-navigation-execution-v0.md` | PROD-DOCS-008 | accepted | experimental | 7 | 0 | 0 | 7 | 6 | 1 | Runs a bounded navigation packet through `corvint-web-flows`. New pairing; uncertain. |
| `application-flow-understanding-v1.md` | PROD-TEST-004, PROD-TEST-005, PROD-TEST-010, PROD-DOCS-004, PROD-DOCS-005, PROD-DOCS-008, PROD-INT-002 | accepted | partial | 100 | 4 | 2 | 94 | 86 | 13 | The flow model. Serves seven product requirements; 94 of 100 lines are unrecorded for acceptance. |
| `application-map-scenario-planner-v0.md` | PROD-DOCS-007 | accepted, no decision named | experimental | 10 | 0 | 0 | 10 | 10 | 0 | Its IDs are cited in `internal/appmap/site_knowledge_how.go`: the closest surviving contract for `corvint how`. New pairing. |
| `application-map-v0.md` | PROD-DOCS-006 | accepted, no decision named | experimental | 191 | 0 | 28 | 163 | 184 | 3 | The screen and route map; 191 requirements. |
| `application-navigation-ledger-v0.md` | PROD-DOCS-006 | accepted | experimental | 56 | 0 | 21 | 35 | 50 | 0 | The navigation edge ledger. |
| `documentation-ci-v0.md` | PROD-DOCS-003 | accepted | experimental | 11 | 0 | 3 | 8 | 10 | 1 | A CI policy for README-only changes. Its prefix DCI-V0 is easily confused with DCI-V1 of the corpus inventory. |
| `documentation-citation-gate-v0.md` | PROD-DOCS-003, PROD-INT-005 | accepted | implemented | 46 | 1 | 21 | 24 | 26 | 1 | The gate that fails when a cited source no longer matches. |
| `documentation-corpus-extraction-delta-v1.md` | PROD-DOCS-009 | proposed | experimental | 89 | 0 | 24 | 65 | 78 | 0 | Companion of the documentation corpus; intent is proposed. New pairing. |
| `documentation-corpus-inventory-v1.md` | PROD-DOCS-009 | proposed | experimental | 25 | 0 | 1 | 24 | 11 | 0 | Companion of the documentation corpus; intent is proposed. New pairing. |
| `documentation-corpus-search-ranking-v1.md` | PROD-DOCS-009 | proposed | experimental | 50 | 0 | 7 | 43 | 44 | 0 | Companion of the documentation corpus; intent is proposed. New pairing. |
| `documentation-corpus-v1.md` | PROD-DOCS-009 | proposed | experimental | 257 | 0 | 176 | 81 | 197 | 1 | The documentation corpus; 257 requirements; intent is proposed. |
| `documentation-corpus-verified-reads-v1.md` | PROD-DOCS-009 | proposed | experimental | 49 | 0 | 8 | 41 | 31 | 1 | Companion of the documentation corpus; intent is proposed. New pairing. |
| `flow-document-maintenance-v0.md` | PROD-DOCS-003, PROD-DOCS-005 | accepted | experimental | 7 | 0 | 0 | 7 | 4 | 3 | Maintenance of generated flow pages. |
| `human-documentation-compiler-v0.md` | PROD-DOCS-002 | accepted | partial | 66 | 0 | 0 | 66 | 3 | 0 | The documentation compiler; only 3 of 66 requirement IDs appear in a test file. |
| `map-coverage-join-v0.md` | PROD-DOCS-006, PROD-TEST-004 | accepted, no decision named | partial | 25 | 0 | 1 | 24 | 11 | 9 | Joins functionality to code and tests. Depends on two retired specs. |
| `mcp-multi-root-v0.md` | PROD-DOCS-001, PROD-DOCS-010, PROD-TEST-009 | accepted | experimental | 17 | 9 | 8 | 0 | 16 | 0 | One MCP process for several repositories; also the home of the MCP affected tool. |
| `mcp-server-2026-07-28-v0.md` | PROD-DOCS-001 | accepted | implemented | 67 | 5 | 19 | 43 | 41 | 0 | The local MCP server. |
| `playwright-external-provider-v0.md` | PROD-DOCS-008 | accepted, no decision named | validated in part | 85 | 6 | 29 | 50 | 73 | 2 | Playwright receipts from an external server. New pairing. |
| `playwright-run-evidence-v0.md` | PROD-DOCS-008, PROD-TEST-010 | accepted | experimental | 74 | 0 | 3 | 71 | 57 | 17 | Reads Playwright JSON reports across runs. |
| `run-verified-navigation-v0.md` | PROD-DOCS-008 | accepted, no decision named | experimental | 8 | 0 | 0 | 8 | 8 | 0 | Marks a map step verified, unverified at head or contradicted from run receipts. New pairing. |
| `source-documentation-draft-v0.md` | PROD-DOCS-002 | proposed | experimental | 23 | 0 | 1 | 22 | 23 | 0 | Drafts source-pinned Markdown. Intent is proposed. |
| `trusted-project-navigation-v0.md` | PROD-DOCS-002 | accepted | experimental | 6 | 0 | 0 | 6 | 4 | 2 | Validates navigation proposals for a trusted MkDocs project. New pairing; uncertain. |
| `workspace-declaration-v0.md` | PROD-DOCS-006 | standing | experimental | 7 | 0 | 2 | 5 | 6 | 0 | The workspace file that the map build reads. Its claim names `corvint-test e2e`; no such binary exists under `cmd/`. New pairing. |

## KEEP: integration and release

17 specs. They hold 696 requirement lines, of which 118 are untraced. No spec in this repository is
proposed as KEEP for the task manager product; finding 2 below explains why.

| Spec | Serves | Intent | Delivery | Reqs | Owner | Standing | None | In tests | Untraced | Note |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `capability-catalog-v0.md` | PROD-INT-003 | standing | experimental | 71 | 3 | 66 | 2 | 42 | 1 | The catalogue of commands and MCP tools. |
| `core-bundled-apps-v0.md` | PROD-INT-004, PROD-DOCS-011 | accepted | in-progress | 11 | 0 | 0 | 11 | 2 | 9 | Speaks of a core with bundled apps; written before the five layers were decided. 2 of 11 IDs in tests. |
| `core-compatibility-freeze-v1.md` | PROD-INT-007 | accepted | experimental | 13 | 3 | 3 | 7 | 12 | 0 | Freezes twelve Core verbs from decision 0332; written before decision 0594 changed the verb list. |
| `corvint-1.0-product-and-release-v1.md` | PROD-INT-008 | accepted | partial | 25 | 0 | 2 | 23 | 6 | 0 | The earlier 1.0 contract. The owner decides whether the product plan amends or supersedes it. |
| `corvint-self-development-v0.md` | PROD-INT-009 | accepted | experimental | 256 | 3 | 236 | 17 | 177 | 79 | Self-use. At the 256-requirement cap with 79 untraced lines; lines that name removed dogfood surface need a pass. |
| `diagnostic-repair-contract-v0.md` | PROD-INT-007 | accepted | implemented | 13 | 0 | 1 | 12 | 11 | 0 | The refusal envelope shared by commands. New pairing. |
| `go-archive-gate-v0.md` | PROD-INT-006 | accepted | implemented | 13 | 0 | 5 | 8 | 9 | 0 | Rebuilds the release archives in the gate. New pairing. |
| `import-ratchet-v0.md` | PROD-INT-001, PROD-DOCS-011 | experimental | experimental | 6 | 0 | 0 | 6 | 6 | 0 | The layer check. Its intent reads experimental; nobody has accepted the layer rule as a requirement. |
| `index-snapshot-v0.md` | PROD-TEST-001, PROD-DOCS-009 | accepted | experimental | 75 | 15 | 39 | 21 | 49 | 2 | The shared index that selection and the corpus read. New pairing. |
| `operator-update-v0.md` | PROD-INT-006 | proposed | experimental | 20 | 1 | 9 | 10 | 5 | 15 | The updater. Still names Tasks as a release channel; 15 untraced lines; intent is proposed. |
| `public-release-v0.md` | PROD-INT-006 | accepted | not qualified | 39 | 0 | 5 | 34 | 17 | 8 | The public alpha contract. Its claim still names an optional dashboard and task manager with a roadmap. |
| `release-artifact-integrity-v0.md` | PROD-INT-006 | proposed | experimental | 28 | 0 | 0 | 28 | 11 | 3 | Archive integrity. Intent is proposed. |
| `requirement-definition-gate-v0.md` | PROD-INT-005, PROD-INT-011 | accepted | implemented | 11 | 0 | 3 | 8 | 4 | 0 | Checks that every indexed ID has a definition line; the nearest gate to the document-before-build rule. |
| `spec-requirement-index-generator-v0.md` | PROD-INT-005, PROD-INT-011 | accepted, no decision named | implemented | 74 | 2 | 66 | 6 | 66 | 1 | Generates the requirement index. |
| `stable-operations-v0.md` | PROD-INT-006, PROD-INT-007 | accepted | implemented | 13 | 0 | 1 | 12 | 2 | 0 | Install lifecycle and recovery checks; 2 of 13 IDs in tests. New pairing. |
| `stable-readiness-record-v1.md` | PROD-INT-007 | accepted | experimental | 12 | 1 | 0 | 11 | 12 | 0 | The readiness record written before a tag. |
| `traceability-test-gate-v0.md` | PROD-INT-005 | accepted | implemented | 16 | 0 | 6 | 10 | 7 | 0 | Checks that named tests exist. |

## MERGE

19 specs into six target files. The merged totals, by line count, are: `affected-zero-unknown-v0.md`
71; `application-flow-understanding-v1.md` 112; `corvint-self-development-gates-v0.md` 106;
`external-evidence-provider-v0.md` 82; `external-test-selection-v0.md` 23;
`playwright-external-provider-v0.md` 96. The target `corvint-self-development-gates-v0.md` is itself
proposed as OUT-OF-1.0, so eleven of these merges tidy repository process and do not serve a product
requirement.

| Spec | Merge into | Intent | Delivery | Reqs | Owner | Standing | None | In tests | Untraced | Note |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `affected-reader-precision-v0.md` | `affected-zero-unknown-v0.md` | experimental | experimental | 17 | 1 | 10 | 6 | 7 | 9 | Bounded readers are part of closing UNKNOWN results; 9 of 17 lines untraced. |
| `application-flow-understanding-v0.md` | `application-flow-understanding-v1.md` | proposed | experimental | 12 | 0 | 0 | 12 | 12 | 0 | Older version of the flow model; proposed; depends on a retired spec. |
| `batch-integration-checks-v0.md` | `corvint-self-development-gates-v0.md` | accepted, no decision named | experimental | 2 | 0 | 1 | 1 | 1 | 1 | Repository process under prefix ASD-V0, split over five small files because of the cap. |
| `ci-control-plane-early-post-v0.md` | `corvint-self-development-gates-v0.md` | proposed | experimental | 1 | 0 | 0 | 1 | 1 | 0 | Repository process for pull requests; one or two requirements. |
| `corvint-self-development-companion-v0.md` | `corvint-self-development-gates-v0.md` | accepted, no decision named | experimental | 0 | 0 | 0 | 0 | 0 | 0 | Repository process with no numbered requirements of its own. |
| `corvint-self-development-selection-v0.md` | `corvint-self-development-gates-v0.md` | standing | experimental | 6 | 0 | 6 | 0 | 5 | 0 | Repository process under prefix ASD-V0, split over five small files because of the cap. |
| `external-evidence-provider-mcp-v0.md` | `external-evidence-provider-v0.md` | accepted | implemented | 4 | 0 | 0 | 4 | 4 | 0 | Five files describe one provider record contract. |
| `external-evidence-provider-transports-v0.md` | `external-evidence-provider-v0.md` | accepted | implemented | 14 | 3 | 0 | 11 | 11 | 0 | Five files describe one provider record contract. |
| `external-evidence-provider-v1.md` | `external-evidence-provider-v0.md` | accepted | experimental | 14 | 0 | 0 | 14 | 0 | 0 | Five files describe one provider record contract. |
| `external-evidence-provider-v2.md` | `external-evidence-provider-v0.md` | accepted | experimental | 15 | 0 | 0 | 15 | 12 | 0 | Five files describe one provider record contract. |
| `external-test-selection-v1.md` | `external-test-selection-v0.md` | accepted | experimental | 9 | 0 | 0 | 9 | 4 | 0 | Version 1 only widens version 0; one file per contract. |
| `greptile-gate-v0.md` | `corvint-self-development-gates-v0.md` | accepted | experimental | 2 | 0 | 0 | 2 | 2 | 0 | Repository process for pull requests; one or two requirements. |
| `lane-check-tooling-v0.md` | `corvint-self-development-gates-v0.md` | accepted, no decision named | experimental | 15 | 0 | 15 | 0 | 9 | 6 | Repository process under prefix ASD-V0, split over five small files because of the cap. |
| `lane-dogfood-tooling-v0.md` | `corvint-self-development-gates-v0.md` | accepted, no decision named | experimental | 17 | 0 | 17 | 0 | 10 | 0 | Repository process for lanes. Check each line against decision 0594 item 4 before merging. |
| `lane-shell-portability-v0.md` | `corvint-self-development-gates-v0.md` | accepted, no decision named | experimental | 23 | 0 | 23 | 0 | 20 | 3 | Repository process under prefix ASD-V0, split over five small files because of the cap. |
| `lane-verification-tooling-v0.md` | `corvint-self-development-gates-v0.md` | standing | experimental | 10 | 0 | 10 | 0 | 9 | 1 | Repository process for lanes. |
| `playwright-per-test-freshness-v0.md` | `playwright-external-provider-v0.md` | proposed | experimental | 11 | 0 | 0 | 11 | 6 | 0 | Same package, `internal/jstestprovider`; proposed; depends on a spec retired with the E2E proof lab. |
| `shared-path-change-policy-v0.md` | `corvint-self-development-gates-v0.md` | standing | experimental | 15 | 0 | 15 | 0 | 0 | 0 | Repository process for lanes. |
| `windows-test-portability-v0.md` | `corvint-self-development-gates-v0.md` | accepted, no decision named | experimental | 9 | 0 | 9 | 0 | 6 | 3 | Repository process under prefix ASD-V0, split over five small files because of the cap. |

## RETIRE

19 specs. Seven already say in their header that they are superseded. Twelve describe surface that
was cut. Five of the twelve rest on a text search and say "Verify first"; one,
`proof-carrying-context-optimization-v0.md`, was accepted by decision 0047, so retiring it needs the
owner's explicit word.

| Spec | Group | Intent | Delivery | Reqs | Owner | Standing | None | In tests | Untraced | Note |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `beamfall-shadow-dogfood-v1.md` | superseded | superseded | superseded | 7 | 0 | 0 | 7 | 0 | 7 | Header says superseded; the successor `analyzer-capability-contract-v0.md` is itself retired. |
| `cem-0.2-canonical-binding.md` | superseded | retired | superseded | 58 | 0 | 0 | 58 | 15 | 23 | Header says retired by decision 0525, but delivery reads superseded, so the index lists it as live. |
| `cem-0.3-structural-mechanical.md` | superseded | retired | superseded | 18 | 0 | 0 | 18 | 8 | 0 | Header says retired by decision 0525, but delivery reads superseded, so the index lists it as live. |
| `cem-external-interop-v0.md` | superseded | retired | superseded | 11 | 0 | 0 | 11 | 0 | 11 | Header says retired by decision 0525, but delivery reads superseded, so the index lists it as live. |
| `cem-go-portability-probe.md` | superseded | retired | superseded | 6 | 0 | 0 | 6 | 1 | 0 | Header says retired by decision 0525, but delivery reads superseded, so the index lists it as live. |
| `external-frontier-obligations-v0.md` | cut surface | accepted | experimental | 9 | 0 | 0 | 9 | 0 | 0 | The command name is absent from source; depends on retired `change-frontier-v0.md`. Verify first. |
| `harness-authority-relation-v0.md` | superseded | superseded | superseded | 11 | 0 | 0 | 11 | 0 | 0 | Header says superseded by decision 0009. |
| `lsp-editor-context-v0.md` | cut surface | proposed | experimental | 10 | 0 | 0 | 10 | 0 | 0 | The LSP server was cut by decision 0594 item 1; no requirement ID appears in a test file. |
| `lsp-editor-qualification-v0.md` | cut surface | proposed | experimental | 6 | 0 | 0 | 6 | 0 | 6 | The LSP server was cut by decision 0594 item 1; `tools/lsp-editors` and one script remain. |
| `lsp-quality-platform-v0.md` | cut surface | accepted | experimental | 22 | 0 | 0 | 22 | 0 | 22 | The LSP server was cut by decision 0594 item 1; `script/measure-lsp-quality.py` remains. |
| `lsp-wrapped-backends-v0.md` | cut surface | proposed | experimental | 10 | 0 | 0 | 10 | 0 | 3 | The LSP server was cut by decision 0594 item 1; no requirement ID appears in a test file. |
| `proof-carrying-context-optimization-v0.md` | cut surface | accepted | not-started | 17 | 0 | 0 | 17 | 0 | 0 | Not started; context delivery was cut by decision 0594 item 1; depends on two retired specs. |
| `reproduction-bundle-v0.md` | cut surface | standing | experimental | 9 | 0 | 2 | 7 | 0 | 9 | The command name is absent from source and catalogue; no ID in a test file. Verify first. |
| `snapshot-batch-v0.md` | cut surface | accepted | experimental | 10 | 4 | 0 | 6 | 0 | 0 | Batches `query` and `context` requests; the command name is absent from source. Verify first. |
| `source-evidence-axes-v0.md` | cut surface | proposed | not-started | 8 | 0 | 0 | 8 | 0 | 0 | Not started; its subject is the console cut by decision 0594 item 2; no ID in a test file. |
| `spec-toolchain-adapters-v0.md` | cut surface | accepted, no decision named | experimental | 16 | 0 | 2 | 14 | 4 | 12 | Describes a reader for OpenSpec and Spec Kit projects that was removed. Keep only if the reader is rebuilt. |
| `trial-replay-journal-v0.md` | cut surface | proposed | not-started | 12 | 0 | 0 | 12 | 0 | 0 | Not started; one of its two dispatchers belongs to retired `cem-reviewer-trial-v0.md`, while `tools/cw-trial` survives; no ID in a test file. Verify first. |
| `upgrade-checklist-v0.md` | cut surface | accepted | experimental | 8 | 2 | 1 | 5 | 0 | 8 | The command name is absent from source and catalogue; no ID in a test file. Verify first. |
| `verified-absence-frontier-v0.md` | superseded | superseded | superseded | 16 | 0 | 0 | 16 | 0 | 16 | Header says superseded; the successor `change-frontier-v0.md` is itself retired. |

## OUT-OF-1.0

59 specs with 1924 requirement lines. By requirement lines the two largest groups are the kept
commands and change evidence. Whether those belong in 1.0 is an owner decision; item 21 of the
acceptance checklist in the plan covers the classes as a whole.

| Spec | Group | Intent | Delivery | Reqs | Owner | Standing | None | In tests | Untraced | Note |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `adoption-setup-report-v0.md` | kept command | accepted | experimental | 11 | 0 | 4 | 7 | 5 | 6 | `corvint adopt --setup`. No product requirement. |
| `agent-instruction-doctor-v0.md` | kept command | proposed | experimental | 7 | 0 | 0 | 7 | 0 | 7 | Part of the context packet; no ID in a test file. |
| `aggregate-local-outcome-v0.md` | change evidence | proposed | experimental | 27 | 0 | 1 | 26 | 7 | 20 | Local outcome record; 20 untraced lines; depends on a retired spec. |
| `analyzer-candidate-profiles.md` | analyzers | proposed | deferred | 12 | 0 | 0 | 12 | 12 | 0 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `analyzer-python-native-candidate.md` | analyzers | proposed | deferred | 10 | 0 | 0 | 10 | 10 | 0 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `applied-intelligence-breakthroughs-v0.md` | experiment | accepted | not-started | 0 | 0 | 0 | 0 | 0 | 0 | Programme statement with no numbered requirement; parent of three specs under decision 0047. |
| `cem-2.0-intent.md` | change evidence | proposed | not-started | 25 | 0 | 0 | 25 | 0 | 23 | A proposed successor; not built. |
| `cem-additive-profiles-v0.md` | change evidence | standing | experimental | 19 | 0 | 0 | 19 | 14 | 5 | Optional profile beside the change evidence map. |
| `cem-completeness-profile-v0.md` | change evidence | standing | experimental | 10 | 0 | 10 | 0 | 6 | 0 | Optional profile beside the change evidence map. |
| `cem-pilot-kit.md` | change evidence | accepted, no decision named | experimental | 33 | 4 | 0 | 29 | 9 | 23 | Describes the experimental 0.1 pilot; candidate for retirement once the stable contract is checked. |
| `cem-stable-v1.md` | change evidence | accepted | implemented | 360 | 148 | 173 | 39 | 139 | 66 | `corvint cem`, kept by decision 0594 item 4. 360 requirements, 66 untraced; depends on two retired specs. |
| `cem2-discovery-replay-v0.md` | change evidence | proposed | experimental | 37 | 0 | 0 | 37 | 0 | 37 | A proposed successor; not built. |
| `confidently-wrong-trial-v0.md` | experiment | proposed | experimental | 17 | 0 | 3 | 14 | 9 | 0 | Measurement or trial programme for context quality; not part of the three products. |
| `context-evolution-program-v0.md` | experiment | proposed | experimental | 16 | 0 | 10 | 6 | 11 | 5 | Measurement or trial programme for context quality; not part of the three products. |
| `context-ranking-v0.md` | kept command | proposed | experimental | 13 | 0 | 5 | 8 | 9 | 4 | Ranking options for context; no declared implementation path. |
| `corvint-rebrand-v0.md` | history | accepted, no decision named | experimental | 20 | 0 | 1 | 19 | 5 | 2 | Record of a finished migration. |
| `corvint-self-development-gates-v0.md` | repository gate | proposed | experimental | 6 | 0 | 6 | 0 | 3 | 3 | The overflow file for self-development gates; proposed target for the merged process specs. |
| `cross-audience-truth-nondivergence-v0.md` | experiment | proposed | experimental | 15 | 0 | 0 | 15 | 2 | 0 | Measurement or trial programme for context quality; not part of the three products. |
| `cross-repository-breakage-map-v0.md` | kept command | proposed | experimental | 18 | 0 | 6 | 12 | 11 | 7 | `internal/breakagemap` remains; the command name is absent from source. |
| `decision-number-gate-v0.md` | repository gate | accepted | implemented | 9 | 1 | 0 | 8 | 1 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `deployment-neutral-index-platform-v0.md` | deferred | accepted, no decision named | not-started | 74 | 0 | 0 | 74 | 1 | 73 | Not started; product invariant 7 cites it as the boundary for anything hosted. |
| `eol-policy-gate-v0.md` | repository gate | accepted | implemented | 9 | 0 | 0 | 9 | 0 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `error-code-ownership-gate-v0.md` | repository gate | accepted | implemented | 9 | 0 | 3 | 6 | 1 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `expanded-range-impact-v0.md` | kept command | proposed | experimental | 14 | 0 | 8 | 6 | 11 | 0 | `corvint impact` range profile; depends on retired `change-frontier-v0.md`. |
| `expert-audit-followup-v0.md` | history | accepted | experimental | 13 | 0 | 0 | 13 | 8 | 5 | Record of audit repairs; depends on a retired spec. |
| `gate-ledger-v0.md` | repository gate | accepted | implemented | 29 | 2 | 20 | 7 | 25 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `genesis-backfill.md` | kept command | accepted, no decision named | experimental | 25 | 0 | 0 | 25 | 2 | 0 | `corvint init` and `corvint adopt` inventory; 2 of 25 IDs in tests. |
| `go-format-gate-v0.md` | repository gate | accepted | implemented | 7 | 0 | 0 | 7 | 0 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `go-only-cutover-v0.md` | history | accepted, no decision named | experimental | 13 | 0 | 0 | 13 | 9 | 0 | Record of a finished migration. |
| `go-production-kernel-migration-v0.md` | history | accepted, no decision named | experimental | 102 | 25 | 22 | 55 | 51 | 14 | Migration record; 14 untraced lines; depends on a retired spec. |
| `go-test-evidence-v0.md` | kept command | proposed | experimental | 7 | 0 | 0 | 7 | 6 | 1 | Operator tool `tools/gotest-evidence`. Intent is proposed. |
| `heading-navigation-experiment-v0.md` | experiment | proposed | experimental | 5 | 0 | 0 | 5 | 5 | 0 | Measurement or trial programme for context quality; not part of the three products. |
| `kotlin-android-candidate-v0.md` | analyzers | proposed | deferred | 10 | 0 | 0 | 10 | 1 | 9 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `learned-trace-admission-v0.md` | experiment | accepted | experimental | 20 | 5 | 1 | 14 | 8 | 0 | Local ledger named by product invariant 4 in `AGENTS.md`; stays as written. |
| `local-completion-policy-v0.md` | change evidence | accepted | implemented | 48 | 5 | 22 | 21 | 3 | 2 | Local completion for enrolled changes; 3 of 48 IDs in tests; depends on a retired spec. |
| `mcp-test-validity-profile-v0.md` | kept command | accepted | experimental | 11 | 0 | 2 | 9 | 9 | 0 | `corvint-test-validity-mcp`. No product requirement. |
| `native-cem-adapter.md` | change evidence | proposed | experimental | 0 | 0 | 0 | 0 | 0 | 0 | One dependency rule with no numbered requirement. |
| `native-jvm-bridge-analyzer-candidates-v0.md` | analyzers | proposed | deferred | 8 | 0 | 0 | 8 | 7 | 1 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `non-go-impact-v0.md` | kept command | proposed | experimental | 63 | 0 | 49 | 14 | 51 | 0 | `corvint impact` for Ruby, JavaScript and TypeScript. No product requirement names impact. |
| `observed-ui-evidence-v0.md` | kept command | proposed | experimental | 43 | 0 | 1 | 42 | 22 | 21 | Observed UI records. Close to PROD-DOCS-008 but proposed and with 21 untraced lines. |
| `optional-artifact-check-scripts-v0.md` | repository gate | accepted | experimental | 8 | 0 | 0 | 8 | 5 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `proof-tour-v0.md` | change evidence | proposed | experimental | 8 | 0 | 0 | 8 | 0 | 8 | Scripts `script/proof-tour.sh` remain; no ID in a test file. |
| `repository-guidance-v0.md` | kept command | accepted | experimental | 18 | 0 | 3 | 15 | 18 | 0 | `corvint features`, `corvint overview` and `corvint review`. No product requirement. |
| `retrieval-bench-diagnostics-v0.md` | experiment | proposed | experimental | 7 | 0 | 0 | 7 | 7 | 0 | Measurement or trial programme for context quality; not part of the three products. |
| `rust-analyzer-candidate-v0.md` | analyzers | accepted, no decision named | deferred | 11 | 0 | 0 | 11 | 7 | 4 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `self-observation-ledger-v0.md` | experiment | proposed | experimental | 16 | 0 | 1 | 15 | 14 | 0 | Local ledger named by product invariant 4 in `AGENTS.md`; stays as written. |
| `shader-glsl-metal-analyzer-candidate-v0.md` | analyzers | proposed | deferred | 6 | 0 | 0 | 6 | 0 | 6 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `sql-native-ratchet-gate-v0.md` | repository gate | accepted | partial | 17 | 0 | 0 | 17 | 9 | 0 | A repository check that keeps running; no product requirement depends on it. |
| `structured-data-analyzer-candidate-v1.md` | analyzers | rejected | deferred | 7 | 0 | 0 | 7 | 0 | 7 | Intent reads rejected-as-specified, yet `cmd/corvint-analyzer-structured-data` exists. |
| `swift-apple-analyzer-candidate-v0.md` | analyzers | proposed | deferred | 8 | 0 | 0 | 8 | 1 | 2 | Candidate analyzer, deferred, not registered with selection. See the analyzers finding below. |
| `task-context-packet-v0.md` | kept command | proposed | experimental | 97 | 5 | 39 | 53 | 66 | 1 | `corvint context`. Decision 0594 lists the verb as kept and also cuts the context command family. |
| `test-attack-v0.md` | kept command | proposed | experimental | 7 | 0 | 0 | 7 | 1 | 6 | `internal/liveverify/mutate` remains; the command name is absent from source. |
| `test-claim-qualification-v0.md` | kept command | proposed | implementation-candidate | 80 | 1 | 21 | 58 | 40 | 1 | `internal/tcq`. Names OCM anchors; depends on three retired or superseded specs. |
| `test-consolidation-planner-v0.md` | kept command | accepted | experimental | 16 | 0 | 0 | 16 | 16 | 0 | `corvint test-plan`. Works on flow variations but serves no product requirement. |
| `test-source-analysis-v0.md` | kept command | standing | experimental | 283 | 0 | 283 | 0 | 243 | 1 | `corvint tests`. The largest spec, 283 requirements, with no product requirement. |
| `unplanned-read-events-v0.md` | experiment | proposed | experimental | 12 | 0 | 0 | 12 | 8 | 0 | Local ledger named by product invariant 4 in `AGENTS.md`; stays as written. |
| `use-case-conformance-v0.md` | experiment | proposed | experimental | 21 | 1 | 5 | 15 | 11 | 0 | Promotion evidence for headline jobs; proposed overall. |
| `verification-planner-observer-v0.md` | deferred | proposed | deferred | 47 | 0 | 0 | 47 | 4 | 0 | Deferred; no declared implementation path. |
| `vscode-extension-v0.md` | deferred | proposed | deferred | 80 | 2 | 2 | 76 | 29 | 0 | Deferred. The editor surface closest to the Wallaby comparison; PROD-TEST-007 covers CLI and MCP only. |

## Findings that contradict the product plan or the layer structure

Each finding names the spec files, what they say, and what the plan, an accepted decision or the
layer declaration in `tools/import-ratchet/areas.json` says. None was repaired here.

1. Two documents define 1.0. The digest of `corvint-1.0-product-and-release-v1.md` reads "1.0
   requires Core change evidence, Flows, navigation, docs, MCP, automatic docs, PRS-V1-018 SDD
   improvements and the Tasks Beamfall takeover." The product plan defines 1.0 as three
   products and leaves change evidence outside them. The older file also depends on three
   retired specs (`daily-change-evidence-workflow-v0.md`, `agent-harness-integration-v0.md` and
   `corvint-tasks-agent-leases-v0.md`). Until the owner says which document governs, the
   release candidate in PROD-INT-008 has two definitions.
2. The task manager has no spec on any main branch. All 34 spec files in this repository whose name
   contains `tasks` are retired, following decision 0594 item 5. Their live copies are 38 files on
   branch `claude/import-from-corvint` at commit `18c75e3` of the `beamfall/corvint-tasks`
   repository; the `main` branch of that repository holds no `docs/specs` files. The nine task
   manager requirements in the plan therefore point at no merged spec, and this report could not
   classify the Tasks specs. Three surviving files here still treat Tasks as part of Corvint:
   `operator-update-v0.md` checks "Core and Tasks release channels", `public-release-v0.md` names
   "an optional dashboard and task manager with a roadmap", and
   `corvint-1.0-product-and-release-v1.md` requires the Tasks takeover of Beamfall.
   `tasks-mcp-read-profile-v0.md`, retired here, describes a tasks tool group inside `corvint mcp`,
   while decision 0594 item 5 says "Tasks owns its MCP tools" and PROD-TASK-007 says the same.
3. The layer rule is not accepted intent. PROD-INT-001 rests on `import-ratchet-v0.md`, which has
   six requirement lines, the intent `experimental` and no acceptance on any line. Decision 0594
   item 8 says of the layers that it "records direction only and changes no code".
   `tools/import-ratchet/baseline.json` tolerates 39 imports that run the wrong way.
   `core-bundled-apps-v0.md` describes the structure as "a core plus bundled apps and plugins",
   which is an older picture than the five layers. One of the two files has to be rewritten to match
   the other, and the result needs the owner to accept it.
4. `corvint how` and the map join lean on a retired spec. `docs/CAPABILITIES.md` documents
   `corvint how` as an alias of `flows appmap how`. Its contract was
   `site-knowledge-howto-v0.md`, which decision 0593 retired. The closest surviving home is
   `application-map-scenario-planner-v0.md`, whose IDs are cited in
   `internal/appmap/site_knowledge_how.go`, but all 10 of its lines are unrecorded for
   acceptance. In `map-coverage-join-v0.md`, requirement MCJ-V0-002 still binds a segment
   identity "only when the SKH-V0 model read at the record's commit holds that identity",
   and SKH-V0 is the prefix of the retired spec. This is the dependency the plan refers to
   under PROD-DOCS-006; it also leaves PROD-DOCS-007 without an accepted contract.
5. Two specs govern running tests. `session-test-runner-v0.md` has 51 lines and the delivery
   `implemented`. `test-runner-execution-v0.md` has 75 lines, the intent `proposed` and 13 untraced
   lines. PROD-TEST-006 needs one contract. Beside them, `go-live-test-provider-v0.md` has 54 lines
   that are all unrecorded for acceptance, `js-live-test-provider-v0.md` has no numbered requirement
   at all, and `cmd/corvint-ruby-test-provider` exists with no spec of its own in the index.
6. The watch contract has a header that does not match its delivery.
   `live-proof-carrying-verification-v0.md` carries 190 lines and serves PROD-TEST-007
   and PROD-TEST-008. Its delivery line begins "not-started (parent product)" and then
   lists slices that are experimental. A reader of the index sees a product that has not
   begun.
7. The analyzers layer has no surviving contract. Decision 0594 keeps every `corvint-analyzer-*`
   binary, and sixteen exist under `cmd/`. The eight analyzer specs that survive are candidate specs
   with the delivery `deferred`. They say the analyzer is not registered, selected or launched; for
   example `analyzer-candidate-profiles.md` claims facts "without Core registration, selection,
   launch, or support". One of them, `structured-data-analyzer-candidate-v1.md`, has the intent
   `rejected-as-specified` although `cmd/corvint-analyzer-structured-data` exists. The spec that
   governed registered analyzers, `analyzer-capability-contract-v0.md`, is retired, and 8 surviving
   specs still declare a dependency on it.
8. Specs survive for surface that decision 0594 cut. Four LSP specs remain although item 1 cut the
   LSP server and no `corvint-lsp` directory remains under `cmd/`. `spec-toolchain-adapters-v0.md`
   claims that "Corvint reads pinned OpenSpec and Spec Kit projects as declared OCM intent forms",
   but the reader package and the `corvint ocm` command are gone;
   `docs/decisions/PROPOSED-openspec-workflow.md` says what remains. `source-evidence-axes-v0.md`
   depends on the retired console and dashboard specs. `snapshot-batch-v0.md`,
   `reproduction-bundle-v0.md`, `upgrade-checklist-v0.md` and `external-frontier-obligations-v0.md`
   describe commands whose names a text search no longer finds.
9. Seven specs read `superseded` and still count as live. Only the delivery status `retired` removes
   a spec from the surviving set. Four of the seven say "retired by decision 0525" in their intent
   line and `superseded` in their delivery line, so the index disagrees with the decision.
10. `corvint context` is both kept and cut. The "Keeps" line of decision 0594 lists the verb
    `context`, item 1 of the same decision cuts the context command families, and
    `docs/CAPABILITIES.md` still documents `corvint context`. `task-context-packet-v0.md`,
    `context-ranking-v0.md` and `agent-instruction-doctor-v0.md` are placed outside 1.0 until the
    owner says which reading holds.
11. Kept commands have no product requirement. 16 specs with 713 lines govern commands that decision
    0594 keeps and the plan does not name: test validity (`corvint tests`, `corvint test-plan`,
    `corvint-test-validity-mcp`), `corvint impact`, `corvint context`, repository guidance and
    adoption. A further 10 specs with 567 lines govern `corvint cem`. The owner's testing goal
    mentions none of them. If they ship in 1.0 they need product requirements; if they do not, the
    release notes should say they are outside the 1.0 promise. `test-claim-qualification-v0.md` also
    still names OCM anchors after `corvint ocm` was removed.
12. The compatibility freeze names an older verb list. `core-compatibility-freeze-v1.md` freezes the
    "twelve Core verbs of decision 0332". Decision 0594 keeps nineteen `corvint` verbs together with
    `docs`, `flows` and `how`. PROD-INT-007 needs the frozen list restated against the commands that
    the three products actually keep.
13. One flow spec spans three layers. `application-flow-understanding-v1.md` serves seven product
    requirements. Its sections cover the flow model, which stays in the shared layer; selection of
    E2E tests, which belongs to the tests layer; and flow documents and the navigation map, whose
    commands move to the docs layer. 94 of its 100 lines are unrecorded for acceptance. The four
    change proposals under `docs/specs/openspec/changes/` add seventeen more lines to it. The file
    should at least name the owning layer for each section.
14. A claim names a binary that does not exist. The digest of `workspace-declaration-v0.md` names
    `corvint-test e2e`. No `corvint-test` directory exists under `cmd/`.
15. The self-use spec describes commands that are gone. `corvint-self-development-v0.md` names the
    make targets `dogfood-change`, `dogfood-check` or `dogfood-seal` on 46 lines. The `Makefile` at
    this commit defines none of them, following decision 0594 item 4. The file is at the limit of
    256 requirements, 79 of its lines are untraced, and PROD-INT-009 depends on it.
16. The editor surface is deferred. The owner's comparison for the watch feature is an editor tool.
    `vscode-extension-v0.md` is deferred with 80 lines, and PROD-TEST-007 covers the command line
    and MCP only. Whether an editor surface is part of 1.0 is an owner decision.

## Specs that depend on retired specs

41 surviving specs declare a dependency on a spec that is now retired, in the `dependsOn` field of
`docs/specs/INDEX.json`. A requirement that points at a retired contract has lost part of its
meaning. Each dependency should be removed, replaced or restated in the surviving file. This report
did not check which requirement lines use each dependency, except for the map join named above.

| Spec | Class | Retired specs it depends on |
|---|---|---|
| `aggregate-local-outcome-v0.md` | OUT-OF-1.0 | `daily-change-evidence-workflow-v0.md` |
| `analyzer-candidate-profiles.md` | OUT-OF-1.0 | `analyzer-capability-contract-v0.md` |
| `application-flow-understanding-v0.md` | MERGE | `ocm-v0-dogfood.md` |
| `beamfall-shadow-dogfood-v1.md` | RETIRE | `analyzer-capability-contract-v0.md` |
| `cem-0.2-canonical-binding.md` | RETIRE | `ocm-v0-dogfood.md` |
| `cem-stable-v1.md` | OUT-OF-1.0 | `tasks-criterion-binding-v0.md`, `cem-criterion-experiments-v0.md` |
| `confidently-wrong-trial-v0.md` | OUT-OF-1.0 | `falsifiable-packet-v0.md` |
| `context-evolution-program-v0.md` | OUT-OF-1.0 | `agent-harness-integration-v0.md` |
| `core-compatibility-freeze-v1.md` | KEEP | `falsifiable-packet-v0.md`, `local-trace-producer-migration-v0.md` |
| `corvint-1.0-product-and-release-v1.md` | KEEP | `daily-change-evidence-workflow-v0.md`, `agent-harness-integration-v0.md`, `corvint-tasks-agent-leases-v0.md` |
| `deployment-neutral-index-platform-v0.md` | OUT-OF-1.0 | `analyzer-capability-contract-v0.md`, `pulse-snapshot-lease-v0.md` |
| `documentation-citation-gate-v0.md` | KEEP | `falsifiable-packet-v0.md` |
| `expanded-range-impact-v0.md` | OUT-OF-1.0 | `change-frontier-v0.md` |
| `expert-audit-followup-v0.md` | OUT-OF-1.0 | `falsifiable-packet-v0.md` |
| `external-evidence-provider-v0.md` | KEEP | `analyzer-capability-contract-v0.md`, `change-frontier-v0.md` |
| `external-frontier-obligations-v0.md` | RETIRE | `change-frontier-v0.md` |
| `go-production-kernel-migration-v0.md` | OUT-OF-1.0 | `agent-harness-integration-v0.md` |
| `heading-navigation-experiment-v0.md` | OUT-OF-1.0 | `retrieval-eval-comparability-v0.md` |
| `learned-trace-admission-v0.md` | OUT-OF-1.0 | `local-trace-producer-migration-v0.md` |
| `local-completion-policy-v0.md` | OUT-OF-1.0 | `agent-harness-integration-v0.md` |
| `lsp-editor-context-v0.md` | RETIRE | `lsp-editor-definition-v0.md` |
| `map-coverage-join-v0.md` | KEEP | `change-compiled-e2e-v0.md`, `site-knowledge-howto-v0.md` |
| `mcp-server-2026-07-28-v0.md` | KEEP | `agent-harness-integration-v0.md` |
| `native-jvm-bridge-analyzer-candidates-v0.md` | OUT-OF-1.0 | `analyzer-capability-contract-v0.md` |
| `non-go-impact-v0.md` | OUT-OF-1.0 | `change-frontier-v0.md` |
| `playwright-per-test-freshness-v0.md` | MERGE | `new-e2e-test-acceptance-v0.md` |
| `proof-carrying-context-optimization-v0.md` | RETIRE | `agent-harness-integration-v0.md`, `session-context-dividend-v0.md` |
| `public-release-v0.md` | KEEP | `local-admin-console-v0.md` |
| `run-verified-navigation-v0.md` | KEEP | `observation-corpus-authority-v0.md` |
| `rust-analyzer-candidate-v0.md` | OUT-OF-1.0 | `analyzer-capability-contract-v0.md` |
| `self-observation-ledger-v0.md` | OUT-OF-1.0 | `observation-corpus-authority-v0.md` |
| `shader-glsl-metal-analyzer-candidate-v0.md` | OUT-OF-1.0 | `analyzer-capability-contract-v0.md` |
| `source-evidence-axes-v0.md` | RETIRE | `local-admin-console-v0.md`, `local-observability-dashboard-v0.md` |
| `stable-readiness-record-v1.md` | KEEP | `host-lifecycle-qualification-v1.md` |
| `swift-apple-analyzer-candidate-v0.md` | OUT-OF-1.0 | `analyzer-capability-contract-v0.md` |
| `test-attack-v0.md` | OUT-OF-1.0 | `falsifiable-packet-v0.md` |
| `test-claim-qualification-v0.md` | OUT-OF-1.0 | `change-frontier-v0.md`, `ocm-v0-dogfood.md` |
| `trial-replay-journal-v0.md` | RETIRE | `cem-reviewer-trial-v0.md` |
| `use-case-conformance-v0.md` | OUT-OF-1.0 | `agent-harness-integration-v0.md` |
| `verified-absence-frontier-v0.md` | RETIRE | `change-frontier-v0.md` |
| `vscode-extension-v0.md` | OUT-OF-1.0 | `agent-harness-integration-v0.md` |

Two more entries are irregular. `local-completion-policy-v0.md` lists a decision file,
`docs/decisions/0009-harness-authority-boundary.md`, among its spec dependencies.
`test-claim-qualification-v0.md` also depends on `cem-0.2-canonical-binding.md`, whose delivery
reads `superseded`.

## Specs with no requirement named in any test file

30 specs have requirement lines and none of their IDs appears in a test file. 5 of them are proposed
as KEEP. The column "In own evidence section" counts the lines that the spec lists in its own
section of tests or measured evidence. "Untraced" counts the lines with neither.

| Spec | Class | Delivery | Reqs | In own evidence section | Untraced |
|---|---|---|---:|---:|---:|
| `affected-edit-oracle-v0.md` | KEEP | experimental | 8 | 8 | 0 |
| `agent-instruction-doctor-v0.md` | OUT-OF-1.0 | experimental | 7 | 0 | 7 |
| `beamfall-shadow-dogfood-v1.md` | RETIRE | superseded | 7 | 0 | 7 |
| `cem-2.0-intent.md` | OUT-OF-1.0 | not-started | 25 | 2 | 23 |
| `cem-external-interop-v0.md` | RETIRE | superseded | 11 | 0 | 11 |
| `cem2-discovery-replay-v0.md` | OUT-OF-1.0 | experimental | 37 | 0 | 37 |
| `dotnet-affected-selection-v0.md` | KEEP | experimental | 10 | 8 | 2 |
| `eol-policy-gate-v0.md` | OUT-OF-1.0 | implemented | 9 | 9 | 0 |
| `external-evidence-provider-v1.md` | MERGE | experimental | 14 | 14 | 0 |
| `external-frontier-obligations-v0.md` | RETIRE | experimental | 9 | 9 | 0 |
| `go-format-gate-v0.md` | OUT-OF-1.0 | implemented | 7 | 7 | 0 |
| `harness-authority-relation-v0.md` | RETIRE | superseded | 11 | 11 | 0 |
| `java-affected-test-v0.md` | KEEP | experimental | 10 | 10 | 0 |
| `lsp-editor-context-v0.md` | RETIRE | experimental | 10 | 10 | 0 |
| `lsp-editor-qualification-v0.md` | RETIRE | experimental | 6 | 0 | 6 |
| `lsp-quality-platform-v0.md` | RETIRE | experimental | 22 | 0 | 22 |
| `lsp-wrapped-backends-v0.md` | RETIRE | experimental | 10 | 7 | 3 |
| `proof-carrying-context-optimization-v0.md` | RETIRE | not-started | 17 | 17 | 0 |
| `proof-tour-v0.md` | OUT-OF-1.0 | experimental | 8 | 0 | 8 |
| `reproduction-bundle-v0.md` | RETIRE | experimental | 9 | 0 | 9 |
| `rust-affected-test-understanding-v0.md` | KEEP | experimental | 10 | 10 | 0 |
| `shader-glsl-metal-analyzer-candidate-v0.md` | OUT-OF-1.0 | deferred | 6 | 0 | 6 |
| `shared-path-change-policy-v0.md` | MERGE | experimental | 15 | 15 | 0 |
| `snapshot-batch-v0.md` | RETIRE | experimental | 10 | 10 | 0 |
| `source-evidence-axes-v0.md` | RETIRE | not-started | 8 | 8 | 0 |
| `structured-data-analyzer-candidate-v1.md` | OUT-OF-1.0 | deferred | 7 | 0 | 7 |
| `swift-affected-test-understanding-v0.md` | KEEP | experimental | 13 | 13 | 0 |
| `trial-replay-journal-v0.md` | RETIRE | not-started | 12 | 12 | 0 |
| `upgrade-checklist-v0.md` | RETIRE | experimental | 8 | 0 | 8 |
| `verified-absence-frontier-v0.md` | RETIRE | superseded | 16 | 0 | 16 |

4 surviving specs have no numbered requirement at all: `applied-intelligence-breakthroughs-v0.md`,
`corvint-self-development-companion-v0.md`, `js-live-test-provider-v0.md`, `native-cem-adapter.md`.
Nothing can be traced to them.

## Specs marked delivered whose requirements lack a test or evidence entry

26 specs have a delivery line that begins with implemented, partial, in progress, validated or
implementation candidate. For each, the table splits the requirement lines into those named in a
test file, those listed only in the spec's own evidence section, and those with neither. A line in
the last column is claimed as delivered with no recorded test or measured result.

| Spec | Class | Delivery | Reqs | In tests | Own section only | Neither | Gap |
|---|---|---|---:|---:|---:|---:|---|
| `application-flow-understanding-v1.md` | KEEP | partial | 100 | 86 | 1 | 13 | 13 lines with no evidence |
| `cem-stable-v1.md` | OUT-OF-1.0 | implemented | 360 | 139 | 155 | 66 | 66 lines with no evidence |
| `core-bundled-apps-v0.md` | KEEP | in-progress | 11 | 2 | 0 | 9 | 9 lines with no evidence |
| `corvint-1.0-product-and-release-v1.md` | KEEP | partial | 25 | 6 | 19 | 0 | most lines only in the spec's own list |
| `decision-number-gate-v0.md` | OUT-OF-1.0 | implemented | 9 | 1 | 8 | 0 | most lines only in the spec's own list |
| `diagnostic-repair-contract-v0.md` | KEEP | implemented | 13 | 11 | 2 | 0 | none found |
| `documentation-citation-gate-v0.md` | KEEP | implemented | 46 | 26 | 19 | 1 | 1 line with no evidence |
| `eol-policy-gate-v0.md` | OUT-OF-1.0 | implemented | 9 | 0 | 9 | 0 | no ID in any test file |
| `error-code-ownership-gate-v0.md` | OUT-OF-1.0 | implemented | 9 | 1 | 8 | 0 | most lines only in the spec's own list |
| `external-evidence-provider-mcp-v0.md` | MERGE | implemented | 4 | 4 | 0 | 0 | none found |
| `external-evidence-provider-transports-v0.md` | MERGE | implemented | 14 | 11 | 3 | 0 | none found |
| `gate-ledger-v0.md` | OUT-OF-1.0 | implemented | 29 | 25 | 4 | 0 | none found |
| `go-archive-gate-v0.md` | KEEP | implemented | 13 | 9 | 4 | 0 | none found |
| `go-format-gate-v0.md` | OUT-OF-1.0 | implemented | 7 | 0 | 7 | 0 | no ID in any test file |
| `human-documentation-compiler-v0.md` | KEEP | partial | 66 | 3 | 63 | 0 | most lines only in the spec's own list |
| `local-completion-policy-v0.md` | OUT-OF-1.0 | implemented | 48 | 3 | 43 | 2 | 2 lines with no evidence |
| `map-coverage-join-v0.md` | KEEP | partial | 25 | 11 | 5 | 9 | 9 lines with no evidence |
| `mcp-server-2026-07-28-v0.md` | KEEP | implemented | 67 | 41 | 26 | 0 | none found |
| `playwright-external-provider-v0.md` | KEEP | validated in part | 85 | 73 | 10 | 2 | 2 lines with no evidence |
| `requirement-definition-gate-v0.md` | KEEP | implemented | 11 | 4 | 7 | 0 | most lines only in the spec's own list |
| `session-test-runner-v0.md` | KEEP | implemented | 51 | 48 | 3 | 0 | none found |
| `spec-requirement-index-generator-v0.md` | KEEP | implemented | 74 | 66 | 7 | 1 | 1 line with no evidence |
| `sql-native-ratchet-gate-v0.md` | OUT-OF-1.0 | partial | 17 | 9 | 8 | 0 | none found |
| `stable-operations-v0.md` | KEEP | implemented | 13 | 2 | 11 | 0 | most lines only in the spec's own list |
| `test-claim-qualification-v0.md` | OUT-OF-1.0 | implementation-candidate | 80 | 40 | 39 | 1 | 1 line with no evidence |
| `traceability-test-gate-v0.md` | KEEP | implemented | 16 | 7 | 9 | 0 | most lines only in the spec's own list |

9 of the 26 have lines with no evidence: `cem-stable-v1.md` (66),
`application-flow-understanding-v1.md` (13), `core-bundled-apps-v0.md` (9),
`map-coverage-join-v0.md` (9), `local-completion-policy-v0.md` (2),
`playwright-external-provider-v0.md` (2), `documentation-citation-gate-v0.md` (1),
`spec-requirement-index-generator-v0.md` (1), `test-claim-qualification-v0.md` (1).

A further 54 specs have the delivery `experimental` and at least one untraced line. Their header
does not say which lines are implemented, so the count is an upper bound on the gap. In order of
size they are: `corvint-self-development-v0.md` (79 of 256), `cem2-discovery-replay-v0.md` (37
of 37), `cem-pilot-kit.md` (23 of 33), `lsp-quality-platform-v0.md` (22 of 22),
`observed-ui-evidence-v0.md` (21 of 43), `aggregate-local-outcome-v0.md` (20 of 27),
`playwright-run-evidence-v0.md` (17 of 74), `operator-update-v0.md` (15 of 20),
`go-production-kernel-migration-v0.md` (14 of 102), `test-runner-execution-v0.md` (13 of 75),
`spec-toolchain-adapters-v0.md` (12 of 16), `affected-reader-precision-v0.md` (9 of 17),
`reproduction-bundle-v0.md` (9 of 9), `kotlin-jvm-affected-test-v0.md` (8 of 12),
`proof-tour-v0.md` (8 of 8), `upgrade-checklist-v0.md` (8 of 8),
`agent-instruction-doctor-v0.md` (7 of 7), `cross-repository-breakage-map-v0.md` (7 of 18),
`adoption-setup-report-v0.md` (6 of 11), `lane-check-tooling-v0.md` (6 of 15),
`lsp-editor-qualification-v0.md` (6 of 6), `test-attack-v0.md` (6 of 7),
`cem-additive-profiles-v0.md` (5 of 19), `ci-narrowing-v0.md` (5 of 51),
`context-evolution-program-v0.md` (5 of 16), `expert-audit-followup-v0.md` (5 of 13),
`context-ranking-v0.md` (4 of 13), `application-map-v0.md` (3 of 191),
`corvint-self-development-gates-v0.md` (3 of 6), `flow-document-maintenance-v0.md` (3 of 7),
`lane-shell-portability-v0.md` (3 of 23), `lsp-wrapped-backends-v0.md` (3 of 10),
`release-artifact-integrity-v0.md` (3 of 28), `runner-selection-plugins-v0.md` (3 of 62),
`windows-test-portability-v0.md` (3 of 9), `corvint-rebrand-v0.md` (2 of 20),
`dotnet-affected-selection-v0.md` (2 of 10), `go-live-test-provider-v0.md` (2 of 54),
`index-snapshot-v0.md` (2 of 75), `language-plugins-v0.md` (2 of 27),
`trusted-project-navigation-v0.md` (2 of 6), `affected-plan-selection-v0.md` (1 of 172),
`application-flow-navigation-execution-v0.md` (1 of 7), `batch-integration-checks-v0.md` (1 of
2), `capability-catalog-v0.md` (1 of 71), `documentation-ci-v0.md` (1 of 11),
`documentation-corpus-v1.md` (1 of 257), `documentation-corpus-verified-reads-v1.md` (1 of 49),
`go-test-evidence-v0.md` (1 of 7), `lane-verification-tooling-v0.md` (1 of 10),
`performance-profiling-and-read-budget-v0.md` (1 of 52), `task-context-packet-v0.md` (1 of 97),
`test-selection-precision-v0.md` (1 of 50), `test-source-analysis-v0.md` (1 of 283).

## Specs whose intent was never recorded as accepted

Only the owner accepts intent. The header of a spec and the acceptance note on each requirement line
are the two places where an acceptance is recorded. Four groups of specs lack a full record. The 8
specs whose header reads retired, superseded or rejected are left out.

### The header does not say accepted

62 specs have a header intent of `proposed`, `experimental` or `per requirement`.

- KEEP (21): `affected-plan-python-playwright-v0.md`, `affected-zero-unknown-v0.md`,
  `documentation-corpus-extraction-delta-v1.md`, `documentation-corpus-inventory-v1.md`,
  `documentation-corpus-search-ranking-v1.md`, `documentation-corpus-v1.md`,
  `documentation-corpus-verified-reads-v1.md`, `dotnet-affected-selection-v0.md`,
  `import-ratchet-v0.md`, `js-live-test-provider-v0.md`, `kotlin-jvm-affected-test-v0.md`,
  `operator-update-v0.md`, `process-group-owner-v0.md`, `release-artifact-integrity-v0.md`,
  `revision-cache-dirty-worktree-v0.md`, `ruby-affected-test-adapter-v0.md`,
  `rust-affected-test-understanding-v0.md`, `source-documentation-draft-v0.md`,
  `swift-affected-test-understanding-v0.md`, `test-runner-execution-v0.md`,
  `typescript-javascript-affected-adapter-v0.md`
- MERGE (4): `affected-reader-precision-v0.md`, `application-flow-understanding-v0.md`,
  `ci-control-plane-early-post-v0.md`, `playwright-per-test-freshness-v0.md`
- RETIRE (5): `lsp-editor-context-v0.md`, `lsp-editor-qualification-v0.md`,
  `lsp-wrapped-backends-v0.md`, `source-evidence-axes-v0.md`, `trial-replay-journal-v0.md`
- OUT-OF-1.0 (32): `agent-instruction-doctor-v0.md`, `aggregate-local-outcome-v0.md`,
  `analyzer-candidate-profiles.md`, `analyzer-python-native-candidate.md`, `cem-2.0-intent.md`,
  `cem2-discovery-replay-v0.md`, `confidently-wrong-trial-v0.md`, `context-evolution-program-v0.md`,
  `context-ranking-v0.md`, `corvint-self-development-gates-v0.md`,
  `cross-audience-truth-nondivergence-v0.md`, `cross-repository-breakage-map-v0.md`,
  `expanded-range-impact-v0.md`, `go-test-evidence-v0.md`, `heading-navigation-experiment-v0.md`,
  `kotlin-android-candidate-v0.md`, `native-cem-adapter.md`,
  `native-jvm-bridge-analyzer-candidates-v0.md`, `non-go-impact-v0.md`,
  `observed-ui-evidence-v0.md`, `proof-tour-v0.md`, `retrieval-bench-diagnostics-v0.md`,
  `self-observation-ledger-v0.md`, `shader-glsl-metal-analyzer-candidate-v0.md`,
  `swift-apple-analyzer-candidate-v0.md`, `task-context-packet-v0.md`, `test-attack-v0.md`,
  `test-claim-qualification-v0.md`, `unplanned-read-events-v0.md`, `use-case-conformance-v0.md`,
  `verification-planner-observer-v0.md`, `vscode-extension-v0.md`

### The header says accepted and names no decision

24 specs say `accepted` and name no decision, pull request or owner statement, so the acceptance
cannot be checked from the file.

- KEEP (10): `affected-edit-oracle-v0.md`, `application-map-scenario-planner-v0.md`,
  `application-map-v0.md`, `java-affected-test-v0.md`, `language-plugins-v0.md`,
  `map-coverage-join-v0.md`, `native-shell-affected-test-v0.md`,
  `playwright-external-provider-v0.md`, `run-verified-navigation-v0.md`,
  `spec-requirement-index-generator-v0.md`
- MERGE (6): `batch-integration-checks-v0.md`, `corvint-self-development-companion-v0.md`,
  `lane-check-tooling-v0.md`, `lane-dogfood-tooling-v0.md`, `lane-shell-portability-v0.md`,
  `windows-test-portability-v0.md`
- RETIRE (1): `spec-toolchain-adapters-v0.md`
- OUT-OF-1.0 (7): `cem-pilot-kit.md`, `corvint-rebrand-v0.md`,
  `deployment-neutral-index-platform-v0.md`, `genesis-backfill.md`, `go-only-cutover-v0.md`,
  `go-production-kernel-migration-v0.md`, `rust-analyzer-candidate-v0.md`

### The header rests only on the standing auto-accept

17 specs have a header that records acceptance only under the standing auto-accept for lane work.
That is a recorded owner approval of the practice, not of each requirement.
`docs/SPEC-DRIVEN-DEVELOPMENT.md` says such a requirement becomes `owner` only when the owner reads
it back, and none of these headers records a read-back.

- KEEP (10): `additive-narrowing-v0.md`, `affected-cli-consistency-v0.md`,
  `affected-plan-reuse-v0.md`, `affected-plan-selection-v0.md`, `capability-catalog-v0.md`,
  `ci-narrowing-v0.md`, `go-line-selection-v0.md`, `performance-profiling-and-read-budget-v0.md`,
  `test-selection-precision-v0.md`, `workspace-declaration-v0.md`
- MERGE (3): `corvint-self-development-selection-v0.md`, `lane-verification-tooling-v0.md`,
  `shared-path-change-policy-v0.md`
- RETIRE (1): `reproduction-bundle-v0.md`
- OUT-OF-1.0 (3): `cem-additive-profiles-v0.md`, `cem-completeness-profile-v0.md`,
  `test-source-analysis-v0.md`

### The header says accepted and no requirement line records it

28 specs have an accepted header while every one of their requirement lines is `unrecorded`. The
lines inherit the header, so the acceptance rests on the header alone. The number after each name is
its line count.

- KEEP (10): `application-flow-navigation-execution-v0.md` 7,
  `application-map-scenario-planner-v0.md` 10, `core-bundled-apps-v0.md` 11,
  `external-test-selection-v0.md` 14, `flow-document-maintenance-v0.md` 7,
  `go-live-test-provider-v0.md` 54, `human-documentation-compiler-v0.md` 66,
  `java-affected-test-v0.md` 10, `run-verified-navigation-v0.md` 8,
  `trusted-project-navigation-v0.md` 6
- MERGE (5): `external-evidence-provider-mcp-v0.md` 4, `external-evidence-provider-v1.md` 14,
  `external-evidence-provider-v2.md` 15, `external-test-selection-v1.md` 9, `greptile-gate-v0.md` 2
- RETIRE (3): `external-frontier-obligations-v0.md` 9, `lsp-quality-platform-v0.md` 22,
  `proof-carrying-context-optimization-v0.md` 17
- OUT-OF-1.0 (10): `deployment-neutral-index-platform-v0.md` 74, `eol-policy-gate-v0.md` 9,
  `expert-audit-followup-v0.md` 13, `genesis-backfill.md` 25, `go-format-gate-v0.md` 7,
  `go-only-cutover-v0.md` 13, `optional-artifact-check-scripts-v0.md` 8,
  `rust-analyzer-candidate-v0.md` 11, `sql-native-ratchet-gate-v0.md` 17,
  `test-consolidation-planner-v0.md` 16

For the 71 KEEP specs the picture is: 30 accepted with a source named, 10 accepted with no decision
named, 10 standing only and 21 whose header does not say accepted. At line level the KEEP specs hold
138 lines with an owner acceptance, 1708 standing and 1649 unrecorded.

## Oversized specs and proposed splits

`make spec-capacity-check` reports three surviving specs at the limit of 256 requirements in one
file: `affected-plan-v0.md`, `corvint-self-development-v0.md` and `documentation-corpus-v1.md`. No
requirement can be added to them, which is why each already has companion files. The table lists
every surviving spec with 170 or more requirement lines and proposes how to split it by product
requirement. No split was performed. The Reqs column counts lines in `docs/specs/REQUIREMENTS.tsv`;
the capacity check counts obligations in its own way, so the two can differ, as they do for
`documentation-corpus-v1.md`, `test-source-analysis-v0.md` and `cem-stable-v1.md`.

| Spec | Reqs | Class | Untraced | Proposed split |
|---|---:|---|---:|---|
| `affected-plan-v0.md` | 256 | KEEP | 0 | Three files by product requirement; see the next section. |
| `affected-plan-selection-v0.md` | 172 | KEEP | 1 | A companion file under the same prefix. Fold its 22 sections into the same three subjects as the affected plan. Not analysed line by line. |
| `corvint-self-development-v0.md` | 256 | KEEP | 79 | Self-use that the release records, for PROD-INT-009, apart from repository process, which goes to the gates file. Nearly all lines sit under one heading, so the split needs a line-by-line pass, and the lines that name removed make targets retire. |
| `documentation-corpus-v1.md` | 257 | KEEP | 1 | Search and verified reads for PROD-DOCS-009 apart from retrieval evaluation and claim audit sampling, which are measurement. Four companion files already exist. |
| `application-map-v0.md` | 191 | KEEP | 3 | The map itself for PROD-DOCS-006 apart from how-to answers for PROD-DOCS-007. All 191 lines sit under one heading, so the split needs a line-by-line pass. |
| `live-proof-carrying-verification-v0.md` | 190 | KEEP | 1 | Watch and save-time selection for PROD-TEST-007; line-level selection for PROD-TEST-008; runtime providers for PROD-TEST-006; and the shared test-validity projection, which serves no product requirement. The file has section headings for each. |
| `test-source-analysis-v0.md` | 283 | OUT-OF-1.0 | 1 | Outside 1.0. No split is proposed unless the owner brings test validity into 1.0. |
| `cem-stable-v1.md` | 360 | OUT-OF-1.0 | 66 | Outside 1.0. No split is proposed unless the owner brings change evidence into 1.0. |

### Proposed split of the affected plan

`affected-plan-v0.md` holds 256 requirement lines under 71 headings. The prefix AFP-V0 already spans
seven files: besides the 256, `affected-plan-selection-v0.md` holds 172 lines,
`affected-plan-python-playwright-v0.md` 48, `affected-zero-unknown-v0.md` 19,
`affected-reader-precision-v0.md` 17, `affected-plan-reuse-v0.md` 15 and `ci-narrowing-v0.md` 7. A
requirement ID therefore does not tell a reader which file to open or which product requirement it
serves.

The proposal sorts the 71 headings into three files. The sorting used the heading text only; the
lines under each heading were not read one by one, so a few will belong elsewhere. Every ID stays as
it is. The counts are requirement lines.

- The plan contract, 86 lines, for PROD-TEST-001. It stays in `affected-plan-v0.md`. Headings: the
  opening block of 14 lines; Go plugin modules, workspaces and command edges; planner inputs,
  receipt projection and advisory checks; path-list plans and impact help; fixture Git identity and
  historical replay; plan observation bounds; range prefixes, directory paths and plugin path
  evidence; Windows production path logic; cross-call graph reuse and batch paths; default graph
  cache location; ignored untracked files in path mode; selector reach shared with the planner;
  independent nested-module units; explained, shared and planner-equal selection; per-path breakdown
  of a path list; a stale self-binary; a merged output stream; a few-line verdict; typed `--path`
  profile refusals.
- The rules that keep a required test from being left out, 64 lines, for PROD-TEST-002. They move to
  `affected-zero-unknown-v0.md`, which would then hold 135 lines after the merge proposed above.
  Headings: declared confined test read scopes and unbounded readers; declared documentation paths
  and scope-bounded literal readers; over-bound sources and the selection audit; two-repository E2E
  inputs for the audit; import-chain refusals; fixture-sited, scoped and change-scoped frontiers;
  frame-metadata calls and compile checks; symlink hints and test-less readers; lexicon tags and
  `--path` recovery; temp-root literals; escaped closing tags and build-constrained test files; Go
  builds as dependency reads; root-resolved refusals; built-binary readers; frontier routes;
  declared binary consumers; agent instruction files; confined driver runs; two Go lexers; unbounded
  readers of declared paths; declared existence readers; closed declarations; remediation files.
- CI execution and narrowing, 106 lines, for PROD-TEST-012. They move to `ci-narrowing-v0.md`, which
  would then hold 157 lines. Headings: the shared container profile; hosted control plane and
  qualification; the complete-universe partition profile; CI run reuse and control-plane jobs;
  pull-request narrowing, shadow selection and the forward bar; shard cost tables; local and CI
  Windows vet; the self-hosted shard runner; release and live-browser jobs; CI wall time; shadow
  selector tiers and cost; the tier exemption; the weekly flake report; pull-request shard
  parallelism; balanced narrowed shards, the largest heading with 22 lines; merge-queue narrowing
  and reuse; artifact listing pages; historical replay of the shadow tiers; narrowing repairs; the
  one unknown allowlist; main failures linked back to narrowed pull requests; sampled full runs; the
  sixteen-shard rebalance; wall-time shard placement.

Performing the split is a spec change. It needs the owner to accept it, one commit that moves the
definition lines and regenerates the index, and a run of `make line-citations-check`, because
citations that point into the file by line would move. The new requirements for one plan with a kind
per test are proposed for a new file named `affected-plan-kinds-v0.md`, which does not exist yet,
for the same reason: the affected plan cannot take another line.

## Language and runner specs for PROD-TEST-011

The plan says under PROD-TEST-011 that every listed language and runner selects and runs, and that
any other gets a full run with a reason. These are the specs behind the list.
`internal/liveverify/affected/` holds directories named cfamily, dotnet, golang, java, kotlin,
languages, python, ruby, rust, shell, swift and typescript.

| Spec | Language or runner | Intent | Delivery | Reqs | In tests | Untraced |
|---|---|---|---|---:|---:|---:|
| `affected-plan-v0.md` | Go | accepted | experimental | 256 | 214 | 0 |
| `additive-narrowing-v0.md` | Go | standing | experimental | 15 | 11 | 0 |
| `go-line-selection-v0.md` | Go, line level | standing | experimental | 29 | 23 | 0 |
| `go-live-test-provider-v0.md` | Go, running | accepted | experimental | 54 | 17 | 2 |
| `affected-plan-python-playwright-v0.md` | Python and Playwright | per requirement | experimental | 48 | 45 | 0 |
| `typescript-javascript-affected-adapter-v0.md` | JavaScript and TypeScript | proposed | experimental | 108 | 92 | 0 |
| `js-live-test-provider-v0.md` | JavaScript and TypeScript, running | proposed | experimental | 0 | 0 | 0 |
| `ruby-affected-test-adapter-v0.md` | Ruby | proposed | experimental | 26 | 11 | 0 |
| `java-affected-test-v0.md` | Java | accepted, no decision named | experimental | 10 | 0 | 0 |
| `kotlin-jvm-affected-test-v0.md` | Kotlin on the JVM | proposed | experimental | 12 | 4 | 8 |
| `rust-affected-test-understanding-v0.md` | Rust | proposed | experimental | 10 | 0 | 0 |
| `swift-affected-test-understanding-v0.md` | Swift | proposed | experimental | 13 | 0 | 0 |
| `dotnet-affected-selection-v0.md` | .NET | proposed | experimental | 10 | 0 | 2 |
| `native-shell-affected-test-v0.md` | C family and shell | accepted, no decision named | experimental | 17 | 7 | 0 |
| `runner-selection-plugins-v0.md` | Any runner through a plugin | accepted | experimental | 62 | 39 | 3 |
| `language-plugins-v0.md` | Any language through a manifest | accepted, no decision named | experimental | 27 | 10 | 2 |

No requirement ID of `java-affected-test-v0.md`, `rust-affected-test-understanding-v0.md`,
`swift-affected-test-understanding-v0.md` or `dotnet-affected-selection-v0.md` appears in a test
file. Every spec in the table has the delivery `experimental`, so none claims to be validated. The
plan lists the supported set as an owner decision.

## The ten most important actions

1. Decide which document defines 1.0: the product plan or `corvint-1.0-product-and-release-v1.md`.
   Everything else in this report assumes the plan.
2. Merge the Tasks specs to the `main` branch of the Tasks repository, classify them against the
   nine task manager requirements, and settle who owns the Tasks MCP tools.
3. Give `corvint how` a surviving contract and remove the dependency of the map join on the retired
   how-to spec.
4. Split `affected-plan-v0.md` three ways as proposed, so that each part names one product
   requirement and new plan requirements have a home.
5. Choose one contract for running tests between `session-test-runner-v0.md` and
   `test-runner-execution-v0.md`, and give the JavaScript and Ruby providers numbered requirements.
6. Accept or rewrite the layer contract: bring `import-ratchet-v0.md` and `core-bundled-apps-v0.md`
   into one accepted statement of the five layers, and name the owning layer in the flow spec.
7. Retire the 19 specs proposed for retirement, after checking the five that rest on a text search,
   so that the index stops listing cut and superseded surface as live.
8. Record acceptance for the KEEP specs. 41 of the 71 have no owner acceptance that names a source,
   and 3357 of their 3495 lines carry no owner acceptance of their own. Start with the specs whose
   every line is unrecorded.
9. Close the evidence gaps in the KEEP specs: 211 untraced lines, and 5 KEEP specs with no ID in any
   test file. Correct the headers that misstate delivery, starting with the watch contract.
10. Decide whether the kept commands, change evidence, the analyzers and the editor extension are
    part of 1.0. If they are, add product requirements; if they are not, say so in the release
    notes. Rewrite `corvint-self-development-v0.md` either way, because it names make targets that
    no longer exist.

## What this report did not do

- It changed no spec. Nothing was merged, retired, split, renumbered or accepted.
- It did not read every spec body. Classes rest on the index, the digests, the requirement index,
  section headings and targeted searches. A class can be wrong where a digest is out of date.
- It ran no Go test. "In tests" and "Untraced" are text counts, not test results.
- It did not open the decisions that accepted headers cite, and it did not check each line of a spec
  against decision 0594.
- It did not classify the 38 Tasks spec files. They are retired here and are not yet on the `main`
  branch of the Tasks repository.
- It did not verify by running a binary that a command is absent; those rows say "Verify first".
