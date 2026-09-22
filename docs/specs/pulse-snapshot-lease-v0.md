# Pulse Snapshot Lease V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: proposed
Delivery status: experimental (P0-A only)
Authoritative inputs: `docs/CORVINT-PULSE.md`,
`docs/specs/live-proof-carrying-verification-v0.md`,
`docs/specs/go-production-kernel-migration-v0.md`, `docs/PRODUCT.md`, `docs/DOGFOOD.md`,
`LICENSE`, and `LICENSING.md`

## Agent digest
- Claim: A local Go coordinator provides bounded P0-A snapshot lifecycle state; full WSI capture, leases, and deltas are not delivered.
- Status: proposed/experimental (P0-A only)
- Exists: `cmd/corvint-pulse`, `internal/pulse`, and frozen P0-A lifecycle transcripts.
- Blocked on: full WSI capture, token/delta schemas, P0-B conformance, and dogfood receipts.
- Read next: Verified current state and contract gap; Delivery stages; Traceability.

## User and measurable job

A normal coding agent should be able to ask one owned local process what source snapshot it observed,
what changed from a prior observed snapshot, and whether a supplied snapshot token still matches a
new bounded observation. Concurrent requests for the same observation should share work. Reuse must
never turn an old snapshot into a current-worktree claim.

The P0 product slice is a local Go coordinator and serialized per-workspace state machine. It proves
the identity and ordering substrate before test providers, affected-test selection, execution, IDE
decorations, or result claims are built. It is useful when it removes repeated repository capture
from agent lifecycle calls and returns an exact, bounded changed-path manifest. It is killed if that
benefit does not appear in registered Corvint-first and Beamfall-second dogfood.

This contract is proposed. The P0-A actor and stdio protocol are experimental; P0-B and later stages
are not started. It does not claim that a full Workspace Source Identity (WSI), snapshot acquisition,
overlays, continuous verification, or any runtime/framework tuple is available.

## Verified current state and contract gap

- `internal/gokernel/repository.go` has a bounded, sanitized, retrying Go repository probe for one-shot
  harness events. It does not build the LPCV WSI and is not a resident coordinator.
- `cmd/corvint` implements an experimental harness-event slice. GPK-V0-010 explicitly prohibits a
  daemon, resident worker, watcher, IPC protocol, or mandatory shared state in that migration.
- Together, `LPCV-V0-001..006` require WSI binding, monotonic events, complete cache keys, and pre/post identity
  checks. They do not define the observation linearization boundary, reusable-token semantics,
  watcher loss, overlay version ordering, restart invalidation, client resynchronization, or which
  requests may share a capture.
- `cmd/corvint-pulse`, `internal/pulse`, and the frozen P0-A transcript corpus now implement only the
  bounded initialization, invalidation, explicit-unsupported, and shutdown surface. Full WSI capture,
  token/delta schemas, P0-B conformance, and Pulse dogfood receipts do not exist and remain `NOT_RUN`.

The missing rules are a separate product contract, not an amendment smuggled into the Go production
kernel migration.

## Relationship to local warming

[`deployment-neutral-index-platform-v0.md`](deployment-neutral-index-platform-v0.md) places optional
hooks, watchers, and an idle-exiting warmer after the exact one-shot local kernel. They are
acceleration only: silence never proves currency, hooks remain best effort, and every result binds
an immutable snapshot identity plus observed lag/freshness. This contract's P0-A state machine and
delivery claims are unchanged. No watcher, startup hook, daemon, or continuously-current claim is
authorized by the deployment direction alone.

## Terms and truth boundary

- **Captured snapshot:** an immutable canonical manifest and bounded retained inputs identified by a
  digest. It may be reused as historical evidence.
- **Observation window:** the bounded interval during which the coordinator constructed and checked a
  snapshot under a named observer model.
- **`VALIDATED_AT`:** a snapshot response stating that the manifest passed the declared observer
  checks during its observation window. It is a snapshot state, not the LPCV result-currency value
  `CURRENT`.
- **Snapshot token:** a structured compare-and-swap input binding one coordinator session, workspace
  generation, WSI or manifest identity, overlay version vector, observer model, and observation
  window. It authenticates nobody and grants no continuing currency.
- **Workspace generation:** a monotonically increasing counter advanced before acknowledging a
  cooperative edit hint or invalidation.
- **Observer model:** the exact declared sources and limitations of change observation. P0 may use
  explicit client invalidations and bounded Git/filesystem captures. Watcher silence, elapsed time,
  or unchanged metadata is never evidence of currency.
- **Historical:** an immutable snapshot that may be inspected or compared but carries no live
  worktree-currency claim.

No portable observer can guarantee that a live multi-file worktree was atomically equal to captured
bytes while arbitrary uncooperative writers run. Even two equal scans have a post-check race and can
observe the same interleaved state. P0 therefore identifies immutable captured evidence and its
observation window. A stronger continuously-current claim requires either an independently qualified
atomic filesystem snapshot/freeze or participation by every writer; neither is P0 scope.

## Delivery stages

| Stage | May land | Must remain explicit |
|---|---|---|
| P0-A actor core | bounded canonical stdio framing, initialization, capability manifest, generation/event state machine, invalidation, shutdown, cancellation | `snapshot.acquire` and overlay operations return explicit unsupported or identity-incomplete errors; no WSI, token, `VALIDATED_AT`, WEI, result, or performance claim |
| P0-B disk capture | full non-overlay WSI construction, immutable manifest, token, changed-path delta, and singleflight | overlays unsupported; no provider, execution, WEI, `CURRENT`, pass/fail, scope, or test-selection claim |
| P0-C overlays | content-addressed versioned overlays and disk-close handshake included in WSI | no overlay execution/materialization claim until LPCV-V0-002 isolation is implemented and tested |
| P0-D dogfood | Corvint agent-session shadow first, then Beamfall authorized-task shadow, with independent fresh-capture comparison | friction and performance evidence only; no independent interoperability, language, provider, safety-of-omission, or market claim |

An implementation reports only its completed stage capabilities. An absent operation is never
inferred from a recognized message name. Partial delivery changes this document and the specification
index to `experimental`; it does not skip the remaining stages.

## Actor states

```text
EMPTY -> CAPTURING -> CAPTURED
  ^          |           |
  |          v           +-> INVALIDATED -> CAPTURING
  +------- UNKNOWN <------+          |
                 |                   v
                 +--------------> STOPPING
```

`VALIDATED_AT` is a transient response produced by a successful capture commit, not a stored promise.
All captured manifests become historical immediately after their response boundary. The actor stores
no `CURRENT` state.

## Local protocol

The proposed protocol name is `corvint-pulse-snapshot/0`. The proposed executable is
`corvint-pulse --root ROOT serve --stdio`. It is an owned child process with canonical newline-delimited
JSON over stdin/stdout, not a login service, socket daemon, or repository-installed background task.

Every request contains exactly `protocol`, `type`, `requestId`, `clientSeq`, and `body`. Every response
or event contains exactly `protocol`, `type`, `requestId` (null for unsolicited events), `sessionId`,
`serverSeq`, and `body`. Unknown or duplicate fields, duplicate JSON keys, invalid UTF-8, noncanonical
numbers, sequence overflow, and trailing data fail closed. Canonical response bytes have no embedded
source bodies and end with one transport LF that is excluded from receipt hashing.

| Request | Required behavior |
|---|---|
| `initialize` | first request only; admits one client ID and boot ID, returns a fresh session ID and exact capability booleans |
| `workspace.invalidate` | generation-CAS request carrying a bounded reason and optional normalized path hints; advances generation and publishes invalidation before acknowledgement |
| `snapshot.acquire` | optional expected token plus exact overlay set; runs or joins an eligible capture and returns `snapshot.validated`, `snapshot.changed`, or `snapshot.unknown` |
| `overlay.replace` | content digest plus bounded bytes, client boot ID, document ID, normalized path, and strictly increasing document version |
| `overlay.close` | removes one overlay only after a matching disk-content digest handshake; otherwise invalidates or becomes unknown |
| `shutdown` | stops admission, cancels/reaps owned work, emits `shutdown.complete`, and exits without residue |

P0-A may advertise only `initialize`, `workspace.invalidate`, and `shutdown`. Calling a known but
unimplemented request returns `unsupported-operation`. Requesting capture before complete WSI support
returns `identity-incomplete` with `snapshotState: "UNKNOWN"`, no token, and no WSI. Overlay requests
before P0-C return `unsupported-operation`; they are never silently ignored.

### Implemented protocol error codes

The `corvint-pulse` stdio server emits the kebab-case codes below (decision 0100). Each row cites the
first emitting site and quotes the `message` of the `error` response written there, adding in
parentheses the condition when the message does not state it.

| Code | First emitting site | At the cited site |
|---|---|---|
| `client-sequence-gap` | `cmd/corvint-pulse/protocol.go:447` | the `reason` of the `workspace.unknown` event emitted when `clientSeq` is not exactly one more than the last; the protocol error that follows carries the same code with "client sequence contains a gap" |
| `client-sequence-regression` | `cmd/corvint-pulse/protocol.go:366` | "client sequence did not advance" |
| `duplicate-request-id` | `cmd/corvint-pulse/protocol.go:382` | "request ID was already used" |
| `invalid-protocol` | `cmd/corvint-pulse/protocol.go:214` | "unsupported protocol" |
| `generation-mismatch` | `cmd/corvint-pulse/protocol.go:527` | "workspace generation does not match" (the actor did not accept the invalidation for the expected generation) |
| `invalid-request` | `cmd/corvint-pulse/protocol.go:403` | "initialize request is invalid" (`clientSeq` is not 1, the body fields are not exactly `clientBootId` and `clientId`, or either is not a token) |
| `pulse-server-failed` | `cmd/corvint-pulse/main.go:49@de6ef80f` | "Pulse stdio server failed" (the stdio server returned an error; exit 2) |
| `resource-exhausted` | `cmd/corvint-pulse/protocol.go:390` | "request ID capacity was exhausted" (65,536 request IDs seen; an exact-field `shutdown` on an initialized session is still accepted) |
| `session-already-initialized` | `cmd/corvint-pulse/protocol.go:458` | "session is already initialized" |
| `session-not-initialized` | `cmd/corvint-pulse/protocol.go:398` | "initialize must be the first request" |
| `session-unknown` | `cmd/corvint-pulse/protocol.go:438` | "session cannot continue after a sequence gap" (any request other than an exact-field `shutdown` once a gap has marked the session unknown) |

## Proposed implementation surface

The smallest expected implementation owns only:

- `cmd/corvint-pulse/**` for the stdio lifecycle and signal boundary;
- `internal/pulse/**` for protocol, actor, manifest, and capture code;
- an internal sanitized Git/process helper shared with `internal/gokernel` only after exact harness
  parity proves that extraction changed no existing bytes, errors, deadlines, or cleanup; and
- `conformance/pulse-snapshot-v0/**` for canonical protocol and independent-verifier vectors. This
  path is already part of the Apache-2.0 interoperability layer defined by `LICENSING.md`.

It does not add Pulse behavior to `cmd/corvint`, a repository startup file, editor package, or test
provider. A different file decomposition is conforming only if it preserves these product and
process boundaries.

A successful P0-B/P0-C token contains exactly:

```text
sessionId
workspaceGeneration
wsi
overlayVersionVector
manifestSha256
observerModel
observationStartSeq
observationEndSeq
```

The snapshot response uses `snapshotState: "VALIDATED_AT"` and an observation window. It does not use
the LPCV result field `currency`, because P0 produces no run result or WEI. A token submitted after an
invalidation, sequence gap, restart, different overlay vector, or different session is stale input and
cannot revive its snapshot.

## Requirements

### Protocol and state

- `PSL-V0-001`: The coordinator MUST be one client-owned local Go process over stdio, one serialized
  actor per resolved worktree, with no network listener, daemon installation, login item, database,
  repository write, or required persistent cache.
- `PSL-V0-002`: Requests and responses MUST use `corvint-pulse-snapshot/0`, strict bounded canonical
  JSON, duplicate-key rejection, monotonic client/server sequences, and explicit capability booleans.
- `PSL-V0-003`: Initialization MUST create a fresh unpredictable session ID. Restart, disconnect,
  sequence gap, or server-sequence overflow invalidates every token and live view from that session;
  historical receipts remain inspectable only after explicit revalidation.
- `PSL-V0-004`: State transition and event publication MUST be one serialized commit. An invalidation
  advances the workspace generation and becomes observable before any response for the newer
  generation. Late, duplicate, reordered, or old-session messages cannot regress state.
- `PSL-V0-005`: Every recognized but unimplemented operation MUST return `unsupported-operation` or
  `identity-incomplete`. Capability absence, malformed output, timeout, cancellation, or internal
  failure MUST NOT emit a token, WSI, `VALIDATED_AT`, WEI, result, or implicit success.
- `PSL-V0-006`: The actor MUST implement `EMPTY`, `CAPTURING`, `CAPTURED`, `INVALIDATED`, `UNKNOWN`, and
  `STOPPING` as explicit states. `VALIDATED_AT` is response-local; `CURRENT` is not an actor state.

### Capture, reuse, and truthfulness

- `PSL-V0-007`: P0-B WSI construction MUST cover the LPCV-V0 WSI fields: repository/worktree
  identity, object format, base commit and tree, canonical index state, every admitted path mode and
  content commitment, relevant configuration, declared execution profile, overlays when supported,
  and an explicit bounded frontier for excluded inputs.
- `PSL-V0-008`: Clean Git-tree content MAY use the immutable tree as an independently expandable
  Merkle commitment only when the observer profile and verifier define that expansion exactly.
  Dirty, untracked, symlink, and overlay inputs MUST use bounded no-follow content observation; an
  unsupported submodule, special file, filter, generated input, or external input enters the frontier
  or makes identity unknown.
- `PSL-V0-009`: Capture MUST use bounded pre/post repository and input observations, descriptor-safe
  reads, and a finite retry count. A race, overflow, changed generation, unequal observation, or
  incomplete input produces retry then `UNKNOWN`; it never produces a mixed manifest.
- `PSL-V0-010`: Matching observations establish only `VALIDATED_AT` under the named observer model.
  Watcher silence, a TTL, path metadata, equal dirty-path names, or a previous WSI MUST NOT establish
  continuing currency or an atomic live-worktree cut.
- `PSL-V0-011`: Captured manifests and derived deltas are immutable and content-addressed. Reuse is
  historical by default. Every operation requiring a live-current view MUST perform a generation,
  overlay-vector, expected-WSI, and fresh-observation CAS inside the actor or explicitly abstain.
- `PSL-V0-012`: Requests with the same worktree, generation, overlay digest, profile, and observer
  model that are admitted before one capture commit MAY singleflight. A request admitted after the
  commit boundary MUST perform or join a newer observation and cannot inherit the prior response.
- `PSL-V0-013`: Cache or retention expiry controls memory only. Eviction loses performance, not
  authority. Cache presence, absence, corruption, or deletion MUST NOT change canonical WSI or delta
  bytes for the same admissible captured inputs.

### Overlays and clients

- `PSL-V0-014`: P0-C overlays MUST bind client ID, client boot ID, document ID, normalized path,
  monotonically increasing version, byte count, content SHA-256, and bytes. The WSI binds the complete
  overlay version vector and content digests; raw bytes are never emitted or persisted by P0.
- `PSL-V0-015`: An old, duplicate-with-different-bytes, conflicting-path, or reordered overlay update
  fails closed. Conflicting clients use isolated namespaces or receive an explicit conflict; their
  bytes are never merged heuristically.
- `PSL-V0-016`: Overlay close/save requires the declared disk digest to match a fresh bounded disk
  observation. A mismatch, disconnect with open overlays, or missing version invalidates the prior
  token and returns `UNKNOWN`; it cannot silently fall back to disk.
- `PSL-V0-017`: Overlay execution and materialization are outside P0-C identity support. Until the
  isolated no-worktree-write path required by LPCV-V0-002 passes, capabilities MUST report overlay
  execution false and no execution claim may consume an overlay WSI.

### Security, limits, and lifecycle

- `PSL-V0-018`: Repository paths, Git output, JSON, overlay bytes, manifests, and environment are
  untrusted. Root and input paths MUST be normalized and confined; symlink swap, traversal, socket,
  device, FIFO, unsafe permission, and special-file cases fail closed before bytes escape the root.
- `PSL-V0-019`: Git subprocesses MUST preserve the sanitized configuration, no-fetch, no-credential,
  no-hook/filter/replacement, bounded-output, aggregate-timeout, owned-process-tree, and descendant
  cleanup rules required by GPK-V0-009. Pulse MUST make no network request or DNS lookup.
- `PSL-V0-020`: Each JSON frame is at most 1 MiB with nesting at most 256. P0-C admits at most 256
  overlays, 1 MiB per overlay, and 16 MiB aggregate overlay bytes. Capture admits at most 100,000
  paths, retains at most 64 MiB of bounded snapshot/overlay material, and has a 30-second aggregate
  deadline with at most three stability attempts. Exceeding a bound returns a typed resource error
  and no validated token.
- `PSL-V0-021`: Raw source and overlay bodies, absolute roots, credentials, environment maps, prompts,
  and task text MUST NOT enter protocol responses, receipts, logs, or persistent files.
  Repository-relative path names and digests are returned only when repository policy permits them.
- `PSL-V0-022`: EOF, cancellation, interrupt, timeout, panic, failed upgrade, and shutdown MUST stop
  admission, invalidate live views, cancel and reap descendants, clear in-memory raw bytes, and leave
  no socket, temporary materialization, current-result, or repository residue.

The coordinator and client launcher jointly own one process group. Qualification sends an
uncatchable termination to that group, not only to the coordinator leader; leader-only `SIGKILL`
cannot portably execute cleanup and is not represented as an in-process guarantee.

### Claims, dogfood, and rollback

- `PSL-V0-023`: P0 MUST NOT discover or run tests, build a verification plan, select or exclude a
  check, create a WEI, publish pass/fail/scope/currency axes, decorate an editor, authorize merge, or
  suppress a repository command. Returned deltas are source observations only.
- `PSL-V0-024`: Corvint dogfood runs first in shadow during normal agent sessions. Each acquisition is
  compared with an independent fresh capture; the existing Corvint commands and full verification gate
  remain authoritative. Self-use is friction evidence, not independent validation.
- `PSL-V0-025`: Beamfall dogfood runs second during an actual roadmap-authorized task. Pulse MUST
  preserve `AGENTS.md`, roadmap authority, context-packet/workflow gates, and `make gate` as
  unsuppressible authority; repository status outside the authorized task diff is unchanged.
- `PSL-V0-026`: Rollback is process termination and selection of existing one-shot Corvint commands.
  It requires no repository, receipt, manifest, or cache migration. Runtime-tagged in-memory state is
  discarded, and old tokens are invalid after restart.
- `PSL-V0-027`: Delivery and performance claims are stage-specific. P0-A cannot inherit P0-B/P0-C
  identity claims; Corvint or Beamfall dogfood cannot qualify a provider, language, operating system,
  Wallaby-class feature, LPCV P1/P2/P3 gate, or commercial product.

## Deterministic acceptance matrix

1. `PSL-AT-001` (`001..006`): strict framing rejects duplicate/unknown fields, invalid UTF-8,
   noncanonical values, over-depth/over-size frames, sequence overflow, and conflicting duplicate
   request IDs; capability bytes are canonical and stage-exact.
2. `PSL-AT-002` (`003..006`): restart, disconnect, old session, duplicate, reordered, dropped, and
   sequence-gap messages cannot regress state or validate an old token.
3. `PSL-AT-003` (`004`, `010..012`): an invalidation is emitted before its acknowledgement and before
   any newer snapshot response; a post-commit request starts or joins a new observation.
4. `PSL-AT-004` (`005`, `017`, `023`, `027`): every pre-stage operation returns the exact unsupported
   or identity-incomplete error and emits no WSI, token, WEI, result axes, or implied capability.
5. `PSL-AT-005` (`007..013`): clean construction, repeat construction, retained-manifest reuse, and
   reconstruction after eviction produce byte-identical canonical WSI and delta bytes.
6. `PSL-AT-006` (`009..012`): 32 concurrent identical acquisitions execute one eligible capture and
   return the same manifest identity; one request admitted after the cut does not reuse its validation.
7. `PSL-AT-007` (`009..011`): mutations at every pre-scan, read, post-scan, commit, and response
   boundary cause retry, `UNKNOWN`, or a newer observation; no old snapshot is represented as current.
8. `PSL-AT-008` (`008..010`): a same-size content edit with restored mtime, create/delete/rename during
   enumeration, changed index, changed config, and changed dirty bytes cannot yield a mixed accepted
   manifest under the declared observer model.
9. `PSL-AT-009` (`008`, `018`): symlink swaps, absolute/parent escapes, submodule boundaries, sockets,
   devices, FIFOs, permission changes, and special-file replacement expose no escaped bytes and fail
   with the registered frontier or typed error.
10. `PSL-AT-010` (`014..017`): overlay v2 invalidates a delayed v1 response; stale versions,
    conflicting clients, changed bytes under one digest/version, and reordered updates fail closed.
11. `PSL-AT-011` (`014..017`): overlay close racing a divergent disk write yields `UNKNOWN`; a matching
    disk digest removes the overlay in exactly one new generation without writing the worktree.
12. `PSL-AT-012` (`013`, `020..022`, `026`): absent, full, evicted, and corrupt disposable state changes
    latency only; bounds, cancellation, EOF, interrupt, timeout, and forced termination leave zero
    descendant, raw-byte, temporary, socket, repository, or live-view residue. Forced-kill cases
    target the coordinator's owned process group.
13. `PSL-AT-013` (`018..022`): repository status, config, index, and worktree bytes before and after
    every read-only case are identical; a network/DNS/process spy observes no undeclared access.
14. `PSL-AT-014` (`003..006`, `009..012`): one million preregistered randomized duplicate, drop,
    reorder, reconnect, invalidation, capture-completion, and old-session schedules produce zero
    newer-generation contamination; any unresolvable gap clears the live view.
15. `PSL-AT-015` (`024`, `027`): registered Corvint normal-agent sessions compare coordinator and
    independent captures with zero unexplained byte mismatch and record redundant captures, agent
    wait, complete response bytes, misses, and outcome without upgrading self-use into validation.
16. `PSL-AT-016` (`025`, `027`): only after Corvint passes, registered Beamfall authorized-task sessions
    show the same capture/delta equality, no authority or gate suppression, and no out-of-task
    repository mutation.

## Latency and safety gates

All measurements use production builds, exact revisions, a preregistered machine/load protocol, and
fresh process sets. They are P0 engineering gates, not LPCV P3 or product qualification.

- explicit invalidation acknowledgement p95 is at most 50 ms;
- protocol decode, actor transition, and canonical response overhead p95 is at most 5 ms excluding
  repository capture;
- 32 same-key requests execute one capture and complete within the capture latency plus 20 ms p95;
- unchanged strict acquisition p95 is at most 250 ms on Corvint and 500 ms on Beamfall, reported
  separately from client/tool latency;
- idle CPU median is below 1% of one core; production RSS is at most 64 MiB plus the explicit retained
  material cap and never exceeds the platform-enforced bound;
- a 100-edit cooperative burst loses no invalidation or response and violates no ordering rule; and
- stale/newer-generation attachment, repository mutation, network/DNS access, leaked descendant,
  escaped path, raw-body disclosure, and residue counts are all zero.

A missed latency target is a failed performance gate, not permission to weaken identity or ordering.

## Failure, degradation, and rollback

| Failure | Required response | Recovery |
|---|---|---|
| incomplete capture or unsupported WSI field | `snapshot.unknown`; no token | repair observer or use one-shot Corvint command |
| generation/expected-token mismatch | invalidate old view; observe again | client resyncs from the new session/generation |
| client sequence gap or reconnect | actor `UNKNOWN`; old tokens rejected | initialize a new session and reconstruct |
| overlay conflict/save mismatch | invalidate or `UNKNOWN`; retain no current claim | resolve client/disk authority, then reacquire |
| resource limit or timeout | typed error; cancel/reap; no partial manifest | reduce admitted scope only through reviewed policy or use baseline |
| coordinator panic/corruption | stop serving and terminate | discard memory and restart; one-shot commands remain available |
| dogfood mismatch | retain fixture and disable reuse | independent fresh capture remains authority |

The kill switch terminates the owned process and returns clients to existing one-shot Corvint commands.
It changes no repository file or persistent state.

## Promotion and kill criteria

Implementation stops immediately on one stale/newer-generation attachment, mixed WSI, sequence
regression, worktree write, unsafe path read, raw source/secret disclosure, network access, descendant
leak, watcher/TTL silence treated as proof, provider/result claim, or mandatory-gate suppression.

Before starting providers, selection, watchers, sockets, persistent caches, or IDE work, register and
run at least 20 Corvint normal-agent sessions followed by at least five Beamfall authorized-task
sessions. Keep the resident coordinator only if, with non-inferior correctness and zero added critical
miss, it eliminates at least two redundant captures per task **and** improves median agent-visible
wait by at least 20% against the one-shot baseline. Otherwise kill the coordinator slice, retain any
independently useful snapshot manifest/verifier, and use one-shot capture.

Passing these criteria promotes only the implemented PSL stage to `experimental`. It does not satisfy
LPCV P0, which separately requires the frozen provider protocol, independent verifier agreement, the
full stale-event corpus, and interruption evidence on every claimed operating system.

## Non-goals and simpler baseline

The simpler baseline is the current terminating Corvint/Python and experimental Go CLI calls. They
remain authoritative and are preferred if coordination does not measurably remove repeated work.

P0 does not include test discovery or execution, affected-test selection, exclusion certificates,
widening, coverage, runtime values, WEI, result axes, IDE decoration, filesystem watcher
qualification, sockets, daemon installation, database, disk cache, cloud service, telemetry, update
discovery, licensing enforcement, account/entitlement code, merge, deployment, publication, or a
Wallaby parity claim.

## Relationship to LPCV, GPK, and licensing

This contract refines the coordinator-side observation and client-ordering prerequisites
of `LPCV-V0-001..006` and `LPCV-V0-032..036`; it does not replace or weaken them. LPCV remains proposed
and not started. No PSL token is a WEI, verification receipt, `CURRENT` result, or exclusion witness.

`GPK-V0-010` continues to prohibit resident state in the Go production-kernel migration. Pulse uses a
separate executable, package boundary, rollout, and kill switch. Shared bounded Git/process code may
be extracted only with exact harness parity evidence; neither the presence of Go nor a shared helper
counts as GPK or Pulse delivery.

This proposal changes no license or path exception. `LICENSE` and `LICENSING.md` remain authoritative.
Portable receipt/verifier or conformance paths and product coordinator paths may receive different
terms only through an explicit owner-approved licensing decision. Commercial terms, activation,
identity, metering, and enforcement do not enter this protocol.

## Traceability

| Requirement group | Delivery | First authoritative evidence |
|---|---:|---|
| `PSL-V0-001..006` protocol and actor | experimental (P0-A) | 12 frozen transcripts, race-enabled actor/protocol tests, and executable conformance |
| `PSL-V0-007..013` WSI capture and reuse | not-started | clean/repeat/eviction equality corpus plus race and singleflight tests |
| `PSL-V0-014..017` overlays | not-started | multi-client version/save/close adversarial corpus |
| `PSL-V0-018..022` security and lifecycle | experimental/partial (P0-A) | bounds and blocked-output SIGTERM pass; full network/process spy, repository nonmutation, and million-schedule qualification remain `NOT_RUN` |
| `PSL-V0-023..027` claims and rollout | not-started | Corvint-first then Beamfall-second registered dogfood receipts |

## Unresolved decisions

- Whether any future observer profile qualifies an OS atomic snapshot/freeze or remains cooperative
  and observation-window-only.
- Whether successful P0 dogfood justifies a filesystem watcher, local socket transport, or persistent
  content cache. None is implied by this contract.
