## 2026-09-27 V1-0429: analyzer-go darwin tests pinned the owner host's Homebrew Go

Attempt 2 of the owner-dispatched release-gate run 36338249213 on the `1.0.0-rc.1` candidate c9019c4b
failed the macos-15 `full-gate` in `cmd/corvint-analyzer-go`. Every other package passed. Six tests
failed:
- `TestCompiledCLICompleteBoundaryMatrix`
- `TestCompiledCLIFrozenGoldens`
- `TestCompiledCLI1024DistinctFreshProcessCorpus`
- `TestProductionAttemptObservingSpy`
- `TestProductionGoRuntimeGetenvTrap`
- `TestProductionCapabilitySandboxSpy`

On darwin, `pinnedGoBuildEnvironment` built the compiled CLI with the owner host's Homebrew receipt,
`/opt/homebrew/Cellar/go/1.27.1/libexec/bin/go`, and set `GOROOT` from it. `getenvTrapOverlay` read
`src/os/env.go` from the same fixed root. The hosted runner's go1.27.1 comes from `actions/setup-go`, so
the path did not exist and the failure was deterministic. Attempt 1 stopped at V1-0427 before this
package ran.

`darwinGoTool` keeps the exact receipt whenever it exists, so the owner host builds exactly as before.
A darwin host without it now uses the Go on `PATH` and the `GOROOT` that tool reports. The rest of the
hermetic darwin build environment is unchanged. Linux still uses its provisioned Go.

The hosted log also shows dyld refusing the arm64 spy dylib in arm64e system binaries. The runner has
SIP disabled, so `DYLD_INSERT_LIBRARIES` reaches `/usr/bin/true`. The test only logs that negative
control output. The hosted macOS full gate for the next candidate is the check that the spy tests pass
there.

Evidence on the owner's darwin host:
- vet passed for darwin and for linux/amd64;
- the package passed with the receipt present;
- with `pinnedGoTool` pointed at a missing path, the six failing tests passed through the fallback.

Rollback: revert the change. The hosted macOS full gate then fails again in this package.
