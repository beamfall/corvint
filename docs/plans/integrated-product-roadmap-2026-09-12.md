# Corvint integrated product roadmap

Owner: Russell Lewis  
Updated: 2026-09-12  
Status: owner-requested outcomes; proposed implementation sequence; not release-qualified

Build order, checkpoints and the remaining autonomous-runtime sequence are in the
[historical execution discussion below](#milestones-and-dependency-order).

## Outcome

Deliver a working Corvint with automatic documentation through MCP, automatic test tracking
for agents and VS Code including E2E, a local dashboard, and task management with a roadmap.
All five remain the broader owner-requested integrated outcome. [Decision 0332](../decisions/0332-verified-local-workflow-scope-2026-09-22.md) narrows the current 0.6 Core qualification target; this broader sequence does not add 0.6 prerequisites. The first useful demonstration is
one real change flowing through context, a roadmap ticket, test feedback, updated documentation,
and the dashboard. Building isolated mock screens does not satisfy that outcome.

“Wallay.js” is interpreted from the owner's clarification as Wallaby-style continuous test
feedback, not a dependency on the commercial Wallaby product. “Valid test” has several axes:
it is discoverable and executable, associated with the claimed behavior, current for the
executed inputs, has an observed result, and has whatever strength evidence was actually
measured. A green run or the presence of an assertion alone proves neither relevance nor adequacy.

Task manager initially means durable planning and ticket management. Its already-planned
autonomous agent execution remains a subsequent milestone with its existing qualification
requirements. The local dashboard is an explicitly started optional companion; the default
Corvint installation remains a single native Go binary.

## What exists and what still needs delivery

| Capability | Existing foundation | Missing integrated outcome |
| --- | --- | --- |
| Corvint | Native Go CLI, immutable context/evidence, CEM, local completion gates | Final release qualification and an installed five-capability workflow |
| Automatic docs | Experimental `docs draft` / `docs consume`, source citations; HDC contract | MCP tools, change-triggered maintenance, useful human pages, stale/conflict handling and renderer qualification |
| Automatic tests | Experimental Go live provider, TCQ, affected advice, mutation witnesses, VS Code testing bridge | Qualified automatic scheduling, shared current-state verification, JS/TS and E2E adapters, usable editor and agent feedback |
| Dashboard | Experimental local console and evidence snapshot; current release edits unqualified | Reviewed process/browser fixes and real installed evidence/test/docs/roadmap views |
| Task manager | Separate native `atm`, journaled ticket edits, dependencies, milestones, paginated `atm roadmap` | Safe onboarding, dashboard roadmap editing, controlled import and eventual qualified runtime |

Reuse the owning contracts and queues rather than creating competing implementations:
[AT-14/15/17](../../ROADMAP.md), [LPCV](../specs/live-proof-carrying-verification-v0.md),
[Go provider](../specs/go-live-test-provider-v0.md),
[TCQ](../specs/test-claim-qualification-v0.md),
[source drafts](../specs/source-documentation-draft-v0.md),
[HDC](../specs/human-documentation-compiler-v0.md),
[console](../specs/local-admin-console-v0.md),
[VS Code](../specs/vscode-extension-v0.md), and task manager's `docs/SPEC.md` / TCP roadmap.
The `IPR-` IDs below coordinate bounded delivery slices; they do not replace `AT-`, `TCP-`,
or numbered requirements. Before implementation, amend the owning contract and bind each
slice to its acceptance evidence. No requirement is accepted merely by generating this roadmap.

## Milestones and dependency order

| Milestone | Tickets | Exit demonstration |
| --- | --- | --- |
| M0 — Bootstrap and prove the connected path | IPR-03, IPR-01 | A real fixture change appears as a Corvint evidence result and an editable task in the running local console |
| M1 — Use the roadmap | IPR-02 | View milestones, edit tickets, inspect blockers and criteria, and import this plan into a separate planning store with provenance |
| M2 — Maintain documentation | IPR-04, IPR-05 | An eligible change refreshes a source-bound capability page through an explicit local session; an agent consumes it through MCP |
| M3 — Track tests continuously | IPR-06, IPR-07, IPR-08, IPR-09 | Go and one pinned JS/TS unit/E2E stack update the agent and VS Code; stale, skipped, failed and unsupported states remain distinct |
| M4 — Release the complete local product | IPR-10, IPR-11 | Extracted artifacts complete the integrated workflow and required gates; then request publication approval |
| M5 — Execute the roadmap with agents | Existing TCP-03..08 | Qualified serial builder/reviewer execution first, then bounded parallelism and automatic routing |

M2 and M3 share the evidence contract but need not wait for one another. Implement with one
owner and one independent reviewer; the table is a dependency order, not a fanout instruction.
There are no calendar promises before the first installed path establishes remaining work.

## Buildable delivery slices

### IPR-01 — Finish the smallest installed core / ticket / console path

**Depends:** none. **Owner:** Corvint console and task-manager TCP-02/02b. **State:** in progress.

- Repair the observed console child-exit classification and align form byte limits with ATM.
- Provide reproducible explicit initialization in an isolated planning fixture, then run actual
  `atm`, `corvint-dashboard-snapshot` and `corvint-console` binaries against it.
- **Accept:** create, reload, refine and inspect one durable ticket; stale-revision edits refuse
  without changes; a source/evidence link resolves; nonzero child exit cannot become success;
  stopping the console leaves no owned descendants. Record actual platform/browser evidence.
- **Rollback:** stop/remove optional companions; preserve the store and the default Corvint CLI.

### IPR-02 — Add the roadmap to task manager's dashboard

**Depends:** IPR-01. **Owner:** task-manager existing roadmap reader; Corvint console.

- Add a Roadmap view grouped by existing milestone values, with ticket status, priority,
  owner, blockers, next action, acceptance criteria and links to requirement/evidence details.
- Reuse `atm roadmap` and bounded ticket detail reads; use stable IDs and existing revision
  checks for milestone, ordering and dependency edits. Keep unassigned tickets visible.
- Show declared status separately from observed gate/acceptance evidence. `NOT_OBSERVED`
  stays unknown. A dependency completion claim cannot be inferred from a green row color.
- **Accept:** browser tests cover multiple milestones, an unassigned ticket, a blocked chain,
  missing evidence, pagination, keyboard operation, revision conflict and cycle rejection.
  Read-only visits do not mutate the store. Editing one ticket updates the roadmap after reload.
- **Rollback:** remove the view; existing ticket records and CLI roadmap remain usable.

### IPR-03 — Bootstrap this plan using existing ticket operations

**Depends:** none; existing TCP-02/02b store and mutation primitives. **Owner:** task-manager
planning fixture under existing ticket contracts. Execute before IPR-01/02 so it can track them.

- Initialize an isolated planning store with execution disabled, using the existing explicit
  fixture setup. Create these slices through existing ticket mutations, preserving `IPR-`
  source IDs, owning `AT-`/`TCP-` references, milestones, criteria and source digest.
- Create tickets first, then set dependencies using their actual native IDs. Preview the
  mapping and use stable request IDs so interrupted seeding can replay completed requests
  without duplicates. Retain the per-operation receipts; this is not an atomic bulk import.
- **Accept:** all eleven slices appear in `atm roadmap`; criteria and dependency details
  match this source; replay creates no duplicates; resume after interruption completes the
  remaining operations. A changed source digest requires a reviewed refinement, not overwrite.
- This is a disposable planning projection, not full TCP-06 import or authority cutover.
  This document stays human-owned and is explicitly labelled the source; the planning store
  is its tracked copy. It does not authorize real work admission or evidence promotion.
  Full TCP-06 import/authority switching remains in M5 behind TCP-03/05 and the existing gates.
- **Rollback:** retain receipts and discard only the disposable planning store. No real-store
  reinitialization, new schema or new import framework is required for this bootstrap.

### IPR-04 — Expose source-bound documentation through MCP

**Depends:** IPR-01. **Owner:** AT-15, source-documentation draft, MCP registry.

- Expose the existing draft/consume implementation through validated MCP tools with exact
  source identities, citations and unknowns. Keep read-only calls read-only.
- **Accept:** a real MCP client generates a page and consumes those exact bytes; edited source,
  tampered drafts, invalid paths and unsupported input refuse or report explicit uncertainty.
  CLI and MCP outputs agree on claims and provenance. Tool errors do not report success.
- **Rollback:** unregister the tools; retain the existing CLI and user-owned documents.

### IPR-05 — Automatically maintain useful human documentation

**Depends:** IPR-04. **Owner:** AT-15, HDC, source-draft and
[deployment-neutral continuous maintenance](../specs/deployment-neutral-index-platform-v0.md).
Freeze an accepted bounded-session slice under that owner before implementing automatic writes.

- In an explicitly enabled, bounded local session, detect eligible changes and update one
  useful Corvint capability page with source-backed API/usage information, retained behavior
  examples and limitations. Expand beyond that page only after this path works.
- Preview proposed writes; apply under configured authorization with atomic replacement and
  conflict checks. Never overwrite human intent or turn inferred prose into accepted requirements.
- **Accept:** add/change/remove a supported capability across real revisions; automatically
  refresh affected content, retain conflicts, avoid unrelated churn, and demonstrate clean versus
  incremental equality. The next agent task uses MCP to consume and rederive the page.
  Qualify the existing first HDC renderer profile before claiming rendered-doc delivery.
- **Rollback:** disable the session and restore prior generated bytes; keep human edits intact.

### IPR-06 — Share honest test validity between agent and editor

**Depends:** IPR-01. **Owner:** TCQ, LPCV and VS Code contracts.

- Expose one native verifier/result representation to CLI/MCP and VS Code. Preserve separate
  association, hygiene, freshness, execution and strength-evidence axes with reasons and anchors.
- Freeze the shared language-neutral provider lifecycle under LPCV for both Go and JS/E2E
  adapters, including bounded execution, cancellation and receipt identity.
- Bind source, configuration, command, provider/toolchain and observed environment identities.
  Amend TCQ's current no-direct-CLI boundary explicitly if adding that interface. Caller reports
  remain caller-reported; existing disabled imports cannot be enabled by trusting arbitrary JSON.
- **Accept:** shared vectors for empty/always-skipped tests, wrong target, stale execution,
  unmatched reports, passing reported results, unsupported input, and surviving/killed mutations.
  Agent and editor agree; passing does not imply adequacy, and a killed mutation only establishes
  the witnessed distinction. An unsupported case never becomes a universal “valid” boolean.
- **Rollback:** disable new projections; preserve raw evidence and existing qualification behavior.

### IPR-07 — Automatic Go test tracking

**Depends:** IPR-06. **Owner:** AT-14, GLTP and LPCV.

- Reuse the experimental Go provider. Explicitly enable a foreground session with bounded
  debounce, cancellation, current-input identity and predictable full/configured-scope fallback
  when affected selection cannot justify omission. Do not install a permanent daemon.
- **Accept:** editing a real assertion automatically transitions feedback through running to
  failed/passed; a second edit makes an older result stale even if it finishes later. Cancellation,
  timeout, output caps and shutdown leave no descendants. Record exact supported platform and
  scope; required repository gates still run. Complete owning provider gates before qualification.
- **Rollback:** disable automatic runs; explicit test commands continue to work.

### IPR-08 — Add one JS/TS unit and E2E provider path

**Depends:** IPR-06 shared LPCV lifecycle; Go qualification is not a prerequisite. **Owner:** new bounded LPCV provider slice.

- Select one installed JS/TS unit runner and one E2E runner from the target application's
  actual stack. Candidate choice if none is present: Vitest plus Playwright; this is a planning
  assumption, not delivered support or a dependency installation. Pin adapters before building.
- Bind test/configuration/application build and environment identities. Manage E2E server,
  browser and worker lifetimes explicitly; retain bounded failure artifacts and declared external
  service requirements. Unsupported/omitted browser configurations remain visible.
- **Accept:** a real unit assertion failure and real browser workflow failure automatically
  surface with source links; a fix reruns and clears only current failures. Demonstrate flaky/retry,
  skipped, infrastructure failure, cancellation and stale-app-build states without false green.
  Interruption cleanup must prove no owned server/browser descendants survive.
- **Rollback:** disable that provider; retain explicit runner commands and evidence artifacts.

### IPR-09 — Deliver editor and agent live feedback

**Depends:** IPR-07, IPR-08. **Owner:** VS Code, MCP, LPCV.

- Connect actual current receipts to Test Explorer, diagnostics/source navigation and agent MCP
  queries. Support explicit enable/disable and rerun controls with configured execution authority.
- **Accept:** in a real VS Code extension host and MCP client, follow the same Go, unit and E2E
  failure/fix sequence and inspect matching identities, reasons and stale states. Reopening the
  workspace must not silently relabel old receipts current. Unavailable providers are explained.
- **Rollback:** disable automatic/editor providers without changing test source or gate policy.

### IPR-10 — Join the dashboard around a real change

**Depends:** IPR-02, IPR-03, IPR-05, IPR-09. **Owner:** console, snapshots, task manager.

- Link the roadmap ticket to current source evidence, test results and generated-doc status.
  Show what needs action and why; preserve provenance when opening detail views.
- **Accept:** create a ticket, inspect Corvint context, change code, observe automatic test failure
  and fix including E2E, refresh/consume docs, and inspect the result from roadmap and dashboard.
  A stale test or conflicting doc blocks its corresponding evidence claim. Ticket completion
  requires the owning accepted criteria/gate workflow; the dashboard cannot invent receipts.
- **Rollback:** remove integrated panels; keep each underlying CLI and stored record usable.

### IPR-11 — Qualify and prepare the public release

**Depends:** IPR-10. **Owner:** PUB-V0, release artifact gate, AT-17, companion bundle.

- Finish independent review, source/CEM/OCM commits, canonical Go/interop/race/extension/taskman
  gates, reproducible native and companion archives, matching source/licenses and installed-path
  smoke tests. Reuse passed evidence only where its exact source/profile remains applicable.
- **Accept:** extracted artifacts repeat IPR-10 on the named qualified platform with actual
  browser/editor/MCP clients. Release notes accurately identify every experimental or unsupported
  surface. Public-readiness requires all five requested outcomes, not only the Go CLI archive.
- External holds: GitHub billing currently prevents hosted checks; report that state separately
  and resolve the required hosted check before claiming it passed. Publication, visibility, tags,
  push and signing await the owner's approval of the concrete candidate.
- **Rollback:** retain the prior artifact and disable/remove optional companions. No destructive
  data migration or source-history rewrite is part of this roadmap.

## Task-manager presentation

Start with a milestone list, not a calendar/Gantt scheduler. Each milestone shows tickets and
derived counts; each ticket opens its existing detail/edit view. Keep blocked work and its
dependency visible. Display completion status and evidence status separately. Use existing
milestone labels initially rather than introducing a new database or milestone service.

The roadmap is a view of task-manager tickets after the authority switch. It does not become
a second planner or a second store. Execution remains manual until the existing TCP-03..06
serial-runtime gates and owner cutover are complete. TCP-07/08 then govern parallel execution
and automatic expert/reviewer routing. That work remains on the roadmap beyond the first
integrated local release; no current ticket-board behavior is advertised as autonomous execution.

## Historical next action at drafting

Bootstrap the isolated planning tickets with IPR-03, then finish IPR-01's two concrete review
repairs and real installed demonstration, then IPR-02.
All other tickets are planned, not completed. Existing experimental implementations remain
useful foundations; their presence alone does not close these integration criteria.
