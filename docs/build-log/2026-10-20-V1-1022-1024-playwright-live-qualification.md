# V1-1022 and V1-1024: live Playwright qualification

## Problem

V1-1022 (obligation ledger) and V1-1024 (step-level negative controls) were delivered in code
but never run against a real Playwright install.

- The V1-1648 log (`2026-10-18-v1-1648-witness-qualification.md`) left the live `qualify-version`
  run of the embedded TOL-V0-060 fixture `NOT_RUN`. The shipped TOL-V0-009 version list was
  therefore empty, and every real report refused `UNSUPPORTED_VERSION`.
- The V1-1024 log (`2026-10-08-v1-1024-negative-controls.md`) ran the checked-in negate matrix
  only on an unqualified tuple (Node v22.23.3), so `LPCV-V0-070` stayed `NOT_RUN` and the
  `LPCV-V0-068` join stayed off.

The owner approved local browsers and toolchains on 2026-10-17. They live under
`/private/tmp/cem10-build/toolchains-2026-10-17/`, with sha-verified `chromium-headless-shell`
1243 (1.63.0) and 1248 (1.64.0).

## Decisions

- **Running `qualify-version` in-process.** The live run goes through a Go test,
  `TestTOLV0127_LiveFixtureQualifies`, which calls the CLI in-process with the same `cli.Run`
  entry the binary uses. The lane does not run the installed `corvint-tasks` binary.
- **Shipping 1.63.0 (TOL-V0-126).** TOL-V0-009's own rule says the list is empty until the live
  fixture passes on a 1.63.x `PWP-V0-008` tuple. It passed, so `1.63.0`, and only that version,
  is shipped. 1.64.0 also passed, but it stays store-local, because the rule names 1.63.x.
- **A qualified mode for the negate live test (LPCV-V0-126).** Setting
  `CORVINT_NEGATE_QUALIFIED_NODE` removes the test-only unqualified-tuple allowance, so a passing
  run is a qualified-tuple run.
- **Leaving the join off (LPCV-V0-129).** `stepNegationJoinQualified` is not flipped. The
  `LPCV-V0-070` gate sentence needs only the retained matrix. The slice's acceptance evidence,
  however, says the owner sample comparison is investigated "before promotion", and it also lists
  the owner's G2 cost measurement. Both are owner-run and `NOT_RUN`, so the promotion rule is not
  fully met. A guard test, `TestLPCVV0129JoinStaysGatedOff`, pins the gate off.

## Measurements and evidence (2026-10-20, macOS arm64)

| Run | Tuple | Result |
|---|---|---|
| `TestTOLV0127_LiveFixtureQualifies` | Node v22.23.2, `@playwright/test` 1.63.0 | PASS (3.3 s); fixture sha256 `382442f6…5674`, report sha256 `046ffc0e…d0b1`; classes credited TOLQ-1,3,6,8; failed TOLQ-2,4,5,7; defectConfirmed TOLQ-9 |
| same | Node v22.23.2, 1.64.0 | PASS (6.5 s); report sha256 `3b4a90c0…b50c` (store-local only) |
| same, control | 1.63.0, `report.go` mutated to credit retry 1 | FAIL `OBLIGATION_QUALIFICATION_FAILED: TOLQ-7 expected failed, observed credited` |
| `TestTOLV0126_ShippedVersionAdmitted` | n/a | PASS; with the shipped list reverted to empty it fails |
| `TestNegateLiveDiagnostic`, qualified mode | Node v22.23.2, 1.63.0, headless shell 1243 | PASS 12/12 in 61.5 s; every document runner `playwright` 1.63.0, Node v22.23.2, tuple `candidate` |
| same, control | Node v22.23.3 (unqualified) | FAIL `runtime-tuple-unqualified` |
| same | Node v22.23.2, 1.64.0, headless shell 1248 | FAIL 7/12 (network-text, static-element, caught-assertion, soft-steps, hard-steps, collateral, third-party); every assertion step `NOT_MEASURED` `step-fault-underivable` with anchors `derivation:network:expected-value-not-literal` and `derivation:dom:no-dom-fault` |
| `TestLPCVV0129JoinStaysGatedOff` | n/a | PASS; it fails when the gate is set true |

Per-case 1.63.0 negate timings, in seconds:

| Case | Time |
|---|---|
| network-text | 6.43 |
| static-element | 5.42 |
| caught-assertion | 5.80 |
| pattern and assertion-free | 2.17 |
| soft steps | 7.47 |
| hard steps | 5.63 |
| collateral | 13.03 |
| third-party | 5.55 |
| failing fixture | 1.92 |
| flaky baseline | 5.39 |
| interruption | 1.85 |
| ambiguous selection | 0.67 |

The whole `internal/tasks/cli` package passed in 700.7 s, with the live test skipped as
`NOT_RUN`. `internal/tasks/obligation` and the `internal/testvaliditydoc` join tests also passed.
Report bytes are not retained (TOL-V0-009); only their digests are recorded here. A rerun on the
final code passed again on both tuples (1.63.0 report `a22c2a97…a5be`, 1.64.0 report
`bce08507…e6ba`). A report digest differs per run, because the report carries timings and
temporary paths, so each digest identifies one run.

## NOT_RUN

- **Owner sample comparison and G2 cost measurement for step negation.** Both are owner-run, and
  they block the join promotion (LPCV-V0-129).
- **A `corvint test-validity negate` CLI invocation against a live application.** The matrix ran
  through the checked-in Go live test, which calls the same `RunNegate` the provider uses.
- **A Linux tuple.** Only macOS arm64 is installed.
- **run-batch with a store-local version.** This was already `NOT_RUN` in V1-1648.

## Residuals

- **Step negation on 1.64.0 traces (LPCV-V0-128).** Derivation abstains on every assertion step.
  A trace-format change in 1.64 is suspected but not confirmed (`internal/stepnegation` trace
  linking of `Frame.expect` to its step, or the expected-value params shape). It is safe, because
  the result is `NOT_MEASURED` and never a false `KILLED`/`SURVIVED`, but 1.64.0 cannot qualify
  step negation until it is fixed.
- **GitHub #680 and #682 are not resolvable.** `gh issue view` on `beamfall/corvint` returns
  "Could not resolve to an issue" for both; the numbers belong to the pre-snapshot lineage
  (decision 0331). The PR therefore uses `Refs V1-1022, V1-1024` and closes no issue.

## Rollback

- **TOL.** Set `qualifiedVersions` in `internal/tasks/obligation/report.go` back to empty. A
  1.63.0 report then refuses again unless the store-local allow-list holds it. Written witness
  events keep their bytes and audit `CONSISTENT`.
- **LPCV.** Drop the qualified mode and the guard test. No product behavior changed.
