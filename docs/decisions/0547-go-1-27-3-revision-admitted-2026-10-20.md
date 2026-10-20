# Decision 0547 — go1.27.3 is admitted as a patch revision of the go1.27.2 pin

Date: 2026-10-20. Status: accepted. Authority: repository owner, in chat on 2026-10-20: "allow
1.27.3 — it's only a revision, not a big deal". Ticket: V1-2118. It amends decision 0522 without
superseding it. go1.27.2 stays the pin, and go1.27.3 is admitted beside it as an equally supported
patch revision.

## Context

Homebrew upgraded the host's default `go` to 1.27.3. Every check that names the exact pin
refused under it. `make go-version` refused, and with it `make windows-vet`. The `CEM-V1-050`
structural runtime gate also refused, so `TestCEMV1144StableStructuralMark` ended UNSUPPORTED with
exit 2. Five `cmd/corvint-go-test-provider` tests failed because their fixture took the harness
GOROOT as the provider's frozen go1.27.2 toolchain.

## Decision

- `go.mod` keeps `go 1.27.2`. The Makefile `go-version` target and the `AGENTS.md` verify line
  accept exactly `go1.27.2` or `go1.27.3`. Neither accepts a range.
- `CEM-V1-050`: `go1.27.3` joins `go1.27.1` and `go1.27.2` in `structuralTuples`
  (`internal/cem/verify/stable.go`). It is admitted on the sealed packet
  `protocol/cem-1.0/structural-tuples/go1.27.3/`, made of `TUPLE.json` plus `manifest.json`. The
  packet is shaped like the go1.27.2 packet. It binds the frozen S0E packet manifest and its two
  result files by SHA-256, and declares a single substitution of `expectedResult.runtime.goVersion`
  from `go1.27.1` to `go1.27.3`. It records the qualification evidence on darwin/arm64:
  - The 70-package closure of the five structural stdlib packages has 2323 files. All of them are
    byte-identical to the official go1.27.2 toolchain.
  - The go/ast, go/format, go/parser, go/printer, go/scanner, go/token, go/build/constraint and
    cmd/gofmt trees are identical across go1.27.1, go1.27.2 and go1.27.3.
  - A 4621-file corpus plus the 144-pair structural fixture matrix produced byte-identical output
    under all three toolchains.
  - The native S0E public cases and the linked control pass through the binding.

  Qualification stays `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`. The portable gate in
  `interop/cem01-go` (`cem.go` `structuralTuples` and its `TestCEMV1050StructuralTupleAdmission`)
  admits go1.27.3 in the same change. Its test treats any toolchain that binds a sealed packet as
  admitted, so it fails on every toolchain once this packet exists without that admission.
- The go-live-test provider tuple stays `go1.27.2` (`internal/liveverify/provider.GoVersion`).
  When the harness is built by another admitted revision and `CORVINT_GO_LIVE_TEST_GOROOT` is
  unset, the `cmd/corvint-go-test-provider` fixture now uses the official go1.27.2 toolchain from
  the module cache if one is installed there. The exact `go version` check is unchanged.

Deliberately unchanged at go1.27.2:

- `.github/**`. CI keeps installing and asserting go1.27.2. Changing it needs the owner's
  ci-control-plane post.
- `script/public-release-check` and `script/corvint-companion-release-gate`. Release builds stay
  reproducible on the pin.
- `.taskman/**`.
- The go-live-test provider and release-identity tuples named in decision 0522
  (`internal/liveverify/provider`, `conformance/**`, `internal/companionrelease`,
  `internal/releasecandidate`, `tools/**`).
- The CI shard and test-slice provenance checkers.

## Consequences

- A go1.27.3 host passes `make go-version` and `make windows-vet` and runs the structural proof.
  Release, companion and CI identity checks still require go1.27.2.
- A go1.27.4 or later host still refuses everywhere. Admitting it needs its own decision and its
  own sealed packet.

## Rollback

Revert this commit. go1.27.3 then refuses the go-version check and structural proof, as before,
and the go1.27.2 pin applies unchanged.
