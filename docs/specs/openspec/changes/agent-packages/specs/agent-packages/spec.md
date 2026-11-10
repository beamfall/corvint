# Spec delta: `agent-packages` (change `agent-packages`)

## ADDED Requirements

The lane copies each statement into `docs/specs/agent-packages-v1.md` inside one `## Requirements`
section, as a numbered definition line beginning `(proposed 2026-11-09; <ticket>; delivery
experimental)`.

### Requirement: AGT-V1-001 Support tiers

Source: G19. Claude Code and Codex MUST meet the full bar (AGT-V1-008 to AGT-V1-021) and qualify
under AGT-V1-023 before 1.0 is released. OpenCode and Pi MUST ship the same generated package marked
preview, MUST NOT block 1.0, and MUST NOT be called supported without a qualifying pass. Pi MUST be
installed only on explicit opt-in.

#### Scenario: OpenCode not qualified at the freeze

- **WHEN** the OpenCode package has no retained pass on 2026-12-15
- **THEN** 1.0 ships with OpenCode marked preview and the release is not held

### Requirement: AGT-V1-002 Home and licence

Source: G20. The packages, generator, skill text and qualification harness MUST live in the public
repository `beamfall/corvint-agents` under Apache-2.0 (owner decision R25, 2026-11-09). The
manifest that the Corvint binary emits MUST contain only data (names, events, versions, reason
codes), not the skill prose.

Earlier wording, superseded by R25 and review fix F12: "under a permissive licence (Apache-2.0
proposed; the owner fixes it)" and "the AGPL binary". `LICENSING.md` lines 5-10 and decision 0586
(integ 9ec3a29d0) place the binary's paths under Apache-2.0, so "AGPL binary" was wrong.

#### Scenario: Licence boundary

- **WHEN** the generator builds a package
- **THEN** every file in the package comes from `beamfall/corvint-agents` sources or from manifest data

### Requirement: AGT-V1-003 Hook command in the binary

Source: G20. `corvint agent hook <event> --host <claude-code|codex|opencode|pi>` MUST read the host
payload on stdin, update session state (AGT-V1-022), and write the host's expected reply on stdout.
All event mapping and decision logic MUST live in the binary; a shim MUST only pass the payload and
print the reply.

#### Scenario: Codex after-edit

- **WHEN** the Codex shim forwards a `PostToolUse` payload for an edit to `internal/a/a.go`
- **THEN** the binary records the path and replies with output Codex accepts and no block decision

### Requirement: AGT-V1-004 Versioned manifest

Source: G20. `corvint agent manifest --json` MUST emit a schema-versioned document listing the
binary version, hosts, events per host, MCP servers and tools, skill names, budgets and reason
codes. A schema change MUST bump the schema version.

#### Scenario: Generator reads the manifest

- **WHEN** the generator runs against Corvint 1.0.0
- **THEN** it reads the manifest and refuses a schema version it does not know

### Requirement: AGT-V1-005 One source, drift check

Source: G20. All four packages MUST be generated from one source against one pinned Corvint release,
and CI in `beamfall/corvint-agents` MUST fail when a committed package differs from regenerated
output.

#### Scenario: Hand edit to a generated file

- **WHEN** a pull request edits a generated Codex file by hand
- **THEN** the drift check fails and names the file

### Requirement: AGT-V1-006 Supported range, visible degradation

Source: G20. Each package MUST declare the Corvint versions it supports. Outside that range, or with
no binary on PATH, the shim MUST print one notice naming the found and supported versions and MUST
exit 0.

#### Scenario: Old binary

- **WHEN** Corvint 0.9 is on PATH and the package supports 1.0 to 1.0.x
- **THEN** one notice appears at session start and no hook output claims a test selection

### Requirement: AGT-V1-007 Distribution

Source: G20. The Claude Code and Codex packages MUST be published through each host's plugin
marketplace mechanism. The OpenCode and Pi packages MUST be published to npm with trusted publishing
and provenance, MUST have zero runtime dependencies and MUST declare no install scripts.

#### Scenario: npm package audit

- **WHEN** the Pi package is packed
- **THEN** its `package.json` lists no `dependencies` and no `preinstall`, `install` or `postinstall`

### Requirement: AGT-V1-008 One documented install step

Source: G19; review ci-agents F26. Each host MUST have one documented install step that installs
the package, registers MCP and enables the hooks, followed by `corvint agent doctor`. A step is
the shortest command sequence the host supports, written as one copyable block. The documented
commands per host are:

- Claude Code: `claude plugin marketplace add <repository>`, then
  `claude plugin install corvint@<marketplace> --scope user|project`. Two commands, because a
  plugin installs in one command only from a marketplace the host already knows.
- Codex: NOT_VERIFIED. Plugins install through `/plugins` inside Codex from a configured
  marketplace; no non-interactive command is confirmed. The documented step is the shortest
  supported one and the gap is reported to the owner.
- OpenCode: `opencode plugin add` with the npm package name.
- Pi: `pi install npm:<package>@<version>`.

Earlier wording, replaced on 2026-11-09 because Claude Code needs two commands: "Each host MUST
have one documented install command".

#### Scenario: Claude Code install

- **WHEN** the user runs the two documented Claude Code commands and then doctor
- **THEN** doctor reports MCP, skills and hooks active

#### Scenario: One block per host

- **WHEN** a reader opens the install page for any of the four hosts
- **THEN** the page shows one copyable block with that host's commands, ending in
  `corvint agent doctor`

### Requirement: AGT-V1-009 Doctor

Source: G19; decided 2026-11-09 under R42, D4. `corvint agent doctor` MUST be read-only and MUST
report, per detected host: binary
version against the package range, MCP registration, hook presence and trust state, skills present,
the kill switch state, the instruction-file check (AGT-V1-020), and whether the agent version is in
the package's qualified list. It MUST exit non-zero only when the package cannot work.

The agent commands MUST use one singular noun: `corvint agent doctor`, `corvint agent hook` and
`corvint agent manifest`. The binary MUST NOT accept `corvint agents` as an alias. Earlier
wording, superseded by D4: "`corvint agents doctor`".

#### Scenario: Plural noun is not a command

- **WHEN** the user runs `corvint agents doctor`
- **THEN** the binary reports an unknown command and exits non-zero

#### Scenario: Untrusted Codex hooks

- **WHEN** Codex hooks are installed but not yet trusted through `/hooks`
- **THEN** doctor reports hooks inactive and names the `/hooks` step

### Requirement: AGT-V1-010 MCP registration

Source: G19, answer 11, owner decisions R17 and R30 (2026-11-09). Each package MUST register
`corvint mcp` (testing tools) and, when corvint-docs is installed, `corvint-docs mcp` (docs tools),
with the tool names and schemas given by the table of change `mcp-tool-table`. Packages MUST NOT
register a Tasks MCP server; there is none in 1.0, and tasks are reached through the
`corvint-tasks` CLI. Where a host has no usable MCP client, the package MUST provide a CLI-backed
tool with the same read operations.

Earlier wording, superseded by R17 and R30: "the `corvint-docs` MCP server and, when installed, the
`corvint-tasks` MCP server". Pi has a built-in MCP client per the
upstream extensions page; whether it is on by default is NOT_VERIFIED.

Pi (decided 2026-11-09 under R42, D5). The Pi package MUST register the MCP servers when Pi's
built-in MCP client is confirmed usable by the upstream fact check (G21), and MUST otherwise ship
the CLI-backed tool. Either way Pi stays preview and never blocks 1.0.

#### Scenario: Pi MCP client not confirmed

- **GIVEN** the upstream fact check has not confirmed that Pi's MCP client is usable
- **WHEN** the Pi package is generated
- **THEN** it ships the CLI-backed tool and registers no MCP server

#### Scenario: MCP tools listed

- **WHEN** a qualified agent session starts
- **THEN** the agent's tool list contains `corvint.affected`

### Requirement: AGT-V1-011 Three skills

Source: G19. Each package MUST include three generated skills: tests (affected tests and how to run
them), docs (query the documentation product) and tasks (find, claim and complete tickets through
the `corvint-tasks` CLI only, using its published supported-command list; owner decision R17,
2026-11-09).

#### Scenario: Skill names

- **WHEN** the package is installed
- **THEN** exactly the three skills are discoverable by the host

### Requirement: AGT-V1-012 After-edit record

Source: G19. The after-edit hook MUST record the edited paths in session state and MUST print
nothing to the model.

#### Scenario: Two edits

- **WHEN** the agent edits two files in one turn
- **THEN** both paths are in session state and no text was added to the conversation

### Requirement: AGT-V1-013 End-of-turn report

Source: G19, G25 item 2, owner decisions R4, R5 and R13 (2026-11-09), review fixes F1 and F19. At
end of turn, when files changed, the hook MUST report the exact tests `corvint affected` selects for
the session's changes and the command to run them, or "UNKNOWN, run all" with the reason code when
the plan is not trustworthy.

- Changes: the session's changes are the diff from the commit recorded at session start to the
  working tree. Paths recorded by the after-edit hook only add to that set; they never replace it
  (F19).
- Delivery: on Claude Code the report MUST be returned as a `Stop` block decision whose reason is
  the report, so the model sees it (F1, R5). The hook MUST NOT return a block decision when the
  payload's `stop_hook_active` is true, and MUST return at most one per turn and at most one per
  tree state (the same working-tree digest gets no second nudge). On Codex the same decision is
  used; that its reason reaches the model is NOT_VERIFIED. On OpenCode and Pi the report is
  printed by the host's own mechanism.
- An unknown `CORVINT_SELECTION` value MUST give "UNKNOWN, run all" with a reason naming the value;
  the hook does not exit 2 (R13).
- The report uses `corvint affected` and test receipts only, not `review` (contradiction 9 below).
  Docs review output, if a later hook adds it, is advisory unless the repository opts in (R4).

Earlier text placed the report in plain `Stop` hook output, which does not reach the model on
Claude Code (F1); that delivery is superseded.

#### Scenario: Exact selection

- **WHEN** the session changed `internal/a/a.go`
- **THEN** the report lists the same tests as `corvint affected` for that change and one `corvint test` command

#### Scenario: Loop guard

- **WHEN** the end-of-turn hook already returned a block decision and the next `Stop` payload has
  `stop_hook_active` true, or the working tree is unchanged since the last nudge
- **THEN** the hook returns no block decision and the turn ends

#### Scenario: Edit missed by the after-edit hook

- **WHEN** a file is changed by a shell command that no after-edit hook saw
- **THEN** the path is in the report, because the session-start commit is diffed against the working
  tree

#### Scenario: Unknown selection value

- **WHEN** `CORVINT_SELECTION=go=full` is set and the turn ends with changes
- **THEN** the report says "UNKNOWN, run all" with a reason naming the value, and the hook exits 0

### Requirement: AGT-V1-014 Edits with no passing receipt

Source: G19, G25 item 2, review fix F20, owner decision R7 (2026-11-09). The end-of-turn report MUST
flag every changed path whose selected tests have no passing receipt covering the current tree,
read from the append-only run result record of PROD-TEST-020. The report MUST say "no receipt",
not "never ran": a run outside `corvint test` leaves no receipt. Until PROD-TEST-020 is delivered
this requirement cannot be qualified. (The earlier title was "Edits whose tests never ran".)

#### Scenario: Agent did not run the tests

- **WHEN** the agent edited a file and ended the turn without running its tests
- **THEN** the report flags the path and repeats the command

### Requirement: AGT-V1-015 Never runs tests, never blocks

Source: G19, owner decision R5 (2026-11-09). Hooks MUST NOT run tests, MUST NOT block an edit and
MUST NOT deny a tool call. The only block decision allowed is the end-of-turn decision of
AGT-V1-013 under its loop guard. Earlier wording, superseded by R5: "MUST NOT return a block or deny
decision". The binary MUST enforce a time budget per event (proposed 1 s after edit, 3 s end of
turn, session start and after compaction; the end-of-turn value is set by AGT-V1-030) and on timeout MUST reply "UNKNOWN, run all" or nothing and exit 0. Shims MUST catch
every error and exit 0.

#### Scenario: Hook crash

- **WHEN** the binary crashes during an after-edit hook
- **THEN** the edit stands, the turn continues and the shim exits 0

### Requirement: AGT-V1-016 Output limits

Source: G19. Hook output MUST contain only test identifiers, repository-relative paths, commands and
reason codes, MUST pass the secret screen, and MUST be bounded with a truncation marker and totals.
It MUST NOT contain file contents or environment values.

#### Scenario: Large selection

- **WHEN** 900 tests are selected
- **THEN** the report shows a bounded list, a truncation marker, the total and the run command

### Requirement: AGT-V1-017 Session-start readiness

Source: G19. At session start the hook MUST print one readiness line: binary version, index state,
kill switch state and any range notice.

#### Scenario: Stale index

- **WHEN** the index is older than HEAD
- **THEN** the readiness line says so and names `corvint index --if-stale`

### Requirement: AGT-V1-018 Compaction restates pending tests

Source: G19, owner decision R5 (2026-11-09), review fix F18. After compaction the hook MUST restate
only the selected tests with no passing receipt for the session's changes, or nothing when none are
pending. On Claude Code and Codex it MUST run on `SessionStart` with source `compact`, whose output
reaches the model, not on `PostCompact` (earlier wording, superseded). The Codex source value is
NOT_VERIFIED.

#### Scenario: Pending tests survive compaction

- **WHEN** compaction happens with three selected tests not yet run
- **THEN** exactly those three and the command are restated

### Requirement: AGT-V1-019 No context injection

Source: G19, 2026-10-26 finding. No hook and no skill MUST add retrieved repository content, code,
summaries or documentation text to the conversation. MCP and CLI answers happen only on the agent's
explicit request.

#### Scenario: Session start

- **WHEN** a session starts with a source other than `compact`
- **THEN** the only hook output is the readiness line

### Requirement: AGT-V1-020 Instruction-file check

Source: G19. Doctor MUST check, read-only, whether the repository's agent instruction files reach
the host according to `integrations/instruction-profiles.json` (or its new path), and MUST report
hosts whose loading rules are unconfirmed as NOT_VERIFIED.

#### Scenario: Unconfirmed host

- **WHEN** doctor runs for OpenCode, which the profiles list as unconfirmed
- **THEN** the instruction-file result is NOT_VERIFIED, not pass

### Requirement: AGT-V1-021 Kill switch

Source: G20, G10. When `CORVINT_HOOKS=off` is set, every shim MUST exit 0 with no output before
calling the binary, and doctor MUST report the switch. MCP and skills are unaffected.

#### Scenario: Switch on

- **WHEN** `CORVINT_HOOKS=off` is exported and the agent edits a file
- **THEN** no hook output appears and the binary is not called

### Requirement: AGT-V1-022 Session state outside the repository

Source: G19, product invariant 4; decided 2026-11-09 under R42, D17 and N5. Session state MUST
live outside the repository and outside
`.corvint/` (proposed: the user cache directory keyed by repository root and session ID), MUST be
bounded in size, MUST expire (proposed 24 hours), and MUST NOT be an input to ranking, evidence,
authority or learning.

Invariant 4 permits this (D17). `corvint agent hook` is a session command, not a read command,
and the amended invariant 4 says that agent hook session state kept in the user cache directory
is not repository or trace state. The state MUST hold only paths, commit SHAs and timestamps; it
MUST NOT hold file content, command output or secrets. State files MUST be created readable and
writable by their owner only. `affected`, `query`, `status` and every other read verb MUST NOT
write session state, with or without a session. Earlier wording, superseded by D17: "Whether
invariant 4 permits this as written is an owner question."

Concurrent and continued sessions (review ci-agents F27). The repository root in the key is the
root of the working tree the session runs in, so each Git worktree has its own state. A resumed
session keeps its session ID and MUST reuse its unexpired state, including the commit recorded at
session start. A forked session has a new session ID and MUST start new state that records the
current `HEAD`; it MUST NOT read or write the parent's state. Edits a subagent makes in the
parent's working tree are part of the parent's session changes through the diff of AGT-V1-013.
In 1.0 the packages MUST NOT register a hook on a subagent's own stop event (Claude Code
`SubagentStop`); the parent's end-of-turn report is the only one (N5; revisit in 1.1 after the
upstream behaviour is checked under G21).

#### Scenario: Read command purity

- **WHEN** a session records edits
- **THEN** `git status` and `.corvint/` are unchanged

#### Scenario: State holds no content

- **GIVEN** a session that edited a file containing a secret and ran a command that printed it
- **WHEN** the session state files are read
- **THEN** they contain only paths, commit SHAs and timestamps
- **AND** each file is readable and writable by its owner only

#### Scenario: Read verb writes no state

- **GIVEN** an active session with recorded edits
- **WHEN** `corvint affected` runs in that working tree
- **THEN** no file under the session state directory is created or changed

#### Scenario: No subagent stop hook

- **WHEN** the generated Claude Code package is inspected
- **THEN** it registers no `SubagentStop` hook

#### Scenario: Two worktrees at once

- **GIVEN** two sessions running in two worktrees of one repository
- **WHEN** each edits a different file and ends its turn
- **THEN** each report lists only the tests for the change in its own worktree

#### Scenario: Subagent edit

- **WHEN** a subagent edits `internal/a/a.go` in the parent's working tree and the parent's turn
  ends
- **THEN** the parent's end-of-turn report covers `internal/a/a.go`

#### Scenario: Resumed session

- **WHEN** a session is resumed with the same session ID before its state expired
- **THEN** the end-of-turn diff still starts at the commit recorded when the session first started

#### Scenario: Forked session

- **WHEN** a session is forked while the working tree has uncommitted edits
- **THEN** the fork's state is new, its start commit is the current `HEAD`, and its report covers
  the uncommitted edits because they differ from `HEAD`
- **AND** the parent's state file is unchanged

### Requirement: AGT-V1-023 Meaning of supported

Source: G21, owner decision R27 (2026-11-09), review fix F21. A package MUST be called supported
for an exact tuple (agent version, model ID, package version, Corvint version, OS) only when a retained pass exists for that tuple from a scripted session on the
real agent binary with the released package and the fixture repository, covering: install; MCP tools
listed; an edit after which the hook names exactly the affected tests and the agent runs them; an
unsure plan giving "UNKNOWN, run all"; a hook crash and a hook timeout that do not block; a docs
query; a task claim on a scratch store through the `corvint-tasks` CLI; compaction restating pending tests; clean uninstall.
Assertions MUST be made on hook and tool transcripts, and every retry MUST be logged. Each host
uses one pinned model, and the model ID and agent version are recorded with the pass (R27).

Pass rule (decided 2026-11-09 under R42, D2; closes NEW-AGT-2). Deterministic steps MUST pass in
every run: install, doctor, hook wiring, hook behaviour (including a hook crash and a hook
timeout that do not block), the hooks-off switch and uninstall. Model-driven scenarios MUST run
three times per host and operating system and pass when at least two of the three runs pass.
Every run and retry MUST be recorded and none dropped. The rule MUST be committed before the
first qualification run and MUST NOT change after it.

#### Scenario: Two of three model-driven runs pass

- **GIVEN** the edit scenario run three times on Codex on Linux, with two passes and one failure
- **WHEN** the evidence file is assembled
- **THEN** the scenario passes and all three runs are listed

#### Scenario: A deterministic step fails once

- **GIVEN** three runs on one host in which uninstall fails in one run
- **WHEN** the evidence file is assembled
- **THEN** the host does not pass

#### Scenario: Prose is not evidence

- **WHEN** the agent says it ran the tests but the transcript has no `corvint test` call
- **THEN** the scenario fails

### Requirement: AGT-V1-024 Gate on pinned versions

Source: G21, G10a, answer 16. The 1.0 gate MUST use the Claude Code and Codex versions pinned at the
freeze on 2026-12-15, on Linux and macOS, and qualification MUST rerun on every package or Corvint
change. The runs follow AGT-V1-029 (owner's machines, subscription logins) and each release
candidate gets one evidence file.

#### Scenario: Package change

- **WHEN** a new package version is tagged
- **THEN** qualification reruns on the pinned versions before release

### Requirement: AGT-V1-025 Weekly check against the newest agent release

Source: G21. A weekly run on the owner's machines (AGT-V1-029) MUST qualify each package against
the newest release of each agent. A
failing new version MUST be marked untested in the published list, the last passing tuple MUST stay
advertised, a report MUST be filed automatically, and doctor MUST warn when the installed agent
version is not in the passing list.

#### Scenario: Agent ships a breaking change

- **WHEN** Codex releases a version that fails the edit scenario
- **THEN** that version is marked untested, the previous pass stays listed and doctor warns on it

### Requirement: AGT-V1-026 Offline pass list

Source: G21, invariant 7. Each package MUST carry the list of qualified tuples it was released with,
so doctor reports qualification without a network call.

#### Scenario: Offline doctor

- **WHEN** doctor runs with no network
- **THEN** it reports qualification from the packaged list and labels it as of the package release

### Requirement: AGT-V1-027 Retire `integrations/`

Source: G20. After the first `corvint-agents` release that passes AGT-V1-023 for Claude Code and
Codex, `beamfall/corvint` MUST remove `integrations/claude-code/**`, `integrations/codex/**` and
`integrations/README.md`, and update every consumer listed under Impact, keeping
`instruction-profiles.json`. Removal before that release MUST NOT happen, so users always have one
installable package.

Migration note (review ci-agents F28). The packages ship exactly three skills (AGT-V1-011), so the
skills of today's plugins are dropped: `affected`, `impact`, `index` and `review` in
`integrations/claude-code/plugins/corvint/skills/`, and `corvint` in
`integrations/codex/plugins/corvint/skills/` (paths at integ 9ec3a29d0). The release notes of the
first `corvint-agents` release and the removal pull request MUST carry a migration note that names
each dropped skill and what replaces it. `affected` is replaced by the `tests` skill. For each of
the other four the note MUST name the replacing skill or command, or say that there is none.

Replacements (decided 2026-11-09 under R42, N6). No new skill is added. The note MUST say:
`impact` is replaced by the `corvint.impact` MCP tool and `corvint impact` (preview); `review` by
the change report command of change `change-report-impact`, reached through the `docs` skill;
`index` by nothing, because the index is prepared by the commands that need it or by
`corvint index --if-stale`; the Codex `corvint` skill by the three shipped skills. The exact
change report command name is UNVERIFIED and MUST be confirmed before the note is published.

#### Scenario: No orphan reference

- **WHEN** the removal lands
- **THEN** `git grep integrations/claude-code integrations/codex` finds hits only in history files

#### Scenario: Migration note names every dropped skill

- **WHEN** the first `corvint-agents` release is published
- **THEN** its notes list `affected`, `impact`, `index`, `review` and the Codex `corvint` skill,
  each with its replacement or the words "no replacement"

### Requirement: AGT-V1-028 Superseding decision first

Source: G19, answer 18. No part of this change MUST be built before a decision superseding decision
0594 item 1 in part and replacing PRS-V1-006 is accepted, with the content under "Superseding
decision".

#### Scenario: Lane start

- **WHEN** a lane is about to start
- **THEN** the accepted decision file exists and is cited in the ticket

### Requirement: AGT-V1-029 Qualification on the owner's machines with subscription logins

Source: fix F22; owner decision R39 (2026-11-09) and owner decision R40 (2026-11-09), which amend
R26. The AGT-V1-024 pinned run and the AGT-V1-025 weekly run MUST use the owner's logged-in
subscription command-line clients: Claude Code for Claude Code, and the logged-in Codex CLI for Codex.
They MUST run on the owner's machines (the Mac, WSL and Unraid hosts), either directly or through a
self-hosted runner on one of those machines. They MUST NOT use agent API keys and have no spend cap.
They MUST NOT run on GitHub-hosted runners. A workflow that sends a job to such a self-hosted runner
MUST trigger only on `schedule`, `workflow_dispatch` or `push` to `main`, and MUST NOT trigger on
`pull_request` from a fork or on `pull_request_target`. A pull request MAY run the same scenarios
with recorded payload fixtures and no login, on any runner.

The result MUST be one evidence file per release candidate, recording for each scenario the tuple of
AGT-V1-023 (agent version, model ID, package version, Corvint version, OS) and the machine, plus one
record per weekly run against the newest agent versions (AGT-V1-025). Where the evidence file is
kept is set by change `acceptance-record`.

Earlier wording, superseded by R39 and R40 and kept for traceability: "A workflow that uses real agent
API keys (the AGT-V1-024 pinned run and the AGT-V1-025 weekly run) MUST run only on `schedule`,
`workflow_dispatch` or `push` to `main`. It MUST read the keys from a protected GitHub environment
and MUST enforce a spend cap per run. It MUST NOT run on `pull_request` from a fork or on
`pull_request_target`. A pull request MAY run the same scenarios with recorded payload fixtures and
no keys." The earlier open-question marker (question 7, on how keys and spend are arranged) is replaced by
owner decision R39 (2026-11-09) and owner decision R40 (2026-11-09).

Failure mode: a workflow file adds a forbidden trigger, targets a GitHub-hosted runner for a
logged-in run, or reads an agent API key; a CI lint on `.github/workflows/**` in
`beamfall/corvint-agents` fails the pull request. A login has expired or the subscription's usage
limit is reached: the run stops, records the scenarios not run as NOT_RUN in the evidence file, and
the weekly record says so; the 1.0 gate for that host cannot pass until a rerun completes. Earlier
failure mode, superseded: "The spend cap is reached: the run stops, records the scenarios not run as
NOT_RUN, and the weekly report says so."

Rollback: stop the logged-in runs; qualification then stays NOT_RUN and the 1.0 gate for Claude Code
and Codex cannot pass until they return.

#### Scenario: Fork pull request

- **WHEN** a pull request from a fork changes a shim
- **THEN** only the fixture-based workflow runs, no job reaches the owner's machines, and no login is
  used

#### Scenario: Forbidden trigger added

- **WHEN** a pull request adds `pull_request_target` to a workflow that sends jobs to the owner's
  self-hosted runner
- **THEN** the workflow lint fails and names the trigger

#### Scenario: Hosted runner refused

- **WHEN** a workflow runs a logged-in qualification job on `ubuntu-latest`
- **THEN** the workflow lint fails and names the runner

#### Scenario: Evidence per release candidate

- **WHEN** a release candidate is qualified
- **THEN** one evidence file exists for it, naming for each scenario the agent version, model ID,
  package version, Corvint version, OS and machine, with every retry logged

### Requirement: AGT-V1-030 End-of-turn budget from measured latency

Source: fix F23. The 3 s end-of-turn budget in AGT-V1-015 is a proposal, not measured. Before
release, the end-of-turn hook latency MUST be measured on the seven pinned repositories (prometheus,
vuejs/core, prettier, freeCodeCamp, pydantic, rubocop, rails activesupport) and the p50 and p95
recorded as acceptance evidence. The released end-of-turn budget MUST be at least that p95 plus a
margin stated with the evidence, or the hook MUST be changed until its p95 fits under 3 s. The
evidence names the host, Corvint version and cold or warm index.

Failure mode: the measured p95 exceeds the proposed budget; the release either raises the budget
with the evidence or keeps 3 s with the hook replying "UNKNOWN, run all" on timeout, and the gap is
filed. A missing measurement is NOT_RUN and blocks the budget value, not the hook.

Rollback: restore the proposed 3 s value; the timeout behaviour in AGT-V1-015 is unchanged.

#### Scenario: Budget set from evidence

- **WHEN** the release candidate is qualified
- **THEN** the retained record shows p95 end-of-turn latency for each of the seven repositories and
  the budget value derived from it
