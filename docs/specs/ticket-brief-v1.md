# Ticket Brief V1 — the context brief for a ticket and for paths

Owner: Russell Lewis
Date: 2026-11-10
Intent status: proposed; its two product requirements are accepted by the owner (decision 0596; 2026-11-10)
Delivery status: experimental; path form built, ticket form not started
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-INT-030, PROD-INT-032), decision 0596, and the change folder
`docs/specs/openspec/changes/ticket-brief-context/` (the proposal, design and spec delta this file
was created from).

## Agent digest
- Claim: `corvint context` prints a brief built only from declared links, for a ticket or for paths, and states what it could not observe.
- Status: proposed; its two product requirements are accepted by the owner (decision 0596; 2026-11-10) / experimental; path form built, ticket form not started
- Exists: `corvint context --path`; `internal/pathbrief`; `.corvint/requirement-links.json`; the coverage record `docs/build-log/2026-11-10-path-brief-link-coverage-check.md`.
- Blocked on: the ticket form (TBC-V1-001 to TBC-V1-008); the flows reader; the pilot (TBC-V1-014); the 1.0.x measurement (TBC-V1-006).
- Read next: Requirements; Delivered limits; Acceptance evidence.

## User and measurable job

An agent that is about to change a file needs to know which instruction files apply, who owns the
file, which tests and flows cover it, which requirements were tied to it and which open tickets
claim it. It needs that without task text matching and without a ticket, and it must be able to
tell "nothing is linked" from "the source was not read".

Measured by: the link-coverage check on the frozen 20-task set (TBC-V1-013, recorded), the pilot
(TBC-V1-014, not run) and the 1.0.x keep-or-remove measurement (TBC-V1-006, not run). No benefit
to an agent is claimed until the measurement has run.

## Scope and non-goals

In scope: the ticket form `corvint context --ticket ID` (PROD-INT-030), the path form
`corvint context --path PATH` (PROD-INT-032), the one-time export of the requirement links held by
the sealed change records, the coverage check, the pilot and the 1.0.x measurement.

Non-goals:

- No ranking, scoring or term matching (TBC-V1-002). A section is cut at 64 items in a fixed order
  and states its total.
- No brief for a change range. `corvint impact --base <sha>` is that report.
- The change record is not revived. No new record is written and no verb reads one at run time.
- Line ranges of kept links are not re-anchored, and renames are not followed.
- The brief blocks nothing and is not a gate input.

## Requirements

Requirement IDs are stable. Each scenario under "Scenarios" names the requirement it tests.

- `TBC-V1-001`: (proposed 2026-11-09; delivery not-started; non-blocking) Brief from declared links only.
  Source: G25 item 3. `corvint context --ticket ID` MUST build the brief only from the ticket's own
  fields (read with `corvint-tasks ticket show <ID> --json`; Corvint MUST NOT read the Tasks store
  files directly), the requirements it cites, the flows it cites with their accepted links, and the affected
  plan for its declared paths.

- `TBC-V1-002`: (proposed 2026-11-09; delivery not-started; non-blocking) No ranking.
  Source: G25 item 3, G25 "still removed". The brief MUST NOT rank, score or match terms, and every
  item MUST name the link that produced it.

- `TBC-V1-003`: (proposed 2026-11-09; delivery not-started; non-blocking) Missing links are stated.
  Source: invariant 2. A missing or dangling link MUST be listed as `NOT_OBSERVED` or
  `dangling-requirement`, never filled by a guess.

- `TBC-V1-004`: (proposed 2026-11-09; delivery not-started; non-blocking) Asked for, never injected.
  Source: G19. The brief MUST be produced only by an explicit CLI or MCP call, and no Corvint hook may
  insert it into an agent's context.

- `TBC-V1-005`: (proposed 2026-11-09; delivery not-started; non-blocking) Ranking code removed.
  Source: G25 "still removed". The nine ranking files in `internal/contextindex` and the ranked path
  of `cmd/corvint/taskcontext.go` MUST be removed, keeping `BuildContext` and `QueryIntent`.

- `TBC-V1-006`: (proposed 2026-11-09; delivery not-started; non-blocking) Keep-or-remove measurement.
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

- `TBC-V1-007`: (proposed 2026-11-09; delivery not-started; non-blocking) Preview, read-only, deterministic.
  Source: G25, invariant 4, owner decisions R28 and R29 (2026-11-09). The brief MUST be labelled
  non-blocking with no claim of measured benefit (earlier: "labelled preview"), MUST write nothing,
  and MUST be byte-identical for identical inputs.

- `TBC-V1-008`: (proposed 2026-11-09; delivery not-started; non-blocking) One MCP table row.
  Source: owner decision R30 (2026-11-09). If the brief is offered over MCP, its server, tool name,
  input schema, output profile and error codes MUST be those of its row in the table of change
  `mcp-tool-table`. The ranked `context` MCP form is removed.

- `TBC-V1-009`: (proposed 2026-11-10; V1-5299; delivery experimental; non-blocking) Path brief needs no ticket.
  Serves PROD-INT-032, which the owner accepted on 2026-11-10.
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

- `TBC-V1-010`: (proposed 2026-11-10; V1-5299; delivery experimental; non-blocking) Six sections, each from a named link.
  Serves PROD-INT-032, which the owner accepted on 2026-11-10.
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

- `TBC-V1-011`: (proposed 2026-11-10; V1-5299; delivery experimental; non-blocking) A missing source never blocks.
  Serves PROD-INT-032, which the owner accepted on 2026-11-10.
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

- `TBC-V1-012`: (proposed 2026-11-10; V1-5299; delivery experimental; non-blocking) Requirement links kept from sealed change records.
  Serves PROD-INT-032, which the owner accepted on 2026-11-10.
  Source: owner instruction 2026-11-10 (the CEM may be amended where a useful context needs it); G8.
  G8 stays as accepted: the `cem` verb, the ceremony and the `cem/1.0` protocol are removed and no
  new change record is written. What is kept is the links the sealed records already hold. A one-time
  export MUST read the sealed change records at one named commit and write
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

- `TBC-V1-013`: (proposed 2026-11-10; V1-5300; delivery experimental; non-blocking) Link coverage is measured before the builder is written.
  Serves PROD-INT-032, which the owner accepted on 2026-11-10.
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

  Bar set by the owner on 2026-11-10 ("Accept proposed bar"): at least half of the tasks have a
  listed test that the closing commits changed or a listed requirement that the ticket cites.
  Earlier text: "Proposed bar for the owner: at least half of the tasks ...".

- `TBC-V1-014`: (proposed 2026-11-10; V1-5300; delivery not-started; non-blocking) Pilot before the release candidate.
  Serves PROD-INT-032, which the owner accepted on 2026-11-10.
  Source: owner request 2026-11-10 (measure first); R28, R29, R39, R40. A pilot of the TBC-V1-006
  design is planned before the 1.0 release candidate on the first 8 tasks of the committed list, with
  a third arm C in which the agent is told the path brief exists and may ask for it. The record MUST
  give, per arm, the tool calls until the agent reports done, the task success and, for arms B and C,
  whether the agent asked for the brief. The pilot is evidence only: it MUST NOT change the
  keep-or-remove rule of TBC-V1-006, permit a benefit claim in 1.0 documentation or block the release.
  A pilot that has not run at the release candidate is recorded as NOT_RUN.

## Scenarios

- Scenario for TBC-V1-001 (Ticket with two requirement refs): when a ticket cites two requirement
  IDs and one path, then the brief lists both requirements with their spec lines and the plan for
  that path.
- Scenario for TBC-V1-002 (Item provenance): when the brief lists a test, then the item names the
  accepted link record or plan reason that selected it.
- Scenario for TBC-V1-003 (No flow cited): when the ticket cites no flow, then the flows item is
  `NOT_OBSERVED`.
- Scenario for TBC-V1-004 (Agent hook): when an agent hook fires at session start, then it does not
  call `context`.
- Scenario for TBC-V1-005 (Build after removal): when the repository builds after the change, then
  none of the nine files exists and `cmd/corvint-docs` still builds.
- Scenario for TBC-V1-006 (Threshold missed): when the 1.0.x median tool-call reduction is 8%, then
  the brief builder and the `context` command are removed in a 1.0.x release.
- Scenario for TBC-V1-006 (Not measured at the release candidate): when the 1.0 release candidate is
  cut with no measurement, then the measurement is recorded as NOT_RUN, the brief ships
  non-blocking, and the 1.0 documentation makes no benefit claim.
- Scenario for TBC-V1-007 (Repeated run): when the brief is built twice for the same ticket revision
  and commit, then both outputs are byte-identical.
- Scenario for TBC-V1-008 (Tool list): when a client lists the tools of both servers, then any brief
  tool appears only on the server its table row names, with that row's schema.
- Scenario for TBC-V1-009 (Repository with no ticket store): when an agent runs `corvint context
  --path internal/x/y.go` in a repository with no `.taskman`, then the brief is printed, exit 0,
  with the `tickets` section `NOT_OBSERVED`.
- Scenario for TBC-V1-009 (Diff asked for): when the command is run with `--path` and `--base`, then
  it refuses, exit 2, and names `corvint impact`.
- Scenario for TBC-V1-010 (Nested instruction file): when `internal/x/AGENTS.md` and the root
  `AGENTS.md` both exist and the path is `internal/x/y.go`, then the `instructions` section lists
  the root file, then `internal/x/AGENTS.md`.
- Scenario for TBC-V1-010 (File with no requirement link): when no kept link and no completed ticket
  ties the path to a requirement, then the `requirements` section is `NOT_OBSERVED` with reason
  `no-link-for-path`, not an empty `OBSERVED` section.
- Scenario for TBC-V1-010 (More items than the limit): when 300 tests are tied to the path, then the
  `tests` section lists 64, with `total` 300 and `truncated` true.
- Scenario for TBC-V1-011 (Link file present, no ticket store): when
  `.corvint/requirement-links.json` ties the path to `ABC-V1-004` and `corvint-tasks` is not
  installed, then the `requirements` section lists `ABC-V1-004` and is `INCOMPLETE` with a reason
  that names `corvint-tasks`, and the `tickets` section is `NOT_OBSERVED`.
- Scenario for TBC-V1-011 (Link file present with no link for the path, no ticket store): when
  `.corvint/requirement-links.json` exists and holds no link for the path, and `corvint-tasks` is
  not installed, then the `requirements` section has no items and is `NOT_OBSERVED` with a reason
  that gives `no-link-for-path` and names `corvint-tasks`.
- Scenario for TBC-V1-011 (Repository that has never used Corvint): when the path brief runs in a
  repository with only source files and one `AGENTS.md`, then `instructions` is `OBSERVED`, the
  other five sections are `NOT_OBSERVED` with their reasons, and the exit code is 0.
- Scenario for TBC-V1-012 (File changed since the link was sealed): when a kept link ties
  `internal/x/y.go` to `ABC-V1-004` at commit C and the file has changed since C, then the item
  names `ABC-V1-004`, commit C and `changedSince` true.
- Scenario for TBC-V1-012 (Two records cite the same requirement for one file): when records sealed
  at commits C1 and C2 both tie `internal/x/y.go` to `ABC-V1-004`, and C2 has the later committer
  date, then the link file holds two entries, and the section lists one item for `ABC-V1-004` with
  count 2, commit C2 and `changedSince` computed against C2.
- Scenario for TBC-V1-012 (Export run twice): when the export runs twice on the same input commit,
  then both outputs are byte-identical.
- Scenario for TBC-V1-013 (Bar missed): when 4 of 20 tasks have such a test or requirement, then the
  result is recorded, the path brief is not built, and TBC-V1-009 to TBC-V1-012 and TBC-V1-014 are
  retired.
- Scenario for TBC-V1-014 (Pilot shows a gain): when arm C uses 30% fewer median tool calls than arm
  A in the pilot, then the result is recorded, the 1.0 documentation still makes no benefit claim,
  and the 1.0.x measurement still decides whether the brief is kept.

## Section reasons of the path form

Each section of a path carries one reason. A reason names what the section's state rests on.

| Section | Reason | State | Meaning |
| --- | --- | --- | --- |
| `instructions` | `tree-at-revision` | `OBSERVED` | `AGENTS.md` or `CLAUDE.md` files on the path's directory ancestry at HEAD are listed |
| `instructions` | `no-instruction-file-on-ancestry` | `OBSERVED` | the ancestry was read and holds no such file |
| `owners` | `last-matching-rule` | `OBSERVED` | the last `CODEOWNERS` rule that matches is listed |
| `owners` | `no-codeowners-rule-matches` | `OBSERVED` | `CODEOWNERS` was read and no rule matches |
| `owners`, `tests`, `flows`, `tickets` | `source-absent` | `NOT_OBSERVED` | every source of the section is missing; each is named |
| `tests` | `plan-and-evidence-read` | `OBSERVED` | the plan and the retained test evidence were both read |
| `tests` | `plan-scope-unknown` | `INCOMPLETE` or `NOT_OBSERVED` | the plan was read and its scope is `UNKNOWN`, so the list is not complete |
| `tests` | `one-source-read` | `INCOMPLETE` or `NOT_OBSERVED` | one of the two sources is missing and named |
| `tests`, `flows`, `requirements` | `path-not-at-head` | `NOT_OBSERVED` | the path is absent from HEAD |
| `flows` | `flows-reader-not-built` | `NOT_OBSERVED` | `.corvint/flows.json` exists and no reader is built |
| `requirements` | `links-not-exhaustive` | `INCOMPLETE` | links are listed; a link file never lists every requirement |
| `requirements` | `no-link-for-path` | `NOT_OBSERVED` | no link names the path |
| `tickets` | `open-ticket-scope` | `OBSERVED` | open tickets whose declared scope or touch paths cover the path are listed |
| `tickets` | `no-open-ticket-covers-path` | `OBSERVED` | the open tickets were read and none covers the path |

## Delivered limits

These are limits of the path form as built on 2026-11-10. Each is a stated absence in the output,
not a silent gap.

- `flows`: no reader for `.corvint/flows.json` is built. The section is `NOT_OBSERVED` with reason
  `flows-reader-not-built` when the file exists and names the missing source when it does not.
- `requirements`: the completed-ticket source is not read, because completed tickets record no
  commits (V1-5301). Every `requirements` section of a path at HEAD names
  `completed-ticket-commits` as missing, and `corvint-tasks` as well when the store cannot be read.
- `tests`: the plan is built from the worktree on disk. With uncommitted changes outside
  documentation the plan is refused and the section names `affected-plan` as a missing source.
- `tests`: make targets, CI jobs and frontier sites that the plan selects are not listed as tests.
  Shared unbounded readers are counted in `sharedReaders` and not listed.
- `tests`: retained test evidence is read for at most 512 files of a call.
- `tests`: retained test evidence lists no test in this build. The evidence reader resolves no
  runner identity, so no retained record counts as fresh and only a fresh record names a test. The
  section therefore names `test-evidence` as missing and never reaches `plan-and-evidence-read`.
- `tickets`: the open tickets are read in pages of 1,000. A ticket that two pages return is listed
  once; a ticket that moves between pages during the read can be missed.
- An absolute `--path` value is accepted when it lies under the repository root as written or
  after links are resolved. A link inside the repository that points outside it is refused when
  the value reaches it through a linked spelling of the root.
- Items are in path or ID order. With more than 64 items the cut is alphabetical, so the most
  direct test can fall outside it; `total` and `truncated` say so.
- 3,866 of the 6,115 evidence spans in the link file hold no requirement definition line. Their
  items name the spec file and span only.

## Failure modes

- Ticket not found, store unreadable or `corvint-tasks` not installed (ticket form): typed refusal,
  exit 2.
- A path outside the repository: refusal `path-outside-repository`, exit 2.
- `--path` with task text, `--task`, `--base`, `--ticket` or any other option of the command: an
  argument refusal, exit 2. For task text and a range it names `corvint impact --base <sha>`.
- A path absent from HEAD: exit 0; `tests`, `flows` and `requirements` are `NOT_OBSERVED` with
  reason `path-not-at-head`.
- An absent or damaged optional source: its section names it as missing and the command exits 0
  (TBC-V1-011). A damaged link file is a missing source, never a partial answer.
- The repository cannot be read at HEAD: refusal `path-brief-unavailable`, exit 2.
- A malformed or unsealed record met by the export: skipped and counted, never repaired.

## Acceptance evidence

| Requirement | Evidence | State |
|---|---|---|
| TBC-V1-001 to TBC-V1-005 | none | `NOT_RUN` |
| TBC-V1-006 | `benchmarks/ticket-brief-task-set/task-set.json` (frozen list and hash) | task list committed; measurement `NOT_RUN` |
| TBC-V1-007, TBC-V1-008 | none | `NOT_RUN` |
| TBC-V1-009 | `internal/pathbrief/brief_test.go` (`TestNormalizePath`, `TestBuildAbsentPath`, `TestBuildLeavesTheRepositoryAlone`, `TestBuildNoCommitIsAnError`); `cmd/corvint/context_path_test.go` (`TestPathContextParse`, `TestRunPathContextRefusesOutsidePath`) | experimental |
| TBC-V1-010 | `internal/pathbrief/brief_test.go` (`TestBuildStructureAndInstructions`, `TestBuildOwners`, `TestBuildTests`, `TestBuildTestsLimit`, `TestBuildFlows`, `TestBuildTickets`, `TestBuildRequirementsStates`); `owners_test.go`; `tickets_test.go` | experimental; `flows` reader and completed-ticket source not built |
| TBC-V1-011 | `internal/pathbrief/brief_test.go` (`TestBuildNeverUsedCorvint`, `TestBuildRequirementsDamagedLinkFile`); `tickets_test.go` (`TestExecTicketsWithoutCompanion`); `cmd/corvint/context_path_test.go` (`TestPathContextNothingRetained`) | experimental |
| TBC-V1-012 | `internal/pathbrief/export_test.go` (`TestExport`, `TestExportWithoutRecords`, `TestExportRefusals`, `TestExportSkipsDamagedRecords`); `links_test.go`; `brief_test.go` (`TestBuildRequirementsReportedEntry`); the run is in `docs/build-log/2026-11-10-path-brief-builder-and-link-export.md` | experimental |
| TBC-V1-013 | `docs/build-log/2026-11-10-path-brief-link-coverage-check.md`: 16 of 20 tasks against the owner's bar of 10 | recorded; bar met |
| TBC-V1-014 | none | `NOT_RUN` |

`make gate` was not run. The path form was read on this repository once by hand; that run is in
the build-log entry and proves no benefit to an agent.

## Rollback

Revert the commits that add `internal/pathbrief`, `cmd/corvint/context_path.go` and the `--path`
branch in `cmd/corvint/taskcontext.go`, and delete `.corvint/requirement-links.json`, this file
and its index row. No stored state or wire format outside the link file changes. If the owner
withdraws PROD-INT-032, or the 1.0.x measurement fails, TBC-V1-009 to TBC-V1-014 are retired and
their IDs are not reused.
