# Spec delta: `docs-mcp-hardening` (change `docs-mcp-hardening`)

## ADDED Requirements

### Requirement: DMF-V1-001 Waived claims show as waived

Flow-doc generation and `flows docs --check` MUST both apply the waiver file. A waived claim MUST
render on the page and in the sidecar as `WAIVED until DATE (now STATE)`, where STATE is the claim's
current evaluated state. A waived claim SHALL NOT render as PROVEN unless its current state is
PROVEN. An expired waiver MUST NOT apply, and the check MUST name it. Serves PROD-DOCS-005.
(finding 2; `internal/appflows/docs.go:637-669`, integ 9ec3a29d0)

#### Scenario: Waived claim on the page

- **GIVEN** a claim whose committed page says PROVEN, whose current state is FAILED, and an active
  waiver until 2026-12-31
- **WHEN** flow docs are generated and then checked
- **THEN** the page and the sidecar say `WAIVED until 2026-12-31 (now FAILED)` and the check passes

#### Scenario: Expired waiver

- **GIVEN** the same claim with a waiver that expired yesterday
- **WHEN** `flows docs --check` runs
- **THEN** the check fails, the claim shows FAILED and the report names the expired waiver

### Requirement: DMF-V1-002 One repository docs check

`corvint-docs docs check` MUST find and check every marker block, every flow-claim page in the
flows manifest and every line citation in tracked Markdown, with no per-page arguments. It MUST print
one JSON report with one entry per part, each `PASS`, `STALE` or `NOT_CHECKED` with a reason, and
each stale item with its path and line. It MUST exit 1 when any part is `STALE` and 0 otherwise.
Serves PROD-DOCS-003. (finding 1; `cmd/corvint-docs/docs_maintain.go:101`)

#### Scenario: One stale block

- **GIVEN** a repository with fresh citations, fresh flow pages and one marker block whose source
  changed
- **WHEN** `corvint-docs docs check` runs
- **THEN** it exits 1 and the report lists that block's page and line under `STALE`

#### Scenario: Nothing to check

- **GIVEN** a repository with no markers, no flows manifest and no citations
- **WHEN** `corvint-docs docs check` runs
- **THEN** it exits 0 and every part is `NOT_CHECKED` with reason `none-found`

### Requirement: DMF-V1-003 Maintain check exit code

`docs maintain --check` MUST exit non-zero when any selected block is stale and 0 when none is.
`docs maintain --preview` MUST keep its current exit behaviour. Serves PROD-DOCS-003. (finding 1;
`cmd/corvint-docs/docs_maintain.go:133-157`)

#### Scenario: Stale block fails the check

- **GIVEN** a page with one stale marker block
- **WHEN** `docs maintain --check` runs on it
- **THEN** it exits 1 and names the block

### Requirement: DMF-V1-004 Complete citation check

The product citation check MUST report `missing-file` for every cited path that is not tracked,
whatever its directory. It MUST skip citations inside fenced code blocks. It MUST count unpinned
citations and list them in the report. It MUST offer `--json` output with one object per failure and
the counts. Serves PROD-DOCS-003. (finding 3; `internal/doccitations/check.go:119-131`, `:193-198`)

#### Scenario: Missing top-level file

- **GIVEN** a tracked document citing `README.md:10` where `README.md` is not tracked
- **WHEN** the citation check runs
- **THEN** it reports `missing-file` for that citation

#### Scenario: Fenced and unpinned citations

- **GIVEN** a document with `a.go:3` inside a code fence and `b.go:4` outside a fence with no pin
- **WHEN** the citation check runs with `--json`
- **THEN** `a.go:3` is not checked, `b.go:4` is listed as unpinned and the JSON counts both groups

### Requirement: DMF-V1-005 Reports say when the worktree differs

Every docs check report (DMF-V1-002, citations, `docs maintain --check`, `flows docs --check`,
`docs flows check`) MUST carry a `worktree` member: `clean`, or `dirty` with the count of changed
tracked paths it read from Git and the first of those paths. The check MUST still answer only about
committed or indexed content. Serves PROD-DOCS-003 and PROD-DOCS-005. (finding 4;
`internal/appflows/docs.go:168-198`, `internal/doccitations/check.go:70`)

#### Scenario: Edited page not yet committed

- **GIVEN** a flow-doc page edited in the worktree and not committed
- **WHEN** `flows docs --check` runs
- **THEN** the report says `worktree: dirty`, counts 1 path and names the page

### Requirement: DMF-V1-006 Byte failures name the changed claims

When `flows docs --check` finds rendered bytes differ, it MUST list each claim whose state or text
changed, with old and new state, and say whether the page, the sidecar or both differ. It MUST name
the command that regenerates the file. When a sidecar is missing, the claim list MUST be `UNKNOWN`
and the byte failure MUST stand. Serves PROD-DOCS-005. (finding 14; `internal/appflows/docs.go:681-686`)

#### Scenario: One claim changed

- **GIVEN** a committed page where claim `login.ok` is PROVEN and its current state is FAILED
- **WHEN** `flows docs --check` runs
- **THEN** the failure lists `login.ok` from PROVEN to FAILED, says page and sidecar differ and names
  the regenerate command

### Requirement: DMF-V1-007 No clean result without comparing pages

`docs flows generate` MUST record its output directory in the manifest. `docs flows check` MUST
compare rendered pages against that directory, or against `--output-dir` when given. When it has no
directory to compare, or the directory is missing, it MUST report `output: NOT_CHECKED` with a reason
and MUST NOT report `clean: true`. Serves PROD-DOCS-005. (finding 13;
`internal/flowdocs/check.go:219-227`, `internal/flowdocs/generate.go:73-85`)

#### Scenario: No output directory

- **GIVEN** a manifest with no recorded output directory
- **WHEN** `docs flows check --revision R --previous P` runs without `--output-dir`
- **THEN** the report says `output: NOT_CHECKED`, `clean: false`

#### Scenario: Recorded directory with an edited page

- **GIVEN** a manifest that records `docs/flows` and a page there edited by hand
- **WHEN** `docs flows check` runs without `--output-dir`
- **THEN** it compares `docs/flows` and reports the edited page

### Requirement: DMF-V1-008 Distinct tool descriptions

Each `corvint.docs_*` MCP tool MUST have its own description that says what task it serves, what it
returns and which other tool to use for the nearest different task. Each input property MUST have a
description. No two docs tools SHALL share a description. Serves PROD-DOCS-001 and PROD-DOCS-009.
(findings 5, 16; `internal/mcp/corpusbridge/corpusbridge.go:304-308`)

The docs tools are re-described, not pruned. After `corvint.docs_recommend_tests` is removed
(DMF-V1-009), each of the 19 remaining `corvint.docs_*` tools MUST get a distinct description and
a description on every input property. A docs tool MAY be removed before the freeze only when a
fixture test shows that its answer is a strict subset of another tool's answer for the same
input. Any other removal MUST wait for 1.1 and MUST be announced one release ahead. (decided
2026-11-09 under R42, B9)

#### Scenario: Agent lists tools

- **WHEN** an MCP client calls `tools/list` on the docs server
- **THEN** every docs tool has a different description, each description names one alternative tool,
  and every input property has a description

#### Scenario: A tool removed without the subset proof

- **GIVEN** a change that removes one of the 19 docs tools and adds no strict-subset fixture test
- **WHEN** the tool table test runs
- **THEN** it fails and names the missing tool

### Requirement: DMF-V1-009 No second test-selection engine

No docs MCP tool SHALL answer which tests to run from its own engine. `corvint.docs_recommend_tests`
MUST be removed, and the docs tool descriptions MUST point test questions to the testing server
`corvint mcp` (R42 (panel pick) B2). Serves PROD-DOCS-001. (finding 16;
`internal/mcp/corpusbridge/corpusbridge.go:147`, `internal/doccorpus/query.go:693`)

#### Scenario: Test question to the docs server

- **GIVEN** the docs MCP server running
- **WHEN** an agent asks a docs tool which tests cover a changed path
- **THEN** no docs tool offers that question, and the docs tool descriptions name `corvint mcp`

### Requirement: DMF-V1-010 Actionable tool errors

Every MCP tool error MUST carry `code`, a `message` that differs from the code and says what failed,
`retryable` (true or false) and `next` (the argument change or command that may fix it, or empty).
A full concurrency limit MUST return code `server-busy` with `retryable: true`. A cancelled call
MUST return code `cancelled`. A failed child process MUST keep its last stderr line in `message`.
These SHALL NOT return `-32603 Internal error`. Serves PROD-DOCS-001. (finding 6;
`internal/mcp/cli/main.go:568-578`, `internal/mcp/server/server.go:219-221`,
`internal/mcp/docsbridge/docsbridge.go:177-192`)

#### Scenario: Server busy

- **GIVEN** the server at its concurrency limit
- **WHEN** another tool call arrives
- **THEN** the error has code `server-busy`, `retryable: true` and a message naming the limit

#### Scenario: Child process fails

- **GIVEN** corvint-docs exits 2 with stderr ending `no corpus artifact at .corvint/corpus`
- **WHEN** the docs bridge returns the error
- **THEN** `message` contains that line and `next` names the build command

### Requirement: DMF-V1-011 Rebuilt corpus served without restart

When the configured corpus artifact changes during a session, the next docs call MUST verify the new
artifact with the same checks as the first open and then answer from it. Each receipt MUST name the
artifact digest it used. If verification fails, the call MUST refuse with the reason and MUST NOT
answer from the old artifact. Serves PROD-DOCS-009 and PROD-DOCS-001. (finding 8;
`internal/mcp/corpusbridge/corpusbridge.go:424-429`, `group.go:223-240`)

#### Scenario: Rebuild mid-session

- **GIVEN** a running docs server that answered one `docs_search` call
- **WHEN** the corpus artifact is rebuilt and `docs_search` is called again
- **THEN** the call answers without a restart and its receipt names the new digest

#### Scenario: Corrupt rebuild

- **GIVEN** a running docs server
- **WHEN** the artifact is replaced with one that fails verification and a docs tool is called
- **THEN** the call refuses with the verification reason

DMF-V1-012 (`how` gaps name what to fix) is retired here: it moved to change `how-rewrite` as
HOW-V1-012 (R42 (panel pick) B1). The ID is not reused.

### Requirement: DMF-V1-013 `docs consume` stale message names what moved

`docs consume` MUST keep its strict byte binding for 1.0. When it refuses a stale draft, the refusal
MUST name the commit or tree that moved since the draft was bound, and say how to re-draft. A
cited-files mode bound to a `source_digest` of the cited files only is a 1.x proposal and SHALL NOT
be added here. Serves PROD-DOCS-003. (finding 11; R42 (panel pick) B5)

#### Scenario: Stale draft refused with the moved revision

- **GIVEN** a draft bound at commit A and HEAD now at commit B with a changed tree
- **WHEN** `corvint-docs docs consume` runs on that draft
- **THEN** it refuses, the message names A and B, and it names the re-draft command
