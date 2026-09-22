# Browser behavior falsification V0

- Owner: Russell Lewis
- Date: 2026-09-21
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: owner request [issue 54](https://github.com/beamfall/corvint/issues/54),
`AGENTS.md`, `docs/specs/documentation-corpus-v1.md`, and
`docs/specs/js-live-test-provider-v0.md`.

## Agent digest
- Claim: An opt-in companion runs digest-approved caller hooks against a marked disposable workspace and emits fail-closed criterion-level falsification receipts.
- Status: proposed/experimental; no control result authenticates semantic adequacy or authorizes test-suite narrowing.
- Exists: the separate experimental planner, approval gate, bounded runner, closed classifier and synthetic conformance fixtures.
- Blocked on: live adopter/browser evidence for each supported control kind and owner acceptance of the proposed contract.
- Read next: Requirements; Trust and mutation boundary; Acceptance matrix.

## Affected user and measurable job

An adopter with a revision-pinned browser behavior contract needs to learn whether one exact
criterion fails for the expected assertion identity under a deliberate perturbation, while unrelated
required behavior and cleanup remain valid. The output must distinguish a killed control from a
survivor, unsupported work, infrastructure loss and an invalid experiment without turning partial
coverage into a general adequacy claim.

The verified starting point is the experimental `corvint-corpus-behavior-provider/1` consumer in
`internal/doccorpus/behavior.go`, which validates declared negative-control events but does not
produce them. `internal/jstestprovider` retains Playwright test/project/browser/attempt and artifact
evidence. `internal/playwrightminimize` demonstrates digest-approved experimental planning and
bounded caller commands, but it diagnoses suite interactions rather than criterion falsification.

## Requirements

- `BBF-V0-001`: Planning and execution are separate opt-in operations. A plan is deterministic,
  closed-schema JSON with a content digest; execution requires explicit approval of that exact
  digest and refuses drift, unknown fields, trailing JSON and missing experimental admission.
- `BBF-V0-002`: Every plan and result binds the exact contract ID/digest, criterion and assertion
  identities, application/test/documentation Git revisions, test ID/path/line/title, Playwright
  project, runner/browser names and versions, configuration digest, perturbation definition and
  digest, attempt/retry identity, cleanup observation and retained artifact digests.
- `BBF-V0-003`: The closed control vocabulary is `wrong-locator`, `wrong-expected-value`,
  `omitted-assertion`, `omitted-event`, `reordered-event`, `wrong-project`,
  `suppressed-persistence`, `opposite-branch` and `changed-fixture-value`. A caller may select any
  supported subset; an absent hook or inapplicable control reports `not_supported` or `not_run`,
  never `killed`.
- `BBF-V0-004`: A control hook is caller-owned, content-digest pinned, argument-free and run only
  after plan approval under bounded process-group containment in a caller-marked disposable
  workspace. The runner stages the executable, supplies the perturbation as bounded JSON, permits
  only explicitly declared environment keys, rejects escaping artifact paths or symlinks, and
  verifies the workspace returns to its pre-control digest after the separately pinned cleanup
  hook. It never edits repository source or implicitly targets shared or persistent external state.
- `BBF-V0-005`: `killed` requires the target criterion to fail on its expected assertion identity
  during the first and only admitted Playwright attempt, every declared unrelated criterion and
  required setup observation to retain its expected outcome, no infrastructure failure, successful
  cleanup and complete identity/artifact binding. A crash, timeout, selector error, wrong assertion,
  unrelated failure, retry-only kill or cleanup failure cannot count.
- `BBF-V0-006`: Each control result is exactly one of `killed`, `survived`, `not_supported`,
  `not_run`, `infrastructure_failed` or `invalid_control`, with stable reason codes. Unknown or
  contradictory evidence fails closed to `invalid_control`; execution-boundary loss is
  `infrastructure_failed`.
- `BBF-V0-007`: Results preserve every requested ordinal and actual attempt without replacement.
  Identical approved inputs are independently reproducible; a later retry or rerun never hides the
  first outcome, and duplicate or missing ordinals invalidate the control set.
- `BBF-V0-008`: Every surviving or invalid control is an explicit criterion-level coverage gap.
  Any gap, unsupported/not-run control, stale binding or incomplete selected vocabulary preserves
  `full-relevant-suite` fallback.
- `BBF-V0-009`: The aggregate reports raw counts for all six statuses and mutation score as
  `killed/(killed+survived)` only when that denominator is nonzero. It reports supported, requested
  and executed denominators separately and never labels partial support universally adequate.
- `BBF-V0-010`: Plans are bounded by at most 64 controls, 16 attempts per control, a 24-hour total
  declared budget that must also exceed the executor's cleanup reserve, 32 MiB input/output and
  64 MiB per retained artifact. Cancellation, timeout and output overflow terminate owned
  processes; missing descendant-cleanup evidence is not success.
- `BBF-V0-011`: Synthetic conformance covers a tautological assertion, hidden duplicate element,
  wrong-value survival, expected kill, unrelated failure, timeout, cleanup failure, retry-hidden
  outcome and stale contract/revision/perturbation/artifact digests.
- `BBF-V0-012`: Receipts are observations, not authenticated caller honesty, browser-behavior
  parity, complete mutation coverage or narrowing authority. Generated control definitions remain
  proposals until the caller approves their exact plan and perturbation boundary.

## Simpler baseline and non-goals

The baseline is a caller manually running one deliberately broken browser test and retaining its
native receipt. V0 standardizes bounded orchestration, identity and classification; it does not
generate product-specific perturbation code, edit Playwright tests, discover criteria, provision a
browser, authenticate hook semantics, sandbox arbitrary caller code, mutate a production service or
replace the full suite. Issue 53's provider-record generation and reconciliation frontier are
independent.

## Trust and mutation boundary

The contract and control definitions are caller declarations. Exact Git revisions and digests make
drift visible but do not prove that a hook implements its label honestly. Approval authorizes only
the digest-bound plan. The execution process receives a fresh staged copy of each pinned hook and a
bounded JSON request; it runs in the explicitly marked disposable workspace with a minimal declared
environment. The runner reads and hashes the workspace, hook output and artifacts, then invokes the
separate cleanup hook and requires the original digest. A hook capable of reaching undeclared
external state remains outside the trust boundary; the plan must declare no persistent external
state, and the result retains that limitation.

Malformed identities, stale bytes, unknown statuses, missing expected observations and boundary
violations fail closed. Infrastructure failures remain distinct from failed assertions. Cleanup is
always attempted with its own bounded context even after execution failure or cancellation.

### Owned emitted error codes

| Code | Meaning |
| --- | --- |
| `operator-authorization-required` | Execution lacks approval of the exact plan digest. |
| `approved-plan-drift` | Rebuilding current immutable inputs does not reproduce the approved plan. |

## Acceptance matrix

| Behavior | Required evidence |
| --- | --- |
| deterministic plan and approval | round-trip, unknown-field, trailing-JSON, digest-drift and no-approval tests |
| expected criterion kill | exact assertion failure plus unrelated-pass and cleanup witnesses |
| survivor and false positives | tautology, hidden duplicate, wrong value, unrelated failure and wrong assertion fixtures |
| execution failures | timeout, crash, overflow, cancellation and descendant-cleanup fixtures |
| immutable binding | stale revision/config/hook/artifact and retry/ordinal adversarial fixtures |
| aggregate | six raw status counts, nonzero raw mutation denominator and partial-support fallback |

Focused package and command tests run first. Repository completion additionally requires the
canonical Go test/vet and interoperability gates, independent review, and the dogfood CEM/OCM
completion loop. Synthetic fixtures prove classifier behavior only; live adopter/browser utility is
`NOT_OBSERVED` until a caller supplies and approves real safe hooks.

## Rollout, rollback and maintenance

The capability ships as a separately built experimental companion and internal package. It is not
part of the default `corvint` command, install path or policy input. Schema or status-vocabulary
changes require a new profile or explicit compatibility rule. Rollback deletes the companion,
package and this opt-in spec; existing corpus behavior records, native Playwright receipts and
default commands remain unchanged.

## Traceability

| Requirements | Implementation boundary | Evidence |
| --- | --- | --- |
| BBF-V0-001..003 | `internal/behaviorfalsify` planner/types; `cmd/corvint-behavior-falsify` | plan/decode/vocabulary tests |
| BBF-V0-004, BBF-V0-010 | live hook boundary and `internal/procgroup` integration | disposable-root, staging, cleanup and interruption tests |
| BBF-V0-005..009 | receipt validator, classifier and aggregate | synthetic falsifier matrix and adversarial receipt tests |
| BBF-V0-011..012 | conformance fixtures and report limitations | end-to-end synthetic command tests and independent review |

## Unresolved decisions and promotion criteria

The exact adopter hook protocol may need a future version after live use. Promotion beyond
experimental requires at least one caller-owned disposable Playwright fixture exercising every
supported control kind, all deterministic repository gates passing, no source/shared-state mutation,
and owner review of the resulting plan and raw receipts. Browser portability, hook authenticity and
universal adequacy remain explicitly unclaimed.
