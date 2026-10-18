# V1-1665: CCE-V0 first slice in experimental companion corvint-test e2e (catalog, obligations, compile, prove)

Spec: `docs/specs/change-compiled-e2e-v0.md`. Authority: owner direction of 2026-10-17
(`docs/build-log/2026-10-17-change-compiled-e2e-direction.md`). Intent stays proposed and delivery is
experimental, so nothing is promoted or advertised as delivered.

## Decisions

- The companion is `corvint-test`, with E2E as its `e2e` group; core `corvint` is unchanged
  (CCE-V0-015).
- `prove` computes the head-pool INTERMITTENT verdict directly with the PWR-V0-003 vocabulary instead
  of `playwrightruns.Stability`, because PWP receipts are not PWR runs.
- An outcome-assertion failure is located from the PWP `FailureMessage`, because PWP does not retain
  the error location.
- Name collisions:
  - Companion-release classifier: `internal/companionrelease/retained_test.go` now matches the
    prefix `corvint-test-validity` (was `corvint-test`), so `corvint-test` is not mapped to
    `corvint-test-validity-mcp`.
  - Core `corvint test` diagnostics: the bare `corvint-test:` stderr prefix
    (`cmd/corvint/session_test_shards_unix_test.go:82`) stays core-owned and unchanged. Companion
    diagnostics use `corvint-test e2e:` (and `corvint-test e2e hook:`), and
    `TestCCEV0015CoreBoundary` refuses a bare `"corvint-test:` literal in companion sources.

## Evidence

- Focused tests (`heavy.sh`, one package per command): `./internal/changee2e/` ok, `./cmd/corvint-test/`
  ok, `./internal/companionrelease/` ok; `go vet` and `gofmt -l` clean over both new packages.
- Live qualification (Playwright 1.63.0, bundled headless shell, Node v22.23.2, 5 repeats, one worker,
  no retries): `TestCCEV0009LiveDiscriminates` reads DISCRIMINATES (head 5/5 pass, base 5/5 fail at the
  outcome assertion, wrong-locator and wrong-expected-value controls killed on every attempt, every
  side qualified with a PWP receipt digest); `TestCCEV0009LivePreserves` reads PRESERVES with both
  controls killed. On Node v22.23.3 the same runs read INCONCLUSIVE `run-without-qualified-receipt`:
  the PWP attempt profile admits only v22.23.2 and v24.11.1 while the freshness profile admits only
  v22.23.3, so no one Node satisfies both (filed as a ticket).
- CCE-V0-015: outside docs the tree is byte-identical to the claim base
  `b74827c7f8f4fc0b8c269da00994075855cd4d0f` except the two new packages and the companion-release
  classifier test; `go list -deps ./cmd/corvint` (347 packages) has no CCE package;
  `TestCCEV0015NeverExecutesCorvint` runs the built companion with a recording `corvint` first on
  PATH and observes no call.
- Review: one Codex read-only round raised eight findings, all fixed before ship: (1) input values
  also pass the shared `secretscreen` patterns; (2) proof-tree extraction skips symlinks; (3) the
  assertion detector requires the per-assertion custom-message marker, so another assertion with
  the same locator and matcher no longer reads as the outcome failure; (4) the hook requires the
  marker; (5) placements carry an obligation-digest suffix so they are injective; (6) changed paths
  outside every screen and outside the unbounded set read UNKNOWN `unplaced-path`; (7) a flow's
  expects are reached from the first changed-screen step; (8) `--out` refuses nested or file
  symlinks (`e2e-out-unsafe`) and validates all scenarios before writing any.
- Live rerun after the marker change, Node v22.23.2: `TestCCEV0009LiveDiscriminates` and
  `TestCCEV0009LivePreserves` PASS with qualified receipts. The Node profile mismatch is V1-1672.
- Dogfood: bound, checked and sealed against base `c976d521ef857e790cc0f4149a338dd13eaffa03`
  (see the sealed CEM in the PR).

## Limits

- CCE-V0-003: one hand-written `.corvint/e2e-blocks.json`.
- CCE-V0-010..014 are out of scope for this slice.
- Folding the other test companions into `corvint-test` is V1-1667.
