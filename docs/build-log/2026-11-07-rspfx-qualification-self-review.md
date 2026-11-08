# Qualification self-review repair: execution cost and admission scope

Requirements: RSP-V0-034, RSP-V0-036, RSP-V0-053, RSP-V0-056.
The two requested review findings are repaired. No live runner qualification or
production selection promotion is claimed.

## Decisions and closed forms

Replay preserves Request.Budget and the original inventory/collection timings.
Record.Wall for retained replay accounts for the sum of those original measured
phases across both parity models and every draw; it no longer reports the replay
invocation's duration as execution cost. The CLI context bounds evaluation
separately and checks the original budget against the operator's current ceiling.
Replay input has no separate original end-to-end elapsed field: setup/teardown
and overlap are NOT_OBSERVED, not fabricated. The aggregate is the conservative
accounting of retained phases, not a new claim of elapsed live-run measurement.
Live Run continues to measure its own elapsed wall time.

The cost soundness class is substitution or omission of execution cost during
re-evaluation. Admission also checks retained phase totals, so an older record
with a cheap replay Wall cannot hide expensive original observations. The closed
allowlist is traced/untraced timing rows with nonnegative inventory and collection
durations and an overflow-safe sum within the original positive budget; that
budget must fit the trusted ceiling. Unknown models, negative durations and
arithmetic overflow retain FULL. Tests cover one hour-long collection, a total
above budget despite individually small durations, and an unknown model.

Ordering is partitioned by runner, corpus repository, execution model, target OS
and architecture, configuration digest and semantic environment digest. Plugin,
runner, tracer and selector versions, corpus commit and verifier revision remain
invalidators inside each scope. The newest admitted failure there blocks older
passes, without invalidating another scope. Unknown target scopes have no
admitted record and stay FULL; there is no inferred cross-target reuse.

The first CLI package test exposed an existing decoder incompatibility:
encoding/json/v2 had no default representation for the time.Duration fields
emitted as numeric nanoseconds by encoding/json. Selecting FormatDurationAsNano
restores that existing input representation. Unknown and duplicate members still
refuse. No field, wire shape, error code, exit code, stderr format, golden, or
existing test assertion is changed.

## Verification

The requested affected command used base
9761ea1f459c0d3b3c963953e24a76eaeda16370 and selected both changed packages
and the preceding repair's testevidence package. Its installed binary reported
self-binary-stale; 90 LANGUAGE_FRONTIER, 39 UNOWNED_DIRTY_PATH and 6
NO_SELECTABLE_TEST unknowns remain in the raw receipt. A retry made while the
spec changed refused unsupported-affected-drift; a stable retry succeeded.

Whole-package tests ran through script/heavy.sh, one package per command, with
GOTOOLCHAIN=local, -count=1 and -timeout 30m, no identity exports or lock override:

- internal/runnerqualification: PASS (0.293 s).
- tools/runner-qualification: initial FAIL on numeric-duration decoding; after
  repairing that blocker, PASS (0.243 s), including the existing replay tests.
- internal/testevidence: PASS (0.177 s), including the preceding repair's tests.

New witnesses: TestRSPV0056ReplayRetainsExecutionCost,
TestRSPV0056AdmissionCostForms, TestRSPV0056CommandPreservesExecutionBudget and
TestRSPV0053IndependentQualificationScopes. Existing package assertions remain.
Focused spec/citation checks and final CEM check results are in the lane handback.
Independent review, repository-wide gates, selection-baseline and live external
qualification are NOT_RUN by instruction; the host owns them. No task writes,
publication or seal is performed.

Self-development routes: affected advice and original evidence inspection used;
pre-change capture reused under the repair instruction; final CEM binding/check
attempt retained in the handback. Other runtime, learning and release routes do
not apply. Rollback: revert this repair; doing so restores both review findings.

Raw receipts remain outside the repository in rspfx-qual-evidence. Digests:

- self-review-affected.json: `4b711ae88c3e3f546e7d0fa1db1cc5d4ba19cc66b0757732c921c575ed701107`.
- self-review-runnerqualification.log: `30d881b2d409ddf8c711923877458b33cc27539b7c60d4aa37fa6f7cc8a469c5`.
- self-review-command.log: `13c1aba7e2481f3516964fbce0256fad10b1af226169f129bed3b4fff1b0be35`.
- self-review-command-final.log: `44fe5c553bdf886759db131cad21cf7d031d51297e80612e7782c06754864c7e`.
- self-review-testevidence.log: `096ddaa4adf3b81103bf3b035beebfd9bdde1bb8f37f51795dfcc15b66e4a2c7`.
