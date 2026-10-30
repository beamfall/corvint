# Decision 0552 — Owner-delegated ticket decisions (2026-10-21)

Date: 2026-10-21. Status: accepted by the orchestrator under the owner's delegation. On 2026-10-21
the owner wrote "make decisions for me" about the 31 owner-blocked tickets an audit listed. The
orchestrator decided each one below. Releases and promotions are excluded from the delegation:
rows that need a release cut, a promotion or an owner-only acceptance are recorded as OWNER-ONLY and
stay with the owner. The delegation covers only these 31 tickets and is not a standing grant. New
requirement IDs that implement these decisions are accepted under the owner's standing auto-accept
of 2026-10-11.

Each entry records the question, the decision and its reason or design. The BUILD rows list the
lane group that owns the work.

## BUILD (19)

Build before 1.0. "Narrowed" rows build only the scope the decision states; the lane group names where the work belongs.

### V1-0288 — BUILD-NOW narrowed

Question: GPK-V0-071 is proposed with an open fork: for a checkout owned by another user, is the remedy only "run as the owner / chown", or an operator opt-in that passes -c safe.directory per Git call?

Decision and design: Accept GPK-V0-071 with the first remedy only. Classify Git's exit 128 "detected dubious ownership" at the shared Git-exec boundary as one coded refusal, the same from every verb. The message names the worktree path and says "run as the owning user or change ownership". Decline the safe.directory opt-in: it bypasses Git's protection against planted config, and the env builders do not neutralize hooks, filters, fsmonitor or textconv (docs/specs/go-production-kernel-migration-v0.md:1369). Add a regression test with a chowned or bind-style fixture, or a stubbed stderr where chown needs root. Rationale: this is the simplest fix that turns an uncoded failure into a coded one without widening trust.

Lane group: core-git-history (internal/contextindex/git.go and sibling env builders)

### V1-0313 — BUILD-NOW

Question: GPK-V0-072 is proposed: should query --task and prove --task skip a non-UTF-8 history path, or keep refusing unsupported-query-history?

Decision and design: Accept GPK-V0-072. Pass skipNonUTF8=true on the query and prove path (the seam exists: internal/contextindex/history.go:149,170). The skipped path feeds no history match, history_digest input or candidate, and the packet discloses the exclusion count and reason. Every other history failure keeps unsupported-query-history. Add a regression test with a committed Latin-1 path. Add the divergence-register entry only if the register is still maintained after the oracle retirement (decision 0088). Rationale: this matches decision 0394 (a non-UTF-8 path is an index exclusion everywhere else), and no existing digest moves because only refusing repositories change.

Lane group: core-git-history (internal/contextindex/history.go)

### V1-0301 — BUILD-NOW narrowed

Question: How should cem prepare handle binary or mode changes in cem/1.0, given that the binary representation would change the frozen wire?

Decision and design: Already decided by decision 0548 row V1-0301, not yet built. The BinaryPatch refusal names the path (today internal/cem/patch/patch.go:172 says only "binary patches are outside the CEM patch grammar") and keeps its code. Record the limit in cem-stable-v1. Mode-only changes are covered by CEM-V1-300 and the decision 0546 item 4 typed refusal. The CEM-PILOT-001 always-unknown hunk kind (docs/specs/cem-pilot-kit.md:43) moves to a post-1.0 profile. Add a test that the refusal names the path. Rationale: an honest typed refusal with no wire change.

Lane group: cem-protocol (internal/cem/patch, docs/specs/cem-stable-v1.md)

### V1-0324 — BUILD-NOW

Question: Reads serve the projection unchecked (SPEC 3.3 permits it), but crash-matrix C6 lists INTENT_DIVERGED under reads. Which reading wins, and should a stale read warn?

Decision and design: Keep reads non-blocking (SPEC 3.3). Add a cheap journal check: compare the head sequence and digest the projection records against head.json. On mismatch, emit a PROJECTION_STALE warning (not a refusal) with the projected and current heads. Amend C6 so reads report PROJECTION_STALE and only writers refuse INTENT_DIVERGED. Add a test with a stale projection. Rationale: the smallest change that makes staleness visible without slowing or blocking reads.

Lane group: tasks-cli-refusals (internal/tasks/cli/cli.go, tasks SPEC)

### V1-0461 — BUILD-NOW narrowed

Question: The 1.0 umbrella mixes doc amendments, frozen-candidate qualification, the Beamfall takeover and owner promotion. What remains in 1.0?

Decision and design: AC1 is met by the release contract amendment (corvint-1.0-product-and-release-v1.md:16,46,172). AC3 (the Beamfall takeover, V1-0464) moved post-1.0 by the 2026-10-21 amendment under the feature cut, so record it as removed. Keep the ticket as the release-readiness tracker for AC2 and AC4: Flows, MCP and docs gates on one frozen candidate, run by their child tickets. AC5's hosted dispatch and stable promotion stay with the owner, as V1-0374 does. Close the ticket when the children pass on the candidate. Rationale: it keeps the umbrella honest without inventing new work.

Lane group: release-readiness (after candidate freeze)

### V1-0595 — BUILD-NOW narrowed

Question: CEM 1.0 wire freeze and consumer migration: what remains open now that the contract is accepted and the repository envelope is qualified on real checkouts (CEM-V1-360.., V1-0595 U2)?

Decision and design: Closeout audit lane. Verify AC2 and AC3 on main (prepare, cite, mark, verify, status, report, OCM, frontier, LRF, proof-review and MCP all on cem/1.0; vectors and schemas under protocol/cem-1.0; independent review). Read the 0.x clause per decision 0525 section 6 (no 0.x readability or rollback). File each real gap as its own child, and do not reopen accepted text. External consumer and outcome evidence stays the CEM-V1-008 limit (decision 0546 item 3).

Lane group: cem-1.0-closeout

### V1-0596 — BUILD-NOW narrowed

Question: CEM cross-language Tasks integration: which criteria can a lane deliver, given that external interop and promotion are not lane work?

Decision and design: Deliver AC1: one real compiled Tasks lifecycle (claim, submit, cem-verdict gate run, complete, drift follow-up) with mixed-language CEM test evidence bound to exact artifacts. Deliver AC2 as portable conformance plus the required checks on the final artifact set, with external independent interop recorded as the CEM-V1-008 limit (decision 0546). AC3's promotion moves to V1-0591 (owner).

Lane group: cem-1.0-closeout

### V1-0775 — BUILD-NOW

Question: Suspected: the lease path reuses audit digests (scan.go:305-310 returns the observed Sha256 without re-reading), so a mapped write after the audit may commit. Real or safe?

Decision and design: Write the AC1 test first: a mapped write to an audited store file between the lease audit and the lock-held commit, run on macOS APFS locally and Linux tmpfs in CI. The code path confirms reuse with only ChangeGuard.Check, which notification-based guards cannot see for mmap writes. If the test commits, then under the lock re-read and compare (length and SHA-256) every reused file the commit binds, as V1-0645 did for Mutate and following decision 0549 R11 (prefix detection by bytes, not stamps). Measure the cost against the 0549 T1 targets. If it refuses, close as safe with the test retained.

Lane group: tasks-journal-segments (internal/tasks/store, with the 0549 lane)

### V1-1091 — BUILD-NOW

Question: btime jitters by 1 s. Migrate process identity to boot_id (a persisted format change) or tolerate the jitter?

Decision and design: Already decided by decision 0548 (V1-1091 row), not yet built. Keep btime, and treat a 1-second difference as the same boot only when the pid and start ticks also match, at every identity comparison site. Persisted ledger, marker and run-record formats are unchanged and need no migration. Add a test for a live process whose btime reads 1 s apart, and retain a GitHub Linux runner measurement as evidence. boot_id is post-1.0.

Lane group: dispatch-supervisor (internal/tasks/supervisor/identity_linux.go)

### V1-1478 — BUILD-NOW

Question: Three JS/TS registry-reader shapes may fail open. Shape 3 would change accepted NGI-V0-008. Resolve or widen?

Decision and design: One fixture per shape. (1) An accessor inside a template substitution ${registry[k]()}: resolve the call site in registryCallSites, plus a setup-select fixture with a TSA-V0 trace row. (2) Post-declaration writes (registry.k = ..., Object.assign): widen with a named reason. (3) Eager new X(args) reaching other modules: widen with a named reason. Record the NGI-V0-008 amendment for shape 3 in this lane's decision row (delegated: widening is the fail-closed choice under invariant 2).

Lane group: js-analysis (internal/contextindex/registry_dialect.go)

### V1-1702 — BUILD-NOW narrowed

Question: Pool release pays a 35-60 s complete audit. Putting POOL_RELEASE on the writer route would reverse the owner's CAL-V0-264 decline (a5827b719).

Decision and design: Keep the CAL-V0-264 decline: no writer route for pool verbs. Meet AC1 through the "equivalent bounded path" clause using decision 0549. R11 gives cross-process resume for the pool verbs (F3 accepted), and R12 gives a complete audit over sealed segments (about 79 segment reads instead of about 40k opens), with CAL-V0-116 prefix detection kept by bytes. Retain a measured release time at 47k receipts against the 0549 T3-class bound (2 s or less). Rationale: no perf deferral and no reversal of an owner decision.

Lane group: tasks-journal-segments (0549 S6, internal/tasks/journal, store)

### V1-1750 — BUILD-NOW

Question: pools.json (MaxPoolStateBytes 1 MiB) can fill at 256 members times 5 operation receipts (already one per pair, pool_operation.go:90). Compact, move, or rebound?

Decision and design: Make the bound derived and provable: a test builds a maximal pool (MaxPoolMembers=256, 5 PoolOperations receipts at maximum field lengths, MaxSharedAttempts shares) and asserts that it encodes under MaxPoolStateBytes. If it does not fit, raise MaxPoolStateBytes to the computed worst case, which stays finite, stated in the spec and fail-closed above it. A full pool must still accept acquire and release. Rationale: simpler and lower-risk than moving receipts out of pools.json.

Lane group: workflow-pools (internal/tasks/snapshot/pools.go, pool_operation.go)

### V1-1964 — BUILD-NOW narrowed

Question: Criteria 1 and 3 (auto-reap a STALE_HOLDER or no-lease supervised attempt) conflict with CAL-V0-121 (STALE_HOLDER is advisory, never release authority) and the ATR non-goal at attempt-runner-v0.md:1751.

Decision and design: Keep CAL-V0-121. Criterion 2 is delivered (ATR-V0-192, TestATRV0192_CollisionNamesHoldingAttempt). Narrow criterion 1: when the holder is STALE_HOLDER or has no live lease, the RESOURCE_COLLISION fix line names the exact supported release or handoff command. Narrow criterion 3: document why a supervised attempt has a null leaseExpiresAt and its explicit release path, with a test that the refusal names it. No automatic reap.

Lane group: tasks-cli-refusals (internal/tasks/cli, attempt-runner spec)

### V1-2030 — BUILD-NOW

Question: The falsification shared-server identity is hook-reported, but flows into the receipt as if observed.

Decision and design: Label ServerIdentity (internal/behaviorfalsify/profiles.go:93-97) with its source, observed or hook-reported. A hook-reported-only identity caps the verdict below complete, and a forged hook identity cannot produce a complete verdict. Independent pid, port or binary-digest observation is optional and not needed for the fix. Add a spec row and a test. Rationale: label and cap is the minimal fail-closed fix.

Lane group: behaviorfalsify (internal/behaviorfalsify)

### V1-2107 — BUILD-NOW

Question: RESULT.schema.json rejects opaque/0 runner receipts for unadmitted maps, and fixing it re-binds the sealed CEM-V1-091 chain, which cem-stable-regen cannot do.

Decision and design: Widen references.runnerReceipts[].profile to accept opaque/0 and corvint-test-runner-receipt/0 in the stable and repository-envelope RESULT.schema.json copies. Re-bind the sealed chain through a recorded re-emission, following the owner-accepted CEM-V1-091 re-emission precedent (V1-1437 option a). Add an enum-widening mode to cem-stable-regen only if that is simpler than one recorded manual regeneration. Add a test that a verify result for an unadmitted fixture map validates.

Lane group: cem-protocol (protocol/cem-1.0/stable)

### V1-2274 — BUILD-NOW

Question: dogfood check-seal runs dogfood-ocm status three times on the same tree.

Decision and design: Compute dogfood-ocm status once per (tree OID, scope set) inside the check and seal flow (internal/dogfoodflow check.go and aggregate_worker.go) and pass the result along. Add a test or timings TSV row that proves one invocation, with check and seal verdicts unchanged.

Lane group: dogfood-perf (internal/dogfoodflow)

### V1-2281 — BUILD-NOW

Question: The untouched-repository-v1 harness.py expects a shared NOT_COMPLETE, so wrong-guard refusals pass.

Decision and design: Mirror the daily-loop-v1 change in ba16a6b83. Each case expects the specific refusal code its guard emits, genuinely shared refusals are marked non-discriminating, and a test shows that a wrong-guard refusal fails the case. This is a prerequisite for a credible V1-0019 untouched-repository run.

Lane group: benchmarks-harness (benchmarks/untouched-repository-v1)

### V1-2288 — BUILD-NOW

Question: code2test r20 fell 0.5116 to 0.4642 (and abstention 0.1707 to 0.0366), attributed to de427c57f (V1-0431) by PR #1467. Restore (variant E) or re-baseline?

Decision and design: Restore with variant E plus disclosure. Amend TCP-V0-004 so a subjectless counterpart test only fills unfilled budget and never displaces a selected lexical test. A counterpart that does not fit is listed as a named omission ("paired test omitted: budget") so the chi orientation case still surfaces wrap_writer_test.go. Update the TestTaskContextSelectedLexicalPairs pin to assert the omission, and bump the analyzer schema. Variant E restores abstention to 0.1707, keeps trace2code and improves edit2ripple. If only the unattributed diffusers c7ea7d6f docs-row sample keeps r20 at 0.5022, record the re-baseline with that cause. Rationale: the abstention collapse is the invariant-2 harm, and the omission line keeps V1-0431's orientation value.

Lane group: context-ranking (internal/contextindex task context, TCP-V0)

### V1-2291 — BUILD-NOW

Question: worksource git children run in their own process groups (groupreap.Contain), survive a producer SIGKILL and race run.Close and scratch cleanup.

Decision and design: Keep per-child groups for timeout containment. Record each child's pgid in the run state, and make run.Close SIGKILL every recorded group and wait for exit before scratch removal. Add a test that kills the producer group mid-git-read and asserts that no git child outlives Close. Rationale: the tracked-and-reaped branch of AC2, without touching containment semantics.

Lane group: process-groups (internal/worksource/process_unix.go)

## DEFERRED (7)

Not 1.0 work. Each stays open with this reason and is revisited after the 1.0 release.

### V1-0418 — DEFER-POST-1.0

Question: Is the dirty-worktree overlay (synthetic blob ids over the HEAD snapshot) 1.0 scope?

Reason: This is a new freshness capability (kind FEATURE, already labelled post-1-0), not a defect. Current behavior is correct and fail-closed: IDX-SNAP-V0-004 reads the HEAD snapshot and reports DirtyPaths and mixed-worktree. The design changes evidence pinning (invariant 1: synthetic worktree blob ids), so it needs its own spec. No slowness has been measured against a 1.0 target, so the no-perf-deferral rule does not apply. Move the milestone to post-1-0.

### V1-0422 — DEFER-POST-1.0

Question: Signed archives, a Homebrew channel and Windows: 1.0 or later?

Reason: Decision 0539 item 3 already deferred this past 1.0, and decision 0544 item 1 keeps "no signing: SHA256SUMS only" for candidates. AC1 and AC2 need owner-held signing identity, a tap repo and .github control-plane changes. Carry AC3 as a one-line INSTALL.md edit in the release-docs pass: the Windows exclusion cites V1-0399/V1-0400 and the unsigned line cites decision 0544. Move the milestone to post-1-0.

### V1-0479 — DEFER-POST-1.0

Question: The LSP adapter chain waits on V1-0478 owner acceptance of LQP-V0 profiles, and V1-0478/V1-0477/V1-0168 are already post-1-0. Is it 1.0?

Reason: Decision 0373 cut editor qualification from 1.0 (release contract lines 26 and 65: editors and broad language coverage are deferred). The parent V1-0477 and the prerequisite V1-0478 are milestone post-1-0. The landed LSP server (PR #386) stays experimental with every tuple UNQUALIFIED, so 1.0 makes no claim. Move the milestone to post-1-0 with the parent.

### V1-0480 — DEFER-POST-1.0

Question: Same LSP chain: shared immutable and editor-overlay evidence join. Is it 1.0?

Reason: The same scope cut applies (decision 0373, parent V1-0477 and V1-0478 post-1-0). internal/lspsnapshot stays experimental, and no Core ranking or authority depends on it.

### V1-0481 — DEFER-POST-1.0

Question: Same LSP chain: optional editor language server qualification. Is it 1.0?

Reason: Editor qualification is excluded from 1.0 by decision 0373. The server landed as experimental (PR #386, all tuples UNQUALIFIED, standard definitionProvider withheld per LQP-V0-012). Keep that labelling, with no promotion.

### V1-0482 — DEFER-POST-1.0

Question: Same LSP chain: gopls-backed semantics in agent CLI/MCP. Is it 1.0?

Reason: It depends on V1-0479 and V1-0480 and on the post-1-0 V1-0478 floors. The PR #364 projection repair is an experimental richer profile, and default Core packet bytes, ranking and authority are unchanged, so the 1.0 MCP companion does not need it.

### V1-0483 — DEFER-POST-1.0

Question: Same LSP chain: qualify and promote exact support tuples. Is it 1.0?

Reason: Promotion depends on the post-1-0 V1-0478 profile and floor acceptance and on V1-0481/V1-0482. With editors cut from 1.0 (decision 0373), every tuple stays UNQUALIFIED and no claim is made.

## CLOSED (2)

No change is needed; the ticket is closed with this reason once its closeout evidence is retained.

### V1-0671 — CLOSE

Question: Is per-phase non-PATH resource admission a missing capability, or does supported composition suffice?

Reason: Not a defect. lease_claim.go:60-68 appending declaredOther is required by CAL-V0-021, now amended by decision 0548 V1-1321: claim scope = DECLARED plus REQUESTED. Adopt the recommendation in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-tasks-admin-checkpoint-and-phase-resources.md: split a phase into a child ticket with narrower declared resources, then serial parent integration. That handoff satisfies AC3's "otherwise" branch. File per-phase admission as a post-1.0 ideas ticket (it needs a phase model in the ticket schema).

### V1-1441 — CLOSE

Question: A read-only Codex review wrote an OWNER-role note. Criteria 1 and 2 are delivered. Does criterion 3, the owner review of the stray V1-1375 note, need the owner?

Reason: Criteria 1 and 2 are delivered by ba16a6b83 (read-only session guard, TestATRV0212_ReadOnlySessionRefusesMutations). The Codex review recipe also forbids corvint-tasks. V1-1375 is COMPLETED, so the stray note has no effect on authority. Under the delegation, the orchestrator records one superseding note marking it as written by a read-only review and not owner-authored, then closes. Before closing, also check whether V1-1256 is the same delivery (decision 0548 asked for that check).

## OWNER-ONLY (3)

The delegation excludes releases and promotions, and these rows need the owner. They stay open and blocked on the owner.

### V1-0318 — OWNER-ONLY

Question: Freezing or archiving beamfall/corvint-tasks and pushing a README pointer there is an outward action on another repository. Who performs it?

Reason: The in-tree import has landed (internal/tasks). The README pointer text can be prepared in the corvint repo for the owner to apply. Archiving or freezing another public repo is outside the standing PR approval (origin only) and is visibility-class. Rollback: the old repo stays intact at 800682d. Readiness: ready now.

### V1-0374 — OWNER-ONLY

Question: The release-gate workflow (release-gates.yml:186, linux-core) and decision 0415 now exist. Only the owner-dispatched native linux/amd64 run on frozen candidate bytes remains.

Reason: AC2 (the workflow and governing decision exist) is met on main. AC1 and AC3 need the hosted dispatch on a frozen candidate, which decision 0544 reserves to the owner. No candidate exists yet. Until that PASS is retained, linux/amd64 stays FALLBACK. Readiness: blocked on candidate freeze, then owner dispatch.

### V1-0591 — OWNER-ONLY

Question: CEM 1.0 parent: after the children land, what remains besides stable-profile promotion?

Reason: Promotion is reserved to the owner (decision 0544). Readiness under decision 0546: the contract is accepted; runner-tuple gaps and CEM-V1-008 ship as recorded limits, not blockers; V1-0609 is a typed refusal. All runner children (V1-0592..0594, 0597..0624) and V1-1263..1265 are COMPLETED. Promotion is ready to ask for once V1-0595 and V1-0596 close.

## Delivered in this change

V1-0324 (PSR-V0-191) and V1-1964 as narrowed (ATR-V0-280, ATR-V0-281) are built in the same change
as this record; see `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-journal-1021s-read-checks.md`. The other BUILD rows
are built by their lane groups.

## Rollback

Each decision is reversible by a later decision that names it. Reverting a BUILD row reverts its
implementing change; a DEFERRED or CLOSED row is reopened by the owner or a later delegation.
