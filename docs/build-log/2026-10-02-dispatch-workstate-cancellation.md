# Dispatcher work-state cancellation: issue 464

Human-owned intent: [GitHub issue 464](https://github.com/beamfall/corvint/issues/464), under the accepted CAL-V0-052/053/056/058 dispatcher contract. This entry records an implementation stage, not full issue acceptance.

## Original evidence and current reproduction

The reporter observed 704 spurious state events around one restart of a 365-ticket queue on `main-25ba271`. That report is retained as historical evidence, not a fresh run.

At base `f51f3c9e6fbfc5a6b219692e8bf46e4e43297a36`, deterministic tests reproduced a canceled active reader returning a successful tick, replacing NONE with UNKNOWN and emitting an alert plus a state event. Cancellation after the first successful release or reap also allowed a second native write, changed backoff and consumed an operator unpark request. Cancellation during re-observation accounted ended workers and consumed requests too. Separate reader child/grandchild cancellation controls confirmed surviving descendants; those fixtures were explicitly retired.

## Reviewed tick-guard stage

Outer dispatcher cancellation now abandons an interrupted observation before publishing reader diagnostics or accepting its state. Tick checks cancellation before supervision, healing and each native release/reap, and between subsequent accounting, unpark and state-comparison checkpoints. Completed supervision and native transaction facts remain recorded. Both bounded and unbounded Run treat outer cancellation as ordinary shutdown.

Ordinary reader failures still yield UNKNOWN plus diagnostics. Explicit successful UNKNOWN values and real state changes retain their usual events. The reader's own timeout remains a reader failure; this stage does not change its timeout or process lifecycle.

Independent plan review initially found missing per-heal cancellation checks and an unresolved reader lifecycle design. The revised plan passed for this bounded tick stage, with full delivery held for the lifecycle dependency. No direct-child test qualifies descendant retirement.

## Evidence and remaining qualification

The dispatcher suite passed after the guard changes, including store-outage supervision and existing worker adoption/cleanup. The targeted dispatcher CLI regression and vet of dispatcher/CLI passed. Added entry-context and durable restart controls passed after their final fixture cleanup change. Corvint affected selected dispatcher/CLI and retained an UNKNOWN frontier with unbounded readers; these focused checks are not repository-wide validation.

Full delivery remains open: inspect the exact qualified shared group-reaping dependency (including error paths on Darwin and Linux), adopt it through the reviewed platform seam, run composed reader cancellation/deadline/leader-exit tests, update the accepted clauses and traceability, bind CEM/OCM, run required checks, obtain independent final review, integrate and complete the native ticket. No helper fallback or ignored cleanup error is accepted by this entry.

Rollback of this stage is the ordinary reversal of the tick guards and their tests. Worker adoption and native-store authority are unchanged.
