# V1-1670: core `corvint` links net/http but never reaches an HTTP client

Spike ticket V1-1670: core `corvint` links net/http through appflows → jstestprovider. The ticket
asks to confirm that path is unreachable or to cut it. Invariant 7 in AGENTS.md says the default
local product has no network dependency. Decisions for spikes were delegated to the orchestrator
by decision 0548.

## Finding

I ran `go list -deps` on `./cmd/corvint` at base `90ab9f33e`. Exactly two module packages in the core
closure import `net/http` directly:

- **`internal/jstestprovider`.** It uses net/http for loopback readiness probes: `waitReady` in
  `runner.go` and `externalReady` in `external.go`. Both are reached only from `RunE2E` and
  `RunFreshE2E`.
  - Their callers are changee2e, playwrightminimize, testacceptance and the
    `corvint-js-test-provider` binary. `cmd/corvint` links none of them.
  - Core links this package only for its report types.
- **`internal/testrunner`.** It uses net/http for a WebDriver session `DELETE` to canonical
  `127.0.0.1` during device cleanup (`device_cleanup_unix.go`).
  - Core reaches `testrunner.Execute` only through `liveverify/rbmutate`, and the request it builds
    always names the `rspec` runner.
  - `DeviceCleanup` is set only by `platform/android.go` and `mobile/mobile.go`.

So no core command can make an HTTP request. Both clients are loopback-only, even in the binaries
that do use them. The binary still links the net/http code, which is the size and audit cost the
2026-10-17 binary-boundary audit noted.

## Proposed recommendation

**Keep the link and guard it.** Do not cut it now.

Cutting it means splitting the E2E runner out of `jstestprovider`, which has about 30 importers. That
split is part of the V1-1667 `corvint-test` restructure, and doing it in a spike would collide with
that work.

The new test `TestCoreHTTPImportersAreAllowlisted` (`cmd/corvint`) runs `go list -deps` and requires
the set of net/http importers in the core closure to equal a reviewed allowlist, with a reason for
each entry. Any new importer, or a stale allowlist entry, fails the test. A mutation check confirmed
this: renaming the `testrunner` entry made the test fail.

## Follow-up

When V1-1667 lands, move `RunE2E`/`RunFreshE2E` and testrunner device cleanup out of the packages
core links. Then delete the two allowlist entries; the test will require that deletion.
