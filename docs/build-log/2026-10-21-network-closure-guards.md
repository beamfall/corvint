# V1-1781: MTV-V0-007 stays direct-only; a separate guard covers the transitive network closure

Spike ticket V1-1781. Decisions are delegated to the orchestrator (decision 0548). Requirement
`MTV-V0-010` in [mcp-test-validity-profile-v0](../specs/mcp-test-validity-profile-v0.md) was accepted
under the owner's standing auto-accept of lane requirements (2026-10-11).

## Finding

I ran `go list -deps ./internal/mcp/testvaliditybridge` at base `90ab9f33e`. The closure contains 62
module packages. Of those that directly import `net`, `net/*` or `os/exec`:

- **Network packages that open connections.** Only `internal/jstestprovider` (`net`, `net/http`). It
  serves loopback E2E readiness probes, reached only from `RunE2E`/`RunFreshE2E`. None of those
  callers are in the tool closure: changee2e, playwrightminimize, testacceptance and
  corvint-js-test-provider are all absent, as V1-1670 found for core.
- **Parsing-only packages.** `net/url` is imported by appflows, appmap, lspprovider, stepnegation,
  testsource and jstestprovider.
- **`os/exec`.** Imported by 11 packages, including the shared Git readers `contextindex`,
  `cem/gitrun`, `cem/verify` and `gitstatus`, plus `procgroup`, `groupreap`, `gokernel` and
  `liveverify/mutate`.

A transitive "no `os/exec`" rule cannot hold without forking the shared index packages. Corvint reads
immutable Git evidence through `git` subprocesses. The tool's no-process behaviour is established by
the `MTV-V0-007` own-source check and the read-only conformance vectors, not by the import closure.

## Proposed recommendation (applied)

1. Keep MTV-V0-007 scoped to direct imports, as its text already says.
2. Add MTV-V0-010, an exact, reviewed allowlist of module packages in the tool closure that import a
   network package capable of opening connections. The allowlist currently holds only
   `jstestprovider`.

The guard `TestTestValidityToolNetworkImportersAreAllowlisted` lives in `cmd/corvint`, next to the
V1-1670 core guard, and shares its `go list -deps` helper. It cannot live in the bridge package:
that package's tests run under a narrow declared read scope (`.corvint/test-read-scopes.json`),
which cannot list the closure.

Mutation check: removing the `jstestprovider` entry made the guard fail with the importer named.
After the V1-1667 split moves the E2E runner out of `jstestprovider`, both allowlists shrink, and both
guards will require that.

## Review follow-up: explicit build configurations

The first version of both guards listed the closure only for the host platform. On Windows,
`internal/testrunner/device_cleanup_unix.go` is excluded, so the core guard would fail there, and a
platform-only importer stayed invisible on other hosts.

Both guards now run `go list -deps` with `CGO_ENABLED=0` and an explicit `GOOS`/`GOARCH` for
`darwin/arm64`, `linux/amd64` and `windows/amd64`. Each configuration has its own exact expected set:

| Configuration | Core `net/http` importers |
|---|---|
| `darwin/arm64` | `jstestprovider`, `testrunner` |
| `linux/amd64` | `jstestprovider`, `testrunner` |
| `windows/amd64` | `jstestprovider` |

The tool path's set is `jstestprovider` in all three configurations. Every expected importer must
carry a review reason. An importer with a reason that no configuration expects fails the guard.

Mutation check: adding `testrunner` to the `windows/amd64` expectation fails with the measured
set named.
