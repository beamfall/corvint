# Browser behavior falsification V0

- Owner: Russell Lewis
- Date: 2026-09-21
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: owner requests [issue 54](https://github.com/beamfall/corvint/issues/54) and [issue 167](https://github.com/beamfall/corvint/issues/167),
`AGENTS.md`, `docs/specs/documentation-corpus-v1.md`, and
`docs/specs/js-live-test-provider-v0.md`.

## Agent digest
- Claim: An opt-in companion runs digest-approved caller hooks against a marked disposable workspace and emits fail-closed criterion-level falsification receipts.
- Status: proposed/experimental; no control result authenticates semantic adequacy or authorizes test-suite narrowing.
- Exists: the separate experimental planner, approval gate, bounded runner, closed classifier, synthetic conformance fixtures, and opt-in approval-bound evidence export with offline verification.
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
  declared budget that must also exceed the executor's 10 s cleanup reserve, 32 MiB input/output and
  64 MiB per retained artifact, and at most 1000 entries each in a control's unrelated-criteria and
  required-setup lists, so the per-receipt output floor can never exceed the 32 MiB document bound.
  Cancellation, timeout and output overflow terminate owned processes; missing descendant-cleanup
  evidence is not success.
- `BBF-V0-011`: Synthetic conformance covers a tautological assertion, hidden duplicate element,
  wrong-value survival, expected kill, unrelated failure, timeout, cleanup failure, retry-hidden
  outcome and stale contract/revision/perturbation/artifact digests.
- `BBF-V0-012`: Receipts are observations, not authenticated caller honesty, browser-behavior
  parity, complete mutation coverage or narrowing authority. Generated control definitions remain
  proposals until the caller approves their exact plan and perturbation boundary.

- `BBF-V0-013`: The opt-in `execute-receipt` operation emits `corvint-browser-behavior-evidence/1` with the exact canonical plan digest preimage, an explicit approval observation bound to that digest, executing tool version/source revision/dirty state/executable SHA-256, and every actual control/attempt. It retains exact hook stdout and the native report declared as a digest-matching hook artifact before cleanup, with independently recomputable SHA-256 digests. Missing reports yield `invalid_control`; credential-shaped data refuses export with a value-free diagnostic. The complete encoded envelope, including base64, is bounded by BBF-V0-010; retained raw bytes additionally share a 16 MiB budget. `verify-receipt` requires caller-pinned plan and executable digests, verifies canonical bytes and approval/raw/report digests, exact inventories, source/runner/assertion identity, cleanup/workspace observations and recomputed outcomes/counts, and never reads a workspace or launches a command. Missing, tampered, stale, unsafe or unsupported evidence cannot verify. The existing `execute` report/0 is unchanged.

### Approval-bound evidence wire

The closed envelope fields are `schema`, `plan_preimage`, `approval`, `tool`, `raw_attempts`,
`report`, `unsupported`, and `digest`, in that order. Byte members use JSON base64. Hashes are
lowercase SHA-256 with the `sha256:` prefix. `plan_preimage` is the existing canonical plan JSON
with its `digest` member set to the empty string, without a trailing newline; hashing the decoded
bytes reproduces the approved plan digest. No workspace is needed to recompute it.

The approval fields, in canonical order, are `schema` (`corvint-browser-behavior-approval/1`),
`plan_digest`, `method` (`explicit-approve-plan`), and `digest`. Its hash covers compact UTF-8 JSON
with `digest` empty and HTML escaping disabled. The envelope hash similarly empties only its own
`digest`; embedded report digests retain the existing report/0 convention. The explicit approval
flag is observed after the executor verifies it against the rebuilt plan; this is not a signature
or an authenticated operator identity.

Each raw row names `control_id`, `ordinal`, `attempt`, `hook_bytes`, `hook_sha256`, `native_bytes`,
`native_sha256`, and `omissions`. A native report must appear in the hook's `artifacts` with the
same digest as `native_receipt_sha256`. Both byte sets are read before cleanup. The hook digest
also equals its process stdout digest; native bytes must match the parsed hook and retained
artifact observation. Setup, target and unrelated assertion observations, retry identity, process
bounds, cleanup and workspace digests remain in the bound report/0. The verifier recomputes
classification instead of trusting a claimed kill. Explicitly unsupported claims are authenticated
operator identity, authenticated hook semantics and independent process/artifact observation.
`verified-observation` describes internally consistent retained evidence, not authenticated execution
or general test adequacy. A non-passing control remains non-passing in the verification output.

Rollback removes the two new operations and evidence profile; prior plan/0, hook/0 and report/0
consumers continue to read the unchanged legacy output.

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
| `receipt-invalid-control` | Offline verification cannot reproduce the retained receipt's shape, pinned identities, digests, inventories, observations or outcome. |
| `receipt-native-path-invalid` | A declared native-report path is absolute, noncanonical, parent-relative or longer than the path bound. |
| `receipt-native-report-invalid` | Reading the native report fails, exceeds the retained-byte bound or does not match its declared digest. |
| `receipt-plan-unsafe` | The canonical plan preimage cannot be encoded or fails the secret screen. |
| `receipt-secret-shaped-data` | The envelope, raw hook output or native report contains secret-shaped data; export refuses without echoing the value. |
| `receipt-tool-identity-invalid` | The executing companion's name, version, revision or executable digest is missing or malformed. |

## Acceptance matrix

| Behavior | Required evidence |
| --- | --- |
| deterministic plan and approval | round-trip, unknown-field, trailing-JSON, digest-drift and no-approval tests |
| expected criterion kill | exact assertion failure plus unrelated-pass and cleanup witnesses |
| survivor and false positives | tautology, hidden duplicate, wrong value, unrelated failure and wrong assertion fixtures |
| execution failures | timeout, crash, overflow, cancellation and descendant-cleanup fixtures |
| immutable binding | stale revision/config/hook/artifact and retry/ordinal adversarial fixtures |
| aggregate | six raw status counts, nonzero raw mutation denominator and partial-support fallback |
| approval-bound retained evidence | `TestBBFV0013OfflineReceipt`, `TestBBFV0013ReceiptTampering`, `TestBBFV0013ReceiptFailuresNeverKill`, `TestBBFV0013SecretOutputRefused`, `TestBBFV0013ReceiptApprovalAndBounds`; actual CLI export/verify smoke |

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
| BBF-V0-013 | `internal/behaviorfalsify/receipt.go`; companion `execute-receipt` and `verify-receipt` | offline, tampering, missing evidence, secret and bound tests |
| BBF-V0-011..012 | conformance fixtures and report limitations | end-to-end synthetic command tests and independent review |

## Unresolved decisions and promotion criteria

The exact adopter hook protocol may need a future version after live use. Promotion beyond
experimental requires at least one caller-owned disposable Playwright fixture exercising every
supported control kind, all deterministic repository gates passing, no source/shared-state mutation,
and owner review of the resulting plan and raw receipts. Browser portability, hook authenticity and
universal adequacy remain explicitly unclaimed.
