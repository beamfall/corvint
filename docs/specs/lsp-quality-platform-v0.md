# LSP quality platform V0

- Owner: Russell Lewis
- Date: 2026-09-29
- Intent status: accepted technical contract (owner approval 2026-09-29; PR 360)
- Delivery status: experimental Go overlay editor definition; no supported profile
- Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/deployment-neutral-index-platform-v0.md`, `docs/specs/task-context-packet-v0.md`, `docs/specs/external-evidence-provider-v0.md`, `docs/specs/vscode-extension-v0.md`, and the owner's V1-0477 request

## Agent digest
- Claim: Shared evidence engine serves opt-in agent LSP enrichment and optional editor server with separate Git and overlay identities.
- Status: accepted technical contract (owner approval 2026-09-29; PR 360)/experimental Go overlay editor definition; no supported profile
- Exists: opt-in Go/gopls definition and reference evidence through CLI and MCP, qualified only on synthetic committed Go module and go.work fixtures.
- Blocked on: accepted exact profile tuples and numerical floors, real-workspace baselines, independent client interoperability and outcome qualification.
- Read next: User job, profile matrix, requirements, qualification.

## User job and scope

An agent investigating a repository should get relevant type-resolved relationships alongside Corvint's governing requirements, impact, tests, change evidence and explicit gaps. An engineer in an editor should get timely navigation and diagnostics tied to the open document, plus inspectable Corvint evidence. Each consumer must be able to distinguish a committed fact from an unsaved observation and know which requested capabilities were unavailable.

The accepted direction has two consumers. The technical profiles in this document are a proposal. Agent reads remain terminating CLI/MCP operations. A separately invoked native-Go stdio companion may be owned by an editor session; shutdown, EOF or session cancellation ends that session and retires its descendants. Cancelling one request stops only its affected work and keeps a healthy session available. It adds no required daemon, account, network service, mutable authority store or Core dependency. Existing deferred VS Code extension work remains under its own contract. The companion is an additional optional profile, not an implicit reopening of that extension.

## Verified current state and simpler baseline

At `cf93522e149c754e002426e422270f24c88777fe`, `internal/lspprovider/provider.go` and `internal/lspevidence/context.go` provide explicit Go/gopls definition and reference path relations for `corvint context --lsp gopls` and the `task-review-lsp` MCP profile. `docs/LSP.md` documents activation, limits and rollback. Project authority and default Core results remain separate. There is no Corvint editor language server, overlay identity or general language adapter registry.

The simpler baseline is the existing Go/gopls opt-in integration plus ordinary Corvint CLI/MCP and the editor's own upstream language server. The retained `script/qualify-lsp.py` run reported five semantic edges on a committed module and six on go.work, exact CLI/MCP packet parity, and dirty-file omission with Go 1.27.1 and gopls 0.23.0. The two single-sample CLI off/on observations were 0.4875/0.4897 s and 0.1194/0.4213 s; packet sizes were 3,681/12,781 and 4,868/15,506 bytes. These are synthetic transport observations, not latency distributions or agent/editor outcome evidence. The complete raw report is retained in the V1-0476 task evidence; see `docs/build-log/2026-09-29-go-lsp-integration.md`. Real-repository, upstream-only, and editor qualification baselines remain `NOT_RUN`. A single exploratory probe on Corvint itself, recorded in `docs/build-log/2026-09-29-lsp-quality-contract.md`, is useful for sizing and failure discovery. The public, repeatable three-arm Go navigation corpus in `benchmarks/lsp-quality/public-v0.json` and `script/measure-lsp-quality.py` now records the first development baseline: one of two expected relations was missed in all three repetitions despite direct gopls resolving both definitions; see `docs/build-log/2026-09-29-lsp-public-baseline.md`. It does not freeze a held-out distribution, editor gate or outcome claim.

## Profile and capability matrix

The support unit is `(consumer, Corvint build, OS/architecture, language, server name/version and executable digest, editor/client version where applicable, workspace layout, position encoding, requested capability set)`. A profile is supported only for a tested tuple. `FALLBACK` and `UNSUPPORTED` are explicit values, not aliases for success.

| Capability | Agent consumer | Editor companion proposal | Current evidence |
|---|---|---|---|
| Definitions, references, types, call/type hierarchy | Type-resolved paths and positions with provenance; only relevant queries run | Delegate where negotiated; enrich with Corvint evidence | Go definitions/references only on synthetic fixtures |
| Hover, signatures, completion, symbols | Bounded contextual evidence when it helps a named task | Standard methods only after per-method interoperability | `NOT_RUN` |
| Diagnostics, semantic tokens, inlay hints | Explain observed diagnostics as external evidence | Publish only current-version results; delegate qualified methods | `NOT_RUN` |
| Rename, refactoring, formatting, code actions | Proposals with explicit source and risk; no execution | Version-checked, client-applied edits only after qualification | `NOT_RUN` |
| Requirements, impact, test and change evidence | Corvint facts and explicit exclusions attached to the task | Namespaced request or editor command with inspectable receipt | Existing Core commands; editor projection `NOT_RUN` |

This matrix lists candidate behavior, not advertised server capabilities. Initialize must advertise only methods qualified for the exact profile. There is no promise of every language, framework, method or use case.

## Requirements

- `LQP-V0-001`: Each consumer MUST select an explicit, versioned profile and report the exact support tuple, requested methods, negotiated methods, provider identities, unavailable methods and reasons. An unknown tuple MUST be `FALLBACK` or `UNSUPPORTED`; it MUST NOT inherit a broader language-level claim.
- `LQP-V0-002`: Agent CLI and MCP MUST remain off by default for upstream semantic work and keep default Core packet bytes and ranking unchanged unless a separately accepted contract authorizes a change. Editor startup MUST be explicit and optional.
- `LQP-V0-003`: The shared engine MUST distinguish immutable Git identity `(repository/root, object format, commit, tree, blob, path)` from live overlay identity `(session, canonical URI, document version, content digest)`. An overlay MUST NOT be represented as committed evidence, and a Git receipt MUST NOT assert the contents of an unsaved buffer.
- `LQP-V0-004`: The engine MUST reject or explicitly mark stale results after edits, saves, close/reopen, rename, branch or root changes. Out-of-order responses MUST NOT replace a newer document version; unknown version order MUST withhold the affected result.
- `LQP-V0-005`: Every emitted fact MUST carry its source, snapshot identity, inclusion reason and authority class. Project-owned instructions/specs outrank upstream semantics, history and inference. Missing, conflicting or inaccessible evidence MUST produce uncertainty or abstention.
- `LQP-V0-006`: A semantic join MUST deduplicate witnessed identities, preserve conflicting claims, and apply declared per-request query, time, memory and output bounds. It MUST NOT infer cross-language or framework links from coincident names alone.
- `LQP-V0-007`: Upstream adapters MUST negotiate each method and position encoding, validate URIs and ranges, pin server/config identity, bound input/output, and report degraded or partial support. They MUST NOT silently install, download or execute a server absent explicit operator configuration.
- `LQP-V0-008`: Adapter failures, cancellation, deadline, repository drift and partial answers MUST be visible at the affected capability boundary, with machine-local paths and secrets redacted. An all-query failure MUST NOT be reported as semantic success.
- `LQP-V0-009`: Every invoked upstream process MUST have an owned lifecycle; session interruption, session deadline, editor shutdown, EOF and crash recovery MUST retire descendants and private cache state. Request cancellation MUST stop the affected work without ending a healthy editor session. Persistent editor sessions require separate resource and liveness bounds.
- `LQP-V0-010`: Agent requests MUST select only task-relevant methods and return bounded facts, provenance, omission counts and stopping reasons through both CLI and MCP. A read MUST NOT launch tests, model calls, learning or edits.
- `LQP-V0-011`: The optional native-Go stdio editor companion MUST negotiate initialize/shutdown, text synchronization, cancellation, workspace roots and position encoding for each qualified client tuple. It MUST use the client's open-document text for overlays and MUST NOT read a file URI as a substitute for that text.
- `LQP-V0-012`: The editor companion MUST advertise only qualified standard methods. Corvint-specific receipts MUST use documented namespaced requests or commands; unsupported standard methods MUST remain unadvertised. Diagnostics MUST be withdrawn or refreshed when their version/root becomes invalid.
- `LQP-V0-013`: Rename, formatting, refactoring and code actions MUST be proposals applied by the client only against the version they were computed for. No LSP request authorizes repository writes, shell commands or test execution.
- `LQP-V0-014`: For a shared admitted snapshot and request, CLI/MCP and editor projections MUST derive from the same evidence facts; transport-specific rendering MUST preserve identity, authority, uncertainty and exclusions. An editor overlay MUST never be used to satisfy a committed-only agent claim.
- `LQP-V0-015`: Qualification MUST compare upstream-only, Corvint-only and combined arms on frozen public fixtures and a separately protected held-out set. Gold labels, supported tuples, scoring, quantitative floors, resource ceilings and rollback triggers MUST be frozen before a profile can be promoted.
- `LQP-V0-016`: A profile MUST report exact correctness, coverage, freshness, cold/warm latency distribution, memory/CPU, query/output bytes and task outcomes for its declared use cases. Missing measurements MUST be `NOT_RUN` or `NOT_OBSERVED`, never silently extrapolated from synthetic parity.
- `LQP-V0-017`: Supported editor tuples MUST pass real VS Code and one independent LSP client against the same conformance corpus, including Unicode positions, multi-root isolation, rapid edits, stale responses, server crash and shutdown cleanup. Unrun clients MUST remain unqualified.
- `LQP-V0-018`: Rollback MUST preserve default Core behavior and offer explicit upstream-off and editor-companion disable paths. Promotion claims MUST name the exact qualified tuples and limits; regression below a frozen floor reverts that tuple to experimental or fallback.

## Trust, limits and failure choices

Git-pinned evidence keeps the existing immutable Core/index contract; live overlays are a separate session view with no authority to alter it. Upstream servers are operator-selected local executables, not trusted project instructions. Their output is data subject to validation and redaction. Project policy continues to govern evidence inclusion. No source edit or command is executed from a provider suggestion.

The existing Go CLI provider keeps its current 3-seed, 2-hop, 64-query, 32-relation, 64 KiB record and 15/20 s query/server bounds until a new accepted profile replaces them. Editor session and other-language bounds must be frozen and tested before implementation; no inherited limit silently covers a resident process. Unsupported, invalid, stale and unsafe responses fail closed for the affected fact or method while unrelated Core evidence remains available. A provider crash cannot promote partial semantic output.

## Qualification and acceptance matrix

| Requirement group | Deterministic evidence | External or outcome evidence | Current state |
|---|---|---|---|
| 001–002, 007–010 | Profile negotiation, default bytes, identity/redaction, hostile bounds and descendant-cleanup tests | Live pinned provider in a real module/workspace | Go synthetic slice only; V1-0167 and V1-0168 open |
| 003–006, 014 | Immutable/overlay identity, dedupe/conflict, stale-order and authority conformance | Real dirty/unsaved/branch/root observations | `NOT_RUN` |
| 011–013, 017 | Protocol transcript and editor-client interoperability, Unicode/rapid-edit/multi-root/cancel tests | Installed VS Code and second independent client tuples | `NOT_RUN` |
| 015–016, 018 | Frozen corpus manifest, exact scoring, rollback and support table | Three-arm held-out task/outcome and cold/warm resource trial | `NOT_RUN` |

For a promotion candidate, the fixture manifest and gold labels must be sealed before candidate execution. Required hard floors are zero invented committed facts, zero stale overlay promotion, zero unauthorized writes/executes, zero unredacted private paths or secrets in diagnostics, and complete retirement of owned descendants under interruption. Numerical precision/recall, p50/p95, memory/CPU, packet/query cost and task-success floors require a recorded real-workspace baseline and owner acceptance before implementation of each profile; no synthetic-only threshold can substitute. A capability is removed from advertisement when its own gate fails even if other methods pass.

## Rollout, drift and traceability

Build in order: contract/baseline; upstream adapter profiles; shared snapshot/evidence engine; editor companion; richer agent projection; profile qualification. Adapters and the shared engine may be developed in parallel after the contract. The editor and agent integrations each require those two inputs. Reuse V1-0167 and V1-0168 for known redaction and workspace-loading gaps rather than closing them by assertion.

For rollback, select `--lsp off` and the existing MCP `task-review` profile, or stop/disable the optional editor companion. A profile/version/capability change invalidates its qualification; retain old receipts and revise this spec and the support table before advertising the new tuple. No persistent evidence migration is authorized.

| Requirement IDs | Planned implementation owner | Evidence status |
|---|---|---|
| 001–002, 010, 014 | CLI/MCP profile and shared evidence projection | Go-specific prior slice only; platform `NOT_RUN` |
| 003–006 | Shared snapshot/evidence engine | `NOT_RUN` |
| 007–009 | Upstream adapter profiles | Go-specific prior slice; broader profiles `NOT_RUN` |
| 011–013, 017 | Optional Go stdio editor companion | `NOT_RUN` |
| 015–016, 018 | `script/measure-lsp-quality.py`, public corpus and future held-out qualification | Public development baseline implemented; protected held-out and promotion `NOT_RUN` |

## Open decisions and promotion boundary

The first supported non-Go server/language, exact LSP protocol baseline and client versions, overlay storage lifetime, Corvint extension wire, editor session budgets, real-workspace corpus, held-out custody and numerical outcome floors were open in the initial proposal. The accepted experimental policy below selects evaluation targets and consumer versions for its exact tuple; decisions outside that tuple remain open. TypeScript and Python are candidates, not accepted order. The [official LSP overview](https://microsoft.github.io/language-server-protocol/) and exact specification for the chosen version govern standard wire behavior; this draft does not infer conformance from method names.

The owner accepted this technical contract on 2026-09-29 (merged PR 360, commit 624e9aecbf71993d26c57b2e20837282469cb02c). That historical acceptance did not select numerical floors; the experimental evaluation targets were subsequently accepted as recorded below. Exact profile tuples remain unqualified; no “world class” or general-use claim is justified until the exact tuple's full matrix passes and outcome evidence shows a material benefit over both baselines. If combined results do not improve an important declared job, retain the simpler upstream-only/Core-only path and do not promote that profile.

## Accepted experimental evaluation targets (2026-09-30)

The owner accepted the experimental policy and full measurement/scoring/custody protocol in [lsp-qualification-evaluation-policy-v0.json](lsp-qualification-evaluation-policy-v0.json). Its SHA256 is `0bd7797cbefa843902e7e2448c9bf36773e4fb892ef977e79087ada85ac607af`; the accepted original proposal SHA256 is `bb5f077c9197776109301f924c6a61d40216dcebbcac02c9e1b9d9b5c2391f1b`, acceptance evidence SHA256 `54ceaf096e1aad987038e9337d9f8e031816acd1345704046419a2561246f8ba`. Human answer: “Accept these evaluation targets and protocol”, 2026-09-30T14:04:10.099139+00:00. Acceptance is policy, not measured qualification or promotion.

`LQP-V0-015`/`LQP-V0-016` evaluation targets are precision 0.95, recall 0.90, combined semantic/governance noninferiority and at least one joint-task benefit. Cold agent/editor P95 ceilings are 30s/20s; warm definition/context 250ms/2s; per-request owned CPU 60s, observed owned-group peak RSS 2GiB, agent packet 256KiB, external record 64KiB, editor message 1MiB, queries64 and relations32. Existing zero-unsafe-fact/write/leak/stale/survivor floors remain. Twenty cold and100 warm operations, five fixed warmups, 100ms RSS sampling, three complete repositories including go.work and twenty protected tasks follow the exact linked protocol. All failures, unsupported required cases, scheduling misses and abstentions keep their denominators; peak sampling and unauthenticated shared-host custody limitations remain explicit. No floors or corpus can change after execution to conceal failures.

The optional whole-root observation guard is a partial experimental freshness improvement only. Its workspace-only marker expressly leaves external SDK/module/cache inputs unbound; it cannot close full `LQP-V0-004` provider input freshness or promote definition. Complete go.work layouts remain required, not silently excluded.

## Experimental public gold scoring amendment

For `LQP-V0-015`/`LQP-V0-016`, the public definition witness freezes query token and target identifier spans against immutable `sourceBase` Git bytes before executing a candidate. Upstream semantic gold requires the repository-local file URI and exact integral line/column/byte-offset target span; another symbol in the same file is a failure. Legacy path-only manifests refuse exact semantic scoring. Existing historical path-level reports remain development observations; combined relation scoring remains path-level and cannot establish exact-symbol correctness. This amendment accepts no profile or quantitative floor.
