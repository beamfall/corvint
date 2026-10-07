# 2026-10-07: application map batch integration — one run-verification fact contract (V1-0956, V1-0957, V1-0959)

## Intent

Integrate the three application-map lanes of issues
[657](https://github.com/beamfall/corvint/issues/657) (V1-0956, AMAP-V0),
[658](https://github.com/beamfall/corvint/issues/658) (V1-0957, RVN-V0) and
[660](https://github.com/beamfall/corvint/issues/660) (V1-0959, AMSP-V0) into one batch branch,
and reconcile the verification seam the two later lanes built independently against the
issue-657 overlay (`Overlay.Facts`, AMAP-V0-014). Every requirement stays proposed, pending
owner acceptance.

## Merges

`git merge --no-ff` in order onto `origin/main` `0b45b052`: `claude/gh657` (`6a1dab87`),
`claude/gh658` (`0cd27805`) and `claude/gh660` (`e7349d77`). None conflicted, and
`make spec-requirements` regenerated the spec index without change.

## The seam disagreement

- The issue-658 producer (`Verification.Overlay`) emits one learned fact per bound step:
  `source: "run-verification"`, `kind: <status>`, `revision: <receipt app revision>`, `text:
  <compact StepVerification JSON>`. It emits nothing for an unbound step and nothing for a
  selector or method ID.
- The issue-660 planner read facts of `kind: "run-verification"` (`VerificationFactKind`) whose
  `text` was the status. It also asked for selector and shown-method IDs and required each to be
  `VERIFIED@` before a step could read `run-verified`.

Read literally, the planner would have ignored every fact the producer emits, so the plan CLI
could never show a receipt's status.

## Decisions

- **One fact contract: the RVN-V0-006 shape.** A run-verification fact is identified by
  `source == appmap.VerificationSource` and carries its status in `kind`. This shape was chosen
  because the `screen` and `flow` projection readers already consume it from `learned[]`. It also
  keeps `kind` a small closed set and leaves `text` free for the cited evidence. The planner now
  filters on `Source` and switches on `Kind`. It does not read `text`. `VerificationFactKind` and
  the planner's private status constants are removed; the planner uses the exported
  `Verified`, `UnverifiedAtHead`, `Contradicted` and `Unverified`.
- **Every fail-closed rule of both lanes is kept.** These are the producer's
  `evidence-too-large` degradation, absence for unbound steps and per-step anchor scoping. On the
  planner side they are: an unknown kind, or a `VERIFIED` fact without a full object ID, reads
  `unverified` and is reported `verification-invalid`; `VERIFIED@` requires the fact's revision
  to equal the evaluated revision and the anchor to be FRESH; the weakest fact wins; an ID shared
  by two apps is `verification-ambiguous`; past 4096 facts, all are discarded; an overlay error
  reports `verification-unavailable`.
- **A step's verification does not cover its selector or methods (fail closed).** A passing
  bound test shows the step was exercised. It does not show that the test reached it through the
  mapped selector or the reused page-object method. The producer's `selector_evidence` label is
  therefore not read, and selector and method IDs stay `unverified`. As a result `run-verified`
  is reachable only for a step with no selector and no shown method. Every step of the current
  fixture has a selector, so with a receipt those steps read `candidate`, with the action
  `VERIFIED@<rev>`. This is recorded as AMSP owner question 9.
- **Revision equality is kept.** RVN-V0 `VERIFIED` cites the receipt's application revision and
  holds while the step's anchors stay unchanged up to the evaluated revision. The planner
  additionally requires the cited revision to be the evaluated one, so planning at a later
  revision reads `UNVERIFIED_AT_HEAD` even when the anchors are unchanged. This is stricter than
  the projections. The deviation is recorded in AMSP owner question 9 rather than relaxed.
- **Multi-map loading.** `appmap.LoadPlanVerification(maps, receipts, binds)` is
  `LoadVerification` over a plan's maps: a binding must name a step of at least one map.
  `LoadVerification(m, …)` now delegates to it, so its behavior is unchanged. The refusal text of
  `appmap-verify-unknown-step` now reads "no step of any --map". `flows appmap plan` gains the
  same `--receipt`/`--bind` flags as `screen` and `flow` and passes one overlay per map. A step ID
  that two maps share is still unattributable and reads `unverified`.
- **The corpus MCP `map_plan` stays unverified.** A receipt argument there needs a strict,
  root-confined, bounded file input of its own. That is not trivial, so the tool passes no
  overlay. AMSP-V0-010 and follow-up 1 say so.

## Independent review

Codex CLI (`gpt-6-astra`, read-only) reviewed the seam and wiring delta
(`6a1dab87..HEAD -- internal/appmap cmd`, scoped to the seam, including the issue-658 overlay
commit that had not been reviewed before).

- Round 1, P1: the planner computed ID ambiguity only over the maps that planned steps select,
  but it asks every supplied map's overlay. A step ID shared with an unselected map could
  therefore borrow that map's `VERIFIED` fact. Fixed: an asked ID held by any two supplied maps
  (as a step, selector or method) is `verification-ambiguous`. Regression: the
  `TestAMSPV0007CollidingElementIDsStayUnverified` unselected-twin case, which fails without the
  fix.
- Round 1, P2: `VerifySteps` skipped the overlay's 1024-byte `evidence-too-large` degradation,
  so the two RVN-V0-006 surfaces could disagree. Fixed: both share `boundedVerification`.
  Regression: `TestRVNV0006OversizedEvidenceParity`, which fails without the fix.
- Round 2 confirmed both fixes. It found no P0 to P2 issues, so no round 3 ran. One P3 was
  retained for filing rather than fixed: an unbound step that does not resolve still emits an
  `unverified`/`step-unresolved` fact, because `verify` checks resolution before binding. That
  spends learned-fact budget, contrary to RVN-V0-006's rule that unbound steps emit no fact. The
  behaviour already exists at the issue-658 lane tip and was not introduced by this integration.

## Analyzer schema

Issue 657 bumps the analyzer schema to `corvint-analyzer/108`
(`internal/contextindex/analyzer_schema.go`). The separate issue-655/656/659 batch also uses
108. This branch keeps 108; whichever batch lands second must bump to 109 when it merges `main`.

## Evidence

- `go test -count=1 ./internal/appmap/ ./internal/flowcoverage ./internal/specindex
  ./cmd/corvint-corpus-mcp/`: pass.
- `go test -count=1 -run 'AMAP|AMSP|RVN|Flows|AFUV1' ./cmd/corvint/`: pass.
- New `TestAMSPV0007ReceiptVerificationOverlay` (library) and
  `TestAMSPV0010PlanReceiptVerificationCLI` (CLI). They check that a passing synthetic PWP
  receipt bound to `step:check-slot-status/read-status` plans as `VERIFIED@<receipt rev>` with
  the selector `unverified` and the step `candidate`. With the selector removed, the step reads
  `run-verified`. An unrelated later commit reads `UNVERIFIED_AT_HEAD`. A router-state change
  reads `UNVERIFIED_AT_HEAD` with the step `STALE`. A binding that names no step of any map
  refuses. The CLI bytes equal the library's.
- The existing AMSP-V0-007 seam tests were moved to the shared contract (`Source`, `Kind`).
  Their decoys (a `VERIFIED` fact from another source, and a fact about an unasked ID) are still
  ignored.
- `gofmt`, `go vet` (also with `GOOS=windows`) on `internal/appmap`, `cmd/corvint` and
  `cmd/corvint-corpus-mcp`: clean.

## Limits and NOT_RUN

- NOT_RUN: `make gate`, the whole `cmd/corvint` package, a live Playwright receipt against a
  real application, and adopter-scale qualification.
- The receipts are synthetic. Selector and method run verification has no producer.
