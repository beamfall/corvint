# Pi integration research and delivery proposal

Date: 2026-09-29, America/Toronto. Status: research and native ticket plan; proposed workflow intent, not accepted specifications or delivered modern integration.

The owner requested a Pi plugin with better functionality than OpenCode and deeper use of Corvint and Corvint Tasks. Evolve the existing `integrations/pi` package. The recommended result is one evidence and ticket workflow, with native inspection, explicit mutations, recovery and measured qualification. A second task database, context graph or orchestration framework would duplicate existing product ownership.

## Observed installation and verification

The native installer registered `/Users/russelllewis/projects/corvint/integrations/pi` in personal Pi settings. Its package is linked to this checkout, not an immutable published artifact. Adapter 0.2.0 admits only Pi 0.85.1 and remains FALLBACK. `CORVINT_BIN` must be an absolute executable path. The installed Core accepts its 0.85.1 startup envelope; this does not qualify a different Pi version.

Pi initially reported 0.85.1, then the same executable reported 0.99.1 during the session. This task did not update Pi. npm's published package version also reported 0.99.1. The registered package is therefore **not a usable or qualified modern installation**. No downgrade was performed. Rollback is `pi remove /Users/russelllewis/projects/corvint/integrations/pi`, followed by reload or restart.

The existing adapter regressions passed 21/21. Two native-host runs, the second with an explicit executable path, each passed 2/4 and failed 2/4. The primary failure was the exact host assertion, actual 0.99.1 versus expected 0.85.1. That prevents the nested interruption test from reaching its child witness; this run does not establish a new descendant-cleanup defect. Logs: `/tmp/pi-baseline-tests.log`, `/tmp/pi-host-tests.log`, `/tmp/pi-host-tests-explicit.log`. No external model was called by these offline fixtures.

The start checkout was HEAD `835789f250d3238e97f68ffb5949555f882cc4a4`, with 71 unrelated dirty paths; captured `origin/main` was `6dc8ed1bceaa563c4e2cddb505b5891741bbce5a`. No adapter source was changed. Research used the installed Corvint query and original integration contracts; mixed-worktree context, omissions and learning limitations remain visible. No change enrollment, CEM seal, frozen retrieval evaluation or repository-wide gate was run: this slice installs an existing package and researches future work, without implementing new product behavior.

## Official release evidence

Checked at 2026-09-30 01:53 UTC, still September 29 locally. Installed Core 1.0.0-rc.1 build 163 matches the official darwin-arm64 binary SHA256 `baac338555524a741fe70327cd95d56b5abf0186cac0cc14e199239639053ebf`. The [official Core candidate](https://github.com/beamfall/corvint/releases/tag/v1.0.0-rc.1) retains its failed untouched-repository final evaluation and unsigned release limits.

Tasks changed from build 163 to build 202 during this session; this task did not replace it. Installed SHA256 `4247bab237b4279fdc3d0f243da7dbe0fa1e4278a1cc52422c130bab5b51ae6a` matches the [official Tasks developer snapshot](https://github.com/beamfall/corvint/releases/tag/tasks-dev-20260929.2). It is unsigned, publisher identity NOT_VERIFIED, not stable 1.0 or application-queue qualification, and excludes the issue 354 multi-repository supervisor. Current help exposes `submit`, `gate run` and `complete`; the earlier missing-verbs observation belongs to build 163 only. The queue reported fixture=true and executionCutover=false; neither is runtime authority.

Pi package and executable hashes, npm integrity metadata, Core/Tasks release JSON and official binary reports are retained in `/tmp/corvint-pi-research-20260929/release-freshness.json` and adjacent files. Pi's installed artifact integrity against the complete upstream tarball remains NOT_OBSERVED. Recheck current official versions and exact identities at implementation and qualification; today's latest is not a permanent target.

## Existing integration comparison

| Area | Existing Pi | Existing OpenCode | Proposed Pi outcome |
| --- | --- | --- | --- |
| Context | Pinned context, exact expansion, ephemeral prompt/recovery | Awaited context, inspector and exact expansion | Typed discovery and evidence navigation covering the applicable public Core routes |
| Change review | Three explicit model tools; no cockpit | Files, impact, proof and gaps cockpit | Requirements, changes, selected tests and bound verification alongside Tasks |
| Native Tasks | No Tasks tools in the Pi package | Existing cockpit is a change/evidence surface | Search, blockers, leases, approved edits, native gates and truthful closeout |
| Recovery | Startup, compaction, fork/tree and session recovery | Context recovery and view invalidation | Revalidated branch-sensitive workflow and attempt handles |
| Completion | Legacy stop is non-authoritative and does not continue | Advisory completion; execution authority NONE | Accepted opt-in Pi local workflow integration; protected authority remains separate |
| Qualification | Pi 0.85.1 experimental FALLBACK | Exact passing stock tuples may report integration FULL | Exact modern tuples and a frozen equal-task comparison before superiority claims |

OpenCode's integration support label does not confer protected execution authority. Pi's optional `pi-protected` distribution is also separate from the ordinary package and remains experimental. Its accepted 0.85.1 image contract cannot silently be upgraded to 0.99.1.

## Pi platform opportunities

Official Pi documentation supports custom tools and commands, request context hooks, branch-sensitive state, and a final actionable `agent_before_settle` boundary. `agent_settled` is notification-only. Hooks can request continuation, but that API alone grants no Corvint completion authority. Tools may run concurrently; cancellation and shutdown ownership need exact-host tests. TUI components need mode guards, while RPC supports only selected UI interactions. These findings were checked through [official extension documentation](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) and Context7 `/earendil-works/pi/v0.99.0`; they do not qualify the actual 0.99.1 binary.

Pi now supplies [built-in MCP and deferred tool exposure](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md). Use a compact default surface and discover less-common operations when needed. Structured receipts can support composed reads without dumping entire reports into the model. Discovery or codemode accessibility never authorizes a write. An MCP server is optional; the existing bounded foreground CLI route avoids adding a default service.

Bundle native extensions, skills and explicit prompts through the [Pi package mechanism](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md). Upstream recommends host-provided peer packages rather than bundled host copies; peer packaging and exact runtime admission are different concerns. Coordinate the existing V1-0446 packaging ticket without treating a peer range as qualification.

## Proposed product workflow

Select a native ticket, read its human-owned requirements and original evidence, inspect immutable source, plan the changed scope and tests, implement in an isolated worktree, run bound checks, inspect independent findings, and submit or complete only through the admitted Tasks policy. The cockpit should make every step inspectable without another model request.

Core owns ranking, source identity, authority, CEM/OCM and local completion. Tasks owns tickets, dependencies, attempts, leases, gates and completion. Pi translates their existing receipts into tools and UI. Use typed allowlisted argv, bounded output and deadlines, no shell interpolation, no background indexing and no competing state store. Optional experimental operations retain their admissions, exclusions and uncertainty; expose applicable capabilities progressively rather than mechanically enabling every verb.

Start with exact darwin-arm64 TUI/RPC/print/JSON cells, personal/project install, trusted/untrusted repositories and Tasks-present/absent states. Linux needs actual qualification; Windows descendant cleanup remains unsupported. Call the shared schemas transport-neutral, not OS-portable without evidence.

Writes need explicit user or accepted queue-policy authorization. Persist one logical operation ID before dispatch. Native idempotent replay reuses that ID; after an ambiguous write or crash, reconcile the receipt, ticket revision and attempt owner before another mutation. UI and model routes share this boundary. Role labels are not authentication. Reads never initialize a missing store or take a write lock.

Retain only minimal branch-sensitive workflow/receipt/attempt handles. Revalidate repository, branch, base/tree, session key, attempt generation and lease on resume, fork, tree navigation, compaction and reload. Abandoned branch data cannot authorize renewal or completion. Avoid additional prompt, transcript or credential storage.

**Keep Pi continuation disabled** until an accepted LCP/AHI amendment admits the exact Pi host/adapter/event tuple, workflow ownership, closed unmet categories, deadlines/refusal codes and queued-input/recursion behavior, and exact-host qualification passes. Existing LCP-V0-008/009 admit Codex and Claude only. A future opt-in enrolled workflow may request at most one idle remediation; errors, aborts, queued input and recursion release visibly unresolved. No hook, green tool or settled event completes a ticket by itself.

## Native ticket plan

| Ticket | Scope | Prerequisites |
| --- | --- | --- |
| V1-0506 | Accepted workflow contract and typed capability facade | Retained exact-host discovery; new behavior waits for accepted intent |
| V1-0447 | Modern adapter implementation and exact-host qualification, reusing the existing Pi ticket | V1-0506; coordinate V1-0446 |
| V1-0507 | Native Tasks reads, approved mutations, leases, gates and closeout | V1-0506 and V1-0447 |
| V1-0508 | Evidence, change, Tasks, verification and gaps cockpit | Contract, modern qualification and Tasks workflow |
| V1-0509 | Branch-safe recovery and accepted completion integration | Contract, modern qualification and Tasks workflow |
| V1-0510 | Pi participant in the existing supervisor | Recovery and the V1-0475 supervisor foundation |
| V1-0511 | Exact support matrix, distribution and measured OpenCode comparison | Cockpit and recovery |

These tickets remain OPEN with incomplete effects until implementation scopes are audited. Mutation receipts and native readbacks are retained under `/tmp/corvint-pi-research-20260929/`. Final receipt audit reported CONSISTENT/AGREES, with actor authentication, historical acceptance, liveness and runtime qualification NOT_OBSERVED and semantic coverage UNKNOWN. No implementation or native ticket completion is claimed.

## Review findings and qualification criteria

One independent Sol/low reviewer performed two bounded passes. Initial findings required explicit Pi LCP/AHI acceptance, exact-host native dependencies, durable uncertain-write reconciliation, a concrete support matrix and a falsifiable comparison. The second pass identified a dependency cycle and missing freshness bindings. The final proposal separates host discovery from governed implementation and retains official hashes. No third review or acceptance claim was made.

Before claiming better functionality, freeze both host/plugin/Core/Tasks/model/effort configurations, task fixtures, scoring rubric, seeds and run order. Proposed minimum campaign: 20 representative tasks with three paired repetitions per arm, including context, edits/tests, ticket lifecycle, interrupted recovery, compaction/fork and review. The owner must accept improvement thresholds and statistical decision rules before runs. Require no correctness or critical-evidence regression and all stale/unauthorized-completion negatives to pass. Count failed and timed-out tasks in the denominator; retain every losing run.

Measure complete correctly integrated and verified outcomes, recovery quality, cold/warm p50/p95 latency, and provider-reported token/cache/cost data where available. Billed tokens, comparative cache use and end-to-end savings remain NOT_OBSERVED here. Fewer tool declarations or source bytes do not establish productivity or cost gains. Protected FULL requires its separate exact-image independent admission, adversarial campaign and explicit operator activation.
