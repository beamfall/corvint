# Corvint Tasks agent workflow V0

Owner: Russell Lewis
Date: 2026-11-01
Intent status: proposed
Delivery status: experimental

Authoritative inputs: the owner-requested native ticket V1-3975 and the owner's words recorded there:
"a better defined process to manage tickets ... the status of them keeps getting lost, or dupes are
created, or the status is wrong", and "cli access that agents use to interact with tickets ... could
also be an MCP ... something that ensures that the process is solid and fallproof". The derived
delivery lifecycle of [delivery lifecycle](corvint-tasks-delivery-lifecycle-v0.md) (`TLC-V0-001` to
`TLC-V0-012`), the progress phase of [lane flow](corvint-tasks-lane-flow-v0.md) (`TLF-V0-880` to
`TLF-V0-883`), the similar check of [the CLI](corvint-tasks-cli-v0.md) (`TCL-V0-004`, `TCL-V0-007`),
release membership and the §5.2 writer are reused unchanged unless a requirement says otherwise.

## Agent digest
- Claim: Lanes report ticket transitions as commit trailers that the store host ingests; agent work verbs dedupe findings, sync releases and guard status edits.
- Status: intent proposed; delivery experimental: the verbs, the agent guard and the MCP group exist with focused tests.
- Exists: `corvint-tasks work start|deliver|block|finding|close-merged|release-sync`, package `internal/tasks/worktrail`, `CORVINT_TASKS_ROLE`, `--reason-code`, REFINE key `supersededBy`, corvint-mcp `--tool-profile tasks`.
- Blocked on: owner acceptance; durable qualification on the live store.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

The store lives on one host. Lane hosts cannot write it, so a ticket's status moves only when the
orchestrator closes out by hand. The 2026-11-01 audit of the 3,992-ticket store found 180 OPEN
tickets whose commits sit only on unmerged branches, 127 COMPLETED tickets citing commits absent
from `origin/main`, 1,434 of 1,875 completions (76%) through a bare `complete-manual`, no ticket with
`supersededBy`, 85 tickets archived while OPEN with no reason, 17 OPEN duplicate pairs at title
Jaccard 0.5 or more, and a v1-0 milestone of 887 OPEN tickets against 606 release members. The
causes: no channel carries a lane's transition to the store host, `ticket create` does not offer
the similar match as the place to attach a finding, and release membership is edited by hand.

## Requirements

### Principal class

- `TAW-V0-001`: The session principal class MUST be read from `Env.Role` or, when empty, from
  `CORVINT_TASKS_ROLE`, and MUST be one of `AGENT`, `OPERATOR` or `OWNER`. Unset MUST keep today's
  behavior exactly. Any other value MUST refuse every mutation and every `work` verb MALFORMED with
  tag `TAW_ROLE_UNKNOWN`. The class is a cooperative guard for agents, not authentication: the store
  role (`--role`) and the trusted actor binding are unchanged and still decide permission.

### Trailer grammar

- `TAW-V0-002`: A lane transition MUST travel as Git trailers on one commit: one or more
  `Corvint-Ticket: <local ID>`, exactly one `Corvint-Ticket-State: started|delivered|blocked`, a
  `Corvint-Ticket-Reason: <text>` that is required for `blocked` and optional otherwise, and an
  optional `Corvint-Ticket-Lane: <label>`. A commit carries one state; different states go in
  separate commits (an empty commit is valid). The local ID MUST be a ticket local token
  (`MaxLocalTokenBytes`); the reason is one line of at most 512 bytes with no control characters.
  Parsing and formatting live in package `internal/tasks/worktrail` and MUST round-trip.

### Lane verbs

- `TAW-V0-003`: `work start|deliver|block TICKET... [--reason TEXT] [--lane LABEL] [--allow-unknown]`
  MUST run offline and MUST NOT write the store, the journal or Git. Each validates every ID's token
  form (MALFORMED, `TAW_TICKET_ID`), then, when a local store copy loads, that the ticket exists
  (MALFORMED, `TAW_TICKET_UNKNOWN`, unless `--allow-unknown`, which records a warning) and is not
  COMPLETED or ARCHIVED there (TICKET_STATE, `TAW_TICKET_CLOSED`). `block` without a reason refuses
  MALFORMED, `TAW_REASON_REQUIRED`. An absent or unloadable store copy is a warning,
  `STORE_NOT_PRESENT`, never a refusal. The one item carries the trailer lines and the
  `git commit --trailer` arguments that produce them.

### Ingestion on the store host

- `TAW-V0-004`: `work close-merged --integration REF [--since COMMIT] [--lane-ref REF]... [--dry-run]`
  MUST walk the commits reachable from REF and not from the cursor (or `--since`), oldest first, and
  parse each commit's trailers with `TAW-V0-002`. For each ticket the latest state on REF decides the
  action. A malformed trailer block is reported as a row with code MALFORMED and skipped; it never
  stops the run. A ticket absent from the store is a row `UNKNOWN_TICKET`.
- `TAW-V0-005`: For a `delivered` state the run MUST record `ticket progress MERGED_AWAITING_CLOSEOUT`
  with head the trailer commit, put one canonical integration evidence document (ticket, commit,
  integration ref, ref commit) in the evidence store, and complete the ticket through
  `COMPLETE_MANUAL` whose reason starts `[TAW-REASON:INTEGRATED]` and whose evidence is that
  document's digest. Each write goes through the §5.2 writer with a deterministic request ID derived
  from ticket, state and commit, so a rerun replays. A COMPLETED ticket is `ALREADY_COMPLETED`, an
  ARCHIVED one `NOT_OPEN`; a writer refusal is a row `FAILED` with its code and leaves the ticket as it
  was.
- `TAW-V0-006`: For a `blocked` state the run MUST record progress `BLOCKED` with the trailer reason;
  for `started`, progress `IN_PROGRESS`. Neither changes status.
- `TAW-V0-007`: A `delivered` trailer reachable from a `--lane-ref` and not from REF MUST record
  progress `DELIVERED_ON_BRANCH` with that branch and head, so the derived lifecycle reads
  `AWAITING_INTEGRATION`, and its row carries reason `NOT_ON_INTEGRATION_REF`. A ticket whose latest
  recorded `DELIVERED_ON_BRANCH` head is no longer on its branch and has no integrated trailer is a
  row `HEAD_NOT_ON_REF` with no write. A rebased commit is ingested by its new commit's trailer.
- `TAW-V0-008`: The cursor MUST be a private local file under the state directory keyed by REF, written
  only after a run that is not `--dry-run` and has no `FAILED` row. Correctness MUST NOT depend on
  it: a lost cursor re-walks and replays. `--dry-run` MUST write nothing and report the same rows with
  disposition prefix `WOULD_`.
- `TAW-V0-009`: `work close-merged` and `work release-sync` MUST refuse under class `AGENT`
  (MALFORMED, `TAW_ROLE_NOT_PERMITTED`).

### Findings

- `TAW-V0-010`: `work finding --payload FILE|- --request-id R [--new-despite ID]` MUST run the similar
  check over the store before any write. A match that is a duplicate by `TCL-V0-004` MUST attach the
  finding to that ticket through `ticket recur` (`TCL-V0-007`) with the finding title as context,
  disposition `ATTACHED`. Only `--new-despite ID`, naming the match, files a new ticket despite it; the
  create carries `--not-duplicate-of ID` and the body records `Filed despite similar ID`. With no
  match the finding is created as given, disposition `CREATED`.
- `TAW-V0-011`: `work finding --ingest FILE --request-id R` MUST take a JSON array of create
  payloads, dedupe them against each other with the same similar rule (a later duplicate folds into
  the earlier one as a recurrence line, disposition `FOLDED`), then send each survivor through
  `TAW-V0-010` with request ID `R-<index>`. A rerun with the same file and request ID replays.
- `TAW-V0-012`: `work finding --supersede DUP --by KEEP --request-id R` MUST set `supersededBy` on DUP
  through REFINE and then archive DUP with reason `[TAW-REASON:DUPLICATE] duplicate of KEEP`. REFINE
  gains the key `supersededBy`: it MUST name an existing ticket other than the target. ADOPT keeps it
  protected.

### Agent guard

- `TAW-V0-013`: Under class `AGENT` the §5.2 writer entry MUST refuse `COMPLETE_MANUAL` and `ARCHIVE`
  unless the reason starts with `[TAW-REASON:CODE] ` and carries nonblank text after it, where CODE is
  `ALREADY_DELIVERED`, `INTEGRATED`, `OWNER_DIRECTED` or `NO_CODE_CHANGE` for completion and
  `DUPLICATE`, `SUPERSEDED`, `OBSOLETE`, `OWNER_DIRECTED` or `NOT_REPRODUCIBLE` for archive (MALFORMED,
  `TAW_REASON_CODE_REQUIRED`). `ticket complete-manual|archive --reason-code CODE` writes that prefix;
  an unknown code refuses MALFORMED, `TAW_REASON_CODE_UNKNOWN`. Under `AGENT`, `REOPEN`, `RESTORE`,
  `HOLD` and `RELEASE_HOLD` MUST refuse `TAW_ROLE_NOT_PERMITTED`. Because the guard sits at the
  writer entry, every caller (batch-close `--manual` included) is covered. `OWNER`, `OPERATOR` and an
  unset class keep today's behavior and may pass `--reason-code`.

### Release membership

- `TAW-V0-014`: `work release-sync --release R --milestone M --request-id ID [--dry-run]` MUST add every
  OPEN ticket whose milestone is M and that is not a member, through one `release membership`
  mutation, and report members whose milestone is not M (`OUTSIDE_MILESTONE`) without removing them.
  When nothing is missing it MUST write nothing. A rerun is a no-op.

### MCP

- `TAW-V0-015`: `corvint-mcp --tool-profile tasks` MUST expose a `tasks` group whose tools are
  `tasks_work_start`, `tasks_work_deliver`, `tasks_work_block` and `tasks_work_finding` for class
  `AGENT` (the default), plus `tasks_work_close_merged` and `tasks_work_release_sync` for `OPERATOR`
  and `OWNER` (`--tasks-role`). Each call runs the same CLI verb in process over stdio, with the
  class set, and returns the command envelope. No daemon and no network listener.

### Reads and protocol

- `TAW-V0-016`: `work start`, `work deliver` and `work block` MUST be allowed in a read-only session;
  the other `work` verbs are writes and are refused there unless `--dry-run`.
- `TAW-V0-017`: The lane protocol (which trailers a lane writes and when) MUST be documented in
  `docs/TASKS-EXTERNAL-AGENTS.md`, replacing the instruction that lanes stay out of the store.

## Failure modes and trust

- A trailer is a lane's claim. Completion needs the claim on the integration ref; the ref, not the
  claim, is the evidence, and the evidence document pins both commits.
- A squash merge that moves trailers into the body is not parsed: the ticket stays open and the
  operator sees no row. The repository merges with merge commits (decision 0319).
- A rebased lane commit is ingested by its new SHA; a dropped one leaves the progress claim, which the
  lifecycle reads `HEAD_NOT_ON_REF`, and close-merged reports it.
- A stale lane store copy may lack a ticket filed after the lane started; `--allow-unknown` keeps the
  lane moving, and close-merged reports `UNKNOWN_TICKET` on the store host.
- The class is set by the caller and can be omitted. It narrows what a cooperative agent does by
  accident; it does not stop a hostile one, which the store role and binding still govern.
- A writer failure mid-run leaves earlier rows written and later rows untouched; the cursor does not
  move, and the rerun replays the written rows.

## Non-goals and simpler baseline

- No network service, daemon or remote store access from lane hosts: the simpler baseline, lanes
  writing the store over a tunnel, breaks invariant 7 and the single writer.
- No new ticket status and no new wire code: tags travel in the message of an existing closed code.
- No change to closeout-debt, delivery-screen, batch-close or dispatch.
- No GitHub query: PR state is not read.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TAW-V0-001, TAW-V0-013 | V1-3975 criterion 5 | `internal/tasks/cli/work_guard.go`; `internal/tasks/cli/mutate.go` hook | `TestTAWV0013_AgentGuard`; `TestTAWV0001_RoleUnknown` | none |
| TAW-V0-002 | V1-3975 criterion 2 | `internal/tasks/worktrail` | `TestTAWV0002_TrailerRoundTrip` | none |
| TAW-V0-003, TAW-V0-016 | V1-3975 criterion 2 | `internal/tasks/cli/work.go` | `TestTAWV0003_LaneVerbs` | none |
| TAW-V0-004 to TAW-V0-008 | V1-3975 criteria 3 and 8 | `internal/tasks/cli/work_close_merged.go` | `TestTAWV0005_CloseMergedIdempotent`; `TestTAWV0007_RebasedAndDropped` | live-store qualification |
| TAW-V0-009 | V1-3975 criterion 5 | `internal/tasks/cli/work.go` | `TestTAWV0013_AgentGuard` | none |
| TAW-V0-010 to TAW-V0-012 | V1-3975 criteria 4 and 8 | `internal/tasks/cli/work_finding.go`; `internal/tasks/mutation` REFINE | `TestTAWV0010_FindingAttachesOrCreates`; `TestTAWV0011_IngestDedupesBatch`; `TestTAWV0012_SupersedeArchives` | none |
| TAW-V0-014 | V1-3975 criterion 6 | `internal/tasks/cli/work_release_sync.go` | `TestTAWV0014_ReleaseSync` | none |
| TAW-V0-015 | V1-3975 criterion 7 | `internal/mcp/tasksbridge`; `internal/mcp/cli` | `TestTAWV0015_TasksToolsPerRole`; `TestTAWV0015_CallsEachTool` | none |
| TAW-V0-017 | V1-3975 criterion 8 | `docs/TASKS-EXTERNAL-AGENTS.md` | doc gates | none |

## Resolved decisions

1. Trailers, not a sync service: Git already moves commits from lane hosts to the store host, so the
   transition rides the commit and needs no new transport.
2. Completion of a delivered trailer goes through `COMPLETE_MANUAL` with a stored evidence document
   and a reason code, not through `complete`: the gate path needs a claimed attempt that lanes no
   longer hold. The receipt and the evidence digest make it auditable.
3. Reason codes travel as a reason prefix, so neither the wire nor the record changes.
4. The guard sits at the writer entry so no verb is a side door.
5. One state per commit keeps the latest-state rule unambiguous.

## Unresolved decisions

- Whether the default class should become `AGENT` when a lane charter sets no class.
- Whether `close-merged` should run from closeout-debt automatically after each merge.

## Rollout and rollback

Rollout is additive: a new `work` verb group, an opt-in class, a new optional REFINE key and a new
MCP profile. With the class unset nothing changes. Rollback is reverting the change; a ticket that
already carries `supersededBy` from REFINE stays valid, because the key was always a record field.
The close-merged cursor is a private local file and may be deleted at any time.
