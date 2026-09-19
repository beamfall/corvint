# Native taskman fixture planning V0

Owner: Russell Lewis
Date: 2026-09-19
Intent status: accepted fixture scope (decision 0322)
Delivery status: experimental
Authoritative inputs: decision 0322, `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/work-queue-observation-v0.md`, and the source-bound sibling task-store SPEC.

## Agent digest
- Claim: Read-only native fixture priority-first planning with explicit incomplete coverage.
- Status: accepted fixture scope (decision 0322)/experimental; GP and production promotion held.
- Exists: native read adapter, pure planner and focused fixture/refusal tests; frozen pre-edit preregistration.
- Blocked on: live reservations, admission, CONFIG_PIN and GP remain executor/promotion dependencies.
- Read next: Requirements; Fixture input and receipt; Acceptance and rollback.

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

## Fixture input and receipt

All JSON is bounded, duplicate-key rejecting, closed at every interpreted record, and canonical
UTF-8 with one terminal LF. Count and Size are decimal strings under the sibling native profile;
Counts are at most 2147483647, Sizes at most uint64. Digests are lowercase SHA-256. This adapter
admits at most 1000 tickets, 4096 resources per closure, 128 reservations, 256 historical plans,
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

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| NTP-V0-001 | internal/taskman capture and process boundary | TestNTPV0001AdapterRefusals, TestNTPV0001InterruptionRetiresDescendant, TestNTPV0001NativeJSON |
| NTP-V0-002 | internal/taskman capture/observations | TestNTPV0002AdapterFinalDrift, TestNTPV0002ObservationRefusals |
| NTP-V0-003 | internal/taskman priority-first planner | TestNTPV0003PriorityFirst; real native fixture priority counterexample |
| NTP-V0-004 | internal/taskman eligibility | TestNTPV0004Eligibility |
| NTP-V0-005 | internal/taskman direct closure/collisions | TestNTPV0005CoverageAndCollisions |
| NTP-V0-006 | internal/taskman capacity/bindings | TestNTPV0006CapacityAndBindings |
| NTP-V0-007 | internal/taskman fixture history | TestNTPV0007DeferralHistory |
| NTP-V0-008 | cmd/corvint opt-in dispatch | TestNTPV0008CommandBoundary, TestNTPV0001AdapterCanonicalReadOnly; native fixture unchanged-state comparison |
