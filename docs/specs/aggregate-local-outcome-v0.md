# Aggregate Local Outcome V0

Owner: repository owner. Proposed build slice: V1-0611. Date: 2026-10-02.
Intent status: proposed
Delivery status: experimental; NOT_QUALIFIED; source candidate implemented, qualification incomplete
Human job: finish a complete explicitly enrolled change whose current-path admission exceeds the
legacy learning recorder's 200-path limit, retaining all evidence and refusing partial completion.

## Agent digest
- Claim: Explicitly enrolled changes can preserve a complete bounded nonlearning outcome after a bound legacy admitted-path-limit refusal.
- Status: proposed; experimental; NOT_QUALIFIED; source candidate implemented, qualification incomplete. Gate A passed the plan only.
- Exists: legacy 200-path admission and an experimental explicit aggregate candidate with native focused witnesses.
- Blocked on: remaining native qualification, independent source review, final gates and the held recovery attempt (transport-adapted lane admitted 2026-10-04, unexecuted; owner-widened adapter including gitrun/session.go passed cleanup witnesses; independent-review findings fixed with both adapted verifier binaries pinned; awaiting the owner's go).
- Read next: Requirements; Wire and authority; Publication and restart; Bounds and ownership; Acceptance and traceability.

This slice proposes a separate, bounded **nonlearning** outcome and an explicit keyed Finish
selector. It does not increase the legacy trace limit, change learning or accept owner intent.
The independent Gate A passed experimental implementation planning, not implementation or release.
Use Requirements for stable IDs, Wire and authority for exact schemas, Publication and restart
for state transitions, Bounds and ownership for resources, and Acceptance for frozen witnesses.

Inputs: [local completion](local-completion-policy-v0.md),
[daily workflow](daily-change-evidence-workflow-v0.md),
[compatibility freeze](core-compatibility-freeze-v1.md), and [dogfood](../DOGFOOD.md).
Existing DCW-V0-005/013/015 and LCP-V0-005/007 still govern real evidence and fail-closed completion.
Unaccepted DCW-V0-030 changes and `--exception` are excluded.

## Current state and scope

The ordinary recorder admits at most 200 current paths. A retained complete aggregate can exceed
that limit even though its selected checks and CEM are valid. Splitting, truncating or reclassifying
the set to evade admission would destroy the outcome's meaning. V0 proposes one new receipt for
this case, with fresh complete policy acquisition and no trace-store access.

The owning source candidate starts from clean actual public main, not a dirty checkout or the
held CEM lineage. The six unchanged `internal/groupreap/owner*.go` files are an explicit dependency
from public commit `f3fba61ba0a4b0192e89a0ad9f1128c021ae5da9`; their exact dependency manifest is
frozen with admission. Existing main helpers must compile with them. No unspecified donor support
or Owner algorithm repair is admitted. Preserve AGPL provenance and all six dependency tests.

## Requirements

- `ALO-V0-001`: Activation MUST require the explicit enrolled aggregate selector and exact bound
  retained legacy refusal; unknown selectors, changed enrollment and other failures refuse.
- `ALO-V0-002`: Legacy trace, learning, ranking, public fixtures and ordinary /0 behavior MUST
  remain unchanged; aggregate production, admission probes and validation open no trace store.
- `ALO-V0-003`: Candidate and admitted sets MUST use the complete pinned current context-index
  admission policy, including tracked ignore rules and current source exclusions.
- `ALO-V0-004`: Set, byte, task and verification limits MUST be simultaneous and fail closed;
  no splitting, truncation, partial success or hidden retry is permitted.
- `ALO-V0-005`: The aggregate receipt MUST use the exact closed canonical schema in Wire and
  authority, rejecting duplicate, unknown, hybrid, reordered or malformed representations.
- `ALO-V0-006`: Path, envelope and logical binding hashes MUST use the specified domains and
  preimages; raw receipt and snapshot digests retain their distinct meanings.
- `ALO-V0-007`: Validation MUST compare independently supplied immutable/enrollment expectations
  and reacquire full sets; self-consistency or artifact hashes alone cannot satisfy completion.
- `ALO-V0-008`: Aggregate saved state MUST persist its closed discriminator before publication,
  preserving original session, generation, base, plan, intents and observations.
- `ALO-V0-009`: Valid pending states MUST load as active and unsatisfied, and same-selector
  Finish must resume exact explained preservation/publication without enrollment reset.
- `ALO-V0-010`: Every positive status, event, early-Finish, coordination and terminal reuse MUST
  perform current semantic validation; status and event perform no repair or filesystem writes.
- `ALO-V0-011`: Aggregate reports MUST use the closed /1 dispatch, removing legacy complete and
  requiring completionState plus real enrollment; /0 rendering and parsing stay unchanged.
- `ALO-V0-012`: All supported current shared writers MUST use the same worktree operation lock
  before cleanup or publication, with only an opaque matching in-process borrow capability.
- `ALO-V0-013`: Prior raw evidence MUST be completely preserved and source-checked before any
  cleanup or shared replacement; unknown, missing or substituted identities refuse.
- `ALO-V0-014`: History MUST use immutable content-identified snapshots and exclusive bounded
  reservations, with state-before preservation, closed manifest commit and exact orphan rules.
- `ALO-V0-015`: History, staging and reservations MUST obey both the 64-ordinal and 64-MiB
  generation bounds, retaining conservative accounting and never evicting immutable evidence.
- `ALO-V0-016`: Publication MUST issue identities before each per-file atomic rename and stay
  pending through faults; terminal commit requires the actual bound zero-exit strict check.
- `ALO-V0-017`: The aggregate sub-operation MUST share one physical-Git budget and original work
  deadline across preflight, nested acquisition, worker and closing authority checks.
- `ALO-V0-018`: Production and validation MUST run in the contained same-binary worker with
  closed requests/results, exact quota accounting and no arbitrary executable or store selector.
- `ALO-V0-019`: Owned descendants MUST retire under one original total retirement bound;
  only observed RELEASED permits publication, while HOLD retains evidence and never signals late.
- `ALO-V0-020`: Predecessors MUST refuse genuine aggregate state and canonical /1 completion;
  unsupported console, receipt-bundle and platform paths remain explicitly unsupported.
- `ALO-V0-021`: Qualification MUST execute the frozen fault, reload, compatibility, boundary and
  native process witnesses with source-bound raw evidence; skips or simulation are not native pass.
- `ALO-V0-022`: Promotion and held recovery MUST require separate concrete admission, actual
  qualification, independent source review and bound gates; plan PASS is not delivered capability.
- `ALO-V0-023`: Held recovery MAY replace the running binary's verifier roles only through the
  recovery-only `dogfood finish --aggregate-outcome-profile corvint-dogfood-aggregate-outcome/0
  --transport-adapted-recovery <request>` route, while the enrollment's aggregate transaction is
  absent (after the validated legacy refusal) or pending (not COMMITTED, under three issued).
  Ordinary Finish, aggregate Finish and public check keep LCP-V0-014; public check needs COMMITTED.
- `ALO-V0-024`: The request MUST be one closed canonical JSON value binding the selected session,
  plan digest, absolute executable paths, digests, historical revisions/trees and adapter patch
  digests. It MUST match a compiled-in owner admission, including each role's pinned executable
  digest, the plan base tree and the current HELD target/tree before any producer, preservation or
  publication; refusal mutates no evidence.
- `ALO-V0-025`: BASE and TREE executables MUST be distinct regular files equal to the request and
  unequal to the running binary at admission, check start, every run and publication. The report
  MUST record exactly those identities with no override; any disagreement or HOLD stays pending.
- `ALO-V0-026`: The result names `TRANSPORT_ADAPTED_HISTORICAL` with the request digest. It never
  satisfies a pristine historical-binary requirement, rewrites original receipts, or reruns after
  COMMITTED; the adapter source, parity and ownership witnesses stay outside the product source.

## Wire and authority

The only selector is keyed `dogfood finish --aggregate-outcome-profile
corvint-dogfood-aggregate-outcome/0`. Initial activation requires current owner, unchanged frozen
enrollment/plan/generation, clean target, qualifying selected checks and acknowledged report set.
A retained failure manifest binds base, target, plan, CEM, report set, exit and raw stdout/stderr
digests. The original failure must have exit 2, empty stdout and strict typed JSON containing the
exact `admitted-path-limit` code and `ok:false`; a regex stderr match cannot activate it.
For older failures, construct provenance from original retained captures; do not invent a rerun.
An independent read-only `trace.AdmissibleCurrentPaths` probe on the fresh complete pinned set
must return that same error. At <=200 or any other error, refuse. There is no writing-record retry.

Aggregate receipt: all following members required, no others.

| Member | Exact type and value |
|---|---|
| profile | `corvint-dogfood-aggregate-outcome/0` |
| state | `aggregate-recorded` |
| objectFormat | `sha1`; other formats explicitly refuse in V0 |
| base, target, tree | lowercase full 40-hex immutable Git IDs |
| admissionPolicy | `corvint-contextindex-record-paths/0` |
| task | current secret-screened grammar, at most 2000 characters |
| verification | sorted unique nonempty array, at most 50 current-grammar strings of 512 characters |
| outcome | author observation `passed`, `failed` or `blocked`; enrolled completion requires `passed` |
| candidates, admitted | sorted unique nonempty UTF-8 normalized repository-relative path arrays |
| candidateCount, admittedCount | nonnegative integer, exactly the corresponding array length |
| candidateSha256, admittedSha256, envelopeSha256 | `sha256:` followed by 64 lowercase hex |

There are no trace/store/trace-ID/mutates/truncated-ancestry fields. Strict `wire.Parse` rejects
duplicate names, unknown members and wrong types; exact bytes must equal `wire.CanonicalValue`
plus one LF, UTF-8 without BOM or floats. In the following hash definitions, `\0` means one NUL
byte, not two textual characters. Hash outputs use the prefixed digest form above.

| Identity | SHA-256 preimage |
|---|---|
| Each path-set digest | `corvint-record-path-admission/v0\0`, then each sorted path followed by NUL |
| Envelope | `corvint-dogfood-aggregate-outcome/v0\0`, then canonical object without envelopeSha256 and without LF |
| Logical binding | `corvint-dogfood-aggregate-binding/v0\0`, then canonical objectFormat/base/target/tree/admissionPolicy/task/verification/outcome and both set digests/counts, without LF |
| Raw receipt identity | Exact encoded receipt bytes, including its LF; no envelope domain |
| Snapshot | `corvint-dogfood-aggregate-snapshot/v0\0`, then canonical snapshot payload without LF or a self-digest member |

Policy /0 means full `contextindex.Build` pinning, `tracerepopaths.Paths` over index sources plus
tracked ignore rules, and the existing shared current-path classifier. Suffix, generated,
forbidden, LFS, binary, source-size, secret and malformed-path behavior remains exact. Deletions
remain candidates and are admitted only by current authority. No ls-tree approximation, persisted
index shortcut or invented historical-source authority is allowed. Material policy changes need
a new version.

Validation receives expected immutable base/target/tree/policy and the real loaded enrollment.
Task is exactly `Local completion ` plus saved.PlanDigest, verification is normalized
displayChecks(saved.Plan), and enrolled outcome is passed. It freshly compares full arrays,
counts, all digests and envelope against those expectations. Standalone author recording is
unsupported. Different tasks/outcomes with self-consistent hashes do not qualify.

Report /1 retains governed /0 members except `complete`, which is removed. It requires profile
`corvint-dogfood-change/1`, `completionState` exactly `complete` or `incomplete`, the closed
enrollment object `{session,generation,planDigest,bindingSha256}`, and localOutcomeProfile equal
to the aggregate profile. Retain the existing report size cap; parse structurally, not by regex.
/1 serves aggregate evidence only. Hybrid, unknown or downgraded representations refuse.

## Publication and restart

Legacy saved-state bytes omit aggregateOutcome exactly as before. Aggregate state adds that
closed top-level discriminator before any /1 publication, clears stale terminal/coordination,
and becomes active without resetting the original enrollment. Required fields:

| Member | Closed value |
|---|---|
| profile | aggregate receipt profile |
| bindingSha256, receiptSha256, legacyFailureSha256 | prefixed digests of their specified identities |
| attemptCount | integer 1..64, highest adopted reservation ordinal |
| phase | PREPARED, OUTCOME_PUBLISHED, REPORT_PUBLISHED or COMMITTED |
| activeSnapshotSha256 | prefixed snapshot digest |
| preservationState | RESERVED or SEALED |
| expectedOld | closed outcome/report roles; null or `{sha256,bytes,kind}` |
| issued | ordered at most three `{role,sha256,bytes}` entries for outcome/report/check-report |
| snapshots | at most 64 `{ordinal,snapshotSha256,reservedBytes}` references |

Expected-old kind is legacy-outcome, legacy-report, aggregate-outcome, aggregate-report or
failed-recorder-output. The last requires bound failure provenance and never means successful
recording. Absent is accepted only when captured absent. Issued outcome equals receiptSha256;
issued report must have bound closed /1 semantics. An intentionally incomplete report is pending
and cannot admit checking. Check-report needs the actual bound check capture.

Close/validate prepared bytes, save their issued identity under the operation lock, then rename,
then acknowledge phase. Phase is a committed lower bound, so interrupted acknowledgment can
accept only later exact issued bytes with all preceding roles/semantics present. It cannot accept
a missing formerly present artifact, unissued report or unexplained substitution.

| Position | Load, status and event | Same-selector Finish |
|---|---|---|
| PREPARED with exact old/absent artifacts | Load active pending, final-check-required. RESERVED needs valid reservation, preserved state-before and remaining original bytes; effective SEALED needs full closure. | Revalidate original prerequisites/provenance; resume the same reservation and prepared bytes. |
| Issued outcome published; report old/absent | Exact bound outcome and old report identity; active pending. Covers rename before phase acknowledgment. | Revalidate full sets; issue and publish bound /1 report, without trace row. |
| Issued /1 report published; check/terminal missing | Exact outcome/report; incomplete report or absent actual qualified terminal stays pending. | Complete prerequisites and actual strict check; issue check-report only with bound capture. |
| COMMITTED | Satisfied only with complete current semantic evidence, checks/review and actual zero-exit terminal. Stale prerequisites are unsatisfied. | Valid identical success is a read/validation no-op before reservation: no state change, ordinal, duplicate or publication. |

Finish must load valid pending state before evaluating/resuming. Same-selector means original
session and identical explicit aggregate selector. Missing/different selectors cannot clear or
downgrade it. Every positive lifecycle.evaluate/status/event/early-Finish, terminalCurrent,
coordination-cache, checkCoordinator/final publication and public native Check/Seal reuse calls
current semantic validation. No artifact-digest-only aggregate success cache is allowed.
Status/event neither create locks/directories nor stage/reserve/adopt/repair/write state or evidence.
They bracket state/artifact bytes and clean immutable identity before/after bounded validation.
Timeout or HOLD never means satisfied. Current event paths must be exercised, including their
existing repeated Evaluate calls. Public Check/Seal require complete /1 evidence; pending refuses.
A supported /1 report replacement first returns state to active REPORT_PUBLISHED, clears terminal
and issues replacement bytes under the lock. Target drift cannot be repaired with a new base/plan.

The shared lock is `<git-dir>/corvint/local-completion/operation.lock`, retaining nonblocking
contention/static-symlink checks. All supported current Change/Check/native Seal writers acquire
it before cleanup. Enrolled calls borrow only an opaque process-local capability bound to the
canonical matching GitDir; no environment token or serialized PID confers ownership. Lock order
is the one operation lock followed by existing artifact publication. Worker has no publication
lock or shared writes. Public Change with aggregate marker/artifact refuses enrollment-required.
Public /1 Check/Seal load real owner/state under lock and verify enrollment/current checks/review;
report-provided identity alone is not authority.

Snapshot path is `history/<binding-digest>/<snapshot-digest>/manifest.json` within the existing
generation's private aggregate-outcome directory. Closed snapshot profile is
`corvint-dogfood-aggregate-snapshot/0`, with bindingSha256, sourceStateSha256 and sorted entries
`{role,present,sha256,bytes}` for state-before, prior outcome/report and retained failed
stdout/stderr/check captures. Absent entries have present=false, null digest and zero bytes.
Roles are fixed admitted artifact roles, never arbitrary paths. State digest is the exact raw
pre-update state. Different progress/failure bytes yield distinct snapshots under one binding.

Under lock: reserve create-exclusive ordinal 001..064 with closed manifest binding ordinal,
logical/snapshot digests, pre-update state/shared identities, entries and maximum reserved bytes;
checked close precedes install. Copy/close/verify immutable state-before **before** updating state.
Then save reservation reference/attemptCount as RESERVED, copy/close/verify remaining payloads,
publish exclusive final snapshot manifest, and acknowledge SEALED. Valid final closure can prove
effective SEALED after interrupted acknowledgment without a read-side save. No shared replacement,
old-stderr cleanup or trace attempt precedes closure. Bounded read-only preparation deriving the
initial binding is allowed under the unchanged budget/capacity rules, without ordering exception
or deadline reset. Missing/corrupt final payloads are invalid, not completion.

A reservation before state update is inert. Same-selector recovery may adopt at most one valid
consecutive orphan with exact original bytes, not steal a stale lock. Gaps, conflicting ordinals,
multiple unexplained orphans or unavailable original bytes refuse. Referenced RESERVED state needs
its already preserved state-before and remaining reserved original bytes. Resume reuses its ordinal;
reload alone is not an attempt. Distinct new observations needing preservation reserve a new
content snapshot; identical sealed bytes reuse their existing reference. Never overwrite snapshots.

Before new cleanup or replacement, preserve all prior raw evidence completely and compare expected
old hashes under lock. Unknown, unreadable, symlinked, drifted or unpreservable evidence refuses.
History is excluded from blanket stderr cleanup. Legacy attempts capture bounded unique owned
stdout/stderr stages from the outset; both must close before typed refusal parsing. Worker output
and outcome/report/check append use unique bounded checked-close same-directory stages. Sequence:
preservation closure, state marker, issued outcome rename, issued /1 report rename, actual strict
check/capture and issued append, then terminal/satisfied state. Later failure leaves pending evidence.
This is per-file atomic visibility and fail-closed sequencing, not multi-file or power-loss durability.
SIGINT/TERM remove only unreferenced owned temporary stages after observed descendant retirement;
immutable issued/prepared retry bytes remain. SIGKILL may retain stages/lock. No automatic lock theft,
deletion/reset, defer assurance or process killing is added. HOLD retains ownership/evidence.

## Bounds and ownership

| Resource | Simultaneous bound |
|---|---|
| Admitted / candidate paths | 512 / 4096 |
| Encoded receipt / diff-name output | 4 MiB each; not a total memory claim |
| Existing tree / batch source bytes | 64 MiB / 128 MiB |
| Source bytes / indexed sources | 1,000,000 per source / 200,000 sources |
| Status / Git stderr | 8 MiB / 64 KiB |
| Physical Git admissions | 64 shared atomic charges; failed/start-refused admission is not refunded |
| Work / individual Git | original 300 seconds / at most 10 seconds |
| Existing Build context | additional stricter 30 seconds |
| Retirement | one original 20-second total bound; post-reap min(remaining, 2 seconds) |
| Generation history, staging, reservation | 64 distinct ordinals and 64 MiB total |

Count committed payload/manifests, valid orphans, live stages and unspent reservation maxima before
reservation. Derive maxima from actual old/known prepared sizes and fixed remaining capture limits.
Count each fixed owned path once, without undeclared cross-path dedup discounts. Close/rename fault
files remain charged. Only observed cleanup reclaims unused reserved bytes, never ordinals or
immutable evidence. HOLD/unobserved cleanup retains conservative reservations. The 64th large
attempt is not promised to fit 64 MiB. Capacity refusal starts no new worker/publication.

The work budget begins before fresh admission/preflight and ends after closing authority recheck
and publication; validation has no artifact write. It excludes all Finish/CEM/OCM work, selected
tests, builds and seal hooks. One mutex-protected context budget charges every physical Git spawn,
including nested NewDefaultBudget users, contextindex.gitRaw, status callbacks/probes, opening and
closing builds, resolution/diffs and parent closing checks. No-option old execution stays exact.
Instrumentation must inventory every reachable spawn; an uncharged helper is a qualification
failure requiring scope refinement. Parent reserves eight remaining admissions for closing work;
worker receives only remaining quota/time, returns exact charges, and missing accounting refuses.
Probe, production and classification run sequentially without in-operation worker retry or timer reset.
If actual aggregate acquisition cannot fit, retain measured refusal and revise the versioned proposal.

One contained native same-binary hidden `dogfood-outcome-worker` verifies its owned group through
gitstatus.EnableOwnedWorker. Opt-in nested Git inherits that group. Timeout/cancel kills only the
direct unreaped Git leader, uses at most two-second WaitDelay and fails worker; parent retires the
whole worker group through the unchanged pinned Owner, including normal-exit descendants. Never
signal an inherited-group child PID as a group ID. Legacy nonworker runner behavior stays unchanged.
Explicit native base/tree verifier runners also require contained cleanup. Arbitrary external
aggregate producers/validators are unsupported.

Worker request profile `corvint-dogfood-outcome-worker/0` has only required mode=produce|verify,
expected immutable/policy/task/verification/outcome fields, remainingGitOperations=1..64,
remainingWorkMilliseconds=1..300000, and receipt=null for produce or exact object for verify.
Production response requires only profile/mode/consumedGitOperations/receipt/legacyAdmission
(strict typed refusal); verification response requires only
profile/mode/consumedGitOperations/verifiedEnvelopeSha256. Requests/results are closed canonical
JSON. Receipt stays at most 4 MiB; framed input/output at most 4 MiB+4 KiB; stderr at most 64 KiB.
Exit zero requires closed parse, exact expectation/accounting agreement and owner RELEASED.
No arbitrary argv, command, output path or store selector is accepted.

Start the single original RetirementBound at first normal-exit retirement or cancellation. All
stop/exit/pre-reap/reap/post-reap stages consume it; never refresh it. Stop only while live when
needed. Accept only observed RELEASED/absence. HOLD is terminal without later numeric-PID signals.
Earlier caller deadline remains authoritative; insufficient retirement admission refuses before
spawn. These bounds cannot guarantee return from an uninterruptible OS call. Only actually
executed native Darwin/Linux tuples qualify. Windows, escaped sessions and foreign PIDs do not.

## Failure modes and compatibility

This spec owns these exact new typed reason strings:
`aggregate-profile-unsupported`, `aggregate-object-format-unsupported`,
`aggregate-enrollment-required`, `aggregate-legacy-failure-unverified`, `aggregate-not-required`,
`aggregate-schema-invalid`, `aggregate-binding-drift`, `aggregate-source-set-drift`,
`aggregate-candidate-limit`, `aggregate-admitted-limit`, `aggregate-receipt-byte-limit`,
`aggregate-git-operation-limit`, `aggregate-operation-deadline`, `aggregate-cleanup-hold`,
`aggregate-history-bound-exceeded`, `aggregate-prior-evidence-drift`,
`aggregate-publication-failed`, `aggregate-worker-invalid`.
Existing originating dirty/path/secret/Git-start/output/timeout errors never activate aggregate.
Missing or stale terminal proof uses existing final-check-required policy; no new satisfied shape.
Bounds/publication errors never emit successful /1 completion or terminal.

Predecessor strict saved-state readers reject the new top-level discriminator at every phase,
including status, Finish and actual event paths. Canonical /1 lacks legacy complete:true. New
readers refuse hybrids/downgrades. A maliciously rewritten /0 object cannot be made tamper-proof
for an old digest-only checker by changing the new binary; that claim is explicitly excluded.
Console /1 support is absent and visibly refuses profile; receipt-bundle's dogfood entry yields
ReasonUnreadable. Tests/documentation disclose both limits without adding exporter/UI support.
Old unsupported writers and unrelated same-UID Git/filesystem writers cannot be controlled by this
lock; real coordination and pre/post observed identity checks are required, not sandbox claims.

Known limit (transport-adapted recovery): a worker's startup ownership check,
`gitstatus.EnableOwnedWorker`, proves only that the worker leads its own process group. It does not
prove that the parent Owner created or will retire that group. Retirement assurance comes from the
coordinator's pinned Owner and observed RELEASED, not from the worker. The admitted historical
adapter patches and their pinned binaries cannot change this check.

Non-goals: learning schema/ranking/slot-weight changes, higher legacy limits, standalone recording,
network/account/daemon, optional products, CEM1 public fixture changes, release/install, wholesale
lineage imports, automatic fallback/reset, hostile same-UID confinement or power-loss durability.

## Acceptance and traceability

Freeze the case/oracle manifest before judged runs. All new witnesses below are **NOT_RUN**.
Planned source paths are prospective, not existing test-symbol evidence. Go **1.27.1** is the
current qualification toolchain. The two Go 1.24.13 results are historical portable-acceptance and
structural-refusal compatibility evidence only; they are not current build/CI qualification.

| Oracle | Requirement IDs | Planned witness paths / required observation |
|---|---|---|
| activation and trace isolation | 001,002 | cmd/corvint/dogfood_record_test.go; internal/trace/record_test.go; exact bound refusal, trace opener count zero, legacy bytes unchanged |
| complete policy and receipt | 003..007 | internal/tracerecordrepo/aggregate_outcome_test.go and adapter_test.go; full current-policy parity and independently expected fields |
| pending restart and terminal reuse | 008..011 | internal/localcompletion lifecycle/storage/Unix tests; actual new-process load/status/event/Finish at every boundary, read-only hashes |
| writer contention and preservation | 012..016 | internal/dogfoodoperation/lock_test.go; internal/dogfoodflow change/check/worker tests; exact old identity, state-before closure, fault and capacity refusal |
| physical budget and owned cleanup | 017..019 | internal/cem/gitrun/operation_budget_test.go; internal/contextindex/git_test.go; unchanged Owner tests and actual native RELEASED witnesses |
| dispatch and predecessor | 020 | cmd/corvint event/portable/worker tests; console/receiptbundle tests; actual original-main predecessor refuses genuine /1 state and legacy /0 still works |
| source-bound qualification | 021,022 | immutable candidate manifests/raw argv/exit/stdout/stderr, fresh process phase snapshots, independent review, CEM/OCM and native gate receipts |
| transport-adapted held recovery | 023..026 | cmd/corvint/transport_recovery_test.go TestTransportAdaptedRecoveryNative; internal/localcompletion/transport_recovery_test.go TestTransportRecoveryRequestClosedCanonical, TestTransportRecoveryAdmissionsClosed; refusals with unchanged evidence bytes, pending disagreement, public-check refusal, recorded A/B identities, committed refusal |

Mandatory inventory includes: legacy 200 success/201 refusal; short-path aggregate 201/299/512
success, 513 refusal, candidate 4097 refusal; byte exact/+1 independent of capacities; source/total
index bounds; generated/LFS/binary/unsupported/deleted/ignore/forbidden/secret/malformed policy cases;
task 2000/2001, command 512/513, verification count 50/51; omitted/foreign/duplicate/reordered sets;
every identity/hash/profile/JSON mutation; physical-spawn 64/65; slow opening and closing timeout;
start/write/close/rename faults; old artifact/conflict; concurrent public/public and public/enrolled;
normal-exit descendants, TERM-ignoring descendants and held pipes, actual SIGINT/SIGTERM RELEASED;
SIGKILL retained stage/lock refusal; real cached current/predecessor status/Finish/event.

Inject before/after reservation install, state-before copy/close, reservation-state save, remaining
payload closure, snapshot install and marker save, then each issued save/rename/phase acknowledgment,
strict-check capture/append/close/rename and terminal save. Restart a **new process** each time;
observe current pending status/event unsatisfied and unchanged bytes, predecessor refusal and
same-selector Finish reaching genuine success when fault removed. Missing/different selector
cannot downgrade. Exercise cache/early-Finish predicates, orphan adoption/conflict/missing originals,
two distinct same-binding snapshots, identical reuse and success no-op, 64/65 ordinals and exact
64-MiB/+1 including orphan/fault/live-stage/HOLD bytes. COMMITTED stale checks/review/target, supported
report invalidation, unknown phases/roles/versions/unissued/substituted bytes and corrupt closure
must never satisfy. Drive one fake retirement clock across every stage without deadline refresh,
plus actual native absence evidence; skipped/HOLD/emulated/cross-built cases are not native passes.

Frozen commands are the admission plan's six exact argv arrays: eight-package test and vet;
selected actual CLI tests; console/receiptbundle/specindex tests; the eight named spec/contract
checks; and core-n1-replay. Use GOTOOLCHAIN=local, GOENV=off, GOWORK=off and Go1.27.1; package timeout
30m remains the hang detector. Run Corvint affected first and retain unsupported/unknown results.
No exhaustive make gate or full host matrix is added by this scoped issue. Command failure stays
visible and cannot be replaced by a narrower unit claim. Qualification retains binary/source/tree
hashes, licenses, exact argv/exit/raw streams, read-only before/after bytes and native runtime tuple.

## Promotion, recovery and rollback

Experimental implementation admission, native effects/claim, actual source qualification and held
recovery entrypoint admission are separate. Gate A PASS does not satisfy any later gate. Source
completion needs independent review, required checks, fresh CEM/OCM/report inspection/check/seal,
integration and native ticket completion. Full CEM1, S1-S9, release and external/native matrix gaps
remain open. Never advertise this prototype as an official release or install dirty source.

The selected prospective held recovery uses a clean qualified fixed native coordinator with
independently built original-base and exact-held verifier binaries. The owner's 2026-10-04
decision admits only the transport-adapted lane of `ALO-V0-023..026` for scoped implementation
and testing; executing it against the held store still needs post-review authorization. An old wrapper's CORVINT_BIN override does not replace
its old coordinator. Do not import ALO into the held source, expand its frozen three intents,
reset session/base/plan or claim changed-target satisfaction. Preserve held a794, original406,
original session/plan/generation, four actual checks and all retained failures. Independent verifier
roles require actual executions/agreement/zero exits; fixed/fixed reuse is insufficient.

Rollback: retain previous installed binaries and original held evidence; prototype binaries stay
isolated. Revert candidate source under normal reviewed authority if needed. Preserve any active
aggregate discriminator, pending bytes, reservations and history; predecessor refusal is expected
and must not be bypassed by deleting state. Diagnosis/recovery requires current qualified reader
and explicit scope. No evidence eviction, automatic lock theft or downgrade/reset is rollback.


## Experimental source candidate and focused evidence (2026-10-03)

This section records source and development observations. It does not change the proposed intent,
waive the mandatory inventory above, promote the profile, or replace the six frozen commands.
The requirement definition positions above are preserved for the separately owned registry pass.

- Evidence for `ALO-V0-001..007`: `internal/tracerecordrepo/aggregate_outcome.go` constructs and independently
  reacquires the complete receipt; `internal/trace/record.go` shares the existing classifier while
  keeping the learning cap at 200. Tests cover real 201/299/512-path acquisition, 513 refusal,
  4096/4097 candidates, an exact 4-MiB canonical receipt and plus-one refusal, metadata limits,
  digest domains, self-consistent omissions, dirty source and zero trace-store opener attempts.
  Current `tracerepopaths.Paths` authority includes every `index.Sources` key, even a retained
  opaque source with `Valid=false`; the aggregate does not add a parser-validity admission rule.
- Evidence for `ALO-V0-008..016`: local-completion state, preservation and publication retain the original
  enrollment, immutable snapshot history and issued-prefix rules. Separate-process fault tests
  cover 14 preservation boundaries and ten outcome/report publication boundaries. The native CLI
  witness completes a genuine reviewed, frozen-check workflow after the actual legacy refusal.
  It verifies unchanged evidence on status and repeated success, pending selector refusal, and
  actual pre-change predecessor status/Finish/event refusal across all four phase discriminators.
  Lower-bound phase dispatch fixtures are distinguished from actual crash witnesses.
- Evidence for `ALO-V0-017..019`: physical Git admission is shared through context. The native outcome worker
  verifies group ownership before ordinary startup. Strict aggregate checks execute native
  verifiers through the separate `dogfood-verifier-worker` entrypoint, whose closed argv grammar
  permits only the exact impact, CEM status, dogfood-OCM status and bound authority-query abstention
  replay reads used by Check. It inherits
  the owned group without charging excluded CEM/OCM work to aggregate acquisition. Executable
  hashes are observed before and after each verifier run. Old verifier binaries without this
  entrypoint cannot supply aggregate verifier qualification; the held recovery entrypoint remains
  pending. No arbitrary aggregate producer or validator was added.
  Both worker paths use bounded streams and the unchanged pinned Owner. They reserve retirement
  admission before spawning and retain the earlier caller deadline. Actual Darwin tests observe
  descendant absence after normal exit, cancellation, timeout and output overflow.
- Evidence for `ALO-V0-020`: console and receipt-bundle tests retain explicit `/1` refusal, including
  `ReasonUnreadable` in the bundle. Legacy default dispatch remains separate.
- Evidence for `ALO-V0-021..022`: independent source review, the complete frozen qualification inventory,
  the six terminal receipts, Linux execution and the held recovery integration remain outstanding.
  Development checks are not terminal gate receipts. Source, raw invocation and result hashes are
  retained by the task's private qualification packet for the coordinator's exact-source review.
- Evidence for `ALO-V0-023..026` (2026-10-04): `internal/localcompletion/transport_recovery.go`
  parses request profile `corvint-transport-adapted-held-recovery/0` with fields `base`, `tree`
  (`adapterPatchSha256`, `historicalRevision`, `historicalTree`, `path`, `sha256`),
  `planDigest`, `profile`, `qualification` and `session`, as `json.Marshal` bytes plus one LF.
  The only admission is #443's key/plan with BASE `406f9dc3`/tree `e0dfeaa5`, HELD
  `a79439af`/tree `d6d6b890` and the two pinned adapter patch digests (`c610b712`, `6f58ea18`;
  owner-widened to `internal/cem/gitrun/session.go` on 2026-10-04); tests link extra rows only
  with `-ldflags -X`. Verifiers still run through the `dogfood-verifier-worker` route; their public
  route refuses. Request provenance is returned on stdout, not stored in local-completion state.
  Refusals are `transport-recovery-request-unavailable`, `transport-recovery-request-invalid`,
  `transport-recovery-fixed-verifier`, `transport-recovery-not-pending`,
  `transport-recovery-enrollment-mismatch`, `transport-recovery-not-admitted`,
  `transport-recovery-verifier-not-admitted`,
  `transport-recovery-base-mismatch`, `transport-recovery-held-mismatch`,
  `transport-recovery-identity-unavailable` and `transport-recovery-identity-drift`; a substituted
  runner invoked outside the aggregate check fails `transport-recovery-requires-aggregate-check`.
  `TestTransportRecoveryRequestClosedCanonical` and `TestTransportRecoveryAdmissionsClosed` cover
  request parsing and the closed admission list. The row also pins each role's adapted binary
  (BASE `0a68e6fe`, HELD `e308360e`, from the adapter build witness); another executable digest
  for admitted provenance fails `transport-recovery-verifier-not-admitted`, covered by
  `TestTransportRecoveryAdmissionPinsVerifierBinaries` and the native `wrong-binary-digest` case.
  `TestTransportAdaptedRecoveryNative` covers Darwin only; the adapter build, parity and owned-group
  cleanup witnesses are recorded in the 2026-10-04 build log. Linux and Windows are NOT_PRODUCED.

The source-level fault hooks are private repository test seams, not public options. No production
environment variable activates faults, chooses an aggregate producer, or bypasses preservation.
The tests found and repaired a missing strict-JSON tag and a post-manifest/pre-state crash
acknowledgement; all failed runs remain retained. A pending state never repairs that acknowledgement
on a read; explicit resumed preservation does.

A frozen 32-boundary native controlled-error campaign exercised new-process recovery across
preservation, publication, actual strict-check captures and terminal save. Its original run had
28 passing boundaries and four report/check-report staging failures. Ordinary publication errors
now retire only the anonymous temporary copy, preserving prepared bytes and issued identities;
the focused four-boundary repair is separately retained. These controlled errors are not SIGKILL
or power-loss witnesses. Actual process-crash publication tests remain separate evidence.
The actual current stop event after successful Finish returned a read-only fail-closed
`final-check-required` result; its captured private evidence hashes were unchanged. This does not
qualify positive event completion under the hook's shorter deadline.

Repair1 retains the first independent review's three source findings and their regression evidence.
Caller HOLD is sticky in the existing writer capability: its lock, Owner reference and bounded
capture identity survive ordinary release; later borrowing, worker starts and publication refuse.
No Owner algorithm or event deadline changed. The query exception admits only the existing exact
authority-start replay and retains its governed task validation and same-pin executable checks.
Strict-check streams now exist as bounded exclusive private files from outset, with an unqualified
start descriptor. Ordinary close records the observed prefix independently of success validation;
failed publication retains its diagnostic. Cancellation/HOLD never turns that observation into a
qualified check. KILL leaves an incomplete observation and lock; no automatic lock recovery exists.
All bytes remain charged to the existing reservation/history limits. Focused native capture-boundary
signal tests and real query replay are development evidence, not the remaining full qualification.
