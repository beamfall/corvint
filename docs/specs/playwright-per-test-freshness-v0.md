# Playwright Per-test Freshness V0

Owner: Russell Lewis
Date: 2026-10-02
Intent status: proposed technical contract; owner approved the implementation plan
Delivery status: experimental
Profiles: `corvint-playwright-freshness/0`; `corvint-new-e2e-request/1`;
`corvint-new-e2e-assessment/1`.
Authoritative inputs: human GitHub issue https://github.com/beamfall/corvint/issues/394;
native V1-0556/V1-0541; AGENTS.md; decision 0179; accepted Playwright External Provider V0;
owner message `approved` on 2026-10-02 for plan SHA-256
`e1efaad50de73efa4aa57b87905b32593df97146934ba98cf616841f00ec0736`.

## Agent digest
- Claim: A bounded source-direct Playwright profile observes source/server/runtime identity and joins every planned native control attempt to every baseline repeat.
- Status: proposed technical contract; owner approved the implementation plan; experimental. Reviewed source and bounded local consuming-path observations exist; final frozen checks, CEM/OCM, integration and native completion remain pending.
- Exists: separate native freshness profile and request/assessment /1; actual stable assertion/control acceptance, stale product/test/imported-source controls, closed native joins and bounded dependency readers. Old /0 remains unchanged.
- Blocked on: final registered clean-head checks, CEM/OCM and integration/native completion. Whole #394/V1-0541 remains partial for new-profile surviving Strength and flaky/order qualification; those are separate from V1-0556 freshness.
- Read next: Requirements; Source-direct observation; Control join; Qualification and rollback.

## User and verified current state

A maintainer reviewing new browser tests needs observed execution freshness and actual target-assertion
strength to make a single acceptance decision. At public base
`f51f3c9e6fbfc5a6b219692e8bf46e4e43297a36`, `ReceiptTestProjection` delegates without execution
facts, so `testvalidity.Project` reports freshness UNKNOWN. The acceptance consumer consequently
blocks stable tests and withholds measured strength even after a legacy verified kill. A generic
falsifier verifies retained hashes and request coherence; its native payload alone is opaque to that
verifier and does not prove runtime/source/assertion identity.

The simpler baseline stays available: old negative-only acceptance plus independent manual
inspection of provider and control evidence. The permanently prose-only JS experiment is unchanged.

## Requirements

- `PTF-V0-001`: The profile is explicit, separate and bounded to trusted local source-direct serving
  code, immutable clean product/test Git revisions and trees, one tracked HTML artifact and pinned
  executable, server/observer entrypoint, configuration, package and complete operator-approved local
  CJS dependency roots/files and their canonical manifest digest. Expected
  identities express the requested target; only actual observations can establish execution currency.
- `PTF-V0-002`: Capture clean product/test repository and file identities before and after execution.
  Capture the owned server leader PID and host start identity through the existing AfterStart hook.
  Independent application observations must identify that same leader and generation, the exact
  tested loopback origin/document path, and actual served bytes equal the tracked loaded artifact
  snapshot. No redirect, foreign healthy server, old served artifact or unrelated current file hash
  may establish CURRENT. Unknown process identity blocks.
- `PTF-V0-003`: Record the actual reporter-selected semantic test identity, source anchor/digest,
  resolved built-in browser/page configuration, attempts, Node and Playwright versions, browser
  executable/version/manifest/hash, pinned runner entrypoint bytes and bounded declared execution
  environment. Bind full selected Name/FullName/Anchor/Project including Use options. Reobserve approved installed
  dependency bytes and explicit inputs before launch and on every publication return; join actual
  worker, server and observer CJS import observations to those approved bytes. Missing/unapproved
  closure is UNKNOWN; observed byte drift is STALE. Reobserve relevant bytes/environment at
  publication. No caller version label or
  lifecycle boolean alone establishes freshness; unsupported tuples and overridden browser/page
  fixtures remain UNKNOWN.
- `PTF-V0-004`: Rederive each selected row's CURRENT, STALE or UNKNOWN from receipt-wide observed
  identities. Complete supported equal observations permit CURRENT; an observed mismatch or drift
  gives STALE even when a later observation is absent; missing or unsupported facts give UNKNOWN.
  Preserve all mismatches and independent validity axes. A false stale flag cannot hide observed drift.
- `PTF-V0-005`: Request /1 and assessment /1 retain complete new-profile baseline receipts and exact
  verified control EvidenceReceipts including raw native bytes. The bounded canonical new-profile
  codec rejects duplicate/unknown fields, wrong profiles, contradictory projections and noncanonical
  encodings, then rederives row identities and projections. Request/assessment /0 remain unchanged
  and negative-only; old envelopes and unsupported retained/MCP/flows consumers reject new fields/profile.
- `PTF-V0-006`: First verify the complete generic EvidenceReceipt against its approved plan,
  tool and digest without stripping Unsupported, CoverageGaps, counts, mutation score or raw results.
  Join every approved runnable control and each planned generic falsifier attempt exactly once to
  one retained native payload and verified hook outcome by control ordinal, control ID and attempt.
  Decode the supported canonical native profile; each selected native row has one actual attempt
  and zero retries. Validate raw bytes/artifact digest, tool, plan/perturbation, selected semantic
  test and the complete observed closure against EVERY baseline under PTF-V0-007 and PTF-V0-008.
  A KILLED attempt requires the registered actual target assertion failure marker; a SURVIVED
  attempt requires an actual PASSED native row and verified passed hook outcome with no omitted,
  infrastructure or cancelled execution. Complete all-killed controls yield Strength KILLED.
  Complete controls containing any survivor yield Strength SURVIVED and preserve rejection.
  Derive each control status and exact ordered survivor CoverageGaps from the joined attempts,
  requiring agreement with the generic results. Missing, duplicate, extra, unsupported, invalid,
  unrun, mismatched or unexplained facts leave Strength NOT_MEASURED; conservative survivor
  rejection remains. Per-run hashed IDs and declared assessment aliases remain distinct.
- `PTF-V0-007`: The only supported mutation delta is one ChangedFixtureValue replacement in one
  source-direct HTML response. Its closed definition names artifactPath, sourceSha256, from, to and
  mutantSha256. From actual retained secret-screened source bytes, verify the source hash and one
  from occurrence; independently derive the replacement bytes and mutant hash. Actual mutant
  response must equal that result and its plan's perturbation digest must match. Product/test source,
  runner, environment, browser and input closure remain identical. Only the approved served-response
  hash and independently qualified new server generation may differ; unexplained changes block.
- `PTF-V0-008`: Join the control's observed semantic/runtime/source closure to EVERY baseline repeat.
  Positive acceptance requires N complete CURRENT, ASSOCIATED, ELIGIBLE, PASSED rows, one actual
  attempt each with retries zero, a qualified target assertion kill, and observed-absent owned
  server/worker/control cleanup. Any stale repeat rejects, unknown evidence blocks and later CURRENT
  never overwrites earlier STALE/UNKNOWN in either verdict or summary axes. Failures, flakes, skips,
  surviving controls and known cleanup survivors preserve rejection; aggregate accepts only if all
  assessments accept. Existing repeat/order evidence and bound rendered body remain retained.
- `PTF-V0-009`: Readiness failure, attestation failure, execution failure and cancellation all cancel
  and join the owned server and every owned observer. Retain actual runner/server/worker cleanup
  separately from reused external lifecycle facts. The 20ms descendant sampler and host identity
  limits remain explicit; observed absence is no hostile-process containment claim.
- `PTF-V0-010`: Qualify the named consuming path on the exact tuple with actual stable positive/control
  joins, stale product/test source, foreign healthy occupied-port server, old served artifact and
  cancellation controls. Retain all failed and unavailable observations. Go fixtures, dependency
  integrity, availability smoke or legacy tuple qualification cannot replace this live matrix.

## Source-direct observation and trust boundary

Only reviewed trusted local source-direct application/test/provider code is eligible. The serving
process loads the pinned tracked artifact snapshot and trusted entrypoint bytes at launch; an
independently pinned observer fetches the tested document and process-generation evidence. Its
endpoint instance PID/start must match the owned leader captured by the parent through AfterStart,
not merely any process with a matching executable. The observer must not echo stdin expectations.
Compilation, images and arbitrary dependency/build provenance are unsupported. Readiness is bounded
HTTP observation at two instants, not continuous availability.

The browser-consuming tuple is Darwin arm64, Node v22.23.3, Playwright 1.63.0, headless bundled
Chromium headless-shell revision1243, manifest153.0.8010.12, executable SHA-256
`a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282` and observed executable version
`Google Chrome for Testing 153.0.8010.12`. The launched browser version is153.0.8010.12.
No old Node v22.23.2 profile gains support. Reporter-specific actual tuple evidence is still required.
The launcher binds PATH/LANG/LC_ALL/TMPDIR; optional OS-derived ambient fields do not silently become
selection authority or imply whole-environment coverage. Legacy authenticated-operator,
authenticated-hook and independent-process-and-artifact-observation unsupported claims remain
visible; this profile adds only the named local native join, not generic falsifier authority.

One source artifact is at most64KiB, actual runner reports at most4MiB, assessments at most16MiB,
processes at most60s, readiness and application observations at most5s. Secret-screen source bytes
after decoding; artifact retention is private evidence and cannot bypass screening via base64.
All new failures are value-free typed reason codes and fail closed.

The approved dependency inventory has at most 8 roots and 512 files, 32 MiB per file and 128 MiB total.
Actual reads cover all declared root entries and explicit inputs; extra entries cannot disappear
from the join. Reject symlinks and non-regular leaves before opening, then recheck descriptor type.
Darwin/Linux use no-follow, nonblocking, close-on-exec opens; other platforms refuse the observer.
Cap each read by remaining bytes plus one overflow sentinel, and stop inventory on exhaustion with
UNKNOWN because any remaining entries cannot be certified. Actual I/O can exceed the total by at
most that one byte. This gives no hard context deadline for regular-file filesystem I/O and no
hostile ancestor replacement, ESM/dynamic remote or native-library closure guarantee.

## Diagnostic and status ownership

This experimental profile owns these existing emitted failure reasons and status
tokens. Requirement IDs identify their current contract; this list adds no
behavior or qualification. The two status tokens do not establish execution.

| Emitted reason or status | Owning requirement | Kind |
|---|---|---|
| `all-baseline-observations` | PTF-V0-008 | status |
| `freshness-attestation-command-invalid` | PTF-V0-001 | failure-reason |
| `freshness-attestation-entrypoint-drift` | PTF-V0-001 | failure-reason |
| `freshness-attestation-executable-drift` | PTF-V0-001 | failure-reason |
| `freshness-command-drift` | PTF-V0-001 | failure-reason |
| `freshness-command-invalid` | PTF-V0-001 | failure-reason |
| `freshness-command-unavailable` | PTF-V0-003 | failure-reason |
| `freshness-config-unsupported` | PTF-V0-001 | failure-reason |
| `freshness-declared-environment-required` | PTF-V0-003 | failure-reason |
| `freshness-dependency-closure-unknown` | PTF-V0-003 | failure-reason |
| `freshness-dependency-closure-unqualified` | PTF-V0-003 | failure-reason |
| `freshness-environment-unsupported` | PTF-V0-003 | failure-reason |
| `freshness-imports-require-separate-codec` | PTF-V0-005 | failure-reason |
| `freshness-initial-identity-unavailable` | PTF-V0-002 | failure-reason |
| `freshness-input-bound-or-secret` | PTF-V0-005 | failure-reason |
| `freshness-input-drift` | PTF-V0-001 | failure-reason |
| `freshness-input-unavailable` | PTF-V0-001 | failure-reason |
| `freshness-leader-unavailable` | PTF-V0-002 | failure-reason |
| `freshness-native-identity-drift` | PTF-V0-005 | failure-reason |
| `freshness-noncanonical-input` | PTF-V0-005 | failure-reason |
| `freshness-output-bound` | PTF-V0-005 | failure-reason |
| `freshness-output-bound-or-secret` | PTF-V0-005 | failure-reason |
| `freshness-profile-shape` | PTF-V0-005 | failure-reason |
| `freshness-projection-or-canonical-drift` | PTF-V0-005 | failure-reason |
| `freshness-readiness-unavailable` | PTF-V0-009 | failure-reason |
| `freshness-request-binding-invalid` | PTF-V0-001 | failure-reason |
| `freshness-requires-separate-codec` | PTF-V0-005 | failure-reason |
| `freshness-response-delta-invalid` | PTF-V0-007 | failure-reason |
| `freshness-root-commit-invalid` | PTF-V0-001 | failure-reason |
| `freshness-served-identity-mismatch` | PTF-V0-002 | failure-reason |
| `freshness-server-cancelled` | PTF-V0-009 | failure-reason |
| `freshness-server-start-unavailable` | PTF-V0-009 | failure-reason |
| `freshness-source-secret` | PTF-V0-005 | failure-reason |
| `freshness-source-unavailable` | PTF-V0-001 | failure-reason |
| `freshness-source-untracked` | PTF-V0-001 | failure-reason |
| `freshness-trailing-data` | PTF-V0-005 | failure-reason |
| `native-control-join-incomplete` | PTF-V0-006 | failure-reason |
| `verified-native-target-assertion-kill-joined-to-every-repeat` | PTF-V0-008 | status |

## Control join

Verify the complete EvidenceReceipt using the existing verifier without stripping Unsupported,
CoverageGaps or raw attempts. Every approved runnable control and every planned falsifier attempt
must bijectively join its retained native row and verified hook outcome; no omitted or extra row
qualifies. Decode only the new canonical profile and rederive its selected native row and closure.
Each native row has one attempt and zero retries, distinct from the generic control attempt count.
The supported single assertion fixture uses actual `PTF-ASSERTION:<assertion-id>` text followed by
Playwright's locator assertion failure evidence; an unrelated error or assertion cannot count as a
kill. An actual PASSED row with a verified passed hook outcome establishes survival only after
all planned attempts and closures join. Infrastructure failures never establish measured survival.
Match source hash/path, declaration location, full Name/FullName and Project/Use options semantically.
The approved dependency manifest/digest, actual before/after complete file inventory and worker
imports must also match every baseline.
Normalize only independently verified output/temp-config/selector transport arguments; no other
argv delta is permitted. Compare to every baseline; derive the sole mutation exception using
PTF-V0-007 before trusting the mutant response. Missing facts leave Strength NOT_MEASURED.

## Non-goals

No Core/MCP/flows registration, external-server lifecycle broadening, old wire/pin changes, JS prose
promotion, general Playwright/browser/platform qualification, hostile-code/process/filesystem
containment, compilation/source provenance, network-denial claim, authenticated operator/hook
semantics, automatic dependency installation in the product, release publication or merge authority.

## Acceptance and traceability

| Requirements | Minimum evidence before delivered |
|---|---|
| PTF-V0-001..004 | Unit CURRENT/STALE/UNKNOWN matrix over observed identity fields; real stable and stale source browser runs |
| PTF-V0-005 | Closed/canonical codec, old-envelope and unsupported-consumer rejection fixtures |
| PTF-V0-006..008 | Complete control/attempt native joins measure KILLED or rejected SURVIVED; missing/duplicate/extra/unsupported/invalid outcomes stay NOT_MEASURED; same-closure target kills join every repeat and accept; stale/unknown aggregation remains |
| PTF-V0-009 | Foreign occupied port, failure and cancellation observations; owned server/runner/browser descendants retired with limitations retained |
| PTF-V0-010 | Exact consuming tuple live matrix, focused tests/vet/doc/error ownership, independent review, final CEM/OCM and native gate receipts |

Source-phase evidence is retained privately with immutable hashes: eight real provider modes
(stable, product source drift, distinct test source drift, imported implementation drift, response
mutation, foreign occupied port, old served artifact and cancellation) passed. The consuming path
accepted two CURRENT/PASSED zero-retry baselines with native KILLED strength and observed-absent
cleanup. Each baseline bound 197 approved files and 21 actual worker imports. Rehashed generic-valid
locale, viewport, title, opaque/wrong assertion and observer controls did not become measured native
strength. Missing/early stale facts and duplicate/forged rows remained nonaccepting.

Independent source review passed after two bounded repairs, retaining actual failing-before and
passing-after witnesses for semantic joins, imported implementation drift and FIFO/total-budget
reads. These are source-phase observations, not final registered frozen checks or CEM/OCM receipts.
The exact source/review/proof hashes and remaining delivery are recorded in
`docs/build-log/2026-10-02-per-test-freshness.md`. Whole #394 remains partial: historical legacy
nonasserting/flaky rejection does not qualify new-profile surviving Strength or mixed order probes.

## Qualification, rollout and rollback

New profile and positive assessment path remain experimental until every numbered requirement has
its named witness and the live matrix passes. Owner plan approval is retained separately from live
qualification. Mark skipped/unavailable matrix cells NOT_RUN and preserve their blockers. General
host/consumer/build qualification remains excluded. Enable only explicit request /1; old paths
remain available. Rollback stops the optional new path and reverts its isolated source changes,
retaining immutable evidence and open native tickets. No state migration or runtime downgrade.
