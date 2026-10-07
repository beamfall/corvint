# 2026-10-07: typed discovery abstention and the Node v24.11.1 Playwright tuple (V1-0976)

## Intent

GitHub issue #665 (native V1-0976) reports a consumer that pins Node v24.11.x with
`@playwright/test` 1.63.0 on Darwin arm64. The external provider projects passing evidence only on
Node v22.23.2 (`PWP-V0-008`), so every consumer run abstained and `test-validity --discover` (and
the MCP `discover` argument) read `source: "none"` with no tests and no reason. The issue asks for
a typed reason, so callers stop retrying, and for qualification of the Node 24.11 tuple or a
consumer-side qualification path.

Proposed requirements, pending owner acceptance: `LPCV-V0-056`
(`docs/specs/live-proof-carrying-verification-v0.md`) and `PWP-V0-009`
(`docs/specs/playwright-external-provider-v0.md`).

## Decisions

- **The reason lives in `discovery`.** The new member is `discovery.abstention {reason, requires,
  observed?}`. `discovery` is emitted only by discovery, so every `--receipt` and no-input byte is
  unchanged. CLI and MCP keep producing one document (`MTV-V0-009`).
- **Closed values.** The values are `no-retained-evidence`, `retained-evidence-unusable`,
  `runtime-tuple-unqualified` and `UNKNOWN`. `requires` is `retained-producer-run`,
  `qualified-runtime-tuple` or `UNKNOWN`.
  - The no-evidence reason is deliberately not "producer did not run". A run without `--retain` is
    indistinguishable from no run, so claiming that would be invented certainty.
  - When a receipt has no observed Node version, discovery reports `UNKNOWN`. It does not guess.
- **No new store.** The reason reuses the retained receipt and the provider's own classification.
  `jstestprovider.ReceiptRuntimeTuple` classifies the receipt-level runner/Node pair, and
  `qualifiedPlaywrightTuple` now consults it first. Behaviour for existing tuples is unchanged.
- **`observed` is bounded.** It echoes only runner and Node versions of at most 64 characters from
  `[0-9A-Za-z.+-]`. Anything else is withheld.
- **Exact admission, fail-closed.** Only the exact release `v24.11.1` is admitted, for the
  bundled-headless-shell tuple under `/0`. The following stay unqualified:
  - `v24.11.0` and every other `v24.x`;
  - the system-Chrome tuple on Node 24, because the predicate now requires v22.23.2 on that branch
    explicitly;
  - `/1`, `/2` and `/3` on Node 24.

  The reporter (`qualifiedBundledIdentity`) accepts v24.11.1 outside the freshness profile, so the
  identity is retained. The Go predicate stays the authority for which profile may project it as
  passing.
- **Explicit Node for the live matrix.** `CORVINT_PLAYWRIGHT_NODE` is an absolute `bin/node` with a
  sibling `npx`. It is prefixed onto that test's `PATH` only, and the test asserts that `node`
  resolves to it and that the run observes that exact version.
  `CORVINT_PLAYWRIGHT_CONTROL_NODE` runs one passing test on an unqualified Node. That run must
  abstain, classify `unqualified`, and be reported by discovery as `runtime-tuple-unqualified` with
  the observed version.
- **Tighter system-Chrome smoke.** The smoke now expects the system tuple to pass only on v22.23.2.

## Evidence

Dogfood pre-change context:
- `corvint --version`: 1.0.0-rc.2, build 360.
- `corvint --root . affected --base 0b45b052`: rc 0. The output is static and PLAN_ONLY; it advises
  `make gate`, which was not run per the lane policy.
- `corvint --root . context --task ... --subject internal/testvaliditydoc/discover.go --limit 12`
  returned `LPCV-V0-053`, `discover_test.go`, `qualifiedPlaywrightTuple`, `cmd/corvint/test_validity.go`,
  `external_test.go`, `testevidence/retain.go` and the MCP bridge. These were the files changed or
  extended.

Regressions. Each fails without the fix: with the `discovery.Abstention` assignment removed,
`TestDiscoverAbstentionReasons` and `TestDiscoverAbstentionWithholdsUnboundedObservedVersion` fail.
The new tests:
- `internal/testvaliditydoc`: `TestDiscoverAbstentionReasons` and
  `TestDiscoverAbstentionWithholdsUnboundedObservedVersion`;
- `cmd/corvint`: `TestTestValidityDiscoveryAbstentionMatchesMCPDocument`;
- `internal/jstestprovider`: `TestPlaywright163Node24TupleAdmissionIsExact` and
  `TestReceiptRuntimeTupleClassification`. The negative controls are v24.11.0, v24.21.0, `v24.11`,
  identity/browser Node drift, system Chrome on v24.11.1, and `/1`, `/2` and `/3` on v24.11.1.

Focused packages (direct importers of `jstestprovider` and `testvaliditydoc`) passed. The
`cmd/corvint` test-validity/vector subset and `conformance/mcp-test-validity-v0` also passed.

Live qualification ran once, on 2026-10-07, on Darwin 25.6.0 arm64. The retained log is
`docs/build-log/evidence/gh665/node24-live.log`.
- Node v24.11.1 is the official darwin-arm64 tarball, used by explicit path only and not installed.
  The tarball SHA-256 matched nodejs.org `SHASUMS256.txt`. The `SHASUMS256.txt` GPG signature was
  **not verified**, because gpg is not installed on the host.
- `@playwright/test`, `playwright` and `playwright-core` 1.63.0 were installed into a scratch
  directory from the local npm cache with `--offline --ignore-scripts`. Nothing was downloaded.
  Lockfile integrity values:
  - `@playwright/test`: `sha512-oxMK4vllB9RK5NQ2l1pq1IfOf2AvnEuj/vYGDj0H2nMtmtZpKtCwt/l00GEO6xjGfpBNAvjovvYdCm50dRQkpQ==`
  - `playwright`: `sha512-+7ziBLidS4NaNCdt57SUDT+wYmmd5fmiQejUic/kb+YsYSCPyOOE9sebzMjNmQrsnNpDJqd4WHvV/8lfKfUDUg==`
  - `playwright-core`: `sha512-rYCsBF/M5HjUch52bbtVONEFjv6Xu8sm8h72dNlR5bzIE1fvC/bxgspzkjSfU+MweEMmPM8KJebG6nnyxo5mCg==`
- The browser was the existing local `chromium_headless_shell-1243`. Its SHA-256 is
  `a0bfe7b4…8282`, equal to the qualified constant.
- The command was:
  `CORVINT_PLAYWRIGHT_MODULES=<scratch>/node_modules CORVINT_PLAYWRIGHT_NODE=<node-v24.11.1-darwin-arm64>/bin/node CORVINT_PLAYWRIGHT_CONTROL_NODE=/opt/homebrew/bin/node GOTOOLCHAIN=local go test -count=1 -timeout 10m ./internal/jstestprovider -run 'TestQualifiedPlaywrightLive$|TestQualifiedPlaywrightLiveDevicesSpread$' -v`
- Result: PASS (31.8s).
  - The full `/0` matrix passed on Node v24.11.1: pass, assertion, timeout, retry/flaky,
    repeat-each, browser infrastructure, two projects, hooks, retained MCP discovery, and
    cancellation with external-server survival.
  - The in-matrix negative controls abstained: Firefox override, headed, `connectOptions`,
    `PW_TEST_CONNECT_WS_ENDPOINT`, executable override and custom fixture.
  - The devices-spread fixture projected passing on v24.11.1.
  - The control on Node v22.23.3 abstained, and discovery reported `runtime-tuple-unqualified`.
  - The system Chrome on the host is now 154.0.8037.98. It was refused as unqualified.

## Limits and NOT_RUN

- `NOT_RUN`: `/3` (`TestQualifiedPlaywrightAttemptsLive*`) on Node 24, because `PWP-V3-006`
  needs its own run. Also not run: `/1` and `/2` on Node 24, Node v24.11.0, Linux, and hosted CI.
- `NOT_OBSERVED`: the reporting consumer's checkout, fixture and CI. The admission rests on the
  checked-in fixtures only.
- A provider refusal before any run (for example `external-playwright-version-unqualified`)
  retains nothing, so discovery still reads `no-retained-evidence`. When a run is unqualified, the
  platform and architecture are not retained (the browser identity is dropped), so `observed`
  carries only the runner and Node versions.
- The `--retain` exit stays nonzero for an unqualified tuple (`project-location-unknown`
  infrastructure). That behaviour is unchanged.

## Owner questions

1. Should further Node patches be admitted per exact release through the predicate (the fail-closed
   default here), or through a consumer-side qualification record retained with the consumer's own
   live run?
2. Should `v24.11.0`, or the whole `v24.11.x` line the issue names, be admitted? Today only
   `v24.11.1` has live evidence.
3. Should `/3` (and `/1`/`/2`) be qualified on Node 24 with their own live runs?

## Independent review

Codex review, round 1, on `0b45b052..e0ac000f`. It found no P0 or P1 issue. Both findings were
repaired:

- **P2.** A Playwright 1.60.0 receipt with no observed Node version was classified as `candidate`.
  As a result, discovery omitted the `UNKNOWN` abstention that `LPCV-V0-056` requires.
  - Repair: `ReceiptRuntimeTuple` now returns `unobserved` for an empty Node version before it
    checks the runner version.
  - This makes the predicate only stricter.
  - Covered by a new `TestReceiptRuntimeTupleClassification` case.
- **P3.** The live control could pass vacuously.
  - Repair: the control now requires one of two outcomes. Either the run is recorded as
    infrastructure, or it retains the one passing outcome, which must still not project passing.
  - This assertion was added after the single live run and has not been re-executed (`NOT_RUN`),
    because the matrix runs once at the terminal boundary.
  - In that run the control was observed as `unqualified`, and discovery reported
    `runtime-tuple-unqualified`.

Codex round 2 on `0b45b052..9b57f2c9`: approved, with no remaining P0-P3 findings.
