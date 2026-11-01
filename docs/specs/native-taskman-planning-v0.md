# Native taskman fixture planning V0

Owner: Russell Lewis
Date: 2026-09-19
Intent status: accepted fixture scope (decision 0322); NTP-V0-020..025 accepted under the owner's standing lane auto-accept (2026-10-11, V1-1580); NTP-V0-246 accepted under the same auto-accept (V1-0912, decision 0548); NTP-V0-026 accepted under the owner's standing auto-accept (2026-10-11, V1-2119)
Delivery status: experimental
Authoritative inputs: decision 0322, `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/work-queue-observation-v0.md`, and the source-bound sibling task-store SPEC.

## Agent digest
- Claim: Read-only native fixture priority-first planning with explicit incomplete coverage.
- Status: accepted fixture scope (decision 0322); NTP-V0-020..025 accepted under the owner's standing lane auto-accept (2026-10-11, V1-1580); NTP-V0-246 accepted under the same auto-accept (V1-0912, decision 0548); NTP-V0-026 accepted under the owner's standing auto-accept (2026-10-11, V1-2119)/experimental; GP and production promotion held.
- Exists: native read adapter, pure planner and focused fixture/refusal tests; frozen pre-edit preregistration; read-only `corvint-tasks plan batches` (NTP-V0-020..025).
- Blocked on: live reservations, admission, CONFIG_PIN and GP remain executor/promotion dependencies.
- Read next: Requirements; Fixture input and receipt; Acceptance and rollback; Tasks plan batches; Shared reservation paths.

## User and boundary

The operator needs deterministic selected/deferred/blocked reasons from the canonical native store.
The competing simpler behavior, WQO maximum-cardinality shadow selection, can displace urgent work.
Native selection therefore uses the separate `taskman-plan/0` / `taskman-priority-first/0` profile.
WQO observation and wave selection keep their existing closed profiles and semantics.

This slice is an explicit trusted-local fixture experiment. It launches only a caller-selected native
executor with fixed read commands, never executes ticket prose, and grants no admission authority.
No daemon, network dependency, queue writer, live runtime, production reservation oracle or default
startup work is added. Missing original ATCP documents remain unrecovered; exact ATCP conformance is
not asserted. Current source identities, not substitutes for those originals, are retained in decision 0322.

## Requirements

- `NTP-V0-001`: The opt-in `corvint work plan-fixture --executor ABSOLUTE_FILE --observations FILE`
  MUST require a native queue declaring `fixture:true` and `canonicalWriter:NATIVE`. It MUST bracket
  queue status, canonical policy bytes and bounded ticket exports with successful native receipt
  audits at exactly equal complete snapshot identities. Pending redo, staging, divergent projections,
  wrong profiles, malformed or duplicate keys, incomplete pages and changed identities MUST refuse.
  Every read uses fixed argv, bounded output and a deadline; cancellation retires owned descendants.
- `NTP-V0-002`: The receipt MUST bind the immutable source commit/tree, executor digest, native
  snapshot, policy content identity, exported canonical record digests and explicit fixture
  observations. Source/index drift and fixture observation identity mismatch MUST refuse. Projection
  bytes alone never establish canonical authority. All captured inputs precede pure selection.
- `NTP-V0-003`: Eligible tickets MUST be processed by priority P0..P3, numeric order, then ticket ID
  byte order. The first runnable ticket MUST be selected and never displaced by a larger wave.
  Continue in that order; live/prior-selection collisions and exhausted capacity defer otherwise
  runnable work. Every ticket appears once as SELECTED, DEFERRED or BLOCKED with deterministic reasons.
- `NTP-V0-004`: Eligibility MUST preserve ticket state/holds, current RUN approval, source/writer,
  completion dependencies, external-unbounded refusal and live-attempt observations. Unobserved
  reservation/attempt facts MUST block. GATE_PASSED dependencies are explicitly unobserved in this
  slice and block; no native gate-result producer is invented. Archived-from-COMPLETED satisfies
  a completion dependency; other archived tickets do not.
- `NTP-V0-005`: Collision sets MUST include declared resources/touch paths plus the existing
  source-bound direct dependency closure. PATH prefix overlap and exact non-PATH keys collide;
  WHOLE_REPOSITORY collides with every reservation. WQO's successful closure boolean MUST NOT hide
  untracked, unindexed, unsupported or unparsed paths. Incomplete coverage blocks unless policy
  permits whole-repository serial fallback and no other reservation/selection is live. External
  unbounded effects always block. Closure describes syntactic direct dependencies, never semantic independence.
- `NTP-V0-006`: The plan MUST bind queueId, policySha256, headSeq, intentTreeSha256,
  reservationSetSha256 and each ticket's acceptanceRevision/resources. Capacity comes from policy
  minus complete fixture live reservations, never caller-supplied unlimited headroom. A ticket
  consumes one fixture builder slot; nonempty capacity classes or required runtime capabilities
  remain unsupported/blocked rather than assumed available. The executor owns atomic whole-plan
  freshness refusal and admission; this producer performs neither.
- `NTP-V0-007`: Deferral history MUST be explicitly observed, including a complete empty history.
  Ordered fixture-pinned plans at the same acceptance revision supply earliest deferral headSeq;
  an intervening selection resets it. A current deferral alone does not invent a prior pin. Unknown
  history refuses the preview rather than giving null the meaning of known absence.
- `NTP-V0-008`: Existing command/WQO behavior MUST remain intact. The new command MUST mutate no
  source, intent, journal or reservation state. The receipt MUST label fixture observations,
  unauthenticated trusted-local execution and unavailable production qualification. GP stays NOT_RUN
  until all preregistered workloads/controls/witnesses and actual executor conditions qualify.
- `NTP-V0-246`: Capture MUST admit at most 16384 tickets (export `page.total` and the queue
  `tickets` count), and a decoded `taskman-plan/0` MUST carry at most 16384 `entries` and 16384
  `blockers` per entry; one more refuses the preview and nothing is truncated. The bound covers the
  Tasks per-queue bound (10000) with headroom; the per-response and 32 MiB aggregate byte caps still
  apply. `corvint work plan-fixture --help` states the bound. Rollback restores the 1000 bound.

### Tasks plan batches (V1-1580, issue 1023)

The read-only `corvint-tasks plan batches [--max-group-size N] [--shared-path PATH]...` verb proposes
integration batches of candidates whose changes do not collide. It is a proposal, never an admission,
merge or integration authority, and it sits beside `plan preview` (`corvint-tasks-agent-leases-v0.md`
CAL-V0-014), not inside the fixture planner above.

- `NTP-V0-020`: Candidates MUST be, for each OPEN or HELD ticket, its highest-generation attempt in
  phase BUILT, CHECKING, REVIEWING or READY_FOR_INTEGRATION that records a candidate tree or a branch.
  They are processed in plan order: priority, numeric order, then ticket ID bytes. Running, terminal
  or tree-less branch-less attempts and other ticket states are not candidates. More than 256
  candidates refuse LIMIT_EXCEEDED rather than truncating.
- `NTP-V0-021`: Each candidate's changed paths MUST be the recursive, rename-free `diff-tree` from its
  recorded base commit's tree to its candidate tree (the recorded tree, else `refs/heads/<branch>`).
  Two candidates collide when a non-shared path of either is equal to, or a directory prefix of, a
  path of the other; an overlapping pair is exempt only when both of its paths are shared, in either
  candidate order. `--shared-path` names an exact path, or a directory prefix when it ends in `/`;
  shared paths are exempt from the disjointness test only. Groups are formed first-fit in candidate
  order: a candidate joins the first group that has room, collides with no member and merges cleanly.
- `NTP-V0-022`: Every join MUST be confirmed by a dry chained three-way merge (`git merge-tree
  --write-tree` over the candidate's base) of the candidate onto the group's merged tree, which
  starts at the intent branch tip. A conflict splits the candidate away: it is reported in `splits`
  with the group and up to 16 conflicted paths, and it tries later groups, then a new group whose
  first merge is onto the tip. Every candidate is first merged onto the tip itself, before any join;
  one that conflicts there is ungrouped as STALE_TREE even when it would merge cleanly onto a group.
  Only ref lookups read the repository's Git directory (`GIT_DIR`, no worktree). Every object read,
  diff and merge runs in a throw-away bare repository view whose object directory is temporary and
  whose only alternate is the repository's objects, so the repository's configuration, attributes,
  hooks and index, and any merge driver or filter they name, never run. Each Git run has a deadline
  that kills its process group, a bounded wait for inherited pipes and a 16 MiB output bound.
  SIGINT or SIGTERM cancels the running Git, and the view is removed before the read refuses; only an
  uncatchable kill can leave a `corvint-plan-batches-*` temporary directory, which holds no
  repository state. The repository MUST gain no object and no ref, and no merged tree is reported.
  The merge's two sides are passed as parentless commits that wrap each tree, written only to the
  view with a fixed identity and date, because Git before 2.45 accepts only commits there; the plan
  is the same on every Git from 2.40, and an older Git, which lacks `--merge-base`, refuses
  UNSUPPORTED (V1-3901).
  Known limit: Git may refresh the mtime of an existing object that a merge recreates (its prune
  grace clock); object content, count and refs are unchanged.
- `NTP-V0-023`: `--max-group-size N` MUST bound every group, from 1 to 64, default 8; the item
  reports the bound. Malformed, out-of-range, repeated or unknown flags refuse with usage.
- `NTP-V0-024`: A candidate whose base commit is absent or unresolvable, whose candidate tree or
  branch does not resolve, or whose changed-path diff fails on a missing tree object, MUST be
  reported in `ungrouped` with ticket, attempt, MISSING_EVIDENCE and a bounded detail, and never
  grouped or guessed. An intent branch that does not resolve refuses the whole read
  MISSING_EVIDENCE; any other Git failure (including a diff failure with no missing object),
  timeout, interrupt or output overflow refuses rather than degrading to a partial plan.
- `NTP-V0-025`: The verb MUST follow the Tasks read contract: one lock-free snapshot-bracketed store
  read (TM-V0-008), then Git observation; it writes no store, intent, journal, reservation, ref or
  object state and its item carries `mutationAuthority:false`. `--help` answers without I/O. Non-goals:
  no admission, reservation, claim, merge, push or integration; no semantic-independence claim (a
  clean textual merge is not a passing build); no cross-repository candidates. Rollback removes the
  verb, its help entry and these requirements; no store migration is needed.

### Shared reservation paths (V1-2119)

Generated spec index files (`docs/specs/REQUIREMENTS.tsv`, `docs/specs/INDEX.json`,
`docs/specs/README.md`) are regenerated by every spec-touching lane and merge mechanically, yet an
exact PATH key on them made a parked attempt (V1-0812) block an unrelated lane (V1-1639) with
RESOURCE_COLLISION. The planner and the lease writer share one collision rule, so the declaration
lives once in the store policy.

- `NTP-V0-026`: The store policy MAY declare `reservations.sharedPaths`, a sorted, unique, non-empty
  array of at most 64 repository-relative PATH keys (a key ending in "/" covers the paths beneath
  it). Two overlapping PATH resources MUST NOT collide when a declared shared path covers each of
  them; an overlap where either side is uncovered (a hand-authored path, or a directory key such as
  `docs/specs/` that no shared path covers) MUST still collide. WHOLE_REPOSITORY and non-PATH
  resources keep their rules. The same rule MUST govern the claim and widen collision check, the
  expired-lease blocking check and the plan's selection against live reservations and earlier
  selections. An absent key keeps every PATH key exclusive; an empty, unsorted, duplicate,
  non-path or unknown member refuses the policy MALFORMED. Submit's OUT_OF_SCOPE check is
  unchanged: a shared path still has to be in the attempt's scope. Non-goals: no per-lane or
  per-ticket declaration, no default shared list, no merge of the shared files. Limit: concurrent
  writers of a shared file rely on regeneration at merge; the store does not serialize them.
  Rollback removes the key from the policy (or reverts the reader) and every PATH key is exclusive
  again; no store migration is needed because reservations record resources, not the rule.

## Fixture input and receipt

All JSON is bounded, duplicate-key rejecting, closed at every interpreted record, and canonical
UTF-8 with one terminal LF. Count and Size are decimal strings under the sibling native profile;
Counts are at most 2147483647, Sizes at most uint64. Digests are lowercase SHA-256. This adapter
admits at most 16384 tickets (NTP-V0-246), 4096 resources per closure, 128 reservations, 256 historical plans,
16 MiB per executor response and 32 MiB aggregate ticket exports. It refuses overflow, never truncates.

Fixture observation profile `corvint-taskman-fixture-observations/0` has exactly:
`profile, sourceCommit, sourceTree, queueId, policySha256, headSeq, intentTreeSha256,
reservationsComplete, attemptsComplete, historyComplete, reservationSet, history`.
`reservationSet` is the sibling `taskman-reservation-set/0` record; its canonical file SHA-256 is
`reservationSetSha256`. It is test-owned observed input, not a claim that the live executor produced
these reservations. Each reservation retains native generation, acceptance revision, resources,
workers, ACTIVE/QUIESCING/BLOCKED_RECOVERY state and coverage; none is released by age.
Unknown tickets and any reservation acceptance-revision mismatch refuse the complete observation;
older resources are never silently released. Reservation resources must cover current declared
effects. Generation is a validated positive fixture value, not an authenticated live generation.
`history` contains ordered `taskman-plan/0` records from fixture-pinned observations. It is not the
currently unimplemented native CONFIG_PIN reader. Every history record must match queue/policy and
have a strictly increasing headSeq no greater than the captured head. History source authority stays
explicitly fixture-only. Unknown or incomplete history is not silently filled.

The inner `taskman-plan/0` is the sibling SPEC 4.3 record without new fields:
`profile, planningProfile, queueId, policySha256, headSeq, intentTreeSha256,
reservationSetSha256, capacity:{maxActiveAttempts,availableWorkers}, entries:[{ticketId,
ticketRevision,resources,closureComplete,state,reason,deferredSinceSeq,blockers}], mutationAuthority:false`.
The capacity field preserves the policy maximum and observed available workers at capture.
Reason and blocker codes retain the sibling SPEC section 11 closed enum. SELECTED fixture entries
use DEVELOPMENT_MODE (state is the selection discriminator); collisions use RESOURCE_COLLISION,
capacity exhaustion LIMIT_EXCEEDED, absent reservations MISSING_EVIDENCE, unsupported capacity
classes UNSUPPORTED, unavailable gates GATE_UNKNOWN and unavailable capabilities CAPABILITY_UNAVAILABLE.
The planner observes no worker pools or roles, so a `requiresPool` or `requiredRoles` requirement is
also CAPABILITY_UNAVAILABLE. A current OPEN decision, scope or blocked typed-escalation question
(`corvint-tasks-escalations-v0.md` ESC-V0-006) is a derived hold and blocks as ESCALATION_PENDING, through the shared `internal/tasks/wire` predicate the
native Tasks reader uses. The planner is stageless, so every `executionPrerequisites` entry
(`corvint-tasks-agent-leases-v0.md` CAL-V0-099) applies: a `GATE_PASSED` prerequisite is
GATE_UNKNOWN and a missing or uncompleted `COMPLETED` prerequisite is PREREQUISITE_UNSATISFIED.
History rejects invalid native IDs, unknown codes, zero/future ticket revisions and deferral
sequences beyond their plan head. Native Identifier and Path bounds remain 128 and 512 bytes. Resource keys use the bound
sibling Resource decoder's Identifier (128), including its extra Path check for PATH; a longer
touch path whose closure cannot fit that record refuses the preview. Whole-repository fallback
uses the fixed key `repository`; collision semantics depend on its class, not that key.

The outer `corvint-taskman-fixture-plan/0` receipt binds `source`, `executorSha256`, `snapshot`,
`observationsSha256`, canonical `ticketDigests`, `plan`, and explicit `unknowns`. Its provenance is
`FIXTURE_OBSERVATIONS_TRUSTED_LOCAL_EXECUTOR`; it is never a production admission receipt.
Errors return a bounded diagnostic without a partial plan. Eligibility reasons precede coverage,
then live collision, previous selection collision and capacity. Blocker arrays sort and deduplicate.

## Acceptance and rollback

Prove the smallest path with a built native task-store executable, a freshly initialized fixture
journal and canonical ticket mutations; then commit fixture source for immutable Corvint indexing.
Read planning twice and compare bytes and complete store/source file snapshots. P0 touching A+B
must beat P1 touching A plus P2 touching B. Also test deterministic ties, holds, stale approvals,
completion/gate dependencies, prefix/non-PATH resources, incomplete coverage and serial fallback,
unknown observations, live reservation capacity, identity drift, canonical digest errors, malformed
pages, hostile queue text, process interruption and explicit history reset.

Native required gates and independent review precede any delivered claim. Exact `cem/0.2` and
`ocm/0.1-experimental` binding, inspected reports, local outcome and strict dogfood completion are
required; structural links alone do not prove behavior. GP remains a separate promotion hold.
Rollback removes the opt-in fixture command/package and registry entry; native store and WQO need
no migration. Atomic admission, CONFIG_PIN, runtime/reservations, exact completion reducer and
real-queue cutover remain in the executor handoff. No automatic task execution is authorized.

Named limitation (V1-0468): the preview reads only `taskman-policy/0` with exactly the closed field
set NTP-V0-001 checks. A policy with any other profile (including `taskman-policy/1`), a non-string
profile, or an optional member the native store admits, such as the opted-in `externalReviews`
member, refuses the whole preview even when its digest binds. The refusal is limited to this
fixture path: `corvint work plan-fixture` is the only caller of `taskman.Preview`, and general task
context does not read the policy through it. It is not a production task-context regression.
Upgrading the reader needs both profile shapes and malformed/unknown refusals qualified, and is
in scope only once a supported product path calls `Preview`.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| NTP-V0-001 | internal/taskman capture and process boundary | TestNTPV0001AdapterRefusals, TestNTPV0001AdapterPolicyProfileBoundary, TestNTPV0001InterruptionRetiresDescendant, TestNTPV0001NativeJSON |
| NTP-V0-002 | internal/taskman capture/observations | TestNTPV0002AdapterFinalDrift, TestNTPV0002ObservationRefusals |
| NTP-V0-003 | internal/taskman priority-first planner | TestNTPV0003PriorityFirst; real native fixture priority counterexample |
| NTP-V0-004 | internal/taskman eligibility | TestNTPV0004Eligibility |
| NTP-V0-005 | internal/taskman direct closure/collisions | TestNTPV0005CoverageAndCollisions |
| NTP-V0-006 | internal/taskman capacity/bindings | TestNTPV0006CapacityAndBindings |
| NTP-V0-007 | internal/taskman fixture history | TestNTPV0007DeferralHistory |
| NTP-V0-008 | cmd/corvint opt-in dispatch | TestNTPV0008CommandBoundary, TestNTPV0001AdapterCanonicalReadOnly; native fixture unchanged-state comparison |
| NTP-V0-020 | internal/tasks/cli/plan_batches.go batchCandidates | TestNTPV0020_CandidatesAreLatestSubmittedOrReadyAttempts, TestNTPV0025_PlanBatchesIsAReadOnlyCLIVerb |
| NTP-V0-021 | internal/tasks/cli/plan_batches.go resolveCandidate, batchDisjoint | TestNTPV0021_GroupsByDisjointChangedPaths, TestNTPV0021_SharedPathCollisionIsSymmetric |
| NTP-V0-022 | internal/tasks/cli/plan_batches.go batchPlan, batchView, batchGit; plan_batches_unix.go | TestNTPV0022_DryChainedMergeSplitsConflicts, TestNTPV0022_DryMergeAddsNoObjectsOrRefs, TestNTPV0022_JoiningCandidateIsCheckedAgainstTip, TestNTPV0022_GitDeadlineKillsProcessGroup, TestNTPV0022_InterruptRemovesTemporaryDirectory, TestNTPV0022_MergeSidesAreViewOnlyCommits |
| NTP-V0-023 | internal/tasks/cli/plan_batches.go planBatches flags | TestNTPV0023_MaxGroupSizeBoundsGroups, TestNTPV0025_PlanBatchesIsAReadOnlyCLIVerb |
| NTP-V0-024 | internal/tasks/cli/plan_batches.go resolveCandidate | TestNTPV0024_UnresolvableCandidateIsReportedUngrouped, TestNTPV0024_MissingNestedTreeIsUngrouped, TestNTPV0024_FatalDiffFailureRefuses, TestNTPV0025_PlanBatchesIsAReadOnlyCLIVerb |
| NTP-V0-025 | internal/tasks/cli plan dispatch, ReadVerbs, command help, batchView | TestNTPV0025_PlanBatchesIsAReadOnlyCLIVerb, TestNTPV0025_RepositoryMergeDriverDoesNotRun, TestCALV0047_AllCommandHelpIsReadOnly |
| NTP-V0-246 | internal/taskman maxTickets (capture, decodePlan); cmd/corvint/taskman_fixture.go help | TestNTPV0246TicketBound, TestNTPV0246PlanFixtureHelpStatesBound |
| NTP-V0-026 | internal/tasks/ticket/collision.go CollideShared; internal/tasks/intent/adopter_policy.go readReservations; internal/tasks/transaction/lease_claim.go collision, expiredBlocking; internal/tasks/transaction/plan.go choose | TestNTPV0026_SharedPathsDoNotCollide, TestNTPV0026_PolicyReservationsSharedPaths, TestNTPV0026_SharedSpecIndexDoesNotCollide, TestNTPV0026_SharedCEMPathsAdmitIsolatedLanes, TestNTPV0026_PlanSelectsAcrossSharedPaths |
