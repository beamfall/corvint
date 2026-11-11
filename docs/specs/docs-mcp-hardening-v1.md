# Docs MCP Hardening V1

Owner: Russell Lewis
Date: 2026-11-11
Requirement prefix: `DMF-V1`
Intent status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5409)
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/docs-mcp-hardening/`
(`proposal.md`, `design.md`, `tasks.md` and `specs/docs-mcp-hardening/spec.md`),
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-DOCS-001, 003, 005, 007 and 009) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (R42 B1 to B5, B9).

## Agent digest
- Claim: One docs check reports staleness honestly; docs MCP tools get distinct descriptions, one actionable error shape and a corpus reload without restart.
- Status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5409) / not-started
- Exists: the change folder only.
- Blocked on: tasks 3 to 14 of the change.
- Read next: Requirements; Delivery and traceability; Rollback.

## User and boundary

A user or agent needs two promises kept by the docs product: its checks say whether documentation
still matches the code (PROD-DOCS-003, 005), and its MCP server gives an agent tools it can choose
between and recover from (PROD-DOCS-001, 009). This spec is the canonical home of DMF-V1-001 to
DMF-V1-011 and DMF-V1-013, copied from the change's spec delta. Decisions, failure modes and open
questions stay in the change's `design.md`, which owns them. Each delivery is recorded in one
`docs/build-log/YYYY-MM-DD-<slug>.md` entry.

Non-goals (from the change): no citation repinning or move detection in the product check; no
change to MCP result sizing, truncation or duplication; no full output schemas for tools; no
caching, memoising or watch-loop changes; no relaxed draft binding for `docs consume` (cited-files
mode is 1.x, R42 B5); no renaming of the docs verb families (owned by `flows-status-report`
AFL-V1-008, R42 B4); no `how` ranking or gap change (HOW-V1-012, R42 B1); no failure for
unanchored documents and no progress notifications.

Trust boundary: every check answers only about committed or indexed Git content (product
invariant 1); the worktree is reported, never read as evidence. A check that cannot compare
reports not-clean with a reason, never clean.

## Requirements

Requirement IDs are stable. Each requirement keeps the change's text, with source paths kept and
line numbers dropped (the change's proposal holds the line numbers at integ 9ec3a29d0); a closing
`Delivery:` line says what this tree delivers.

- `DMF-V1-001`: (accepted automatically under G14, decision 0596; V1-5409) Waived claims show as waived

Flow-doc generation and `flows docs --check` MUST both apply the waiver file. A waived claim MUST
render on the page and in the sidecar as `WAIVED until DATE (now STATE)`, where STATE is the claim's
current evaluated state. A waived claim SHALL NOT render as PROVEN unless its current state is
PROVEN. An expired waiver MUST NOT apply, and the check MUST name it. Serves PROD-DOCS-005.
(finding 2; `internal/appflows/docs.go`, integ 9ec3a29d0)

**Scenario: Waived claim on the page**

- GIVEN a claim whose committed page says PROVEN, whose current state is FAILED, and an active
  waiver until 2026-12-31
- WHEN flow docs are generated and then checked
- THEN the page and the sidecar say `WAIVED until 2026-12-31 (now FAILED)` and the check passes

**Scenario: Expired waiver**

- GIVEN the same claim with a waiver that expired yesterday
- WHEN `flows docs --check` runs
- THEN the check fails, the claim shows FAILED and the report names the expired waiver

Delivery: `internal/appflows/docs.go` reads the waiver file in `buildDocs` for both `RenderDocs`
and `CheckDocs`; `markWaived` sets `waived_until` and `mark` on the sidecar claim and the page line,
and `lostClaims` excuses a lost PROVEN without restoring the committed form. `corvint flows docs`
accepts `--waivers` in generation too. Expired waivers are listed in `expired_waivers`. Tests:
`TestDMFV1001WaiverMarks`, `TestDMFV1001WaiverErrorLine` (`internal/appflows`) and
`TestDMFV1001WaivedClaimRendersWaived` (`internal/command/flows`). `flows docs maintain` still
refuses `--waivers` (not widened here).

- `DMF-V1-002`: (accepted automatically under G14, decision 0596; V1-5409) One repository docs check

`corvint-docs docs check` MUST find and check every marker block, every flow-claim page in the
flows manifest and every line citation in tracked Markdown, with no per-page arguments. It MUST print
one JSON report with one entry per part, each `PASS`, `STALE` or `NOT_CHECKED` with a reason, and
each stale item with its path and line. It MUST exit 1 when any part is `STALE` and 0 otherwise.
Serves PROD-DOCS-003. (finding 1; `cmd/corvint-docs/docs_maintain.go`)

**Scenario: One stale block**

- GIVEN a repository with fresh citations, fresh flow pages and one marker block whose source
  changed
- WHEN `corvint-docs docs check` runs
- THEN it exits 1 and the report lists that block's page and line under `STALE`

**Scenario: Nothing to check**

- GIVEN a repository with no markers, no flows manifest and no citations
- WHEN `corvint-docs docs check` runs
- THEN it exits 0 and every part is `NOT_CHECKED` with reason `none-found`

Delivery: `corvint-docs [--root PATH] docs check` takes no arguments and prints `corvint-docs-check/0`: parts `markers` (every committed Markdown page carrying a begin marker, through `docmaintain.Check`), `flows` (every tracked `generation.json` of the flows schema, checked at HEAD against its recorded output directory through `flowdocs.CheckRevision`; a manifest whose output cannot be compared is `NOT_CHECKED`, never `PASS`) and `citations` (`doccitations.Check` over tracked Markdown), each `PASS`, `STALE` or `NOT_CHECKED` with a reason and stale items with path and line, plus `worktree` per part where the composed check gives one and for the report. Exit 1 on any `STALE` part, 0 otherwise, 2 when the repository cannot be read. "The flows manifest" is read as the tracked `docs flows` generation manifests: `flows docs` claim pages need their intents directory and evidence files, which no committed file names, so they are not discovered here. Tests: `TestDMFV1002DocsCheckNothingFound`, `TestDMFV1002DocsCheckPassThenOneStaleBlock` (`cmd/corvint-docs`); the run on this repository is kept as evidence.

- `DMF-V1-003`: (accepted automatically under G14, decision 0596; V1-5409) Maintain check exit code

`docs maintain --check` MUST exit non-zero when any selected block is stale and 0 when none is.
`docs maintain --preview` MUST keep its current exit behaviour. Serves PROD-DOCS-003. (finding 1;
`cmd/corvint-docs/docs_maintain.go`)

**Scenario: Stale block fails the check**

- GIVEN a page with one stale marker block
- WHEN `docs maintain --check` runs on it
- THEN it exits 1 and names the block

Delivery: `corvint-docs docs maintain --check --page PATH [--source PATH --package DIRECTORY]` needs no `--enable`, reads the page as committed at HEAD and drafts at HEAD (`docmaintain.Check`), prints `corvint-docmaintain-check/0` and exits 1 naming each stale, missing or uncheckable block by page line, 0 when none, 2 when it cannot run. `--preview` keeps exit 0. Tests: `TestDMFV1003MaintainCheckExitCode` (`cmd/corvint-docs`) and `TestDMFV1003CheckStates` (`internal/docmaintain`).

- `DMF-V1-004`: (accepted automatically under G14, decision 0596; V1-5409) Complete citation check

The product citation check MUST report `missing-file` for every cited path that is not tracked,
whatever its directory. It MUST skip citations inside fenced code blocks. It MUST count unpinned
citations and list them in the report. It MUST offer `--json` output with one object per failure and
the counts. Serves PROD-DOCS-003. (finding 3; `internal/doccitations/check.go`)

**Scenario: Missing top-level file**

- GIVEN a tracked document citing line 10 of `README.md` where `README.md` is not tracked
- WHEN the citation check runs
- THEN it reports `missing-file` for that citation

**Scenario: Fenced and unpinned citations**

- GIVEN a document with `a.go:3` inside a code fence and `b.go:4` outside a fence with no pin
- WHEN the citation check runs with `--json`
- THEN `a.go:3` is not checked, `b.go:4` is listed as unpinned and the JSON counts both groups

Delivery: `internal/doccitations` reports `missing-file` for any untracked cited path, skips and counts tokens inside fenced code (`Report.Fenced`), and lists tracked citations with no pin in `Report.Unpinned`; `docs citations check --json` prints `corvint-docs-citations-check/0` (failures, unpinned, counts) and the text summary counts unpinned and fenced citations. SDD-V0-633 is amended to match. Tests: `TestDMFV1004CitationsMissingFencedUnpinned` (`internal/doccitations`), `TestDMFV1004CitationsCheckJSON` (`cmd/corvint-docs`).

- `DMF-V1-005`: (accepted automatically under G14, decision 0596; V1-5409) Reports say when the worktree differs

Every docs check report (DMF-V1-002, citations, `docs maintain --check`, `flows docs --check`,
`docs flows check`) MUST carry a `worktree` member: `clean`, or `dirty` with the count of changed
tracked paths it read from Git and the first of those paths. The check MUST still answer only about
committed or indexed content. Serves PROD-DOCS-003 and PROD-DOCS-005. (finding 4;
`internal/appflows/docs.go`, `internal/doccitations/check.go`)

**Scenario: Edited page not yet committed**

- GIVEN a flow-doc page edited in the worktree and not committed
- WHEN `flows docs --check` runs
- THEN the report says `worktree: dirty`, counts 1 path and names the page

Delivery: the new shared package `internal/docworktree` observes `git status` for the tracked paths a check read, against HEAD for committed reads and against the index for the citation check, and returns `{"state":"clean"}`, `dirty` with `changed` and the first five `paths`, or `unknown` with a `reason`. It is the `worktree` member of `docs citations check` (JSON, and a text line when not clean), `docs maintain --check`, `flows docs --check` (`flow-doc-check/0`) and `docs flows check`; `corvint-docs docs check` carries it too (DMF-V1-002). No check reads worktree bytes as evidence. Tests: `TestDMFV1005ParseStatus`, `TestDMFV1005ObserveRepository` (`internal/docworktree`), `TestDMFV1005CitationsWorktree` (`internal/doccitations`), `TestDMFV1005FlowsDocsCheckWorktree` (`internal/command/flows`), `TestDMFV1005DocsFlowsCheckWorktree` and `TestDMFV1005MaintainAndCitationsWorktree` (`cmd/corvint-docs`).

- `DMF-V1-006`: (accepted automatically under G14, decision 0596; V1-5409) Byte failures name the changed claims

When `flows docs --check` finds rendered bytes differ, it MUST list each claim whose state or text
changed, with old and new state, and say whether the page, the sidecar or both differ. It MUST name
the command that regenerates the file. When a sidecar is missing, the claim list MUST be `UNKNOWN`
and the byte failure MUST stand. Serves PROD-DOCS-005. (finding 14; `internal/appflows/docs.go`)

**Scenario: One claim changed**

- GIVEN a committed page where claim `login.ok` is PROVEN and its current state is FAILED
- WHEN `flows docs --check` runs
- THEN the failure lists `login.ok` from PROVEN to FAILED, says page and sidecar differ and names
  the regenerate command

Delivery: `flows docs --check` adds `byte_detail` on a byte failure: `differs` (page, sidecar or both), `claims` from a diff of the committed and regenerated sidecars (added, removed, state with old and new state, or detail), `claims_state` `UNKNOWN` with a reason when the committed sidecar is missing, and `regenerate`, the `corvint-docs flows docs` command with the same inputs. Bytes stay the authority. Test: `TestDMFV1006ByteFailureNamesChangedClaims` (`internal/command/flows`).

- `DMF-V1-007`: (accepted automatically under G14, decision 0596; V1-5409) No clean result without comparing pages

`docs flows generate` MUST record its output directory in the manifest. `docs flows check` MUST
compare rendered pages against that directory, or against `--output-dir` when given. When it has no
directory to compare, or the directory is missing, it MUST report `output: NOT_CHECKED` with a reason
and MUST NOT report `clean: true`. Serves PROD-DOCS-005. (finding 13;
`internal/flowdocs/check.go`, `internal/flowdocs/generate.go`)

**Scenario: No output directory**

- GIVEN a manifest with no recorded output directory
- WHEN `docs flows check --revision R --previous P` runs without `--output-dir`
- THEN the report says `output: NOT_CHECKED`, `clean: false`

**Scenario: Recorded directory with an edited page**

- GIVEN a manifest that records `docs/flows` and a page there edited by hand
- WHEN `docs flows check` runs without `--output-dir`
- THEN it compares `docs/flows` and reports the edited page

Delivery: `docs flows generate` records an output directory inside `--root` as the manifest's `output_dir` (omitted otherwise, so existing version 4 manifests rederive byte for byte); `docs flows check` compares `--output-dir`, else the recorded directory, and reports `output` `CHECKED` or `NOT_CHECKED` with `output_reason` `no-output-dir` or `missing-dir`, never `clean: true` without comparing. A missing `--output-dir` is now `NOT_CHECKED` rather than a refusal. Tests: `TestDMFV1007RecordedOutputDirectory` (`internal/flowdocs`), `TestDMFV1007DocsFlowsCheckRecordedOutput` (`cmd/corvint-docs`).

- `DMF-V1-008`: (accepted automatically under G14, decision 0596; V1-5409) Distinct tool descriptions

Each `corvint.docs_*` MCP tool MUST have its own description that says what task it serves, what it
returns and which other tool to use for the nearest different task. Each input property MUST have a
description. No two docs tools SHALL share a description. Serves PROD-DOCS-001 and PROD-DOCS-009.
(findings 5, 16; `internal/mcp/corpusbridge/corpusbridge.go`)

The docs tools are re-described, not pruned. After `corvint.docs_recommend_tests` is removed
(DMF-V1-009), each of the 19 remaining `corvint.docs_*` tools MUST get a distinct description and
a description on every input property. A docs tool MAY be removed before the freeze only when a
fixture test shows that its answer is a strict subset of another tool's answer for the same
input. Any other removal MUST wait for 1.1 and MUST be announced one release ahead. (decided
2026-11-09 under R42, B9)

**Scenario: Agent lists tools**

- WHEN an MCP client calls `tools/list` on the docs server
- THEN every docs tool has a different description, each description names one alternative tool,
  and every input property has a description

**Scenario: A tool removed without the subset proof**

- GIVEN a change that removes one of the 19 docs tools and adds no strict-subset fixture test
- WHEN the tool table test runs
- THEN it fails and names the missing tool

Delivery: each of the 19 corpus tools has its own description (task, what it returns, the nearest other tool) in `internal/mcp/corpusbridge/descriptions.go`, followed by the shared trust statement and the testing-server note, and `describeProperties` sets a description on every input property and every member of the retirement object; `corvint.docs_draft` and `corvint.docs_consume` name each other and describe their properties. No tool is pruned. The descriptions live beside the bridge's `tools` map, since the `mcp-tool-table` change that would own a shared table has not landed. Test: `TestDMFV1008DistinctToolDescriptions` (`internal/mcp/corpusbridge`) fails on a shared description, a description naming no alternative or not naming `corvint mcp`, a property without a description, or a missing table tool; the `tools/list` golden is re-recorded (descriptions only).

- `DMF-V1-009`: (accepted automatically under G14, decision 0596; V1-5409) No second test-selection engine

No docs MCP tool SHALL answer which tests to run from its own engine. `corvint.docs_recommend_tests`
MUST be removed, and the docs tool descriptions MUST point test questions to the testing server
`corvint mcp` (R42 (panel pick) B2). Serves PROD-DOCS-001. (finding 16;
`internal/mcp/corpusbridge/corpusbridge.go`, `internal/doccorpus/query.go`)

**Scenario: Test question to the docs server**

- GIVEN the docs MCP server running
- WHEN an agent asks a docs tool which tests cover a changed path
- THEN no docs tool offers that question, and the docs tool descriptions name `corvint mcp`

Delivery: `corvint.docs_recommend_tests` is gone from the corpus tool table, its repository qualifier branch, the MCP catalog row, `catalog.json` and `docs/CAPABILITIES.md`; a call to it is refused `unsupported-tool`. Every docs tool description names `corvint.affected` on `corvint mcp` for test questions. The `docs corpus recommend-tests` CLI read and its doccorpus operation stay (DMF-V1-009 governs MCP tools; removing the CLI read is outside this change). DCS-V1-595, DCP-V1-195, DCP-V1-310 and the corpus guide are amended. Tests: `TestDMFV1009NoDocsTestSelectionTool` (`internal/mcp/corpusbridge`); the callers in `cmd/corvint-docs`, `internal/doccorpus` and `internal/mcp/corpusbridge` tests now exercise the CLI read or the refusal; the `tools/list` golden is re-recorded without the tool.

- `DMF-V1-010`: (accepted automatically under G14, decision 0596; V1-5409) Actionable tool errors

Every MCP tool error MUST carry `code`, a `message` that differs from the code and says what failed,
`retryable` (true or false) and `next` (the argument change or command that may fix it, or empty).
A full concurrency limit MUST return code `server-busy` with `retryable: true`. A cancelled call
MUST return code `cancelled`. A failed child process MUST keep its last stderr line in `message`.
These SHALL NOT return `-32603 Internal error`. Serves PROD-DOCS-001. (finding 6;
`internal/mcp/cli/main.go`, `internal/mcp/server/server.go`,
`internal/mcp/docsbridge/docsbridge.go`)

**Scenario: Server busy**

- GIVEN the server at its concurrency limit
- WHEN another tool call arrives
- THEN the error has code `server-busy`, `retryable: true` and a message naming the limit

**Scenario: Child process fails**

- GIVEN corvint-docs exits 2 with stderr ending `no corpus artifact at .corvint/corpus`
- WHEN the docs bridge returns the error
- THEN `message` contains that line and `next` names the build command

Delivery: every tool-error object (`toolgroup.Failure`/`FailureResult`, the corpus `Failure` and the `corvint-mcp` tool-error object) keeps its members and adds `message` (the supplied message, or the code's own when the supplied one is empty or equals the code), `retryable` and `next` from one code table (`toolgroup.Detail`). A cancelled call is the `cancelled` tool error in the docs, test-validity, corpus, affected and help paths and in `corvint-mcp`. A request at the concurrency limit gets the `server-busy` tool error for `tools/call` and JSON-RPC error `-32000` with the same object otherwise, retryable and naming the limit. A failed `corvint-docs` process is `child-process-failed` with its last stderr line in `message` and a `next` (the corpus build command when the line names a corpus). Every `-32602` data object gains `code`, `message`, `retryable: false` and `next` after the MCPV0-952 members (amended there). Tests: `TestDMFV1010ServerBusy` (`internal/mcp/server`), `TestDMFV1010CancelledAndChildFailure` (`internal/mcp/toolgroup`), `TestDMFV1010ChildFailureKeepsStderr` (`internal/mcp/docsbridge`), `TestDMFV1010InvalidArgumentError` (`internal/mcp/cli`); the MCPV0-952 tests compare their closed members exactly and require the new ones.

- `DMF-V1-011`: (accepted automatically under G14, decision 0596; V1-5409) Rebuilt corpus served without restart

When the configured corpus artifact changes during a session, the next docs call MUST verify the new
artifact with the same checks as the first open and then answer from it. Each receipt MUST name the
artifact digest it used. If verification fails, the call MUST refuse with the reason and MUST NOT
answer from the old artifact. Serves PROD-DOCS-009 and PROD-DOCS-001. (finding 8;
`internal/mcp/corpusbridge/corpusbridge.go`, `group.go`)

**Scenario: Rebuild mid-session**

- GIVEN a running docs server that answered one `docs_search` call
- WHEN the corpus artifact is rebuilt and `docs_search` is called again
- THEN the call answers without a restart and its receipt names the new digest

**Scenario: Corrupt rebuild**

- GIVEN a running docs server
- WHEN the artifact is replaced with one that fails verification and a docs tool is called
- THEN the call refuses with the verification reason

Delivery: when a call reads configured artifact bytes whose digest differs from the served one, the registry verifies those same bytes (header, then `doccorpus.OpenRead` as the first call does) under the load slot and replaces the served digest, header, artifact, pins and index and freshness memos at once; a call already holding the previous artifact finishes on it. Receipts already name the artifact identity they used (`artifact_sha256`). A verification failure is `corpus-refused` naming the reason and saying the previous artifact is not served; nothing changes, so every later call checks the file again. The reload reuses the bytes and stamp the call read (no second read, DCR-V1-417). DCP-V1-143 and DCX-V1-498 are amended (no restart). Tests: `TestDMFV1011RebuiltArtifactServedWithoutRestart`, `TestDMFV1011CorruptRebuildRefused` (`internal/mcp/corpusbridge`), `TestDCXV1498MCPServesArtifactsAcrossBuilderUpgrades` (`cmd/corvint-docs`, now expecting the running server to answer); a live `corvint-mcp` session rebuilt the artifact between two `docs_search` calls and the second answered with the new digest.

DMF-V1-012 (`how` gaps name what to fix) is retired here: it moved to change `how-rewrite` as
HOW-V1-012 (R42 (panel pick) B1). The ID is not reused.

- `DMF-V1-013`: (accepted automatically under G14, decision 0596; V1-5409) `docs consume` stale message names what moved

`docs consume` MUST keep its strict byte binding for 1.0. When it refuses a stale draft, the refusal
MUST name the commit or tree that moved since the draft was bound, and say how to re-draft. A
cited-files mode bound to a `source_digest` of the cited files only is a 1.x proposal and SHALL NOT
be added here. Serves PROD-DOCS-003. (finding 11; R42 (panel pick) B5)

**Scenario: Stale draft refused with the moved revision**

- GIVEN a draft bound at commit A and HEAD now at commit B with a changed tree
- WHEN `corvint-docs docs consume` runs on that draft
- THEN it refuses, the message names A and B, and it names the re-draft command

Delivery: not yet built.

## Delivery and traceability

| Requirement | Task | Implementation | Evidence |
|---|---|---|---|
| DMF-V1-001 | 3 | `internal/appflows/docs.go`, `internal/command/flows/flows_docs.go` | `TestDMFV1001*` |
| DMF-V1-002 | 9 | `cmd/corvint-docs/docs_check.go`, `docmaintain.MarkerPages`, `flowdocs.TrackedManifests` | `TestDMFV1002*`; `.agent-evidence/docs-mcp-hardening/docs-check-corvint.json` |
| DMF-V1-003 | 4 | `internal/docmaintain/check.go`, `cmd/corvint-docs/docs_maintain.go` | `TestDMFV1003*` |
| DMF-V1-004 | 5 | `internal/doccitations/check.go`, `cmd/corvint-docs/docs.go` | `TestDMFV1004*` |
| DMF-V1-005 | 6 | `internal/docworktree`, the four check reports | `TestDMFV1005*` |
| DMF-V1-006 | 7 | `internal/appflows/docs.go` (`byteDetail`), `internal/command/flows/flows_docs.go` | `TestDMFV1006ByteFailureNamesChangedClaims` |
| DMF-V1-007 | 8 | `internal/flowdocs/generate.go`, `internal/flowdocs/check.go`, `cmd/corvint-docs/docs_flows.go` | `TestDMFV1007*` |
| DMF-V1-008 | 10 | `internal/mcp/corpusbridge/descriptions.go`, `internal/mcp/docsbridge/docsbridge.go` | `TestDMFV1008DistinctToolDescriptions`; `internal/mcp/cli/testdata/tools-list-groups.golden.jsonl` |
| DMF-V1-009 | 11 | `internal/mcp/corpusbridge/corpusbridge.go`, `internal/capcatalog/mcp.go` | `TestDMFV1009NoDocsTestSelectionTool`; `internal/mcp/cli/testdata/tools-list-groups.golden.jsonl` |
| DMF-V1-010 | 12 | `internal/mcp/toolgroup/errordetail.go`, `internal/mcp/protocol/types.go`, `internal/mcp/server/server.go`, `internal/mcp/docsbridge/docsbridge.go`, the corpus group and the `corvint-mcp` handler | `TestDMFV1010*` |
| DMF-V1-011 | 13 | `internal/mcp/corpusbridge/corpusbridge.go` (`reload`) | `TestDMFV1011*`; `.agent-evidence/docs-mcp-hardening/live-reload.txt` |
| DMF-V1-013 | 14 | not yet built | not yet built |

## Rollback

Each requirement lands as its own commit and reverts on its own. Reverting restores today's
behaviour: waivers stay in the report only, `docs check` and `--check` disappear, error objects lose
the added members and the artifact pin returns. DMF-V1 IDs are retired, not reused.
