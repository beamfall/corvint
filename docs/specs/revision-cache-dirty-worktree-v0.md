# Revision Cache on Dirty Worktrees V0

Owner: Russell Lewis
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Dirty checkouts may reuse the immutable HEAD index only with explicit worktree exclusion and freshness uncertainty.
- Status: proposed/experimental
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
specific user path is treated as harmless. Changing the zero-result abstention reason and adding
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

Measured evidence, linked worktrees (V1-0198, 2026-09-23, `docs/BUILD-LOG.md`): the native Go
snapshot lives in each worktree's own `.corvint/index/`. That path is
`internal/contextindex/snapshot.go:142-144@fe5dad2c` under the `--root` worktree, and its file
name is `internal/contextindex/snapshot.go:166-167@74ceb0e3`. The "repository root" in
`DIRTY-CACHE-002`'s key is therefore the worktree root. `script/measure-worktree-index-share.sh`
created three linked worktrees at one commit, and each built and stored its own clean base: three
`index --if-stale` builds, 72,803,003 bytes each, and no snapshot in the Git common dir. A query
with no snapshot rebuilt in memory on every run. The host load average was about 80, so wall times
are diagnostic only. The dirty view did behave as `DIRTY-CACHE-003` requires within each
worktree. A tracked edit gave that worktree's query `mixed-worktree` without a rebuild. The
snapshot bytes did not change, a sibling worktree's query stayed `fresh`, and the edited worktree
returned to `fresh` after the restore. Sharing one clean base across linked worktrees is not
delivered; BUG V1-0212 tracks it.

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
