# 2026-10-07: run-verified navigation — PWP receipts bound to application-map steps (V1-0957)

## Intent

[Issue 658](https://github.com/beamfall/corvint/issues/658) (native V1-0957) asks for application-map
steps and selectors to carry run evidence: `VERIFIED` at the revision a passing Playwright receipt
ran against, stale when the step's source changes, contradicted when a run fails, and never
verified on an unknown revision. This entry records the proposed contract
`docs/specs/run-verified-navigation-v0.md` (RVN-V0-001..008, each pending owner acceptance) and its
implementation in `internal/appmap/verify.go`, built on the AMAP-V0 map of V1-0956 (gh657 lane head
`43c65094`), which is not yet on `origin/main`.

Already delivered before this change: AMAP-V0 freshness (FRESH/STALE/UNKNOWN per call) and the
overlay seam `internal/appmap/overlay.go`, which names issue 658 but implements no verification;
PWP qualified receipts with content-derived test IDs, `QualifiedReceiptBindingReady` and
`QualifiedApplicationRevision`; AFU-V1 declared/inferred `test` links. Nothing joined them.

## Decisions

- **Reuse, no new receipt format.** Receipts are read through `testvaliditydoc.Decode`/`Project`
  (canonical bytes, 4 MiB bound) and must project to a Playwright external document (any PWP
  profile). An outcome binds by its PWP test ID only, never by title, file or URL.
- **Bindings.** Declared AFU-V1 `test` links compile onto `Step.tests`; inferred links never bind.
  Agents may add `--bind STEP_ID=TEST_KEY`; a key absent from every receipt refuses with
  `appmap-verify-test-absent`, an unknown step with `appmap-verify-unknown-step`.
- **App source is the router-state lineage only.** The first draft also anchored the flow intent,
  which made every earlier receipt stale as soon as an intent gained a declared link. The test side
  is already covered: a changed test file changes the PWP test ID.
- **Fail closed on revisions.** An app revision must be a full lowercase commit OID that resolves
  to itself under `--root`; anything else (including short OIDs, tree OIDs and non-OID labels)
  reads `app-revision-unresolved`. An unplaced failure blocks `VERIFIED`. A placed failure outranks
  any pass; there is no recency rule.
- **Typed field, not overlay fact.** `stepView.verification` (`appmap.StepVerification`) so the
  V1-0959 scenario planner can read status without parsing text; `appmap.VerifySteps` returns the
  same statuses keyed by step ID. Authority is always `learned`; selector strength is untouched and
  run evidence is a separate `selector_evidence` label.
- **Stateless and opt-in.** Nothing is stored; without `--receipt` output is byte-identical to
  AMAP-V0. Freshness at each distinct app revision is computed once per call.

## Limits

- Synthetic receipts only (built like `internal/flowcoverage` fixtures); no live Playwright run.
- Component templates, page objects and backend code are not app-source anchors.
- No `flows navigate` or MCP surface; no per-screen status.

## Evidence

- `GOTOOLCHAIN=local go test -count=1 ./internal/appmap` ok (all AMAP-V0 and RVN-V0 tests).
- `go test -run 'AMAPV0|RVNV0|Flows|AFUV1' ./cmd/corvint` ok, including
  `TestRVNV0FlowsAppmapVerificationCLI`.
- Mutation checks (each reverted): dropping the unplaced-failure guard, citing stale evidence as
  `VERIFIED`, disabling `CONTRADICTED`, accepting a resolved-but-different revision, and binding
  inferred links each fail a named RVN-V0 test. Removing only the OID-shape guard is caught by the
  resolve-to-itself check (defence in depth, no separate failing test).
- Doc gates (`spec-requirements-check` through `unbounded-readers-check`) and
  `use-case-receipts-check`/`-test` pass; `internal/specindex` ok. `cmd/corvint/main.go` unchanged,
  so no receipt repin.

## Independent review

- Codex round 1 (`e72b30ec..21ab9950`): two P2, no P0/P1.
  - Receipt read could block on a FIFO swapped in after `Lstat`, and did not check content
    stability. Repaired: no-follow, non-blocking open (`internal/appmap/open_unix.go`), regular-file
    check on the descriptor, and identity, size and mtime compared before and after the read. The
    swap race itself has no deterministic regression test; the static symlink and missing-file
    refusals are covered.
  - Verification freshness used every projection anchor, so an unreadable unrelated anchor (partial
    clone) made the CLI print `freshness-unknown` while `VerifySteps` printed `VERIFIED`.
    Repaired: each step reads only its own router lineage at the app and evaluated revisions,
    cached per (revision, lineage). Regression `TestRVNV0006StatusIgnoresUnrelatedAnchors` fails on
    the round-1 code and passes after.
- Codex round 2 (`e72b30ec..1ec5c6d4`): both round-1 repairs confirmed; one P2: an in-place
  rewrite between `Lstat` and open was accepted because only identity was compared there.
  Repaired: size and mtime are compared at all three observations (`unchanged`); regression
  `TestRVNV0001ReceiptRewriteDetected` covers the comparison (the window itself is not
  deterministically reproducible).
- Codex round 3 (`e72b30ec..7057ada2`): approved, no P0-P3 findings remain (review cap of 3
  rounds reached; nothing declined). Codex could not run tests in its read-only sandbox; its
  approval is source review, and the tests above were run locally.
- Out of scope, reported to the orchestrator: `appmap.LoadMap` (V1-0956) has the same
  `Lstat`-then-`os.Open` FIFO window for `--map`.

## Dogfood

- Pre-change: `corvint affected --base 43c65094e3b7d29405f065b41937d9f215702f3f` (scope UNKNOWN, clean tree) and
  `corvint --root . context --task … --subject internal/appmap/overlay.go`; used results were
  `docs/specs/application-map-v0.md`, `internal/jstestprovider/receipt.go`, `external.go`,
  `cmd/corvint/flows_appmap.go` and the receipt fixture in `internal/flowcoverage/coverage_test.go`.
  The plain-text `context "<text>"` form refused with invalid arguments; `--task`/`--subject` worked.
- Post-change `corvint affected` selected 70+ units, mostly through the spec index files; the
  focused set above was run, the rest is `NOT_RUN` (docs-only dependency; no Go change outside
  `internal/appmap` and `cmd/corvint/flows_appmap.go`).
- The session hook reported `FALLBACK degraded: corvint-event-rejected:dogfood-event-deadline`.
- CEM: `NOT_PRODUCED` in the lane; the orchestrator binds the integrated batch.

## NOT_RUN

- `make gate` (lane rule) and the full `cmd/corvint` package.
- Live Playwright receipt production and adopter qualification.
