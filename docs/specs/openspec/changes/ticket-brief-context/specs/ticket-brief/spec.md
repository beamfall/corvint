# Spec delta: `ticket-brief` (change `ticket-brief-context`)

## ADDED Requirements

The lane copies each statement below into `ticket-brief-v1.md`, inside its `## Requirements`
section, as a numbered definition line that begins `(proposed 2026-11-09; delivery not-started;
non-blocking)`.

Amendment of 2026-11-10: TBC-V1-009 to TBC-V1-014 serve PROD-INT-032, which the owner accepted
into 1.0 on 2026-11-10 ("Accept into 1.0"). They add a second form of the brief that needs no
ticket and change no statement of TBC-V1-001 to TBC-V1-008. The lane copies each one with a
definition line that begins `(proposed 2026-11-10; delivery not-started; non-blocking)`. The
coverage check of TBC-V1-013 runs first; the export and the builder wait for its result against
the owner's bar, which is not set yet.

### Requirement: TBC-V1-001 Brief from declared links only

Source: G25 item 3. `corvint context --ticket ID` MUST build the brief only from the ticket's own
fields (read with `corvint-tasks ticket show <ID> --json`; Corvint MUST NOT read the Tasks store
files directly), the requirements it cites, the flows it cites with their accepted links, and the affected
plan for its declared paths.

#### Scenario: Ticket with two requirement refs

- **WHEN** a ticket cites two requirement IDs and one path
- **THEN** the brief lists both requirements with their spec lines and the plan for that path

### Requirement: TBC-V1-002 No ranking

Source: G25 item 3, G25 "still removed". The brief MUST NOT rank, score or match terms, and every
item MUST name the link that produced it.

#### Scenario: Item provenance

- **WHEN** the brief lists a test
- **THEN** the item names the accepted link record or plan reason that selected it

### Requirement: TBC-V1-003 Missing links are stated

Source: invariant 2. A missing or dangling link MUST be listed as `NOT_OBSERVED` or
`dangling-requirement`, never filled by a guess.

#### Scenario: No flow cited

- **WHEN** the ticket cites no flow
- **THEN** the flows item is `NOT_OBSERVED`

### Requirement: TBC-V1-004 Asked for, never injected

Source: G19. The brief MUST be produced only by an explicit CLI or MCP call, and no Corvint hook may
insert it into an agent's context.

#### Scenario: Agent hook

- **WHEN** an agent hook fires at session start
- **THEN** it does not call `context`

### Requirement: TBC-V1-005 Ranking code removed

Source: G25 "still removed". The nine ranking files in `internal/contextindex` and the ranked path
of `cmd/corvint/taskcontext.go` MUST be removed, keeping `BuildContext` and `QueryIntent`.

#### Scenario: Build after removal

- **WHEN** the repository builds after the change
- **THEN** none of the nine files exists and `cmd/corvint-docs` still builds

### Requirement: TBC-V1-006 Keep-or-remove measurement

Source: G25 item 3; owner decisions R28 and R29 (2026-11-09); review docs F10. The brief ships in
1.0 non-blocking with no claim of measured benefit. In 1.0.x the brief MUST be measured on a fixed
task set, and it MUST be removed in a 1.0.x release unless the measurement meets the threshold the
owner sets. At the 1.0 release candidate the measurement is NOT_RUN, and that MUST NOT block the
release or remove the brief. The runs MUST use the owner's logged-in subscription CLIs on the
owner's machines, with no API keys and no spend cap (owner decisions R39 and R40, 2026-11-09).

Earlier wording, superseded by R28 and R29: "Before 1.0 stable, the brief MUST be measured on a
fixed task set, and it MUST be removed unless the measurement meets the threshold the owner sets."

Measurement design (proposed):

- Task set: 20 closed tickets of this repository with at least one `requirementRefs` ID and a
  declared path, chosen and hashed before any run; the list and hash are committed first.
- Arms: the same pinned agent version, prompt, tools and permissions; arm A starts from the ticket
  text, arm B also receives the brief as the first tool result it asks for. Three runs per task per
  arm, at the ticket's base commit.
- Metric: tool calls until the agent reports done; secondary: task success by the ticket's frozen
  acceptance checks.
- Proposed threshold for the owner: keep if the median tool-call count drops by at least 20% and the
  success rate is not lower; otherwise remove.

#### Scenario: Threshold missed

- **WHEN** the 1.0.x median tool-call reduction is 8%
- **THEN** the brief builder and the `context` command are removed in a 1.0.x release

#### Scenario: Not measured at the release candidate

- **WHEN** the 1.0 release candidate is cut with no measurement
- **THEN** the measurement is recorded as NOT_RUN, the brief ships non-blocking, and the 1.0
  documentation makes no benefit claim

### Requirement: TBC-V1-007 Preview, read-only, deterministic

Source: G25, invariant 4, owner decisions R28 and R29 (2026-11-09). The brief MUST be labelled
non-blocking with no claim of measured benefit (earlier: "labelled preview"), MUST write nothing,
and MUST be byte-identical for identical inputs.

#### Scenario: Repeated run

- **WHEN** the brief is built twice for the same ticket revision and commit
- **THEN** both outputs are byte-identical

### Requirement: TBC-V1-008 One MCP table row

Source: owner decision R30 (2026-11-09). If the brief is offered over MCP, its server, tool name,
input schema, output profile and error codes MUST be those of its row in the table of change
`mcp-tool-table`. The ranked `context` MCP form is removed.

#### Scenario: Tool list

- **WHEN** a client lists the tools of both servers
- **THEN** any brief tool appears only on the server its table row names, with that row's schema

### Requirement: TBC-V1-009 Path brief needs no ticket

Proposed 2026-11-10; PROD-INT-032 accepted by the owner 2026-11-10.
Source: owner request 2026-11-10 (useful for every agent); PROD-INT-032. `corvint context --path
PATH [--path PATH ...]` MUST build a brief for the named paths from the committed tree at HEAD, with
no ticket, no task text and no Tasks store, and MUST name that commit. It MUST refuse a path outside
the repository with a typed refusal and exit 2. It MUST NOT accept `--task`, `--base` or a diff: the
report for a change range is `corvint impact` (CRI-V1). It MUST refuse `--path` together with
`--ticket`, exit 2: one call has one form. A path names a file or a directory. A directory stands
for the tracked files beneath it, and their items are merged under the limit of TBC-V1-010. The
sections are printed once per `--path`, in the order given. A path absent from HEAD is not refused
and the command exits 0: its `instructions`, `owners` and `tickets` sections are answered (the
first two for its directory), and its `tests`, `flows` and `requirements` sections are
`NOT_OBSERVED` with reason `path-not-at-head`.

#### Scenario: Repository with no ticket store

- **WHEN** an agent runs `corvint context --path internal/x/y.go` in a repository with no `.taskman`
- **THEN** the brief is printed, exit 0, with the `tickets` section `NOT_OBSERVED`

#### Scenario: Diff asked for

- **WHEN** the command is run with `--path` and `--base`
- **THEN** it refuses, exit 2, and names `corvint impact`

### Requirement: TBC-V1-010 Six sections, each from a named link

Proposed 2026-11-10; PROD-INT-032 accepted by the owner 2026-11-10.
Source: G25 item 3, invariant 2, CRI-V1-002. The path brief MUST carry these sections in this order,
each with the state and reason of CRI-V1-002 and two fields this requirement defines: `total`, the
number of items before the limit, and `truncated`, true when items were left out. Every item MUST
name the link that produced it (TBC-V1-002 applies):

1. `instructions`: each `AGENTS.md` and `CLAUDE.md` in the path's directory and its parents, root
   first. Link: directory ancestry.
2. `owners`: the last `CODEOWNERS` rule that matches the path, with its line. Link: that rule.
3. `tests`: the tests the retained test evidence and the affected plan tie to the path, each with
   its plan reason, and the plan's scope verdict. Link: the evidence record or plan reason.
4. `flows`: each flow of `.corvint/flows.json` that passes through the path, its variations and
   their linked tests. Link: the accepted link record.
5. `requirements`: each requirement tied to the path by a kept link (TBC-V1-012) or by a completed
   ticket whose recorded commits changed the path. Link: the link record or the ticket ID.
6. `tickets`: each open ticket whose claim scope or `effects.touchPaths` covers the path, read with
   `corvint-tasks`. Link: the ticket field that matched.

A section MUST list at most 64 items for each `--path`, ordered by path or ID ascending. The
`requirements` section MUST NOT be `OBSERVED`: the links are not a full record of what governs a
file. With items it is `INCOMPLETE` with reason `links-not-exhaustive`; with none it is
`NOT_OBSERVED` with reason `no-link-for-path`. A missing source adds its name to either reason and
never changes that state (TBC-V1-011). The ticket form (`corvint context --ticket ID`, TBC-V1-001)
is not changed by this requirement.

#### Scenario: Nested instruction file

- **WHEN** `internal/x/AGENTS.md` and the root `AGENTS.md` both exist and the path is
  `internal/x/y.go`
- **THEN** the `instructions` section lists the root file, then `internal/x/AGENTS.md`

#### Scenario: File with no requirement link

- **WHEN** no kept link and no completed ticket ties the path to a requirement
- **THEN** the `requirements` section is `NOT_OBSERVED` with reason `no-link-for-path`, not an
  empty `OBSERVED` section

#### Scenario: More items than the limit

- **WHEN** 300 tests are tied to the path
- **THEN** the `tests` section lists 64, with `total` 300 and `truncated` true

### Requirement: TBC-V1-011 A missing source never blocks

Proposed 2026-11-10; PROD-INT-032 accepted by the owner 2026-11-10.
Source: invariant 2; owner request 2026-11-10. Every source except the Git repository is optional.
The optional sources are `corvint-tasks`, `.corvint/flows.json`, `CODEOWNERS`, the retained test
evidence and `.corvint/requirement-links.json`. Absence is judged per source, not per section. A
section whose sources are all absent MUST be `NOT_OBSERVED` with a reason that names each missing
source. A section with two sources (`tests`: retained evidence and the affected plan;
`requirements`: the link file and completed tickets) of which one is absent MUST keep the items of
the source that is present. With such items it MUST be `INCOMPLETE`; with none it MUST be
`NOT_OBSERVED`. In both cases the reason names the missing source. For `requirements` the reason
of TBC-V1-010 comes first and each missing source is added to it, so the two requirements give one
state: items give `INCOMPLETE`, no items give `NOT_OBSERVED`. The command MUST still exit 0.
TBC-V1-004 and TBC-V1-007 apply to the path brief unchanged.

#### Scenario: Link file present, no ticket store

- **WHEN** `.corvint/requirement-links.json` ties the path to `ABC-V1-004` and `corvint-tasks` is
  not installed
- **THEN** the `requirements` section lists `ABC-V1-004` and is `INCOMPLETE` with a reason that
  names `corvint-tasks`, and the `tickets` section is `NOT_OBSERVED`

#### Scenario: Link file present with no link for the path, no ticket store

- **WHEN** `.corvint/requirement-links.json` exists and holds no link for the path, and
  `corvint-tasks` is not installed
- **THEN** the `requirements` section has no items and is `NOT_OBSERVED` with a reason that gives
  `no-link-for-path` and names `corvint-tasks`

#### Scenario: Repository that has never used Corvint

- **WHEN** the path brief runs in a repository with only source files and one `AGENTS.md`
- **THEN** `instructions` is `OBSERVED`, the other five sections are `NOT_OBSERVED` with their
  reasons, and the exit code is 0

### Requirement: TBC-V1-012 Requirement links kept from sealed change records

Proposed 2026-11-10; PROD-INT-032 accepted by the owner 2026-11-10.
Source: owner instruction 2026-11-10 (the CEM may be amended where a useful context needs it); G8.
G8 stays as accepted: the `cem` verb, the ceremony and the `cem/1.0` protocol are removed and no
new change record is written. What is kept is the links the sealed records already hold. A one-time
export MUST read the `cem/1.0` records under `.corvint/changes` at one named commit and write
`.corvint/requirement-links.json`. For each hunk of a record and each of its `basis` entries whose
relation is `specification` or `decision`, the `evidenceId` is joined to the record's `evidence`
entry, giving one link from the hunk's path to that evidence file, byte span and pinned `blobOid`.
The export writes one entry per distinct path, evidence file, span, `blobOid` and record commit;
entries of different records are never merged. Each entry MUST carry the path, the evidence file,
span and `blobOid`, every requirement ID whose definition line lies inside that span at the pinned
blob (none is allowed), and the record's commit. That commit is the SHA in the record's file name
(`<sha>.cem.json`); a record holds no other commit of its own. The export MUST be deterministic, and
its input commit, its output hash, and its counts of links with no requirement ID and of links
whose path is absent at the input commit MUST be recorded in the build log. The `requirements`
section MUST list one item per requirement ID, or per evidence file and span for a link with no
ID. An item supported by several entries MUST state their count and report one of them: the entry
whose record commit has the latest committer date, ties broken by SHA ascending. The item MUST
state that entry's commit and whether the path's blob at HEAD differs from its blob at that
commit. A link is never moved to another path or line by a guess: a file renamed after its
link was sealed loses that link, and rename following is not attempted.

#### Scenario: File changed since the link was sealed

- **WHEN** a kept link ties `internal/x/y.go` to `ABC-V1-004` at commit C and the file has changed
  since C
- **THEN** the item names `ABC-V1-004`, commit C and `changedSince` true

#### Scenario: Two records cite the same requirement for one file

- **WHEN** records sealed at commits C1 and C2 both tie `internal/x/y.go` to `ABC-V1-004`, and C2
  has the later committer date
- **THEN** the link file holds two entries, and the section lists one item for `ABC-V1-004` with
  count 2, commit C2 and `changedSince` computed against C2

#### Scenario: Export run twice

- **WHEN** the export runs twice on the same input commit
- **THEN** both outputs are byte-identical

### Requirement: TBC-V1-013 Link coverage is measured before the builder is written

Proposed 2026-11-10; PROD-INT-032 accepted by the owner 2026-11-10.
Source: owner request 2026-11-10 (measure first); value audit 2026-10-26. Before the export and the
path brief builder are written, the `tests` and `requirements` sections MUST be computed for the
frozen task set of TBC-V1-006 by a scratch script and no agent run. The script runs in a checkout
of each task's base commit, with the paths the closing commits changed as its input. It takes
`tests` from the existing `corvint affected --by-path` and `corvint test-evidence` commands. It
takes `requirements` straight from the sealed records whose commit is an ancestor of the base
commit, joined as TBC-V1-012 states. The other four sections are not part of the check. The record
MUST give, per task, the share of the test files changed by the closing commits that `tests` lists
and the share of the ticket's `requirementRefs` that `requirements` lists. A test listed only
because the plan's scope is `UNKNOWN` does not count. The result MUST be recorded in the build log
whatever it is. The export and the path brief are built only if the result meets the bar the owner
sets.

Proposed bar for the owner: at least half of the tasks have a listed test that the closing commits
changed or a listed requirement that the ticket cites.

#### Scenario: Bar missed

- **WHEN** 4 of 20 tasks have such a test or requirement
- **THEN** the result is recorded, the path brief is not built, and TBC-V1-009 to TBC-V1-012 and
  TBC-V1-014 are retired

### Requirement: TBC-V1-014 Pilot before the release candidate

Proposed 2026-11-10; PROD-INT-032 accepted by the owner 2026-11-10.
Source: owner request 2026-11-10 (measure first); R28, R29, R39, R40. A pilot of the TBC-V1-006
design is planned before the 1.0 release candidate on the first 8 tasks of the committed list, with
a third arm C in which the agent is told the path brief exists and may ask for it. The record MUST
give, per arm, the tool calls until the agent reports done, the task success and, for arms B and C,
whether the agent asked for the brief. The pilot is evidence only: it MUST NOT change the
keep-or-remove rule of TBC-V1-006, permit a benefit claim in 1.0 documentation or block the release.
A pilot that has not run at the release candidate is recorded as NOT_RUN.

#### Scenario: Pilot shows a gain

- **WHEN** arm C uses 30% fewer median tool calls than arm A in the pilot
- **THEN** the result is recorded, the 1.0 documentation still makes no benefit claim, and the
  1.0.x measurement still decides whether the brief is kept
