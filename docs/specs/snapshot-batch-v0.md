# Snapshot Batch V0

Owner: Russell Lewis
Date: 2026-09-04
Requirement prefix: `SBQ-V0`
Intent status: accepted (decision 0052)
Delivery status: experimental
Revision status: revision 10 (round 9 Claude + Codex applied)
Authoritative inputs: `docs/specs/index-snapshot-v0.md` (snapshot identity and the one writing
verb), `docs/specs/go-production-kernel-migration-v0.md` (`GPK-V0-040` coverage denomination, query
and impact receipts), `docs/specs/task-context-packet-v0.md` (`context` packet),
`docs/specs/deployment-neutral-index-platform-v0.md` (`DNIP-IDX-006/007` bounded reader),
`AGENTS.md` invariants 4 and 7, roadmap ticket AT-03.

## Agent digest
- Claim: `corvint batch` answers up to 16 `query`, `context` and path `impact` requests from one loaded snapshot with standalone receipts and per-operation errors.
- Status: accepted (decision 0052)/experimental
- Exists: nothing before this slice; `cmd/corvint/batch.go` and its test are the implementation.
- Blocked on: parity tests against the standalone verbs; measured three-to-five-read latency/RSS against separate calls; adoption on the next eligible task (AT-05/06); possession/delta clauses `SBQ-V0-007..010`, accepted for AT-07, awaiting verified AT-06 delivery.
- Read next: Requirements; Non-goals and authority; Failure modes.

## Intent and scope

An investigation of one change needs three to five evidence views: the task query, the task context
packet for the changed file, and the impact of one or two paths. Today each is a separate process:
`context` and repository `query` load the snapshot, path `impact` rebuilds the index from Git every
call (`cmd/corvint/main.go` `contextindex.Build`), and the repeated load and build changes no
retrieved meaning. This slice adds one read-only verb that loads the snapshot once and runs a
bounded operation list against it. Affected user: an agent orienting on one task. Measurable job:
the standalone verbs' own receipts at one snapshot with fewer process launches and index loads.

## Requirements

- **SBQ-V0-001:** `corvint [--root PATH] batch` MUST read one canonical JSON request object from
  stdin through the kernel's bounded reader (`gokernel.MaxInputBytes`, 128 KiB) with exactly the
  member `operations`: a non-empty list of at most 16 objects. Each operation has `id` (a distinct
  non-empty string of at most 64 bytes), `verb` (`query`, `context` or `impact`) and that verb's
  arguments: `query` takes `task` and optional `limit`, `budget_bytes`; `context` takes `task`,
  optional `subject`, `limit`; `impact` takes `paths` and optional `limit`. Any other member, verb,
  duplicate id, or oversize request MUST be refused before any repository read with
  `invalid-arguments` (a `gokernel.Error`, as `argumentError` renders) and exit status 2. Accepted amendment (AT-07, decision 0052): a later amendment adds an optional `possessed` member to each
  operation here, never at the batch level. End of accepted amendment.
- **SBQ-V0-002:** The verb MUST load the current snapshot exactly once through
  `contextindex.LoadSnapshot`'s full `decodeSnapshot` variant (`snapshot.go:283-290`), never a
  `Tracked`-less one (`eventSnapshot`, `:33-49`; the forced re-decode at `:306`), which
  `SBQ-V0-008` cannot verdict against (`IDX-SNAP-V0-002`, `IDX-SNAP-V0-004` for a dirty worktree).
  When no snapshot matches the tree and binary it MUST refuse with `unsupported-batch-snapshot` (a
  `gokernel.Error` naming `corvint index`), exit status 2, build nothing, and write no unsupported
  observation. A deliberate divergence from `IDX-SNAP-V0-003`, where a miss is never an error: a
  batch answers at one identity; a caller wanting build fallback calls the standalone verbs.
- **SBQ-V0-003:** Each operation MUST run the standalone verb's own code path over the loaded
  index, so its receipt is byte-equal to that verb's `context` member for the same arguments at
  that snapshot: `query` dispatches on `contextindex.QueryIntent(task)` exactly as
  `standaloneQueryContext` does (`project-operations` through `QueryAuthorityStartBudget`, the
  repository and agent-tooling intents through the repository path, whose read-only
  `tracerecordrepo.Read` therefore runs once per query operation); `context` through
  `contextindex.TaskContextWeighted` under the same admitted slot-weight file the standalone verb
  loads (`LoadAdmittedSlotWeights`, `LTA-V0-011`; absent, it is `contextindex.TaskContext`
  exactly); `impact` through `contextindex.Impact`. Reading `impact` from the
  snapshot (the standalone verb rebuilt when this was written; standalone path `impact` now reads it
  too under `IDX-SNAP-V0-019`) is admitted experimentally and holds parity:
  `Impact` reads only tables the snapshot carries, skipping a build's re-observation retry — the
  race `IDX-SNAP-V0-004` already accepts. Operations run in request order; nothing is read ahead,
  and no result changes another's. Accepted amendment (AT-07, decision 0052): a later amendment
  scopes this byte-equality to operations that suppressed nothing. End of accepted amendment.
- **SBQ-V0-004:** Stdout MUST be one canonical JSON line with exactly the members `ok=true`,
  `mutates=false`, `tool="batch"`, `profile="snapshot-batch/0"` (which no standalone verb emits),
  `snapshot` (`tree` and `commit` from the loaded index's `Revision` and `CommitRevision`, which a
  hit sets to the live HEAD commit under `IDX-SNAP-V0-002`) and
  `operations`: one object per request operation, in order, carrying `id`, `verb` and either
  `ok=true` with `context` (the standalone receipt) or `ok=false` with `error` (`message`, plus
  `code` only when the standalone error carries one; codeless `contextindex.Error` values omit it,
  as `emitError` does; a refusal that carries a `DRC-V0-006` diagnostic adds the same bounded
  `subject`, `evidence`, `supported_fixes` and, when set, `terminal` members, as `emitError` does,
  and `unsupported-impact-path-suffix` is the one such row today). A failing operation MUST NOT
  stop later ones. Exit status is 0 when the
  batch executed, whatever the per-operation outcomes. A later amendment adds an optional `delta`
  member here at the batch level, never inside an operation object; it is absent when no operation
  supplied `possessed`.
- **SBQ-V0-005:** The verb MUST be read-only: no index, trace, cache or ledger write on any path.
  Parity is receipt-level only: standalone `impact` appends an unsupported observation on failure
  (`observeUnsupported`); batch deliberately never does, on any path, including `SBQ-V0-002`'s
  `unsupported-` prefixed refusal (invariant 4).
- **SBQ-V0-006:** Work is bounded by `SBQ-V0-001`'s request bounds and each verb's own (`limit`
  1–50, `impact` at most 100 paths, the query byte budget). The verb MUST spawn no process of its
  own: the only children are the Git observations `LoadSnapshot` makes once, plus whatever each
  operation's standalone path already spawns for the same arguments (the repository-intent query's
  trace read and history learning, `context`'s cochange `git log`); batch adds none and removes the
  per-verb index builds. It MUST open no network connection and MUST NOT hold the snapshot after
  the response is written.
- **SBQ-V0-007:** (AT-07 accepted contract; delivery requires verification) (a) An operation MAY carry `possessed`, which
  amends `SBQ-V0-001`'s closed per-operation member set to admit it as `SBQ-V0-004` admits the
  batch-level `delta`: at most 32 objects, each `{"path", "blob_hash"}` (non-empty strings) plus an
  optional non-negative integer `line`, validated and ignored in V0: no clause reads it, so a
  stored `line` and one dropped after validation are indistinguishable on the wire. (b) A
  `possessed` that is not an array, exceeds 32 entries, has an entry missing or non-string `path`
  or `blob_hash`, or repeats a `(path, blob_hash)` pair
  MUST be refused with `invalid-arguments` before any repository read, like `SBQ-V0-001`'s outer
  request; one `path` under several hashes is legal. (c) The binding admission limit is the byte
  bound `gokernel.MaxInputBytes` (131072, `internal/gokernel/harness.go:24`), enforced before
  parsing: `path` and `blob_hash` are unbounded, so the 32-entry cap bounds no byte total and is
  secondary — 32 entries
  at ~4,200-byte paths exceed 128 KiB and the byte bound refuses them first.
- **SBQ-V0-008:** (AT-07 accepted contract; delivery requires verification) At the one loaded snapshot (`SBQ-V0-002`) every
  entry gets exactly one verdict, matched by literal repository-relative path against its `Index`
  tables: no normalization, case folding, symlink resolution, `..` cleanup or Git process. No tree
  holds both a blob `p` and an entry under `p/`, so file and directory readings never collide; a
  symlink, when it is an admitted source (`admittedEntries`, `index.go:505-518`), is a blob
  verdicted on its link text's hash, not its target; an unmatched literal,
  absolute or `../` escaped, falls to (f). Precedence: (a) `unframable`: no blob hash at this tree
  — a gitlink or other non-blob entry `readTreeEntries` skips (`git.go:319`), never in `Tracked`
  (`index.go:487-490`); or a directory, a proper slash-terminated prefix of a `Tracked` or of a
  skipped path. `ls-tree -r` (`git.go:300`) emits no tree entries, so a submodule-only parent has
  no `Tracked` descendant and would else verdict `absent`; `Index` MUST record skipped entry paths
  at build time, making them and their ancestors unframable with no Git process (`SBQ-V0-006`). (b)
  `unsupported`: in `Tracked`, absent from `Sources` — generated (`index.go:479-483`), or dropped
  by `admittedEntries` (`index.go:506-517`) as forbidden, oversize, or carrying no admitted
  suffix, that last drop (`:511-513`) recording no `Exclusion` at all.
  (c) `dirty`: in `DirtyPaths` (`index.go:466-467,477,486`, from `git status --porcelain=v1 -z
  --untracked-files=all`, `git.go:242`, so untracked and worktree-deleted too), whatever the hash.
  (d) `stale`: in `Sources`, hash unequal. (e) `retained`: hash equal. (f) `absent`: not in
  `Tracked` and not `unframable` — absence from this tree, not a claim about history; a worktree
  deletion is `dirty` under (c). (g) A verdict is a snapshot fact, never a claim the model read the
  path (`PCCO-V0-009`). (h) An operation whose loaded `Index` carries a nil `Tracked` or nil
  skipped-entry set MUST refuse in `SBQ-V0-004`'s per-operation shape — that operation's
  `ok=false` with `code: unsupported-possession-tables` — and verdict nothing; the batch still
  executes its other operations and exits 0 (`SBQ-V0-004`), never a batch-level `gokernel.Error`.
  `SBQ-V0-002`'s variant always fills `Tracked`, so this refusal is unreachable there and stands
  only to fail closed if that variant changes: a table-less index would silence (b) and send every
  path to (f).
- **SBQ-V0-009:** (AT-07 accepted contract; delivery requires verification) Suppression runs in `cmd/corvint/batch.go` on
  the receipt `runBatchOperation` returned, so no verb's code path changes (`SBQ-V0-003`). (a)
  Suppression and invalidation matching are separate: a row is suppression-matched when `path` and
  `blob_hash` both equal some entry's; a row whose `path` equals an entry's but whose hash differs
  is invalidation-matched, takes its `SBQ-V0-008` verdict, and stays in the packet. Note: verdict
  precedence across hashes for one path is unreachable under `SBQ-V0-008`, whose (a)-(c) verdicts
  read the path alone. (b) Where a path is possessed under several hashes, a row equal
  to any one is suppression-matched; otherwise the path invalidates once and `delta.invalidated` is
  deduplicated by `(kind, id, path)`. (c) A row with an empty `blob_hash` (an unguarded
  `Index.Sources` miss: `taskcontext.go:1424,1427` or `eval_dependency.go:355,358`) is neither
  suppression- nor invalidation-matched, supplied hashes being non-empty.
  (d) A result is suppressible only with at least one
  evidence row, every row suppression-matched. (e) A result with any critical selector for its verb
  (`SBQ-V0-010`) MUST NOT be suppressed. (f) A suppressed result leaves `results` for
  `delta.suppressed` as `{"kind", "id", "rows", "reason": "possessed-retained", "profile":
  "snapshot-batch/0", "operation": "<this operation's id>", "recover": "resend the operation
  without `possessed`"}`, `rows` being a positive integer count of that result's evidence rows.
  (g) That entry is NOT a `PCCO-V0-010` omission entry: a caller-supplied operation id is not trial
  evidence identity (`proof-carrying-context-optimization-v0.md:98`). (h) A result carrying any
  `stale` row is emitted in full and listed in `delta.invalidated` as
  `{"kind", "id", "path", "verdict"}` — the one `delta` list keyed without `operation` — with
  one entry per (result, invalidation-matched path), deduplicated by `(kind, id, path)` per (b). A
  `dirty` or `unsupported` row never reaches this clause: any `dirty` verdict requires a non-empty
  `Index.DirtyPaths`, the batch-wide `"mixed-worktree"` fallback predicate (`SBQ-V0-010`(d)), so
  every possessing operation carrying one already fell back before invalidation matching runs; an
  `unsupported` row matching a result is by definition `"unsupported-possession"`, also a fallback.
  An `absent` or `unframable` entry never matches a row, every row being
  built from an indexed path, and is echoed under `delta.ignored` (`SBQ-V0-010`(g)) instead.
- **SBQ-V0-010:** (AT-07 accepted contract; delivery requires verification) (a) Semantic key and wire shape differ per verb
  and both are fixed, so suppression MUST key `delta` on the semantic pair, reproduce that verb's
  wire shape, and add no member beyond the `coverage.suppressed_results` (b) requires — (b)'s one
  appended `uncertainty` line is a new element of an existing member, not a new member. `query`
  (`compileReceipt`, `internal/contextindex/query.go:93` authority-start,
  `internal/contextindex/eval_query.go:376` repository) carries
  `freshness.state` (`receipt.go:65-66`),
  `coverage.{budget_bytes,packet_bytes,within_budget,uncertainty}` (`:490-494`) and `"kind:id"`
  critical selectors over `(kind, id)` (`selector`, `:890`). `impact` (`impact.go:70,233`) carries
  those members with `budget_bytes` JSON `null`, not `0` (`var budgetValue any` unset,
  `receipt.go:486-488`), `within_budget` structurally `true` (`receipt.go:494,503`), and every
  `kind: "path"` result critical (`receipt.go:514-521`), again strings. `context`
  (`cmd/corvint/batch.go:291-292`; packet
  `taskcontext.go:1445-1457`)
  carries none of `freshness`, `packet_bytes`, `within_budget` or `uncertainty`, and keys critical
  selectors as `{"relation", "path"}` objects (`taskcontext.go:1332`). (b) All coverage counts
  freeze at their pre-suppression values — `requested_results`, `included_results`
  (`len(results)`) and `omitted_results` (`receipt.go:491`) as much as `authoritative_results`,
  `advisory_results` (`:492`), `critical` and `critical_missing` (`:493`) — and so does the
  `uncertainty` omission line, which derives from that frozen `omitted` (`:453`, `:467-475`) and
  recomputed would read possession as ranking loss. Suppression instead APPENDS to `uncertainty`
  exactly one line reading `N results suppressed by caller possession` for the integer N:
  additive, never a recomputation of the frozen omission line. `context` freezes its own
  vocabulary alike, the whole of it: `candidates`, `included_results`, `omitted_results`,
  `governance`, `critical`, `critical_missing`, `unexamined` and `budget_shortage`
  (`taskcontext.go:1449-1454`); the counts that are `query`/`impact` members only are
  `requested_results`, `authoritative_results` and `advisory_results`. `state` freezes too, on
  every verb: this amends `TCP-V0-006` and `GPK-V0-040` (the accepted amendments in
  `task-context-packet-v0.md` and `go-production-kernel-migration-v0.md` accepted by decision 0052) to compute `state` before suppression and hold it there, so a fully suppressed
  `context` packet keeps its pre-suppression `state` (`READY`, never forced to `NO_CANDIDATES`)
  beside its frozen `included_results`, and a fully suppressed `query` or `impact` receipt keeps
  its pre-suppression `state` rather than falling to `OUT_OF_SCOPE` (`receipt.go:40-47`).
  `coverage.suppressed_results`, a positive integer, alone carries
  the count difference and MUST appear on any packet that suppressed anything; it is absent, not
  `0`, on an operation that suppressed nothing or took a (d) fallback, so such an operation's
  receipt stays byte-equal to the standalone verb's (`SBQ-V0-003`). Under the same amendment,
  `context`'s coverage gains an `uncertainty` member (absent on a `context` packet today,
  `taskcontext.go:1445-1457`) carrying only the one appended suppression line above, on the same
  terms as `query` and `impact`. `packet_bytes` and
  `within_budget` recompute per (c). On `query` and `impact` (the TCP packet carries neither
  member), the top-level `verification` and `exclusions` members freeze
  alongside `coverage` at their pre-suppression values (`receipt.go:69-70`, rebuilt per trial at
  `:140`), with `verification`'s 20-path cap (`receipt.go:687-706`) recomputed over the
  surviving results' evidence paths alone and any suppressed path's contribution appended after
  them, so a retained result's own path is never displaced from the cap by a suppressed one.
  (c) `query` and `impact` MUST re-run the fixed-point byte
  stabilization over each operation receipt after suppression; `query` recomputes `within_budget`
  in `setCoverage`'s own order — stabilize, encode, compare `len(encoded)` to `budget_bytes`,
  stabilize again (`receipt.go:496-504`) — and reports it truthfully. The budget is measured over
  the operation receipt alone (`stabilizePacketBytes`, `receipt.go:668`); the batch-level `delta`
  bytes never enter it. Pre-suppression `within_budget` is structurally `true` on every compiled
  receipt — `compileReceipt` returns only where `len(encoded) <= *budget` (`receipt.go:191-192`)
  and an unset budget sets the flag `true` unconditionally (`:503`) — and the re-run MUST report
  it truthfully: after suppression, `within_budget == (packet_bytes <= budget_bytes)`, computed
  over the final canonical bytes of the operation receipt alone, whatever value that comparison
  yields — the flag is not assumed to stay `true` because suppression happened to remove bytes.
  `impact`
  keeps its structural `true` (`budget_bytes` is always `null` there, `SBQ-V0-010`(a)), `context`
  carries no `within_budget` member at all.
  `contextindex` MUST export that stabilization (`stabilizePacketBytes`, `receipt.go:668`) as a
  wrapper returning a `contextindex.Error` when `coverage` is absent or mistyped, never the
  function itself, whose first statement is an unchecked `result["coverage"].(map[string]any)`
  (`:668-669`). Suppression is forbidden where `request` carries `omitted_by_budget` (`:206-208`):
  `compileReceipt` (`:100,120,127-150`) already replaced `request` with `{"budget_bytes",
  "omitted_by_budget", "request_sha256"}` and compacted the coverage lists, which re-running
  `stabilizePacketBytes` alone leaves as stubs; that operation takes (d)'s `"budget-compacted"`
  fallback, its entries echoed under `delta.ignored` (g). (d) Fallback (`PCCO-V0-007`,
  whole-packet): for each operation that supplied `possessed`, suppression is disabled for that
  operation, with one `{"operation", "reason"}` entry in `delta.fallback`, when (c)'s budget
  compaction holds (`"budget-compacted"`), an entry that verdicts `unsupported` whose `path`
  appears as the `path` of an evidence row of at least one result of that operation
  triggers `"unsupported-possession"`, or the worktree is mixed
  (`"mixed-worktree"`) — the last a batch-wide condition that still yields an entry per possessing
  operation and none for an operation that supplied no `possessed`; an `unsupported`
  entry whose `path` appears as the `path` of no evidence row of any result of that operation
  triggers no fallback and is echoed
  under `delta.ignored` (g) alone. The mixed-worktree fallback holds when the loaded index has any
  dirty path: `len(Index.DirtyPaths) != 0`, the same test `receipt.go:49-51` runs to set
  `freshness.state` to `mixed-worktree` for `query` and `impact`; `"unsupported-possession"`
  wins when both hold, and it wins over `"budget-compacted"` the same way when both hold, matching
  `possessionFallback`'s check order (`batch.go:212-230`). Every fallback operation, whatever its reason, echoes all of its
  `possessed` entries
  under `delta.ignored`. (e) The verb stays stateless: no possession state crosses requests. (f) The
  batch-level `delta` (`SBQ-V0-004`) carries exactly one `delta.binding`, and determinism and
  replay hold over that binding, a quintuple, not over request bytes: a repository query reads a
  ledger outside the snapshot (`tracerecordrepo.Read`, `cmd/corvint/harness_context.go:63`) and
  the loader reattaches live dirty state. `delta.binding` MUST carry `tree`, `commit`, `engine`
  (the id `loadSnapshot` computes to name the snapshot file it opens, `snapshot.go:256,281`, a
  digest of the running binary taken with no child process, `snapshot.go:93-105`; two binaries that
  both wrote a snapshot at one tree otherwise share every other member yet load different indexes),
  `status_sha256` (`Index.StatusSHA256`, the digest of the raw `git status
  --porcelain=v1 -z --untracked-files=all` bytes, `git.go:242,250`) and `trace_digests`: one
  `{"operation", "state", "digest"}` per ledger-reading operation, in order, `digest` being
  `canonicalSHA256` (`receipt.go:600`) over `{"state", "records"}`, `records` a JSON array of the
  `tracerecordrepo.Read` result's `trace.Record` values, sorted by `TraceID` (the record schema's
  own unique key, `internal/trace/record.go:31-39`), each projected to exactly
  `{schema_version, revision, trace_id, task, opened_paths, changed_paths, verification, outcome}`
  in that member order, a `nil` slice field normalized to an absent member rather than JSON `null`;
  an operation reading none (a
  `project-operations` query, `harness_context.go:54-55`; `context`; `impact`) contributes no
  entry. `status_sha256` suffices as the dirty term only because the batch never builds
  (`SBQ-V0-002`): a build's `DirtyPaths` adds pin-diverged paths (`index.go:476-477,486`), a load
  takes live status alone (`snapshot.go:316`). `delta.binding` is always present; each of
  `delta.suppressed`, `delta.invalidated`, `delta.ignored` and `delta.fallback` is absent, not an
  empty array, when it holds nothing. Every `delta` list except `delta.invalidated` is ordered by
  operation order, then by result order within the operation (`delta.ignored`: by that
  operation's `possessed` entry order). `delta.invalidated`, the one list keyed without
  `operation`, is exempt from operation order entirely and sorts by `(kind, id, path)` alone, so
  a duplicate semantic entry produced by two operations dedupes to one, wherever in operation
  order it first appears. The acceptance fixture's two runs, over one bound quintuple and one
  equal request, are byte-equal receipts; this is not a general replay guarantee, and two named
  inputs stay unbound across an otherwise-equal replay: a repository query's live history read
  (`eval_query.go:1096-1103`; `taskcontext.go:707-715`, both `git log ... HEAD`, not the bound
  `tree`) and the snapshot payload itself, which carries no digest a load can compare
  (`snapshot.go:118-119,160-168,283-290`). A differing binding is no counterexample to the
  fixture, and the gate MUST compare binding before
  bytes and fail naming the differing member. (g) A forged or stale hash for a tracked, non-dirty
  path yields `stale`, never an error; in a non-fallback operation, an entry whose `path` appears
  as the `path` of no evidence row of any result is echoed under `delta.ignored` as
  `{"operation", "path", "blob_hash"}` — the same
  shape (d) echoes every entry of a fallback operation in. (h) Rehydration is inexact: `score`,
  `summary`, `action` (`taskcontext.go:1426`) and each row's `reason`, `confidence`, `authority`
  (`impact.go:342-344`, free-form and possibly empty, so no evidence row has a guaranteed minimum
  byte size) are built at result time. (i) Promotion needs the fixtures, replay and
  savings gate the table below names. The savings gate's denominator is the sum, over every
  operation in the fixed three-operation fixture, of the canonical JSON byte length of each
  operation's baseline (unpossessed) receipt's `results[*].evidence` arrays; the numerator is the
  same sum restricted to the results the fully-possessed (treatment) run of the identical
  operations moves to `delta.suppressed`. The percentage is `100 * numerator / denominator`,
  computed in integer arithmetic truncated toward zero (floor for a non-negative ratio); the gate
  requires it at least 30.

## Non-goals and authority

No new ranking or retrieved meaning; no cross-operation caching, deduplication or delta expansion;
no range impact, `--working-tree-untracked` impact, `prove` or `affected` in V0; no build fallback
when the snapshot is missing; no persistent process, server or daemon (invariant 7); no change to
any standalone verb's receipt or code path beyond letting it accept an already-loaded index. A
batch receipt carries no more authority than the standalone receipts it contains. Admitting
`impact` to the snapshot readers in `index-snapshot-v0.md` (accepted, decision 0049) is
accepted by decision 0052. The `SBQ-V0-007..010` amendment
adds no cross-request memory or host-cache claim, no compaction inference, no access-change
fallback, no trial-backed omission claim, and no partial fallback: it disables suppression per
whole operation. No general byte-equal replay guarantee across a repository query's live history
read or an unhashed snapshot payload is a goal of `SBQ-V0-010`(f); the acceptance fixture picks
inputs that hold both fixed across its two runs instead of binding them. It also amends the
no-code-path-change non-goal: no standalone verb's receipt
changes, but shared code does, in four additive amendments — the build-time skipped-entry set on
`Index` for `SBQ-V0-008`(a), a trace state returned by `repositoryQueryContext` for
`SBQ-V0-010`(f), the exported byte-stabilization wrapper `SBQ-V0-010`(c) specifies, and an
accessor exposing the engine id the load path already computes, which `SBQ-V0-010`(f) binds on.
That load-path id, not `SnapshotReceipt.Engine`, is the source: `Engine` is write-side only, what
`WriteSnapshot` returns (`snapshot.go:66-72`); `LoadSnapshot` returns `(*Index, bool, error)` and
no engine (`snapshot.go:213-215`) though `loadSnapshot` computed one to open the file
(`snapshot.go:256,281`); and the one exported carrier, `SnapshotProbe`, is obtained by
`ProbeSnapshot`, which re-observes Git's identity (`readIdentity`, `snapshot.go:226`) even
though its own call to `engine()` returns the already-memoized digest (`sync.Once`,
`snapshot.go:85-105,219`) rather than re-reading the binary — a Git process
`SBQ-V0-006` forbids regardless.
Exposing the already-computed id is therefore the one additive accessor the extraction needs, and
adds no process. Neither
amends `IDX-SNAP` nor `GPK`: snapshots serialize the whole `Index` value less `Root`, `DirtyPaths`
and `StatusSHA256` (`encodeSnapshot`, `snapshot.go:160-162`) and are keyed by object format, tree
and running-binary digest (`snapshot.go:118-119`), so an older reader never meets derived metadata
it lacks, and GPK parity is denominated in observable bytes
(`go-production-kernel-migration-v0.md:108,111`). `coverage.suppressed_results` and the
`context`-only `coverage.uncertainty` member ARE wire changes to the packets that carry them
(`GPK-V0-002`: "a field addition ... is a wire change owned by its existing spec"), assigned by
that clause to whichever spec owns the emitting packet — `query` and `impact` to GPK, `context` to
TCP — which is exactly what the "AT-07, decision 0052" amendment blocks added to
`go-production-kernel-migration-v0.md` and `task-context-packet-v0.md` name and authorize; neither
member is added in `setCoverage` (`receipt.go:437`) itself, so no standalone receipt gains the
field outside a batch operation that supplied `possessed`.

## Failure modes

Stdin unreadable or cancelled: `invalid-harness-input` / `harness-input-cancelled` as the kernel
reader reports, exit 2. Malformed JSON, unknown member, unknown verb, duplicate or missing id, a
`null` where a scalar is required, more than 16 operations, or more than 128 KiB:
`invalid-arguments`, exit 2, nothing read from the repository (the request is parsed before the
root is resolved). A per-operation argument error the batch parser can see (empty task,
out-of-range `budget_bytes`) is that operation's `ok=false` with code `invalid-arguments`; an
out-of-range `limit` is refused by the verb itself with its codeless message. Neither aborts the
batch. Duplicate JSON members in one object are not detected (Go's decoder keeps the last), a known
V0 limit. No matching snapshot: `unsupported-batch-snapshot`, exit 2. Repository unreadable during
the one load: the `LoadSnapshot` error, exit 2. A per-operation refusal (unknown impact path,
oversize limit, missing task, module not slash-qualified, or the repository query path's
`unsupported-query-trace-state` / `-drift` / `-history` / `-repository`): that operation's
`ok=false` with the standalone verb's error, the batch continues. Output cannot be written:
`output-failed`, exit 2 — the `query`/`impact` convention, which standalone `context` now shares
too (2026-09-13 amendment: `context`'s own stdout-write failure was a bare, uncoded exit 1;
`cmd/corvint/taskcontext.go` now emits `output-failed` and exits 2 like every other verb).

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| SBQ-V0-001 | `cmd/corvint/batch.go` `parseBatchRequest` | `TestBatchRefusesMalformedRequestsBeforeReading` |
| SBQ-V0-002 | `runBatch` snapshot load | `TestBatchRefusesWithoutSnapshot` (also asserts no `.corvint/self-observations.jsonl` write) |
| SBQ-V0-003 | `runBatchOperation`; `standaloneQueryContext`, `authorityStartQueryContext` and `repositoryQueryContext` take an optional loaded index through `preloadedIndex` | `TestBatchMatchesStandaloneVerbsAtOneSnapshot` (byte-equal `context` members against `query` in both intents, `context`, `impact`), `TestBatchContextAppliesAdmittedSlotWeights` (byte-equal weighted `context` under an admitted slot-weight file) |
| SBQ-V0-004 | `batchReceipt` | `TestBatchContinuesPastAFailingOperation` |
| SBQ-V0-005 | no write path in `batch.go` | `TestBatchWritesNothing` (`.corvint/` and `git status` unchanged) |
| SBQ-V0-006 | request bounds; per-verb bounds reused | covered by `TestBatchRefusesMalformedRequestsBeforeReading` and the per-verb tests it delegates to |
| SBQ-V0-007 | implemented experimentally; fresh Codex review and focused tests pass; full gate and Claude review pending | acceptance obligations: `TestBatchRefusesMalformedPossessed_SBQ007`: shape, the 32-entry cap, a duplicate `(path, blob_hash)` pair, and admission by byte bound alone including a 32-entry request over 128 KiB, all refused before any repository read; a large-but-under-bound accepted request (32 entries at path/hash lengths kept well under 128 KiB total), a request whose exact encoded byte length is 131072 accepted, and a request one byte over that (131073) refused with `invalid-arguments`, distinguishing the byte bound from any per-field length cap; plus one accepted request sent with and without `line` on every entry, asserting the two receipts byte-equal, so a stored `line` is indistinguishable from one dropped after validation |
| SBQ-V0-008 | implemented experimentally; fresh Codex review and focused tests pass; full gate and Claude review pending | acceptance obligations: `TestPossessionVerdictPrecedence_SBQ008`, one expected verdict per fixture: a directory (`docs/specs`), a gitlink, a dirty gitlink and a parent holding only a gitlink, all `unframable`; a generated/excluded path and a dirty excluded file, both `unsupported`; a dirty tracked path whose supplied `blob_hash` equals its `Sources` hash, still `dirty`; a prefix lookalike (`docs/spec`), an absolute path and a `../` escape, all `absent`; a symlink with an admitted text suffix (`docs/specs/link.md`) `retained` on its link-text hash and `stale` when that hash differs; a tracked file with no admitted suffix (`script/run`), `unsupported` and recorded in no `Exclusion`; plus, for `SBQ-V0-008`(h), four separate fixtures over a loaded index's `Tracked` and skipped-entry table — nil `Tracked` with a non-nil skipped-entry table, a non-nil `Tracked` with a nil skipped-entry table, both nil, and a both-present control — the first three each giving that operation `ok=false` with `code: unsupported-possession-tables` before any verdict and the control verdicting normally, every case with the batch still executing its other operations and exiting 0 |
| SBQ-V0-009 | implemented experimentally; fresh Codex review and focused tests pass; full gate and Claude review pending | acceptance obligations: `TestSuppressionAndInvalidationMatching_SBQ009`: exact-pair suppression, a path-only mismatch kept in the packet, carrying its 008 verdict (`stale` here), a two-row result with exactly one row suppression-matched asserted unsuppressed and listed in `delta.invalidated`, a result whose two rows are invalidation-matched on two different paths asserted to yield exactly two `delta.invalidated` entries, one path under several hashes, a unit-level matcher fixture asserting an empty-`blob_hash` row (an unguarded `Sources` miss) is neither suppression- nor invalidation-matched, plus a full operation-level fixture where that same row verdicts `unsupported` (008(b)) and its path matches a result, asserting the operation takes 010(d)'s `unsupported-possession` fallback rather than a direct suppression or invalidation outcome, a result carrying no evidence row asserted not suppressible, a critical row never suppressed |
| SBQ-V0-010(a-c) | implemented experimentally; fresh Codex review and focused tests pass; full gate and Claude review pending | acceptance obligations: `TestPossessionPerVerbAccounting_SBQ010abc`: the `query`, `impact` and `context` member sets, one protected (critical) and one suppressible result per verb, `impact`'s `budget_bytes` asserted JSON `null` with `within_budget` `true`, a `query` packet compiled at exactly its `budget_bytes` and then suppressed, asserting `within_budget == (packet_bytes <= budget_bytes)` over the final canonical bytes, `packet_bytes` restabilizes and `budget_bytes` is unchanged (the batch-level `delta` bytes never enter the comparison), plus a companion fixture that seeds a pre-suppression receipt whose recomputed `within_budget` would be `false` and asserts the re-run reports `false`, not a preserved `true`, on `query` and `impact` the top-level `verification` and `exclusions` asserted byte-equal to their pre-suppression values except `verification`'s path list, which is asserted recomputed over surviving results' evidence paths with any suppressed path's contribution appended after them, a fully suppressed `query`, `impact` and `context` fixture each asserting `state` stays at its pre-suppression value (`READY`, never `NO_CANDIDATES` or `OUT_OF_SCOPE`) beside `coverage.suppressed_results == included_results`, every coverage count unchanged, `included_results` and `omitted_results` among them, and `uncertainty`'s omission line with them (context's `uncertainty` member newly present), carrying exactly one appended `N results suppressed by caller possession` line, while `coverage.suppressed_results` alone accounts for the count difference, a budget-compacted receipt (`omitted_by_budget` in `request`) asserted unsuppressed with a `budget-compacted` fallback, `packet_bytes` restabilized, and the exported stabilization wrapper returning a `contextindex.Error`, not a panic, on a receipt with no `coverage`, a scalar `coverage`, a `null` `coverage` and a `coverage` typed as an array |
| SBQ-V0-010(d) | implemented experimentally; fresh Codex review and focused tests pass; full gate and Claude review pending | acceptance obligations: `TestPossessionFallbackReasons_SBQ010d` fixtures: `unsupported` on an entry whose `path` appears as the `path` of an evidence row of a result of that operation, an `unsupported` entry whose `path` appears as the `path` of no evidence row of any result of it asserted to trigger no fallback and to be echoed under `delta.ignored` alone, a mixed worktree over a four-operation batch with at least two possessing and two non-possessing operations, asserting exactly one `delta.fallback` entry per possessing operation with its exact `id` and none for either non-possessing operation, both fallbacks together on one operation (`unsupported-possession` wins), `context`'s `DirtyPaths` test alongside `query`/`impact`'s `freshness.state` test of the same underlying condition, each `delta.fallback` entry carrying its `operation`, and a clean fully-suppressible control that fails if suppression is disabled unconditionally; every fallback operation's receipt asserted byte-equal to the standalone verb's, with no `coverage.suppressed_results` member at all rather than `0`, and all of its `possessed` entries echoed under `delta.ignored` whatever the reason; TCP `coverage` (`taskcontext.go:1449-1454`) and `state` asserted unchanged in every case |
| SBQ-V0-010(e-i) | implemented experimentally; fresh Codex review and focused tests pass; full gate and Claude review pending | acceptance obligations: `TestPossessionReplayAtBoundQuintuple_SBQ010f` over `delta.binding`: the fixed fixture's two runs (identical `delta.binding` and identical request) reproduce the receipt byte-for-byte, the control fixture asserting binding equality as its precondition and failing on the differing member rather than reporting a byte difference, with the repository query and `context` request in this fixture chosen so their live history read and the loaded snapshot payload do not change between the two runs (the two named non-goal exceptions to a general replay guarantee); changed traces, drift between two operations of one batch, a changed dirty state and a snapshot loaded under a different `engine` id at one tree each report binding drift naming that member; `delta.binding` asserted present on every possessed batch and each of the four lists absent, not an empty array, when it holds nothing; `delta.suppressed`, `delta.fallback` and `delta.ignored` asserted in operation order, then result order within the operation (`delta.ignored`: `possessed` entry order), and `delta.invalidated` asserted sorted by `(kind, id, path)` alone with no dependence on operation order, each with a fixture whose map-iteration ordering would differ and, for `delta.invalidated`, a fixture where two operations produce the same `(kind, id, path)` in reverse operation order to confirm the exemption; `trace_digests.digest` asserted reproducible across the two runs and asserted to change when a ledger record's `TraceID` changes, confirming the frozen preimage; the fixture repository carries root `.gitignore` entries for every `.corvint/` path Corvint writes, since the snapshot self-ignore covers only `.corvint/index` (`snapshot.go:26,114-115`) and any other `.corvint/` write moves `status_sha256` (`git.go:242,250`) between invocations; separate request-difference expectations that a possessed request replayed unpossessed at one binding returns the full pre-suppression receipt with no `delta`, that a forged hash yields `stale`, and that an entry matching no result is echoed under `delta.ignored`; plus `TestBatchPossessionEvidenceByteSavings_SBQ010i`'s savings gate over a pinned three-operation fixture (one `query`, one `context`, one `impact`, each with a fixed `possessed` list matching a fixed subset of that operation's baseline results): the baseline run sent without `possessed` and the treatment run sent fully possessed, asserting the formula in `SBQ-V0-010`(i) yields at least 30, and no added `critical_missing` in the treatment run |

Compatibility and drift: the operation receipts are the standalone verbs' receipts; a change to
those receipts changes this wire in the same commit. Unresolved (owner review): the
`index-snapshot-v0.md` amendment above; whether `prove` rows join a later version; whether a
harness adapter calls this verb directly.

Rollout: experimental Go candidate only; no adapter calls it. Rollback: delete
`cmd/corvint/batch.go`, its test, the help entries, this spec and its README/INDEX rows. Promotion
or kill: promote only after the parity tests pass and `benchmarks/` records, with the snapshot
identity, the wall latency and RSS of a three-to-five-operation batch against the same operations
as separate processes; kill if any operation receipt differs from its standalone verb.
