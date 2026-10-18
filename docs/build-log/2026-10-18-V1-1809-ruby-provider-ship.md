# V1-1809, V1-1810, V1-1811: ship the Ruby provider, share retention ignore, portable helpers

Owner input: tickets V1-1809, V1-1810 and V1-1811 (GitHub #1128, #1130). Base `59cc560b9`.
`CRB-V0-020` (new) and the in-place amendments of `LPCV-V0-095` and `LPCV-V0-084` are accepted
under the owner's standing auto-accept of lane requirements (2026-10-11).

## V1-1809: companion bundle profile `/3`

`CRB-V0-019` fixed the core bundle at exactly nine binaries, so adding
`corvint-ruby-test-provider` to `/2` would have made every retained `/2` bundle unreadable or the
reader open. The writer now emits `corvint-companion-bundle/3` (the `/2` inventory plus the Ruby
provider, its manifest row, `SHA256SUMS` line and one usage smoke step); `/2` stays a closed,
readable profile, and core installed qualification accepts either closed binary set. The smoke
step checks only exit 2 usage admission, so it never claims RSpec execution. No release was built
or published.

## V1-1811: shared retention ignore helper

`internal/rubytestprovider` could not import `internal/testevidence` because of the cycle
`rubytestprovider -> testevidence -> testvaliditydoc -> rubytestprovider`. `testevidence` now
declares its own `EvidenceDirectory` and imports no other Corvint package; the one test that
needs `testvaliditydoc.Discover` moved to the external `testevidence_test` package, and
`TestEvidenceDirectoryMatchesDiscovery` pins the two constants equal. The provider's private
`.gitignore` writer is gone: its report directory calls `testevidence.IgnoreRetained` and its
retention goes through `testevidence.Retain`. Behavior change, recorded as the `LPCV-V0-095`
amendment: an existing `.corvint` now gets `.gitignore` in the report and evidence directories
only (mode 0600, empty file repaired, non-regular entry refused), never `.corvint/.gitignore`.

## V1-1810: portable discovery helpers

The `discover_test.go` helpers moved to the unconstrained `discover_helpers_test.go`;
`boundReceipt` now JSON-encodes its path key so a Windows path is valid JSON.
`TestDiscoveryHelpersArePortable` uses them on every platform. Tests that need FIFOs or
Discover's receipt reads stay in `darwin || linux` files. `GOOS=windows go vet` passes.

## NOT_RUN

- Exact two-build and installed qualification of a `/3` bundle (needs a release cut).
- Real Ruby/RSpec execution by the provider.
- Windows test execution (only Windows vet ran).
- The 137-unit affected plan beyond the focused packages listed in the PR body.
