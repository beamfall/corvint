# Experimental new-test assessment companion

Build with `go build -o /tmp/corvint-tests-accept ./cmd/corvint-tests-accept`.
This separate local command implements the proposed `new-e2e-test-acceptance-v0.md` contract.
It adds no Core command and executes only operator-approved disposable fixtures.

Prepare the closed request described by `internal/testacceptance.Request`: pinned clean product
and test Git roots/commits/trees, hashes for tracked inputs, actual Node/CLI executable and entrypoint
hashes, an owned loopback server command, declared new test identities, repeat count, environment
label, and optional independently reviewed behavior-falsification plans with their separate executable
and tool identity pins. `TestNEAV0002ActualBrowserAssessment` is a complete executable fixture.
Never pass an author-selected plan or execute third-party code under a credentialed account.

Inspect before executing:

```sh
/tmp/corvint-tests-accept plan --plan /private/operator/request.json
/tmp/corvint-tests-accept accept --plan /private/operator/request.json \
  --approve-plan sha256:REVIEWED_DIGEST --new /absolute/test/root/new.spec.mjs \
  --repeat 2 --environment disposable-loopback > /private/operator/assessment.json
```

Repeat `--new` for multiple files. The flags must match the frozen request; changing inputs requires
reviewing its new digest. `plan` reads and validates, without test execution. `accept` launches a
sanitized self-worker for every repeat/probe/control. It emits JSON with a fixed `body` field suitable
for copying into a draft PR. No connector or outward write occurs. `worker` is an internal stdio
protocol requiring the same explicit request digest and pin validation. Direct worker invocations
with any environment key/value outside the fixed safe four-key environment are refused before
execution; inherited credentials/runtime injection cannot enter this surface.

Current qualification: experimental rejection/blocked assessment only. Existing per-test Freshness
and qualified baseline identity remain UNKNOWN. Stable asserting tests therefore stay **blocked**;
actual surviving controls and flaky outcomes can be **rejected**. Missing controls block. V1-0556
tracks positive per-test freshness. Requested original/reversed file filters do not establish observed
execution order. Cleanup observes known descendants at bounded intervals; fast detach can be missed.
Imported SDK/browser closure, authenticated hook semantics and external outcomes are unqualified.

Focused tests:

```sh
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/testacceptance ./cmd/corvint-tests-accept
GOTOOLCHAIN=local go vet ./internal/testacceptance ./cmd/corvint-tests-accept
CORVINT_ACCEPTANCE_PLAYWRIGHT_CLI=/absolute/installed/playwright/cli.js \
  CORVINT_ACCEPTANCE_REPORT=/private/operator/live-assessment.json \
  GOTOOLCHAIN=local go test -count=1 -timeout 30m \
  -run '^TestNEAV0002ActualBrowserAssessment$' -v ./internal/testacceptance
```

Without the explicit installed Playwright CLI the live test skips with `NOT_RUN`; focused unit
success does not imply browser qualification. The live fixture owns port 4173; a collision fails
qualification and must not reap another process. It retains Node/browser versions, actual assertion
kill/survival, alternating repeats, requested probes and explicit unknowns. It installs no dependency.
