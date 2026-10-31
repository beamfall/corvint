# Pi Workflow V0

Owner: Russell Lewis
Date: 2026-09-29
Intent status: accepted direction (owner instruction “do it. parallel”, 2026-09-29)
Delivery status: experimental
Authoritative inputs: `agent-harness-integration-v0.md`, `local-completion-policy-v0.md`,
`corvint-tasks-agent-leases-v0.md`, `core-compatibility-freeze-v1.md`

## Agent digest
- Claim: Pi offers bounded native Core reads and an explicitly authorized Tasks workflow without acquiring evidence or execution authority.
- Status: accepted direction (owner instruction “do it. parallel”, 2026-09-29) / experimental; technical and exact-host qualification remain separate.
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
- `PWV-V0-006`: Tasks mutation dispatch MUST durably retain one native logical request identity before dispatch and revalidate repository, session, branch, attempt and generation. Ambiguous results MUST require native readback/audit/reconciliation before a new mutation; automatic retry, fork lease renewal and manual-completion bypass MUST be absent. A local intent ledger is metadata, never authority. Durable replay identity MUST use stable repository, branch and session bindings; a transient in-flight generation MUST fence late results but MUST NOT become persisted replay scope. Native gate-run replay can
  reexecute before receipt lookup, so an uncertain gate retry MUST refuse without dispatch
  and retain pending reconciliation. Initial claim's requested ticket revision has no
  native atomic CAS in build202; a preflight read MUST NOT be described as such a guarantee.
- `PWV-V0-007`: The user cockpit MUST display evidence, change, tasks, proof and gaps with deterministic escaped text and bounded layouts. Refresh MUST be explicit, with no polling, indexing, test execution or model call. TUI, RPC and print MUST use their qualified host surfaces.
- `PWV-V0-008`: Local completion continuation MUST remain unavailable unless an explicit LCP/AHI amendment owns the exact Pi tuple and typed envelope. Legacy stop and agent_settled MUST NOT establish completion. Any admitted bridge MUST have bounded cleanup, initial settlement deadline, one-only idle remediation and visible unresolved release on abort, error, retry, queued messages or recursive settlement.
- `PWV-V0-009`: Compatibility MUST name the exact tested host, adapter and native binaries. Package peer ranges MUST NOT imply runtime qualification. Native install, update, disable, rollback and uninstall MUST preserve previous artifacts and unrelated state.
- `PWV-V0-010`: Promotion MUST retain focused executable checks, fresh independent review and exact native host qualification. Provider tokens, cache and billed cost MUST remain NOT_OBSERVED without measurements; superiority needs a separately frozen accepted comparative protocol.
- `PWV-V0-011`: The registered Pi tools, commands and lifecycle events, and the Core read, Tasks read and Tasks write operations, MUST equal the closed `corvint-pi-capabilities/0` inventory below, and its unavailable set MUST equal the package `unavailableCapabilities`. A Tasks write is negotiated per call: it MUST refuse with `capability-unavailable`, before any mutation argv, unless the installed native `help` result lists the exact verb as implemented; an operation outside the inventory MUST refuse with `unsupported-mutation` without any native call. Deferred routes stay absent until their owning ticket amends this inventory. A new route, state or unavailable entry MUST amend this inventory in the same change.
- `PWV-V0-012`: The read facade MUST carry an actual named task through the native chain: governing clause, immutable source, changed-path impact and selected tests, each retaining the native receipt verbatim, its pinned blob identities, unknowns and plan-only limits. Those reads and native Tasks inspection in a repository without a task store MUST leave every byte of the worktree and Git directory unchanged and MUST NOT initialize a task store or index.
- `PWV-V0-013`: The `corvint_tasks` read tool MUST expose native `ticket search` and `ticket list` as the closed `search` and `list` operations. `search` accepts only `status`, `kind`, `priority`, `owner`, `milestone`, `label`, `text`, `offset`, `limit` and `summary`; `list` accepts only `status`, `offset` and `limit`. `status` and `kind` MUST be members of the native closed status and kind sets; a `list` status set MUST hold distinct members, and `search` takes one status. `owner`, `milestone` and `label` MUST match native label validation (1 to 64 UTF-8 bytes, so spaces and Unicode are accepted, with no control, bidirectional-override or byte-order-mark code point). Unknown keys, a value that could become a flag (leading `-`), control characters, out-of-range pages (`limit` 1 to 100), values outside those sets and duplicate statuses MUST refuse with `invalid-input` before any native call. The native receipt, page and warnings MUST be returned verbatim.
- `PWV-V0-014`: `corvint_core_read` MUST expose the native read-only `overview` and `features` (no input) and `witness` (`base` required, optional `head`, both full commit IDs, optional `evidence`, `--json`) operations. Their native refusals, such as a dirty worktree, MUST stay native receipts; `witness` MUST NOT accept `--cem` or `--stable-artifact-root`, so the facade never applies a CEM map. (Amended, experimental under the standing lane auto-accept, 2026-10-27, V1-2824.) `evidence` is an array of 1 to 8 clean checkout-relative file paths, each passed only as `--evidence-in-repo` (AGW-V0-011), never as `--evidence`; an empty, absolute, `..`-escaping, unclean, flag-like, NUL-bearing or over-1024-character path MUST refuse with `invalid-input` and a named `reason` before any native call, and a link that resolves outside the checkout MUST stay the native exit-2 refusal naming it. No path is silently dropped.
- `PWV-V0-015`: Every registered model tool MUST declare Pi `annotations`: `NATIVE_READ` tools are `readOnlyHint` true, `destructiveHint` false, `idempotentHint` true and `openWorldHint` false; the `EXPLICIT_RECORD` tool is `readOnlyHint`, `destructiveHint`, `idempotentHint` and `openWorldHint` false. Annotations are unverified host hints for permission extensions and grant no authority.
- `PWV-V0-016`: The lifecycle runner MUST refuse a `hostVersion` other than the exact admitted `0.99.1` with `unsupported-host-version` before starting any process. The protected image's ordinary fallback MUST declare its embedded Pi version from one constant that equals its pinned SDK dependency; while that version is not admitted, every fallback MUST be a typed, visible refusal that injects no context.
- `PWV-V0-017`: The package MUST ship Pi skills through its `pi.skills` manifest entry, exactly the inventory `skills` set. Each skill directory MUST contain a `SKILL.md` whose frontmatter `name` equals the directory name and whose body routes only to registered Corvint tools or the operator's explicit `/corvint-tasks` command. A skill MUST NOT instruct the model to write the task store, claim completion, or run a native command outside the registered tools.
- `PWV-V0-018`: (proposed, V1-2573) An interrupted native Tasks writer dispatch MUST receive a per-call TERM grace of 40,000 ms before KILL: the native gate's 5-second wait delay for its reaped group, the CAL-V0-319 30-second interrupted-record write bound, and a 5-second margin for post-run observations, which are not themselves bounded (PWV-V0-019 covers a KILL that still lands after commit). Leader exit ends the grace and its process group is swept 100 ms later; an exit reported after the run settled signals nothing. Reads keep the 100 ms grace. The process helper MUST refuse a `graceMs` that is not an integer from 100 to 60,000 with `invalid-request` before spawn. Failure mode: a KILL that lands after the native receipt commit and before the head apply leaves the store `REDO_PENDING`.
- `PWV-V0-019`: (proposed, V1-2573) When every failing reconciliation read refuses with native code `REDO_PENDING`, the Tasks service MUST return `pending-operation` with mutation `not-attempted`, retain the native receipts under `reconciliation`, and dispatch no writer. The wrapper MUST NOT redo the pending receipt (TM-V0-008): the next native writer redoes it. Any other reconciliation refusal stays `reconciliation-unavailable`.
- `PWV-V0-020`: (proposed, V1-2783) A request with empty input MUST give the child `/dev/null` as stdin and write no stdin pipe; only a non-empty input opens one, and a failed write of it remains `stdin-failed`. Failure mode: a successful leader that exits without reading stdin, such as `git rev-parse` on a fast Linux host, raced the pipe close into EPIPE, reported `stdin-failed`, and Git identity returned `identity-unavailable` (rc.3 gate on linux/amd64).

## Trust boundary, limits and failure modes

The process helper defaults to 15 seconds and 65,536 aggregate stdout/stderr bytes; configuration
cannot exceed 120 seconds or 1 MiB. Argv is at most 128 members and 65,536 serialized bytes; stdin
shares the configured byte bound; an empty input is `/dev/null` rather than a pipe (PWV-V0-020). Each request uses no shell and creates no persistent service.
Normal leader exit also cleans its process group; TERM receives 100 ms before KILL (a native Tasks
writer dispatch receives 40,000 ms, PWV-V0-018) and bounded pipe closure. Darwin/Linux are admitted; other platforms refuse. The native executable and its configured
environment are operator-trusted. This is process lifecycle containment, not a hostile executable
sandbox: descendants that deliberately create a different session are outside this profile.

The wrapper rejects unknown keys, operations and conflicting selectors before spawn. Native errors
remain visible as raw stderr/stdout. Invalid JSON, spawn failure, interruption and overflow are typed
faults and never successful receipts. Frontier exit 1 is a valid open queue, not completion failure.
Native read-only semantics retain only Core's already contracted bounded private observation-ledger
exceptions. An older native Tasks build without `ticket search` or `ticket list` returns its own
refusal receipt; `witness` output above the 65,536-byte bound is a typed `output-too-large` fault, so
narrow the range. A host version mismatch refuses before spawn, so it costs no Core start. File reads and proof profiles do not acquire any new authority from the Pi wrapper.

## Capability contract

This inventory is the closed route contract that the Tasks workflow (V1-0507), workflow recovery
(V1-0509) and qualification release (V1-0511) build on. States: `NATIVE_READ` is a closed native
read; `LIFECYCLE` is an `adapter pi` event under AHI-024 with continuation false; `EXPLICIT_RECORD`
is explicit outcome persistence under AHI-025; `LOCAL_POLICY` is the caller-owned workflow under
LCP-V0-008; `OPERATOR_NATIVE_WRITE` is an explicit operator command whose Tasks verb is negotiated
per call against the installed native `help` result. The exact tuple is the package's, and
`FALLBACK` support never implies authority. The 0.3.10 package ships no copy of this inventory, so
the exact qualified tuple is unchanged; a later adapter version may ship it as a declaration, never
as runtime admission.

```json
{
  "profile": "corvint-pi-capabilities/0",
  "tuple": {"adapterVersion": "0.3.12", "host": "pi", "hostVersion": "0.99.1", "support": "FALLBACK", "surface": "extension"},
  "tools": {"corvint_context": "NATIVE_READ", "corvint_core_read": "NATIVE_READ", "corvint_expand": "NATIVE_READ", "corvint_record_outcome": "EXPLICIT_RECORD", "corvint_tasks": "NATIVE_READ"},
  "commands": {"corvint": "NATIVE_READ", "corvint-context": "LIFECYCLE", "corvint-outcome": "LIFECYCLE", "corvint-record": "EXPLICIT_RECORD", "corvint-tasks": "OPERATOR_NATIVE_WRITE", "corvint-workflow": "LOCAL_POLICY"},
  "events": ["agent_before_settle", "agent_end", "agent_settled", "before_agent_start", "context", "input", "session_compact", "session_shutdown", "session_start", "session_tree", "tool_result"],
  "coreReadOperations": ["affected", "cem-status", "context", "dogfood-status", "features", "frontier", "impact", "ocm-status", "overview", "prove", "query", "review", "witness"],
  "tasksReadOperations": ["attempt", "audit", "blockers", "gate", "gates", "help", "list", "plan", "queue", "readiness", "release", "releases", "roadmap", "search", "ticket"],
  "tasksWriteOperations": ["claim", "complete", "gate run", "release", "renew", "submit", "ticket prioritize"],
  "tasksWriteAdmission": "native-help-implemented-per-call",
  "skills": ["corvint-context", "corvint-impact", "corvint-review", "corvint-tasks"],
  "unavailable": ["Windows descendant cleanup", "authoritative completion continuation", "formal FULL qualification", "protected authority"],
  "deferred": {"approved ticket create and refine": "V1-0507", "comparative qualification and support-matrix release": "V1-0511", "non-fixture native write qualification": "V1-0507", "workflow recovery qualification": "V1-0509"}
}
```

`complete-manual`, `init`, indexing, provider execution and test execution are not routes. The
contract records what the adapter can attempt; each native receipt still decides what happened.

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
| PWV-V0-020 | `integrations/pi/process.js` | `process.test.mjs`: `PWV-V0-020 empty input gives the child no stdin pipe; non-empty input keeps it` |
| PWV-V0-004 | Core dispatch plus host handle/cockpit owners | Untrusted dispatch test; canonical identity and transition invalidation require host integration checks |
| PWV-V0-005, PWV-V0-006 | Tasks service and operation metadata owner | Native capability and temporary-store mutation/CAS/reconciliation qualification required; mocks alone insufficient |
| PWV-V0-007 | Cockpit owner | Narrow/wide and control-character fixture tests plus exact TUI/RPC/print host checks required |
| PWV-V0-008 | Explicit future LCP/AHI amendment and native bridge | Continuation is unqualified until closed-envelope, permission, cancellation and exact-host negative tests pass |
| PWV-V0-009 | Pi package/compatibility and native qualification owner | Exact candidate install/load/unload/rollback observations required |
| PWV-V0-010 | Integration owner | Independent review, scoped frozen evidence, host tuple and honest measurement exclusions required |
| PWV-V0-011 | Capability contract above plus every registering module | `capabilities.test.mjs`: registered tools/commands/events, operation sets and unavailable set equal the inventory; per-call native write negotiation and out-of-inventory refusal |
| PWV-V0-012 | `integrations/pi/core.js`, `tasks.js` reads | `core-host.test.mjs` with actual native Core and Tasks binaries: named task chain and byte-identical worktree/Git directory. Manual qualification only: excluded from `make host-adapter-test`, CI and every gate; run by hand with `node --test integrations/pi/core-host.test.mjs` and `corvint`/`corvint-tasks` on `PATH`, or `CORVINT_BIN`/`CORVINT_TASKS_BIN` set to their paths |

| PWV-V0-013 | `integrations/pi/tasks.js` reads | `tasks.test.mjs`: literal search/list argv, Unicode and spaced labels, and every refusal (unknown status or kind, duplicate status, label bounds) with zero native calls; `tasks-host.test.mjs` returns tickets, page and warnings from native search/list on an initialized store without mutation; `core-host.test.mjs` shows the `UNINITIALIZED` refusal without a store |
| PWV-V0-014 | `integrations/pi/core.js` | `core.test.mjs`: literal overview/features/witness argv, required base, refused CEM flags, `evidence` mapped to `--evidence-in-repo` and every lexical refusal named with zero native calls; `core-host.test.mjs` byte-identical worktree after the real reads, an in-checkout evidence document reported as a source, and an outside link refused natively |
| PWV-V0-015 | `core.js`, `tasks.js`, `tools.js` registration | `capabilities.test.mjs`: each tool's annotations follow its inventory state; live `getAllTools` receipt in the build-log matrix |
| PWV-V0-016 | `integrations/pi/runtime.js`, `integrations/pi-protected/host-version.mjs` | `runtime.test.mjs` refuses before spawn; `pi-protected/host-version.test.mjs` drives the protected fallback through the real runner. Built-image confirmation is NOT_RUN until the image is built |
| PWV-V0-017 | `integrations/pi/skills/`, `package.json` | `capabilities.test.mjs`: shipped skill set, frontmatter names, tool routes, and no named native command or instruction to run tests or checks; `core-host.test.mjs`: the review skill's head rule (a head other than HEAD is refused); live `get_commands` receipt in the build-log matrix |
| PWV-V0-018 | `integrations/pi/process.js`, `tasks.js` writer dispatch | `process.test.mjs`: a TERM-handling leader finishes its record under a 5,000 ms grace and is killed under the default and a 150 ms grace; out-of-range `graceMs` refuses; a leader exit after a graced sweep sends no further signal; `tasks.test.mjs`: the writer call carries 40,000 ms and reads carry none |
| PWV-V0-019 | `integrations/pi/tasks.js` reconciliation | `tasks.test.mjs`: `REDO_PENDING` gives `pending-operation` with no dispatch and other refusals stay `reconciliation-unavailable`; manual `tasks-host.test.mjs` restores the pre-apply head after a real native gate run and requires `pending-operation`, then a native writer redo and clean reads |

A test path names required evidence, not an assertion that it passed. Final outcome belongs in the
combined build-log and bound CEM/OCM. Missing or failed acceptance rows keep delivery experimental.

## Rollout, rollback and drift

Roll out closed read tools first, then qualify Tasks, cockpit and lifecycle slices independently.
Keep candidate artifacts separate from verified releases. Disable the added registration or select
the prior native package/binary to roll back; preserve task stores, receipts and unresolved mutation
metadata for reconciliation. Never erase state to make rollback appear clean. Requalify exact host
API/version changes and native CLI contract drift; refuse unsupported tuples rather than guessing.
The PWV-V0-013 to PWV-V0-017 additions roll back independently: drop the operation from the
`reads` or `fields` map, the `annotations` field, or the `pi.skills` entry, together with its
inventory entry. None of them persists state, so rollback leaves no residue.
PWV-V0-018 and PWV-V0-019 roll back by dropping the `graceMs` argument from the writer dispatch and
the `REDO_PENDING` branch in reconciliation; neither persists state.

## Open decisions and kill criteria

Protected continuation and supervisor participation depend on their owning accepted amendments and
native interfaces. Comparative productivity, token/cache/cost savings and broad host support are
NOT_OBSERVED. Halt a slice if it cannot preserve native admission, bound process lifetime, distinguish
uncertainty, or reproduce its exact-host evidence. Do not weaken those boundaries to advertise delivery.
