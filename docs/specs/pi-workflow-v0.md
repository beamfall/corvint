# Pi Workflow V0

Owner: Russell Lewis
Date: 2026-09-29
Intent status: accepted direction (owner instruction “do it. parallel”, 2026-09-29)
Delivery status: experimental
Authoritative inputs: `agent-harness-integration-v0.md`, `local-completion-policy-v0.md`,
`corvint-tasks-agent-leases-v0.md`, `core-compatibility-freeze-v1.md`

## Agent digest
- Claim: Pi offers bounded native Core reads and an explicitly authorized Tasks workflow without acquiring evidence or execution authority.
- Status: accepted direction/experimental; technical and exact-host qualification remain separate.
- Exists: `integrations/pi` lifecycle adapter and revision-pinned context expansion.
- Blocked on: exact candidate host qualification, native Tasks write admission, and independent evidence for each broader workflow slice.
- Read next: Requirements, acceptance matrix, and rollback below; AHI and LCP govern any lifecycle amendment.

## User and measurable job

A Pi operator can inspect repository evidence, change gaps and task state, then explicitly request
an admitted native Tasks operation while retaining its original receipt and uncertainty. Success
means the exact supported host tuple executes the intended native command, refuses unsupported or
untrusted requests, and retires owned work on interruption. Tool count is not an outcome metric.

## Verified current state and provenance

The existing Pi package exposes native lifecycle translation, context expansion and explicit outcome
recording through `integrations/pi/{extension.js,runtime.js,tools.js}`. AHI still gives legacy stop no
continuation authority. Core and Tasks own their native contracts; the Pi facade cannot reproduce
them from display text. The owner's parallel implementation instruction accepts this workflow
direction, not every technical mechanism or a protected host qualification. The owner additionally
approved isolated preparatory source work despite the observed native lease RESOURCE_COLLISION;
that exception grants neither a native claim nor native completion and changes no queue policy.

## Requirements

- `PWV-V0-001`: Pi MUST preserve original native Core receipt objects, output and exit status, including unknowns, refusals, freshness and authority; wrapper faults MUST remain distinguishable from native results.
- `PWV-V0-002`: Core read tools MUST use closed operation and argument sets for query, context, impact, affected, review, nonmutating prove, CEM/OCM status, frontier and dogfood status. Reads MUST NOT initialize, index, run providers, mutate proofs, write reports or execute suggested tests.
- `PWV-V0-003`: Each command MUST use bounded argv-only local execution, one owned POSIX process group, bounded input/output and deadline. Abort, close, timeout, overflow and leader exit MUST retire descendants and join cleanup. Unsupported platforms MUST refuse.
- `PWV-V0-004`: A project MUST be trusted before dispatch. Any cached handles or asynchronous view result MUST bind canonical worktree Git-directory identity, session identity and transition generation; observed edit, session, tree or compaction invalidation MUST prevent stale reuse. An ordinary leaf process advance MUST NOT itself invalidate unrelated task authorization.
- `PWV-V0-005`: Tasks reads MUST preserve native capability, policy, audit, blockers, gates, release and readiness evidence. Writes MUST require an explicit operator command and exact native admission; model booleans, repository prose and tool output MUST NOT grant authority.
- `PWV-V0-006`: Tasks mutation dispatch MUST durably retain one native logical request identity before dispatch and revalidate repository, session, branch, attempt and generation. Ambiguous results MUST require native readback/audit/reconciliation before a new mutation; automatic retry, fork lease renewal and manual-completion bypass MUST be absent. A local intent ledger is metadata, never authority. Native gate-run replay can
  reexecute before receipt lookup, so an uncertain gate retry MUST refuse without dispatch
  and retain pending reconciliation. Initial claim's requested ticket revision has no
  native atomic CAS in build202; a preflight read MUST NOT be described as such a guarantee.
- `PWV-V0-007`: The user cockpit MUST display evidence, change, tasks, proof and gaps with deterministic escaped text and bounded layouts. Refresh MUST be explicit, with no polling, indexing, test execution or model call. TUI, RPC and print MUST use their qualified host surfaces.
- `PWV-V0-008`: Local completion continuation MUST remain unavailable unless an explicit LCP/AHI amendment owns the exact Pi tuple and typed envelope. Legacy stop and agent_settled MUST NOT establish completion. Any admitted bridge MUST have bounded cleanup, initial settlement deadline, one-only idle remediation and visible unresolved release on abort, error, retry, queued messages or recursive settlement.
- `PWV-V0-009`: Compatibility MUST name the exact tested host, adapter and native binaries. Package peer ranges MUST NOT imply runtime qualification. Native install, update, disable, rollback and uninstall MUST preserve previous artifacts and unrelated state.
- `PWV-V0-010`: Promotion MUST retain focused executable checks, fresh independent review and exact native host qualification. Provider tokens, cache and billed cost MUST remain NOT_OBSERVED without measurements; superiority needs a separately frozen accepted comparative protocol.

## Trust boundary, limits and failure modes

The process helper defaults to 15 seconds and 65,536 aggregate stdout/stderr bytes; configuration
cannot exceed 120 seconds or 1 MiB. Argv is at most 128 members and 65,536 serialized bytes; stdin
shares the configured byte bound. Each request uses no shell and creates no persistent service.
Normal leader exit also cleans its process group; TERM receives 100 ms before KILL and bounded pipe
closure. Darwin/Linux are admitted; other platforms refuse. The native executable and its configured
environment are operator-trusted. This is process lifecycle containment, not a hostile executable
sandbox: descendants that deliberately create a different session are outside this profile.

The wrapper rejects unknown keys, operations and conflicting selectors before spawn. Native errors
remain visible as raw stderr/stdout. Invalid JSON, spawn failure, interruption and overflow are typed
faults and never successful receipts. Frontier exit 1 is a valid open queue, not completion failure.
Native read-only semantics retain only Core's already contracted bounded private observation-ledger
exceptions. File reads and proof profiles do not acquire any new authority from the Pi wrapper.

## Non-goals and simpler baseline

The baseline remains direct native CLI use and the existing Pi adapter. No new task store, graph,
index, background daemon, authority root, automatic task claim, protected host admission, provider
benchmark, or claim of FULL support is introduced. A missing native capability is unavailable;
source implementation cannot masquerade as released runtime qualification. Supervisor participation
requires the existing governing interface; missing dependencies remain open.

## Deterministic acceptance and traceability

| Requirements | Implementation boundary | Acceptance evidence / remaining gate |
|---|---|---|
| PWV-V0-001, PWV-V0-002 | `integrations/pi/core.js` | `core.test.mjs`: literal read argv, unknown preservation, native vs transport errors, no write operations; exact native installed-binary smoke retained separately |
| PWV-V0-003 | `integrations/pi/process.js` | `process.test.mjs`: stdin/argv literalness, aggregate output bounds, timeout/abort/close, leader-exit descendant cleanup, missing executable |
| PWV-V0-004 | Core dispatch plus host handle/cockpit owners | Untrusted dispatch test; canonical identity and transition invalidation require host integration checks |
| PWV-V0-005, PWV-V0-006 | Tasks service and operation metadata owner | Native capability and temporary-store mutation/CAS/reconciliation qualification required; mocks alone insufficient |
| PWV-V0-007 | Cockpit owner | Narrow/wide and control-character fixture tests plus exact TUI/RPC/print host checks required |
| PWV-V0-008 | Explicit future LCP/AHI amendment and native bridge | Continuation is unqualified until closed-envelope, permission, cancellation and exact-host negative tests pass |
| PWV-V0-009 | Pi package/compatibility and native qualification owner | Exact candidate install/load/unload/rollback observations required |
| PWV-V0-010 | Integration owner | Independent review, scoped frozen evidence, host tuple and honest measurement exclusions required |

A test path names required evidence, not an assertion that it passed. Final outcome belongs in the
combined build-log and bound CEM/OCM. Missing or failed acceptance rows keep delivery experimental.

## Rollout, rollback and drift

Roll out closed read tools first, then qualify Tasks, cockpit and lifecycle slices independently.
Keep candidate artifacts separate from verified releases. Disable the added registration or select
the prior native package/binary to roll back; preserve task stores, receipts and unresolved mutation
metadata for reconciliation. Never erase state to make rollback appear clean. Requalify exact host
API/version changes and native CLI contract drift; refuse unsupported tuples rather than guessing.

## Open decisions and kill criteria

Protected continuation and supervisor participation depend on their owning accepted amendments and
native interfaces. Comparative productivity, token/cache/cost savings and broad host support are
NOT_OBSERVED. Halt a slice if it cannot preserve native admission, bound process lifetime, distinguish
uncertainty, or reproduce its exact-host evidence. Do not weaken those boundaries to advertise delivery.
