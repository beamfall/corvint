# Local Admin Console V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `LAC-V0`
Intent status: accepted (decision 0081)
Delivery status: experimental (S1, S2 ticket verbs, S3, and the chain pane)
Authoritative inputs: `../../AGENTS.md` invariants 4, 7 and 8, `../PRODUCT.md`,
`local-observability-dashboard-v0.md` (the six evidence axes and the P1 gate this document answers),
`../plans/AGENT-TASK-MANAGER-2026-09-04.md`, `corvint-taskman` `docs/SPEC.md` at commit `7bc52a2cf8953931764c57c09fd1b55ec9762e96`
(`taskman-command-result/0`, ticket records, transitions), `work-queue-observation-v0.md`, and each
rendered artifact's own owning specification.

## Agent digest
- Claim: A local, loopback-only console may present task management, evidence, spec and code without becoming repository, execution, or promotion authority.
- Status: accepted (decision 0081)/experimental (S1, S2 ticket verbs, S3, and the chain pane)
- Exists: retained source package `cmd/corvint-console` builds the standalone `corvint-console`, and `internal/console` delivers S1, the ticket half of S2, and S3 — board, ticket detail, spec pane, code pane and delegated mutation over `corvint-tasks` and `git`, plus the Corvint evidence snapshot, the dogfood report, the committed benchmark results and the agent-memory backlogs, and the `/chain` pane that walks a sealed change from hunk to cited evidence, governing requirement and recorded verification (LAC-V0-033..036). The current snapshot executable is `corvint-dashboard-snapshot`; frozen `corvint-dashboard-*` wire/error profiles remain unchanged. The closed release bundle migration is deferred.
- Blocked on: nothing for S1/S2/S3 or the chain pane. U4 — the operator-time measurement against the CLI baseline — still has no instrument.
- Read next: Human intent and scope; Requirements; Trust boundary, limits, and failure modes.

## Human intent and scope

Owner instruction, 2026-09-07, verbatim:

> Corvint should be able to have a dashboard for admin of the task management. It should work like a
> jira board and allow me to see the details of the ticket, where it comes from as well as the spec
> and code. basicically a full web ui for all the user functionality

Two owner calls were recorded in the same session and are binding on this document:

1. **Sequencing — store first.** The ticket store is built after, not alongside, the console. The
   console is not started until `atm` can hold real tickets.
2. **Surface — one admin console.** Scope is every user-facing verb across both repositories: task
   management, Corvint evidence verbs, dogfood runs, benchmarks and the agent-memory backlogs.

Affected user: the owner, or an agent acting for the owner, administering work that is currently
spread across two CLIs, six backlog files, a spec corpus and a decision log. Measurable job: for one
ticket, reach its board position, its originating intent record, the requirement clause that governs
it, and the code at the exact revision it names — without composing CLI invocations by hand, and
without the surface ever asserting more than its sources support.

Scope is deliberately an *inspector with delegated mutation*: the console renders what the owning
tools report and performs a change only by calling the owning tool's own verb. It fails its job if a
board column, a chart, or a green tick is reachable without the artifact that justifies it.

## Verified current state

This section records the pre-console snapshot used for the original proposal, not the current
delivery state. The delivered-behaviour sections below supersede its store and console blockers;
the Agent digest states the current experimental S1/S2/S3 scope.

- `corvint-taskman` builds at `b2796e6` and `atm help` reports thirteen implemented read verbs
  (`ticket list|search|show|blockers|export`, `queue status`, `roadmap`, `gate list|show`,
  `archive export|verify`, `help`, `version`) and thirty-five omitted verbs, including `init` and
  every mutation. Each read emits one `taskman-command-result/0` envelope.
- Every one of those read verbs refuses today. `atm ticket list`, `atm queue status` and
  `atm roadmap` each return `outcome: REFUSED`, `codes: ["UNINITIALIZED"]`, with the warning
  `state dir does not exist; run \`atm init\` (TCP-02)`. There is no ticket data in existence, so a
  board built at this base renders exactly one thing: that refusal.
- `corvint-taskman` `docs/ROADMAP.md` records TCP-02 as partial (authority, journal, archive and
  fixture primitives reviewed; callable writer binding, reservations, restore and the mutation CLI
  held), TCP-02b as not-started, and TCP-03..TCP-09 as not-started.
- Corvint has `internal/dashboard` (five packages) and `cmd/corvint-dashboard-snapshot` (1,509 lines).
  This is the P0 snapshot compiler: it writes one canonical snapshot to stdout and starts no server.
- `local-observability-dashboard-v0.md` blocks all P1 frontend work on four conditions: owner
  acceptance of a capability contract, a separate UI shape brief, a scoped `AGENTS.md` and
  `docs/PRODUCT.md` amendment, and a frozen browser and operating-system matrix. This document is
  the UI shape brief for condition 2 and proposes conditions 3 and 4; condition 1 is the owner's.
- `AGENTS.md` invariant 7 fixes the default local product at one native-Go binary with no UI, no
  hosted service and no permanent daemon, and requires a capability outside that boundary to carry
  its own accepted profile. No such profile exists for a console.

## Requirements

### Boundary and deployment

- `LAC-V0-001`: The console MUST ship as its own opt-in executable and MUST NOT be linked into
  `corvint`, so that the invariant-7 local product remains one binary for an operator who never
  starts the console.
- `LAC-V0-002`: The console MUST bind a loopback address only, MUST require no account or
  credential, and MUST open no outbound network connection for any rendering path.
- `LAC-V0-003`: The console MUST run only for the lifetime of an explicit foreground invocation and
  MUST NOT install a daemon, launch agent, service unit, or login item.
- `LAC-V0-004`: The console MUST hold no database and no durable cache of rendered state; every
  value it shows MUST be read from an owning tool at request time.
- `LAC-V0-005`: Starting, using, or never installing the console MUST NOT change the measured hot
  path of `corvint` or `corvint-tasks`; the task-manager TM-V0-027 gate GP decision rule applies unchanged.

### Provenance and epistemic honesty

- `LAC-V0-006`: Every rendered value MUST be attributable to the exact command invocation, tool
  version, and repository revision that produced it, and the console MUST make that attribution
  reachable from the value without a second query.
- `LAC-V0-007`: Every rendered value MUST carry the six independent axes of
  `local-observability-dashboard-v0.md` — `validity`, `epistemicClass`, `authorityClass`,
  `completeness`, `currency`, `deliveryStage` — taken from the source, never re-derived or defaulted
  by the console.
- `LAC-V0-008`: A missing, invalid, inaccessible, unsupported, or unretained input MUST render as
  `NOT_OBSERVED`, `PARTIAL`, or `INVALID`, and MUST be visually distinct from a measured `0`. The
  console MUST NOT render an empty collection for a source it did not successfully read. A present
  but undecodable part of a source (a metric family, including a family or metric stated as JSON
  `null`, or a requirement row naming a line its file does not have) is `INVALID` and MUST render as
  a gap naming the defect, never be omitted or rendered as empty text.
- `LAC-V0-009`: An envelope whose `outcome` is `REFUSED` MUST render as that refusal with its exact
  codes and warning text; the console MUST NOT present a refusal as an empty board, an empty list,
  or a zero count.
- `LAC-V0-010`: A value derived from more than one source MUST carry the weakest contributing
  authority and MUST name every contributing source.
- `LAC-V0-011`: A rendered page MUST state the observation boundary it was compiled at and MUST mark
  itself historical rather than current once that boundary has passed.

### Mutation

- `LAC-V0-012`: Every state change the console offers MUST be executed by invoking the owning tool's
  own mutation verb. The console MUST NOT write the ticket store, the journal, the repository tree,
  or any Corvint trace directly.
- `LAC-V0-013`: A verb the owning tool reports as unimplemented MUST render as a visible disabled
  control carrying that tool's exact reason string. The console MUST NOT hide it, MUST NOT enable
  it, and MUST NOT simulate its effect locally.
- `LAC-V0-014`: A mutation MUST carry the `expectedRevision` of the record the operator was actually
  shown, and a revision conflict MUST surface as a conflict for the operator to re-read; the console
  MUST NOT re-fetch and retry a mutation on the operator's behalf.
- `LAC-V0-015`: The console MUST NOT be able to accept a specification, advance a delivery stage,
  record an owner decision, qualify a gate, or authorize a cutover. Those remain owner-and-record
  actions under `AGENTS.md` invariant 8.

### Board, ticket, spec and code

- `LAC-V0-016`: Board columns MUST be derived from the owning tool's status and eligibility
  enumeration at request time; the console MUST NOT carry its own column list, and an unknown status
  MUST render in an explicit unmapped column rather than being dropped.
- `LAC-V0-017`: Ticket detail MUST present, for one ticket, its intent record and that record's
  origin, its dependency and blocker closure, its gates, its review dispositions, its attempts, and
  its completion manifest, each carrying the axes of LAC-V0-007.
- `LAC-V0-018`: From a ticket, the console MUST reach the governing requirement clause by
  requirement ID through `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json`, and MUST render
  the spec's declared intent and delivery status alongside the clause. Reading `REQUIREMENTS.tsv`,
  `INDEX.json`, or the named clause file MUST refuse a symlink or other non-regular file rather than
  follow it, and MUST bound the read itself (not a size check performed before the read) so that a
  committed symlink to a device such as `/dev/zero` cannot exhaust the console process. Every path
  component MUST resolve beneath the worktree root, so a symlinked parent directory (a committed
  `docs/specs` link) cannot lead outside it, and the open MUST NOT block on a FIFO substituted after
  the type check.
- `LAC-V0-019`: The code pane MUST render content read at an immutable Git object id and MUST name
  that id. When the worktree is mixed or dirty the console MUST label the difference and MUST NOT
  present working-tree bytes as the committed content. The path checked for that difference MUST be
  matched as a literal name (`git --literal-pathspecs`), not as a pathspec glob, so a differently
  named dirty file whose name happens to match the requested path as a glob cannot mislabel it. A
  failed difference check MUST be labelled as not observed rather than rendered as a clean path, and
  every dirty path, including the first porcelain record, MUST be counted.
- `LAC-V0-020`: Ticket text, review text, agent output, and backlog entries are untrusted input.
  They MUST render as inert text with no markup, script, or link activation, and MUST be reported in
  the envelope's `untrusted` field.

### Public alpha preparation (owner scope, 2026-09-12)

- `LAC-V0-021`: Before invoking any tool, requests MUST match the actual loopback listener's
  Host and port. Foreign or null Origin values MUST fail. State-changing requests MUST be POST,
  carry a per-process cryptographically random token compared in constant time, and fit a bounded
  form body before parsing. Read routes accept GET/HEAD only. Responses MUST prohibit caching,
  framing and MIME sniffing. This is browser request protection, not account authentication.
- `LAC-V0-022`: Tool stdout and stderr MUST be bounded during collection. Timeout, cancellation
  and overflow MUST remain failures even if a complete success envelope was captured. Ordinary
  tool refusal envelopes MUST retain their codes, including a refusal the tool writes to stderr
  with a non-zero exit and no stdout (`corvint-dashboard-snapshot`'s `corvint-dashboard-error/0`);
  only that declared error profile is admitted from stderr, and only alongside a non-zero exit. An
  error object with no code is a failure on either stream, never a refusal (decision 0238). On native Unix, the console MUST own and clean
  up the command process group on every exit; qualified tools cannot deliberately escape that
  group. This is process cleanup, not a sandbox.
- `LAC-V0-023`: HTTP request contexts MUST descend from the foreground invocation. Every server
  return MUST cancel owned work and join shutdown handling; failed graceful shutdown MUST close
  active connections. Tests MUST prove HTTP cancellation and foreground shutdown leave no
  request-owned descendant or listener.
- `LAC-V0-024`: Console Git status MUST use the existing private-metadata status isolation.
  Other Git reads MUST use scrubbed environment and configuration. A browser GET MUST NOT invoke
  repository clean/process filters or honor ambient repository redirection.
- `LAC-V0-025`: The board MUST offer structured ticket creation and detail MUST offer title/body
  editing. The console MUST generate closed payload JSON, not require raw JSON for these tasks.
  Mutations MUST be in both a reviewed console allowlist and the tool's implemented capabilities.
  CREATE uses null expectedRevision; REFINE uses the revision actually shown and never retries
  a conflict automatically. Successful creation MUST link to the returned ticket ID.
- `LAC-V0-026`: An uninitialized store MUST show the owning tool's refusal and an explicit
  initialization action or command. Reads MUST NOT initialize it. Initialization is delegated to
  `atm init` only after an operator action and does not authorize real task execution.

### Roadmap and dependency editing (IPR-02)

- `LAC-V0-027`: The console MUST offer a roadmap view built from `corvint-tasks roadmap` alone, grouping
  its items by the milestone the tool assigned; a ticket the tool assigned no milestone MUST still
  render, in its own group, never dropped. A field the tool reports as `NOT_OBSERVED` (gate
  results) MUST render as such and MUST NOT be upgraded to a pass or fail the tool did not state.
  A ticket the tool marks `BLOCKED` MUST show its blocker closure; a ticket that is not blocked
  MUST NOT trigger a blocker read.
- `LAC-V0-028`: Ticket detail MUST offer editable milestone, order/priority, and dependencies
  using the existing mutation delegation (`refine`, `prioritize`, `set-dependencies`), each in
  both the reviewed console allowlist and the tool's implemented capabilities. Every edit form
  MUST carry the revision shown on that page as `expectedRevision`. A stale revision or a cycle
  refusal MUST render as a refusal that keeps every submitted entry, using the codes the owning
  tool actually returns rather than an assumed name. Because `set-dependencies` replaces the whole
  list, the dependencies form MUST represent every obligation the tool reports: a line holding a
  ticket id is `COMPLETED`, a line holding a ticket id and a gate id is `GATE_PASSED` on that gate,
  and a line with more fields is refused with the entries retained, so re-saving an unchanged
  form never rewrites an existing edge.
- `LAC-V0-029`: The roadmap list MUST paginate when more than 50 tickets are reported, honoring
  the tool's own page report for whether a further page exists rather than guessing from a full
  page alone. A page number that is not positive, or whose offset would not fit the native integer,
  MUST be read as page 1, never wrapped into another offset. Every roadmap and dependency-editing
  control MUST be operable without a mouse:
  labelled fields and submit buttons only, no control reachable solely by pointer gesture.
- `LAC-V0-030`: A requirement page MUST link each path cited in the Implementation column of
  its owning spec's Traceability row, read at the page's commit, to that path's code page. Each
  code page MUST link back to every requirement whose Traceability row cites that exact path.
  Every link MUST carry the commit it was compiled at, and a page opened with a pin that differs
  from the repository's current commit MUST render that as a gap and nothing else. An uncited
  requirement, a cited path absent at the commit or naming a directory, and a citing id absent
  from `REQUIREMENTS.tsv` MUST each render as an explicit gap, never as a link. A spec whose
  Traceability tables expand (ranges multiplied by the paths each row cites) past a fixed bound of
  100000 requirement citations MUST render as an explicit gap on both pages rather than be expanded
  or partially read (amended 2026-09-13).
- `LAC-V0-031`: Before binding, the console MUST refuse to start unless `--repo`, and `--specs` when
  given, each resolves (symlinks followed) to the Git toplevel reported by `rev-parse
  --show-toplevel`. A directory inside a repository, or one inside no repository, MUST be refused
  rather than read as the enclosing repository (decision 0255).
- `LAC-V0-032`: The roadmap page MUST repeat its same read-only request every 30 seconds so derived
  blockers are re-evaluated against current `corvint-tasks` output. The console MUST NOT convert
  this refresh into a mutation, approval, external/manual completion, unknown-evidence waiver,
  admission, release candidacy, attestation, or promotion. The page MUST state those hard stops;
  a blocker clears only when the owning tool no longer reports it. Pages containing mutation forms
  MUST NOT refresh automatically. The roadmap MUST offer a keyboard-accessible pause/resume link
  that preserves the current page, and the hard-stop notice MUST remain present on refused or failed
  reads.

### Change evidence chain (V1-0025, decision 0362)

- `LAC-V0-033`: The console MUST offer one `/chain` pane that lists the sealed change maps committed
  under `.corvint/changes/<bind-commit>.cem.json` at the page's commit and, for one selected change,
  renders the chain changed hunk → cited evidence → governing requirement → recorded verification
  result. Its only inputs are artifacts the CLI already writes: the sealed CEM read at its object id,
  the untracked OCM maps `.corvint/change.ocm.json` and `.corvint/change.ocm.NNN.json`, the local
  trace `.context-corvint/traces/<change>.jsonl`, and Git objects the maps pin by object id. The pane
  MUST NOT add a provenance store, database, cache across requests, outbound connection, write, or
  default-core UI, and MUST NOT consume frontier or any other artifact.
- `LAC-V0-034`: Every edge MUST name the artifact and the field that justify it, and MUST exist only
  when that field names its target by identifier or digest: hunk → evidence by
  `hunks[i].basis[j].evidenceId` equal to one `evidence[k].id`; hunk → requirement by
  `obligations[i].hunkIds[j]` of an OCM map whose `targetRevision` is the change and whose
  `cem.mapSha256` is the SHA-256 of the sealed map's bytes; requirement → clause by the obligation id's
  requirement line inside the OCM `intentScope` span, whose bytes MUST match `spanSha256` at `blobOid`;
  requirement → test claim by `claimIds` equal to one `claims[].id`; change → recorded verification by
  a trace row whose `revision` is the change. The sealed file name MUST be confirmed by
  `<change>:.corvint/change.cem.json` naming the same object. A cited span MUST be re-read at its
  `blobOid` and its bytes MUST match `spanSha256`. No edge may be inferred from position, text,
  a shared path or proximity. A linked edge is structural: it MUST NOT be presented as semantic
  support, as a test result, or as a verification result for one requirement; the recorded outcome
  is the operator's record for the whole revision. Each edge MUST carry the weakest axes of the
  sources it joins under LAC-V0-010; the untracked OCM and trace files state none (LAC-V0-007).
- `LAC-V0-035`: An edge the artifacts do not establish MUST render as a gap row with its class and
  reason, never as a link and never omitted:
  - `missing`: no bound OCM map, an identifier with no target, or an unreadable object, commit or file;
  - `stale`: a map naming the change with another digest, a span whose bytes no longer match its
    digest, an intent scope that no longer defines the obligation, a trace row naming another
    revision, or a sealed name its bind commit does not confirm;
  - `ambiguous`: two or more evidence entries, claims, obligations or trace rows answering one
    identifier or revision, all shown and none chosen;
  - `unverified`: no recorded verification result names the change;
  - `unsupported`: a hunk the map marks `unknown` or `mechanical`, an obligation that lists no hunk or
    claim, an unknown CEM or OCM profile, or a trace row that is not `schema_version` 1.
- `LAC-V0-036`: The pane MUST be reachable from the primary navigation and operable by keyboard with
  plain links only (LAC-V0-029's matrix). Change and hunk links MUST carry the page's commit and render
  nothing when it moved (LAC-V0-030's pin rule). Hunk detail MUST show the hunk's lines read at the
  change commit (or at `baseRevision` for a deletion) and each cited span's bytes read at its object
  id. Only a change the listing at the commit named is read; every object id MUST be full lowercase
  hex before it reaches Git; all artifact text renders inert under LAC-V0-020.

## Non-goals and simpler baseline

Not in scope, at any stage: a hosted service, a shared or multi-user deployment, authentication,
accounts, telemetry, background collection, repository upload, an embedded or external database
service, embeddings, a mobile surface, arbitrary command execution from the page, editing repository
files through the page, and any surface that keeps its own copy of the queue.

The chain pane (LAC-V0-033..036) is not a verifier: it does not validate a CEM or OCM map (that is
`corvint cem verify` and `corvint ocm verify`), does not assess entailment, does not run a test, does
not report a per-requirement verification result, and does not read frontier artifacts, none of
which exist per change today.

The simpler baseline is the pair of CLIs plus `corvint-dashboard-snapshot`, which already answer every
question the console will answer. The console must beat that baseline on one measurable axis —
operator time from a ticket to its governing clause and its code at revision — or it is not worth
its own binary. If TCP-02/TCP-02b land and the CLI turns out to be sufficient, this contract is
withdrawn rather than delivered.

## Trust boundary, limits, and failure modes

| Failure mode | Guard |
|---|---|
| An attractive board hides an unavailable denominator | LAC-V0-008, LAC-V0-010 |
| A refusal renders as "no tickets" | LAC-V0-009 |
| The console becomes the authority its sources are not | LAC-V0-015, LAC-V0-012 |
| A stale page is acted on as current | LAC-V0-011 |
| Ticket or agent text injects markup or instructions into the operator's surface | LAC-V0-020 |
| A mutation lands on a record the operator never saw | LAC-V0-014 |
| The console's own copy of the queue diverges from the store | LAC-V0-004 |
| Working-tree bytes are read as committed evidence | LAC-V0-019 |
| A subdirectory given as `--repo` renders the enclosing repository | LAC-V0-031 |
| A committed symlink is followed to a device or a file outside the worktree | LAC-V0-018 |
| A committed Traceability range expands a bounded spec read into unbounded memory | LAC-V0-030 |
| A chain edge is inferred from a requirement ID in text, a shared path, or an unbound OCM map | LAC-V0-034, LAC-V0-035 |
| A structural chain edge is read as semantic support or a passing test | LAC-V0-034 |
| A chain request reads an unlisted change or passes an unvalidated object id to Git | LAC-V0-036 |
| The console silently broadens the invariant-7 local boundary | LAC-V0-001, LAC-V0-003, §9 |

The console is never a gate. It cannot produce `PASSED`, `SAFE`, `READY_TO_MERGE`, or a correctness
claim, and a page it renders is historical the moment its scan boundary passes.

## Acceptance criteria and testing matrix

Rows are `RUN` where a test in `internal/console/console_test.go` asserts them and `NOT_RUN`
otherwise. The two `NOT_RUN` rows are the GP baseline rerun for LAC-V0-005, which needs a
measurement outside this package, and the operator-time comparison of U4, which has no instrument.

| Requirement | Acceptance evidence |
|---|---|
| LAC-V0-001, LAC-V0-005 | GP baseline rerun showing `corvint` and `atm` timings unchanged with the console absent and present |
| LAC-V0-002, LAC-V0-003 | a bound-address and process-lifetime test; no listener survives the invocation |
| LAC-V0-004 | a test that a second request after an out-of-band store change renders the new state |
| LAC-V0-006, LAC-V0-007, LAC-V0-010 | a rendering test over a fixture envelope asserting axis pass-through and weakest-authority derivation |
| LAC-V0-008, LAC-V0-009 | the live `UNINITIALIZED` refusal rendering as a refusal, not as an empty board |
| LAC-V0-011 | a page asserting its own scan boundary and historical marking |
| LAC-V0-012, LAC-V0-013, LAC-V0-015 | a test that every offered control maps to an owning verb and every omitted verb is disabled with its reason |
| LAC-V0-014 | a stale-`expectedRevision` mutation surfacing a conflict and performing no write |
| LAC-V0-016 | an unknown status rendering in the unmapped column |
| LAC-V0-017, LAC-V0-018 | a ticket fixture reaching its requirement clause and its spec status, and a committed symlink standing in for `REQUIREMENTS.tsv`, `INDEX.json`, or a clause file refused rather than read |
| LAC-V0-019 | a dirty worktree labelled, with committed bytes read at the named object id |
| LAC-V0-020 | a hostile ticket title containing markup rendering inert |
| LAC-V0-021 | a foreign Host, a wrong port, a foreign Origin, a null Origin, a POST to a read route, a missing token, a token supplied only in the query, an oversized body and an unreviewed verb, each rejected before the tool binary runs, with `no-store` and a frame-ancestors policy on every response |
| LAC-V0-022 | a complete `OK` envelope captured alongside a non-zero exit, alongside an overflowing stream, and alongside a signal, each still a failure; a refusal envelope retaining its own code, on stdout and as the dashboard compiler's stderr refusal with exit 2; a stdout error object with an empty code a failure, not a refusal |
| LAC-V0-023 | an HTTP request cancelled mid-tool and a foreground shutdown, each leaving no request-owned descendant process and no listener |
| LAC-V0-024 | a browser read over a repository configured with a clean filter and with ambient redirection, running neither |
| LAC-V0-025 | the board's create form and the detail page's title/body form, the generated closed payload JSON, the shown `expectedRevision` on REFINE, and each ATM byte maximum accepted at the bound and refused one byte over |
| LAC-V0-026 | an uninitialized store rendering the owning tool's `UNINITIALIZED` refusal and the explicit `atm init` action, with the read itself initializing nothing |
| LAC-V0-027 | a roadmap fixture with a milestoned ticket, an unassigned ticket, and a BLOCKED ticket, asserting grouping, the blocker read, and `NOT_OBSERVED` gate results rendered as "not observed" |
| LAC-V0-028 | a `set-dependencies` refusal on ATM's real `STALE_TICKET` and `CYCLE` codes, asserting the submitted dependency entries and shown revision are retained; a record holding a `GATE_PASSED` edge rendered into the form and re-saved as the same edge |
| LAC-V0-029 | the roadmap page-fallback arithmetic, an overflowing page number, and a full page rendering pagination controls as plain links and buttons |
| LAC-V0-030 | a committed fixture whose cited file links to its code page and back at the commit, whose uncited requirement and absent path each render a gap with no link, whose code link pinned to another commit renders no content, and whose over-bound Traceability expansion renders a gap with no link on both pages |
| LAC-V0-031 | a repository toplevel accepted, and a subdirectory of it and a directory inside no repository each refused |
| LAC-V0-032 | the roadmap carries a 30-second refresh, a page-preserving pause/resume link and the hard-stop notice on successful and refused reads, while the board carrying mutation forms has no automatic refresh |
| LAC-V0-033, LAC-V0-034 | a committed sealed change with a bound OCM map and a trace row rendering every edge linked with its artifact and field, the hunk's lines and the cited span bytes (`TestConsoleChainComplete`) |
| LAC-V0-034 | a decoy: the requirement ID in the changed line and a cited span, an OCM map listing the hunk but bound to another map and revision, and a trace naming the changed path under another revision, each linking nothing (`TestConsoleChainTextMatchDecoy`) |
| LAC-V0-035 | one fixture per gap class rendering that class and its reason (`TestConsoleChainGaps`) |
| LAC-V0-036, LAC-V0-020 | the pane current in navigation with plain pinned anchors and no pointer-only affordance, a moved pin and unlisted changes rendering nothing, and markup in the map, OCM and trace text rendering escaped (`TestConsoleChainKeyboardNavigation`, `TestConsoleChainHostileContent`) |
| U4 on the chain pane | `NOT_OBSERVED`: no operator-time comparison against the CLI with a human was run, and no timing is claimed |

## Traceability

| Requirement | Implementation | Test |
|---|---|---|
| LAC-V0-001..005 | `cmd/corvint-console/main.go`, `internal/console/server.go` | `TestConsoleBoundary` |
| LAC-V0-006..011 | `internal/console/axes.go`, `internal/console/taskman.go` | `TestConsoleAxisPassthrough` |
| LAC-V0-012..015 | `internal/console/ticket.go`, `internal/console/server.go` | `TestConsoleMutationDelegation` |
| LAC-V0-016..020 | `internal/console/board.go`, `internal/console/views.go`, `internal/console/code.go`, `internal/console/spec.go` | `TestConsoleRendering`, `TestReadDirtyCheckUsesLiteralPathspec`, `TestHeadKeepsFirstDirtyPath`, `TestReadLabelsUnobservedWorktree` |
| LAC-V0-021 | `internal/console/server.go` | `TestBrowserBoundaryBeforeTools` |
| LAC-V0-022 | `internal/console/process.go`, `internal/console/taskman.go` | `TestConsoleExitClassification`, `TestConsoleOverflowCannotAcceptOKEnvelope`, `TestConsoleProcessLifecycle` |
| LAC-V0-023 | `internal/console/server.go`, `internal/console/process.go` | `TestConsoleHTTPProcessCleanup` |
| LAC-V0-024 | `internal/console/code.go` | `TestBrowserGitReadsDoNotRunFiltersOrRedirect` |
| LAC-V0-025 | `internal/console/forms.go`, `internal/console/ticket.go`, `internal/console/views.go` | `TestStructuredForms`, `TestFormLimitsMatchATM`, `TestATMCanonicalFormEscapes` |
| LAC-V0-026 | `internal/console/views.go`, `internal/console/taskman.go` | `TestConsoleBoundary`, `TestConsoleRendering` |
| LAC-V0-006..011 and LAC-V0-022 on the S3 panes | `internal/console/evidence.go` | `TestConsoleEvidencePane` |
| LAC-V0-008 on a stale requirement row | `internal/console/spec.go` (`line`) | `TestSpecLookupReportsLineOutsideFile` |
| LAC-V0-008 on an absent local artifact | `internal/console/dogfood.go` | `TestConsoleDogfoodPane` |
| LAC-V0-019, LAC-V0-020 on committed directories | `internal/console/code.go` (`Worktree.List`) | `TestConsoleListingPanes` |
| LAC-V0-018 on a committed symlink | `internal/console/spec.go` (`readBounded`), `internal/console/dogfood.go` (`ReadDogfood`) | `TestReadBoundedRefusesSymlink`, `TestSpecLookupRefusesSymlinkedParent` |
| LAC-V0-027 | `internal/console/roadmap.go`, `internal/console/views.go` | `TestRoadmapGroupingAndBlockers`, `TestRoadmapGateResultsNotObserved` |
| LAC-V0-028 | `internal/console/forms.go`, `internal/console/ticket.go`, `internal/console/server.go`, `internal/console/views.go` | `TestTicketPageOffersOnePrioritizeForm`, `TestSetDependenciesStaleRevisionRefusalKeepsEntries`, `TestSetDependenciesCycleRefusalKeepsEntries`, `TestDependenciesPayloadCanonicalJSON`, `TestDependenciesFormKeepsGateObligation` |
| LAC-V0-029 | `internal/console/roadmap.go`, `internal/console/views.go` | `TestRoadmapPageFallback`, `TestRoadmapPageOffsetCannotOverflow` |
| LAC-V0-030 | `internal/console/links.go`, `internal/console/server.go`, `internal/console/views.go` | `TestConsoleRequirementCodeLinks` |
| LAC-V0-031 | `cmd/corvint-console/main.go`, `internal/console/code.go` (`RequireToplevel`) | `TestConsoleRootMustBeGitToplevel` |
| LAC-V0-032 | `internal/console/server.go`, `internal/console/render.go`, `internal/console/views.go` | `TestRoadmapSafeAutoRecheck` |
| LAC-V0-033..036 | `internal/console/chain.go`, `internal/console/server.go`, `internal/console/views.go`, `internal/console/render.go` | `TestConsoleChainComplete`, `TestConsoleChainGaps`, `TestConsoleChainTextMatchDecoy`, `TestConsoleChainHostileContent`, `TestConsoleChainKeyboardNavigation` |

Current executable compatibility is owned by CRB-V0-015. The console accepts explicit `--tasks`
and the legacy `--atm` fallback with equality/conflict/empty handling before any child starts. Its
default lookup prefers adjacent current and legacy companions before current and legacy `PATH`
names. The snapshot roadmap command preserves the stricter explicit-only boundary and performs no
discovery. The current invocation is
`corvint-console -repo REPO [-specs REPO] [-tasks PATH | -atm LEGACY_PATH] [-addr 127.0.0.1:7777]`.
These names do not alter the authority, mutation, lifecycle, or wire requirements above.

## Rollout, rollback, and drift

Staged, and gated in this order:

1. **Original prerequisite (subsequently satisfied).** `corvint-taskman` TCP-02 and TCP-02b, so a store exists and mutation verbs answer.
   No console work starts before the store holds a real ticket.
2. **S1, read-only.** Board, ticket detail, spec pane, code pane over the `atm` read verbs and the
   Corvint snapshot. Every mutation control present and disabled under LAC-V0-013.
3. **S2, delegated mutation.** Controls enable exactly as the owning verbs land, one verb at a time.
4. **S3, one console (delivered 2026-09-08).** The Corvint evidence snapshot, the dogfood report, the
   committed benchmark results and the agent-memory backlogs join the same surface under the same
   axes.
5. **Chain pane (experimental, 2026-09-23, decision 0362).** `/chain` over the sealed change maps,
   the untracked OCM maps and the local trace (LAC-V0-033..036). Rollback to the existing panes is
   deleting `internal/console/chain.go`, the `/chain` route and view fields in `server.go`, `chainView`
   in `views.go` and the navigation link in `render.go`; no artifact, wire or other pane depends on it.

Rollback at every stage is deleting the console binary and its package. Nothing in `corvint` or
`atm` may come to depend on the console, which is what LAC-V0-001 and LAC-V0-004 exist to guarantee;
a dependency in that direction is the drift signal that voids this contract.

## Unresolved

- U1: **resolved** by decision 0081. `AGENTS.md` invariant 7 and the `docs/PRODUCT.md` local-profile
  paragraph carry the §9 amendment.
- U2: **resolved** by decision 0081, frozen as proposed in §8. The delivered surface is
  server-rendered HTML with no client framework, no build step and no external asset, so the matrix
  bounds what is claimed rather than what is relied on.
- U3: **resolved** by decision 0081 — Corvint, as `cmd/corvint-console`. Three of the four panes are
  Corvint-native, and the taskman side is reached through `taskman-command-result/0` rather than a Go
  import. Corvint must never link `corvint-taskman`; that dependency is the drift signal.
- U4: **open, and deliberately unclaimed.** The operator-time comparison against the two CLIs has no
  instrument and no control arm, so no baseline-beating claim is made for the delivered stages.
  The chain pane's operator-time comparison against the CLI with a human is `NOT_OBSERVED` (V1-0025
  AC4); no timing is recorded or implied.

## Delivered behaviour (2026-09-07)

`corvint-console --repo REPO [--specs REPO] [--atm PATH] [--addr 127.0.0.1:7777]`. `--specs` is
separate from `--repo` because a ticket store and the spec corpus that governs it need not be the
same repository; it defaults to `--repo`. Each must be its repository's Git toplevel (LAC-V0-031).

Two facts about the delivered surface are worth stating precisely, because both are places the
implementation is weaker than a careless reading of this contract would suggest.

**The six axes are mostly unstated.** `taskman-command-result/0` carries none of them, and
LAC-V0-007 forbids the console from deriving or defaulting one. Every closed value would therefore
be a claim `atm` never made, so the console renders a sixth state, `NOT_STATED`, visually distinct
from every measured value. Only the code pane states axes today, because content read at an
immutable Git object id genuinely is `OBSERVED` by an `OWNING_VERIFIER`. Making the board's axes
real requires `atm` to emit them, not the console to infer them.

**The unmapped column depends on the tool publishing its enumeration.** LAC-V0-016 forbids the
console from carrying its own column list, so `atm help` was extended to publish the §3.1 status and
§3.2 eligibility vocabularies. Against a build that does not publish them the console draws no board
at all and says why, rather than inventing columns.

The mutation controls mint an idempotency key and an `issuedAt` when the form is rendered and carry
both in the form, so resubmitting it replays the original request through `atm` instead of
committing a second one.

## Delivered S3 behaviour (2026-09-08)

Four panes join the surface: `/evidence`, `/dogfood`, `/benchmarks` and `/backlogs`.

**The evidence pane is the first surface whose axes are real.** `corvint-dashboard-snapshot` states all
six axes for every value it emits, so the pane renders measured axes where a board card renders
`NOT_STATED`. Measured against this repository at `271ffe12`: 180 metrics in five families, of which
167 state `VALID`/`OBSERVED`/`VALIDATED_AT`/`COMPLETE` and 13 state
`UNSUPPORTED`/`NOT_OBSERVED`/`UNKNOWN`/`UNKNOWN` with `authorityClass` `NONE` — an unavailable
measurement saying so, which is what LAC-V0-008 is for. Authority is not uniform either: 68 metrics
are `ADAPTER_QUALIFIED` and 99 are `ADVISORY`, a distinction the pane shows per value rather than
flattening into one page-level claim. Thirteen metrics carry no value at all and render as unmeasured rather than as a
zero.

**Axes are admitted, never translated.** Each of the six is passed to `Axes.Set`, which refuses a
value outside that axis's closed set, so a vocabulary the console does not know leaves the axis
`NOT_STATED`. There is deliberately no mapping table: renaming a source's own vocabulary into the six
axes would be the console deriving an axis, which LAC-V0-007 forbids.

**`query` and `impact` are not on this surface, and the reason is the same one.** Their evidence
states `authority` and `confidence` in a vocabulary of its own — `project-instructions`,
`git-history`, `syntax` — none of which is a closed value of any of the six axes. Admitting them
would leave every axis unstated, and mapping them would be derivation. Putting those verbs on the
surface under real axes requires Corvint to emit the six, exactly as the board's axes require `atm` to
emit them. `corvint-dashboard-snapshot` is the Corvint verb that already does, so it is the one the
evidence pane runs.

**The dogfood report is the loop's own account of itself.** `.corvint/dogfood-report.json` is untracked
local derived state with no content digest and no owning verifier, so the pane states none of the six
axes for it and says so. Every `NOT_PRODUCED` step is rendered with the reason the loop wrote, and no
count is aggregated into a verdict: the console is never a gate. A repository with no report renders
`NOT_OBSERVED`, which is not a pass and not an empty run.
The pane labels the fixed file as the last-written report shared by sessions in that worktree,
shows its path, base and target, and distinguishes file modification time from observation time.
The source carries no session identity; session ownership remains `NOT_OBSERVED` even if its target
matches the currently viewed revision. This is an attribution clarification under `LAC-V0-006`,
not a new currency axis or a session-matching claim (`TestConsoleDogfoodPane`).

**Benchmark results and backlogs are committed bytes, nothing more.** Both panes list a directory of
the tree at the page's revision through `git ls-tree`, so every entry is named by the immutable object
id its bytes come from, and both read content through the same path as the code pane. A result file
declares no evidence class of its own, so its axes are the axes of the read — content at an object id,
observed by Git — and what a run measured is the file's content, not a claim the page makes. Backlog
entries are agent-written untrusted text and render inert under LAC-V0-020. A pane reads only a path
its own listing named, so a listing route cannot be turned into a reader for an arbitrary path. The
listing is read NUL-terminated (`git ls-tree -z`): without it Git quotes a name holding a non-ASCII
or special byte, and the quoted spelling names no committed file, so such an entry could be neither
linked nor read (amended 2026-09-13).

**The evidence pane refuses while the tree is moving, and that was observed rather than stubbed.**
`corvint-dashboard-snapshot` answers `DASHBOARD_REPOSITORY_UNAVAILABLE` when the repository changes
under its scan more times than it will retry, so running the pane during a full `go test ./...` wave
renders that refusal with its exact code instead of a snapshot. Under the same load the compiler can
also exceed the console's per-invocation timeout, which renders as the failed invocation it was. Both
are the tool's own outcome shown as itself; neither produces an empty evidence pane. The snapshot is
the slowest source the console invokes, so `--timeout` may need raising on a loaded host.

U4 is unchanged: no operator-time instrument exists, so S3 makes no baseline-beating claim either.

## Delivered chain pane (2026-09-23)

`/chain` lists the sealed maps under `.corvint/changes` at the page's commit; selecting one renders its
change binding, the local artifacts read and whether each is bound, the recorded verification rows,
every hunk with its cited evidence and governing-requirement edges, and every obligation of a bound OCM
map with its pinned clause, listed hunks, test claims and the change's recorded verification. A hunk
link opens its detail: the lines at the change commit and each cited span's bytes at its object id.

Observed against this repository at base `1894b9e`: all 35 sealed changes list, and change
`f6e68755` renders its binding and its three evidence edges linked while its three requirement edges
are `missing` and its verification is `unverified`. That is the truthful state of a fresh clone: OCM maps and traces are untracked local
files, so a change sealed in another worktree has neither here. The dogfood loop's OCM obligations are
`unknown` until `corvint ocm link` records hunk and claim identifiers, so their requirement edges
render `unsupported` rather than linked.

Launch behaviour is unchanged: foreground-process cleanup is still `TestConsoleHTTPProcessCleanup`.
U4 remains open and the chain pane's operator-time comparison is `NOT_OBSERVED`.

## §8 Proposed qualification matrix

Frozen only on owner acceptance. Proposed: the two most recent stable major versions of one
Chromium-based browser and one Firefox release channel, on macOS at the host version used for
Corvint development, with no support claim for any other browser or platform. A browser outside the
matrix must be refused explicitly at load, never degraded silently.

## §9 Proposed amendment text

Proposed for `AGENTS.md` invariant 7 and the matching `docs/PRODUCT.md` prohibition, on owner
acceptance only:

> An optional, separately-built, loopback-only, read-through local console may be started explicitly
> by the operator. It is not part of the default local product, installs no service, holds no
> database, opens no outbound connection, and carries no authority; the default install remains one
> native-Go binary with no UI.

Until that amendment is accepted and recorded as a decision, this contract authorizes no code.
