# Revision Cache on Dirty Worktrees V0

Owner: Russell Lewis
Intent status: proposed; DIRTY-CACHE-014..015 accepted (decision 0518; V1-1141); DIRTY-CACHE-016 accepted (decision 0519; V1-1174); DIRTY-CACHE-017 accepted under the owner's delegated 1.0 feature cut (decision 0548; V1-2068); DIRTY-CACHE-018 accepted under the owner's standing auto-accept (2026-10-11; V1-2218); DIRTY-CACHE-019 accepted under the owner's standing auto-accept (2026-10-11; V1-2880)
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Dirty checkouts may reuse the immutable HEAD index only with explicit worktree exclusion and freshness uncertainty.
- Status: proposed; DIRTY-CACHE-014..015 accepted (decision 0518; V1-1141); DIRTY-CACHE-016 accepted (decision 0519; V1-1174); DIRTY-CACHE-017 accepted under the owner's delegated 1.0 feature cut (decision 0548; V1-2068); DIRTY-CACHE-018 accepted under the owner's standing auto-accept (2026-10-11; V1-2218); DIRTY-CACHE-019 accepted under the owner's standing auto-accept (2026-10-11; V1-2880)/experimental
- Exists: immutable-HEAD cache reuse with explicit dirty-worktree freshness and uncertainty.
- Blocked on: promotion evidence and kill-criteria measurements; mutable content remains outside authority.
- Read next: User and measurable job; Requirements; Evidence and kill criteria.

## User and measurable job

An agent working in a normal dirty checkout should receive the same immutable-tree context it would
receive at the checkout's `HEAD` tree without paying for a full re-index on every query. The receipt
must remain explicit that mutable worktree content was not indexed and may contain better or newer
evidence.

This is a narrow V4 cache and freshness repair. It does not make mutable content authoritative,
snapshot the worktree, or claim that revision-only context is complete.

## Requirements

- `DIRTY-CACHE-001`: query index sources MUST come only from the resolved `HEAD^{tree}`. A tracked
  worktree file may be read only when its exact Git blob hash matches the tree entry; otherwise the
  tree blob is read. Git-status-dirty paths MUST NOT be opened. A clean tracked candidate MUST use a
  bounded descriptor-relative no-follow regular-file read with pre/post descriptor and path identity
  checks; refusal or race falls back to the immutable tree blob. Untracked paths, ignored paths,
  symlink targets, and modified worktree bytes MUST NOT enter sources, symbols, records, evidence,
  or cache payloads.
- `DIRTY-CACHE-002`: memory and disk cache lookup MUST use repository root, immutable tree OID, and
  cache-engine digest regardless of worktree cleanliness. A dirty hit MUST NOT run `ls-tree` or
  rebuild the immutable index.
- `DIRTY-CACHE-003`: a dirty query MUST receive an unshared freshness view with its own sources map
  and exact sorted dirty paths. It MUST NOT mutate the cached clean base object or change a later
  clean query's `READY` state or bytes.
- `DIRTY-CACHE-004`: on a dirty cache miss Corvint MUST build the same immutable-tree base object as a
  clean query, persist and remember only that clean base, then return the unshared dirty view.
- `DIRTY-CACHE-005`: dirty query, feature, impact, and evaluation receipts MUST retain their
  result, proof, or evaluation `state` and set `freshness.state=mixed-worktree`. Every receipt MUST set
  `freshness.scope=git`; that scope is defined as Git-visible changes with ignored paths
  `UNOBSERVED`, and `fresh` means only that Git status reported no visible change. A zero-result
  dirty query MUST retain its ordinary `OUT_OF_SCOPE` retrieval state and abstain with reason
  `unindexed-worktree-changes`, making explicit that the state covers only the immutable tree and
  does not claim the mutable worktree is out of scope. The CLI exit contract remains unchanged.
- `DIRTY-CACHE-006`: worktree status capture MUST be bounded to 8 MiB including the overflow byte.
  It MUST have a fixed ten-second deadline and run in a dedicated process group whose leader is
  reaped and whose descendants are killed after success, timeout, overflow, exception, or interrupt.
  Overflow, timeout, Git failure, invalid status structure, or path-decoding failure MUST fail closed
  with a named error; Corvint MUST NOT truncate status and report a falsely complete dirty set.
- `DIRTY-CACHE-007`: existing cache bounds remain: 128 MiB per disk entry, eight disk entries,
  256 MiB estimated memory, and 1 MiB per source. Cache artifacts remain disposable, private local
  derived state and never become evidence authority.
- `DIRTY-CACHE-008`: regressions MUST cover memory and fresh-process disk dirty hits, dirty cold
  population, tracked mutation, untracked content, symlinks, cache-view isolation, status overflow,
  byte budgets, deterministic bytes, and clean-after-dirty recovery.
- `DIRTY-CACHE-009`: a 30-sample dogfood run MUST distinguish cold and primed-dirty phases, keep
  complete output within the configured packet bound, and require primed-dirty p95 below the
  accepted one-second V4 gate. Relative speedup is diagnostic until measured on more repositories.
- `DIRTY-CACHE-010`: the implementation MUST add no overlay, session, background process, database,
  network access, ignored-path scan or exception, or secret-content scanner. Ignored files are
  outside the observed freshness universe, not evidence that the filesystem or repository has no
  additional context. Delete the dirty-view and bounded-read helpers and restore clean-only cache
  lookup to roll back; remove the additive freshness-scope fields only with a receipt compatibility
  change.
- `DIRTY-CACHE-011`: each index build MUST resolve the repository storage object format and exact
  `HEAD^{tree}` identity together. SHA-1 and SHA-256 are supported; any other, missing, malformed, or
  mismatched format/OID fails closed. Clean-worktree blob verification MUST use Git's blob framing
  and the resolved repository hash algorithm. Cache loading MUST accept only lowercase hexadecimal
  40-character SHA-1 or 64-character SHA-256 blob OIDs matching that resolved format. The memory and
  disk cache key remains repository root, tree OID, and engine digest; this MUST NOT introduce a
  generic hash registry or alternate evidence identity.
- `DIRTY-CACHE-012`: the separate `working-tree-untracked` impact authority profile (`GPK-V0-029`)
  is outside `DIRTY-CACHE-005`'s dirty-cache-reuse receipts: it binds real working-tree byte content
  as contained evidence for its included targets, not only Git status. Its receipts MUST set
  `freshness.scope=git+working-tree`, defined as `DIRTY-CACHE-005`'s Git-visible dirty-path status
  plus the disclosed stable twice-read working-tree bytes bound to each included target.
  `freshness.scope=git+working-tree` MUST NOT be emitted by any `DIRTY-CACHE-005` receipt, and
  `freshness.scope=git` MUST NOT be emitted by a `working-tree-untracked` receipt.
- `DIRTY-CACHE-013`: the native disk snapshot store (`index-snapshot-v0`) MUST live at
  `corvint/index/` under the repository's Git common directory, so every linked worktree and the
  main worktree share one clean base per (object format, tree OID, engine digest); for this store
  that key refines `DIRTY-CACHE-002`'s repository root. The common directory MUST be resolved from
  the `.git` marker and `commondir` pointer the way Git status does and with no Git process, so an
  absent store stays a spawn-free miss (`IDX-SNAP-V0-009`); the resolution refuses a `.git` marker
  that is a symlink and reads each pointer file with a bounded no-follow open, but follows symlinks
  when it resolves the gitdir and commondir paths those pointers name. When the common directory
  cannot be resolved (a symlinked `.git`, a root that is not the worktree top level) the store is
  the worktree's own `.corvint/index/`. Only the clean base is shared: dirty paths and status digest are
  applied per worktree at load and never written (`DIRTY-CACHE-003`, `DIRTY-CACHE-004`). There is
  no lock: each concurrent `index` writer publishes a complete, synced temporary file by atomic
  rename onto its key, the last rename wins, and a reader keeps the complete file it opened, since
  snapshot bytes for one key need not be identical across builds. The writer MUST refuse a worktree
  `.corvint`, a store parent `corvint`, or a store `corvint/index` that exists as a symlink or other
  non-directory (`IDX-SNAP-V0-005`). The `DIRTY-CACHE-007` entry bound for the shared store is eight
  per worktree, 8 x (1 + the number of entries under `<common>/worktrees/`), capped at 64, because
  one shared eight-entry bound would let worktrees on different trees evict each other's snapshot
  on every write; the worktree fallback store keeps eight. Only the fallback store writes a
  `.gitignore`, since Git never tracks the common directory. The store is created 0755 with 0600
  files, so when several OS users share one common directory (`core.sharedRepository`) a second
  user's `index` fails and their reads miss. A repository whose common directory resolves no longer
  reads or deletes a `.corvint/index/` left in its worktree by an earlier release: those snapshots
  carry an older engine digest and cannot match, so the first `index` rebuilds into the shared store
  and the user may remove the old directory. In fallback mode `.corvint/index/` is the store, read
  and evicted as before. Rollback: restore the worktree-joined store path; the shared directory is
  disposable derived state.

- `DIRTY-CACHE-014`: **accepted, decision 0518 (V1-1141)**. A normal
  path-impact receipt over a MIXED root MUST include a top-level `warning` string when the
  normalized requested paths are not a subset of the complete captured dirty-path set. The
  warning MUST name the bound HEAD commit (not the tree OID), the dirty-path count, and the
  exclusion of mutable content. It MUST survive packet budgeting and be included in packet-byte
  accounting. Clean roots and requests entirely within the dirty set retain their existing
  receipt shape; freshness uncertainty remains. Missing commit identity MUST be named UNKNOWN,
  never inferred from a tree OID. This applies to default and non-Go syntax path profiles.
- `DIRTY-CACHE-015`: **accepted, decision 0518 (V1-1141)**. Path impact
  MUST accept the experimental boolean `--require-clean`. On any captured MIXED root it MUST
  return `state=NEEDS_WIDENING`, `abstention.active=true`, reason `mixed-worktree`, empty `results`,
  and a warning naming the bound commit and dirty count, even when every requested path is dirty.
  It MUST abstain before impact ranking or external-provider execution. A clean root retains
  ordinary behavior. The flag rejects values, repetition, `--base`, and
  `--working-tree-untracked`; ordinary argument validation and read-only behavior remain.
  Structured abstention exits 0 (the existing context-receipt exit contract); invalid arguments
  exit 2. Ignored paths remain UNOBSERVED. This adds no index, trace, or repository writes.
- `DIRTY-CACHE-016`: **accepted (decision 0519; V1-1174)**. Every verb
  `runContext` dispatches (`topLevelCommands`) MUST either honour `--require-clean` or refuse it;
  none may accept and ignore it. Only `impact` (DIRTY-CACHE-015) and `affected` (AFP-V0-045)
  honour it. Every other verb, every `prove` mode (path, `--task`, `--cem`, `--checkpoint`) and
  `feature` MUST exit 2 with a typed diagnostic and no success document, without writing
  repository state. Most verbs report `invalid-arguments`; verbs with their own input contract
  keep their existing code (`work` emits its `ERROR` result document). The fail-open host hook
  forms `adapter codex` and `adapter claude-code <event>` exit 0 by contract, so they refuse
  through their degraded `unsupported-hook-event` output instead (other `adapter` subcommands keep
  their own refusal); `adapter codex` now degrades on any trailing argument as `adapter claude-code` already
  did. The test runs each verb in a valid form with and without the flag and requires the flag to
  cause the refusal; a dispatched verb missing from its form table fails the test until a form is
  added. Rollback: revert the `adapter codex` argument check and remove the test.
- `DIRTY-CACHE-017`: **accepted under the owner's delegated 1.0 feature cut (decision 0548; V1-2068)**. When the captured dirty set is non-empty and every
  dirty path is a recognised Corvint Tasks store projection file (the AFP-V0-088 layout:
  `.taskman/policy.json`, `.taskman/queue.json`, `.taskman/tickets/<name>.json`,
  `.taskman/releases/<name>.json`; `internal/taskprojection`), the worktree is source-clean: the
  query/impact receipt reports `freshness.state=fresh`, `mixed_paths=[]`, `mixed_path_count=0`
  and a separate `freshness.task_state` member `{class: TASK_STORE_PROJECTION, path_count, paths}`
  (paths capped like `mixed_paths`; the budget-compacted receipt keeps `class` and `path_count`).
  Impact emits no DIRTY-CACHE-014 warning and `--require-clean` does not abstain
  (DIRTY-CACHE-015). The dogfood prompt packet behind host hooks reports `freshness=clean` and a
  top-level `task_state`, which the hook projection carries as `{class, path_count}`. Any other
  dirty path, including an unrecognised `.taskman/` path, keeps the full MIXED contract with every
  dirty path listed and no `task_state`. Projection files are never indexed, read, or treated as
  evidence, even when committed: the index path screen excludes them as `task store projection`; the rule adds no writes (invariant 4). Non-goals: trace-learning eligibility
  (`authorityTraceState`), batch fallback (SBQ-V0-010), `impact --working-tree-untracked`, the
  repository `worktreeState` identity and dogfood/handoff dirty digests keep counting projection
  files; `corvint context` task packets carry no freshness member and are unchanged. Failure mode:
  a source edit hidden as projection dirt; guarded by the exact-layout match and the mixed-set
  regression. Compatibility: CCF-V1-005 member names and types are unchanged and `task_state` is
  additive, but `fresh` now means source-clean (the CCF-V1-005 narrowing accepted with this requirement).
  Rollback: make `SourceDirty` equal `len(DirtyPaths) != 0`, drop `task_state`, and remove the
  path-screen exclusion with an analyzer schema bump.
- `DIRTY-CACHE-018`: **accepted under the owner's standing auto-accept (2026-10-11; V1-2218)**.
  The DIRTY-CACHE-014/015 warning MUST also name the complete captured dirty set's paths in
  sorted order, Go-quoted, at most 8, followed by ` (+N more)` when more exist (the affected
  AFP-V0-356 detail shape), so a reader can see which mutable paths the immutable answer
  excludes. An unbudgeted receipt names up to 8. A budgeted receipt selects evidence against the
  count-only form (`dirty path names omitted`) and then spends only the remaining slack: it names
  the largest prefix of up to 8 paths that still fits `budget_bytes`, else keeps the count-only
  form, so names never evict a result and `MinPacketBytes` is unchanged. The commit, the dirty
  count and the exclusion clause stay in every form. Failure mode: long dirty paths that do not
  fit; the count-only form remains. Non-goals: naming paths in `freshness`, changing
  `mixed_paths` sampling, or naming task-store projection paths (DIRTY-CACHE-017 emits no
  warning). Rollback: restore the count-only warning text and drop `fitImpactWarningNames`.
- `DIRTY-CACHE-019`: **accepted under the owner's standing auto-accept (2026-10-11; V1-2880)**.
  A sealed CEM -- a `<name>.cem.json` file directly under `.corvint/changes/`, the layout the
  dogfood seal writes and the affected planner recognises (AFP-V0-152) -- is change evidence
  derived from the commit that bound it, not source. The index path screen
  (`forbiddenPath`, IDX-SNAP-V0-018) MUST exclude it from index sources with the named reason
  `sealed change evidence`, so it adds no bytes toward the aggregate blob-admission bound
  (`validateBlobAdmission`), never ranks as evidence, and samples after every non-policy
  exclusion in a task-context exclusions summary, as task store projections do (TCP-V0-121). An
  impact request naming one is answered `OUT_OF_SCOPE`; a range touching one keeps its existing
  non-Go omission. The `.corvint/change.cem.json` sidecar and any other path under
  `.corvint/changes/` are not recognised and stay admitted, so an unrecognised file is never
  hidden. Because `ForbiddenPathReason` is shared (decision 0102), learned-trace and
  documentation-corpus admission refuse a sealed CEM path as they refuse a projection path.
  Non-goals: raising `maxBatchBytes` (bound scaling stays with V1-2749), and changing how
  `affected`, the gate selector or dogfood read sealed CEMs, which read Git directly. Failure
  mode: a source file hidden by the exclusion; guarded by the exact-layout match and its
  regression. Rollback: remove the `sealedChangeEvidencePath` arm of `forbiddenPath` with an
  analyzer schema bump.

## V1-1141 scope and acceptance

Intent for DIRTY-CACHE-014..015 is accepted (decision 0518; V1-1141), and for
DIRTY-CACHE-016 accepted (decision 0519; V1-1174); implementation
remains experimental. The user job is recognizing that a clean requested path in a dirty checkout
is answered against a committed revision. The baseline is freshness metadata alone. Verify clean,
all-dirty, partial-overlap, disjoint, renamed, normalized and untracked dirty-path cases; verify
strict abstention, clean strict parity, minimum packet budget and unchanged repository bytes.
No overlay or mutable source authority is introduced. The warning is additive for affected path
receipts; exact-byte consumers must admit it. Rollback removes the warning and flag together.

`affected` is not covered here: `cmd/corvint/affected.go` builds a separate mutable-worktree graph
and selects the dirty set plus optional range paths; it does not use the immutable path-impact
compiler or accept explicit requested paths. Its own contract is `AFP-V0-044` (a warning when a
`--base` range path lies outside a non-empty dirty set) and `AFP-V0-045` (`--require-clean` as a
typed refusal), both accepted (decision 0519; V1-1141).

## Trust, failures, and compatibility

The Git tree is the sole content authority. Live status contributes freshness metadata only. A path
listed as dirty is neither evidence nor permission to read it. Ignored paths are not enumerated,
opened, counted, hashed, or claimed absent. Cache loading continues to validate format, engine
identity, tree identity, payload digest, source paths, and blob identities against the repository's
resolved Git object format.

`freshness.state=mixed-worktree` remains intentionally fail-closed because Git-visible mutable files
may contain a stronger answer. Top-level `state` continues to describe the immutable-tree retrieval,
proof, or evaluation result; it never implies that mutable content was indexed, and rerunning
`index` is not presented as a way to clear worktree dirtiness. An ignored file may also contain
context, but is explicitly unobserved rather than a reason to claim the Git-visible index stale. No
specific user path is treated as harmless, except the exact Tasks store projection layout
(`DIRTY-CACHE-017`), which is task state rather than source, and the exact sealed CEM layout
(`DIRTY-CACHE-019`), which is derived change evidence. Changing the zero-result abstention reason and adding
freshness-scope fields are additive semantic clarifications; consumers that compare exact receipt
bytes must update their fixtures. The cache format does not change; modifying the index engine
naturally changes its engine digest and cache filename.

Named failure is required for oversized status rather than treating the tree as clean. If bounded
capture requires a broad subprocess abstraction or causes process leaks, omit it from delivery and
leave `DIRTY-CACHE-006` open rather than weakening the bound.

## Evidence and kill criteria

Unit tests prove identity, isolation, bounds, and failure behavior only. Dogfood timing is local
diagnostic evidence; it proves neither token savings nor product value. Keep the revision-only
repair but remove dirty cache reuse if it changes clean receipt bytes, admits any mutable-content
canary, leaves a child process after interruption, or creates treatment-only critical evidence
misses. Do not add worktree overlays unless a separately accepted contract defines explicit consent,
snapshot identity, secret handling, and independent outcome gates.

Measured evidence, linked worktrees (V1-0198, 2026-09-23, `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md`, before V1-0212):
the native Go snapshot then lived in each worktree's own `.corvint/index/`, so the "repository
root" in `DIRTY-CACHE-002`'s key was the worktree root. `script/measure-worktree-index-share.sh`
created three linked worktrees at one commit, and each built and stored its own clean base: three
`index --if-stale` builds, 72,803,003 bytes each, and no snapshot in the Git common dir. A query
with no snapshot rebuilt in memory on every run. The host load average was about 80, so wall times
are diagnostic only. The dirty view did behave as `DIRTY-CACHE-003` requires within each
worktree. A tracked edit gave that worktree's query `mixed-worktree` without a rebuild. The
snapshot bytes did not change, a sibling worktree's query stayed `fresh`, and the edited worktree
returned to `fresh` after the restore. V1-0212 (`DIRTY-CACHE-013`) since shares one clean base:
the store is `corvint/index` under the Git common directory, or the worktree's own
`.corvint/index` when that directory cannot be resolved
(`internal/contextindex/snapshot.go:226-231@e10b4d1c`), and each snapshot's file name is
`internal/contextindex/snapshot.go:268-269@cd3a98c5`.

## Traceability

The Python citations below remain candidate behavior only; decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules that implementation
non-authoritative and slated for separate removal. `DIRTY-CACHE-005` additionally cites the native
Go receipt and its query, impact, prove, and harness regressions.

| Requirement | Implementation | Evidence |
|---|---|---|
| `DIRTY-CACHE-001..004`, `DIRTY-CACHE-006..007` | candidate: `src/context_corvint_index.py`, `src/context_corvint.py` | focused adversarial cache/query tests |
| `DIRTY-CACHE-005` | `internal/contextindex/receipt.go`, `src/context_corvint.py` | `TestImpactMixedRenameUsesCommittedBlobAndBothEndpoints`, `TestFreshProcessAuthorityStartQueryMatchesPythonCleanAndMixed`, `TestProveDirtyPrimaryRowLeavesTheResultUnproven`, `TestHarnessUserPromptRetainsRetrievalStateInMixedWorktree`, Python mixed-worktree receipt tests |
| `DIRTY-CACHE-008` | candidate: `tests/test_context_corvint_cache.py`, `tests/test_context_corvint.py` | focused and full suite |
| `DIRTY-CACHE-009` | candidate: private dogfood measurement | 30 samples per eligible phase; no token/value claim |
| `DIRTY-CACHE-010` | code and contract review | diff and independent review |
| `DIRTY-CACHE-011` | candidate: `src/context_corvint_index.py` | SHA-1/SHA-256 clean/dirty cold, memory, disk, corruption, receipt-identity, and unsupported-format regressions |
| `DIRTY-CACHE-012` | candidate: `internal/worktreeimpact/compiler.go` | `TestWorktreeImpactFreshnessScopeIsDistinctFromDirtyCacheReuse` |
| `DIRTY-CACHE-013` | `internal/contextindex/snapshot.go`, `internal/gitstatus/status.go` | `TestLinkedWorktreesShareOneCleanSnapshot`, `TestConcurrentWorktreeWritersPublishCompleteSnapshotsByRename`, `TestSharedSnapshotStoreKeepsTheEntryBoundAcrossWorktrees`, `TestSnapshotStoreBoundScalesByWorktreeUpToTheCap`, `TestWriteSnapshotDoesNotRewriteMatchingGitIgnore`, `TestSnapshotStoreFallsBackToTheWorktreeWhenTheCommonDirectoryIsUnresolved`, `TestWriteSnapshotRefusesCommittedSymlinkedSnapshotDirectory`, `TestSnapshotReadersMissThroughCommittedSymlinkedSnapshotDirectory` |
| `DIRTY-CACHE-014` | `internal/contextindex/receipt.go`, `internal/contextindex/impact_freshness.go` | `TestImpactMixedWarning`, `TestImpactMixedWarningBudget`, `TestImpactMixedCLIReadOnly` |
| `DIRTY-CACHE-015` | `cmd/corvint/main.go`, `internal/contextindex/impact_freshness.go` | `TestImpactRequireCleanCLI`, `TestImpactRequireCleanArguments`, `TestImpactRequireCleanBudget`, `TestImpactRequireCleanValidatesRequest` |
| `DIRTY-CACHE-016` | `cmd/corvint/prove_options.go`, `hostAdapterOutput` in `cmd/corvint/host_adapter.go`, `topLevelCommands` in `cmd/corvint/main.go` | `TestRequireCleanHonouredOrRefusedByEveryCommand`, `TestProveRefusesRequireClean` |
| `DIRTY-CACHE-017` | `internal/taskprojection/taskprojection.go`, `internal/contextindex/index.go`, `internal/contextindex/task_state.go`, `internal/contextindex/receipt.go`, `internal/contextindex/local_completion_context.go`, `cmd/corvint/host_adapter_projection.go`, `cmd/corvint/main.go` | `TestOnlyProjection`, `TestTaskStoreProjectionDirtIsSourceClean`, `TestDogfoodPromptTaskStoreProjectionDirtIsClean`, `TestImpactTaskStoreProjectionDirtIsSourceCleanCLI`, `TestHookProjectionCarriesTaskState`, `TestImpactTrackedTaskStoreProjectionIsNotEvidence` |
| `DIRTY-CACHE-018` | `internal/contextindex/impact_freshness.go`, `internal/contextindex/receipt.go` | `TestImpactMixedWarningNamesDirtyPaths`, `TestImpactMixedWarningNamesWithinBudget` |
| `DIRTY-CACHE-019` | `internal/contextindex/index.go`, `internal/contextindex/taskcontext.go` | `TestSealedChangeEvidencePathLayout`, `TestBuildExcludesSealedChangeEvidence`, `TestAnalyzerSchemaOutput` |
