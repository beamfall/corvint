# Independent implementation Gate A — 2026-09-19

Reviewed `/private/tmp/corvint-taskman-evidence-20260919/implementation-plan.md:1–12` after the owner approved amendment A–E. Scope: sequencing and read-only fixture adapter/planner safety; no code review, repository edits, nested delegation or full gates. Task-fit Astra/high; parent supplied weekly usage 10%.

**Verdict: PASS for the plan with the concrete implementation constraints below.** No further owner permission is required for the described fixture work. This is not verification that preregistration has already been frozen: parent must retain that actual artifact before any Corvint Go edit. The approved amendment resolves the earlier pin/harness/authority blockers within its stated scope.

Path keys: C = `/private/tmp/corvint-taskman-native-planning-20260919`; T = `/Users/russelllewis/projects/corvint-tasks`.

## HIGH

None remaining in the proposed sequence when its stated incomplete-coverage and missing-observation refusals are implemented as specified below. These are boundary requirements for code review, not additional owner decision requests.

## MED — Use native coverage qualification, not WQO's closure boolean alone

Plan line 8 correctly requires incomplete paths to remain visible. Existing `C/internal/workqueue/collision.go:179–224` retains an untracked or unparsed path as itself and still returns true; this is intentional WQO closure semantics, not proof that a native ticket has complete semantic collision coverage. Reuse its path set, but qualify native closure independently: source/index revision agrees with the outer source identity; unsupported/unparsed/unindexed paths remain explicitly incomplete. Never change WQO's existing behavior to repair the new profile. `T/docs/SPEC.md:963–968` permits only declared QUALIFIED plus complete plan coverage, or policy-enabled WHOLE_REPOSITORY fallback with zero live reservations; externalUnbounded always blocks. Prefix paths and special resource classes must retain their existing collision semantics (`T/docs/SPEC.md:1309–1315`).

Minimum evidence: qualified indexed path; malformed/unparsed and untracked path; unavailable/stale index; whole-repository fallback with zero versus one live reservation; P0 conflict with two mutually compatible cheaper tickets.

## MED — Preserve eligibility unknowns beyond reservation coverage

`T/docs/SPEC.md:301` binds GATE_PASSED dependencies to a current-acceptance-revision, non-stale PASSED result from the dependency's most recent attempt. `:341–346` additionally requires current RUN approvals where applicable, no live attempt, allowed execution class and source/writer rules. The current queue status deliberately emits attempts and publication as NOT_OBSERVED (`T/internal/cli/cli.go:930–931`); its status counts are not sufficient eligibility evidence. Reservation fixtures alone do not establish gate results or arbitrary attempt absence.

Minimum implementation: explicitly bound complete fixture observations may supply the facts actually represented by their schema; otherwise BLOCKED with the corresponding uncertainty. GATE_PASSED can remain blocked in this slice rather than adding an unnecessary gate-result subsystem. An empty supplied reservation set is different from a missing observation. Capacity is computed from observed live usage and policy limits, not an unbounded caller number. Preserve ARCHIVED-from-COMPLETED dependency semantics. Unknown state never becomes zero headroom or a satisfied dependency.

## MED — Audit bracketing is sufficient only with full bounded capture validation

The selected trusted-executor boundary avoids reimplementing the journal. `T/internal/cli/receipt.go:14–25` binds audit head/intent identities to the envelope; `:33–40` supplies structural/projection/semantic status and staging. `T/internal/cli/cli.go:609–648` exports canonical records with digest/length and uses **offset + actual item count**, because a byte bound may truncate before the requested limit. `T/internal/cli/cli.go:909–927` supplies queue fixture/writer/policy/count and barrier evidence. `T/internal/snapshot/probe.go:223–246` exposes only the named envelope identities, not every byte of the internal head/barrier; do not describe this as hostile-editor exclusion or linearizability.

Minimum implementation: require successful known command/profile, non-null complete equal snapshot identities at every response, matching queue/head/intent/policy bindings, appropriate structural/projection audit success, no staging/pending redo, and the explicit fixture declaration. Decode canonical bytes under bounds; check exported record digest/length/id against its record; enforce stable totals, sorted unique IDs, exact offsets, progress and complete final count. Hash the captured policy according to its real profile definition, compare to queue status, and reject symlink/oversize or malformed input. Fixed argv and a bounded subprocess deadline/output envelope only; queue prose cannot choose executable, flags, environment, files or policy. Retain executor digest and explicit trusted-local/observed safety premise. Process-group cleanup and interruption witness belong to this adapter as well as the GP runner.

Bracket Corvint source/index identity separately: task-store snapshot equality alone does not pin Corvint's Git tree. The outer experimental receipt must bind the actual source/index, executor and fixture observations used. Once captured, the pure planner consumes those immutable bytes; it must not reread mutable state during selection. Unknown audit qualification fields stay visible and cannot silently become runtime or production qualification.

## Preregistration versus promotion

The approved C exception permits a genuine content-addressed pre-edit baseline instead of CONFIG_PIN now, retaining the identical-document CONFIG_PIN promotion hold. Approved B permits current-base corpus repinning and an external native comparison runner. Therefore no new approval is needed solely because allocations, I/O, runtime conditions, platform controls or GP measurements remain NOT_RUN.

A complete preregistration still needs the closed `taskman-perf-baseline/0` identities and required row inventory, exact harness source/digest and retained immutable corpus/index/task state identities, conditions, sample counts, witnesses, budgets and decision rule. Put unavailable-workload explanations and control-qualification status in a bound sidecar if they do not fit the closed profile; do not invent unsupported profile fields or pretend null fields are measured values. Retain STOPPED/IDLE_INITIALIZED/MAX_ADMITTED, COLD/WARM and all current command entrypoints. Help/startup/negative-path rows are genuine separate workloads, but cannot stand in for a command's meaningful normal workload; missing normal fixtures remain named NOT_RUN obligations. No baseline timing or qualification may be established from the observed contended host.

The diagnostic runner may exercise a safe row early to demonstrate executable orchestration and cleanup. Its current report always remains NOT_RUN and names a diagnostic profile (`native-gp/main.go:206–213`); neither a successful diagnostic exit nor a computable wall bound satisfies GP. Do not spend a full gate on the diagnostic harness, absent evidence that doing so advances the native fixture delivery.

## Sequence and completion

The plan's order is appropriate: immutable external baseline preparation → record the already-approved owner decision/spec/registry → smallest native read-only fixture path → priority/collision/unknown-state regressions → one integrated independent code review/repair cycle → frozen final-source native gates and exact CEM/OCM inspection. No duplicate review of unchanged executor source is needed. Preserve its verified manifest binding and exercise only the integration path used.

History remains an explicit fixture input. Use only observed matching ticketId/acceptanceRevision history; preserve reset-on-selection and pin semantics, and do not invent deferral age from absent history (`T/docs/SPEC.md:1330–1335`). If omitted history would make the closed plan's null ambiguous, require an explicit complete empty-history observation or refuse; explain fixture provenance in the outer receipt.

No claim of real admission, live reservations, native runtime, completion acceptance, atomic stale-plan refusal or no slowdown follows from these tests. Those remain the named executor/GP dependencies, without blocking the approved experimental fixture delivery after actual preregistration freeze.
