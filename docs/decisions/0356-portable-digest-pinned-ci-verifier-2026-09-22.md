# Decision 0356 — Portable digest-pinned CEM CI verifier as a separate `cem01-go ci` mode

Date: 2026-09-22. Status: proposed (experimental delivery; ticket V1-0015). Amends
`docs/specs/cem-pilot-kit.md` (CEM-PILOT-020 to CEM-PILOT-023) and adds one non-goal note to
`docs/specs/cem-go-portability-probe.md`; it changes no CEM wire, no `protocol/**` or
`conformance/**` material, and no `cem01-go verify` behaviour.

## Context

The existing CI example (`examples/cem/verify-pr.sh`, CEM-PILOT-009) asks the adopter to build and
check in a `corvint` executable. Its no-network and content-free claims were asserted, not tested,
and its failures share one non-zero exit. The dependency-free `interop/cem01-go` consumer already
verifies `cem/0.1` maps, and as a Go module it can be pinned by pseudo-version and checksum
database hash with no checked-in binary.

## Decision

1. Add a `ci` mode to `cem01-go` rather than change `verify`. `verify` keeps its adapter ABI
   (CEM-GO-002); `ci` derives the patch with the `docs/CEM-CI.md` profile, reads the map from the
   head tree, delegates all structural and drift checks to `verify` through private temporary
   files, and writes one `cem-ci-report/0` line.
2. The report has 14 fixed members. It holds digests, paths, spans, counts and verdicts, never
   source or diff text. It is encoded without HTML escaping so that 4096 drift items stay under
   the 6 MiB bound.
3. Exits are distinct: 0 accepted, 1 rejected, 2 operational, 3 missing evidence, 4 unsupported
   profile, 5 repository mismatch. Unsafe drift ranks before unknown hunks.
4. The example workflow pins the module version and the built executable's SHA-256. It builds only
   in a fetch step, and the script checks the digest before executing. Verification runs under
   `unshare --net`, so the no-network claim is enforced. The test suite reproduces that step with
   an OS network sandbox (`sandbox-exec` or `unshare`), after proving that the sandbox refuses a
   loopback connection.

## Rejected alternatives

- Proving no-network statically from the import graph: `crypto/x509` links `net`, so the proof
  fails for any Go binary that hashes.
- Reinstalling with `GOPROXY=off` to show offline reproducibility: `go install MODULE@VERSION`
  still looks up deprecation and fails. Two installs with fresh module caches from the same
  proxy agreeing on the digest is the reproducibility evidence instead.
- Reading the map and patch in memory by refactoring `verify`: rejected to keep `verify`'s error
  precedence byte-identical.

## Rollback

Delete `interop/cem01-go/ci.go`, `ci_test.go`, the `ci` dispatch in `main.go`, the `gitTo` split
in `cem.go`, `examples/cem/verify-portable.sh`, `examples/cem/github-actions-portable.yml` and
`examples/cem/README.md`, and withdraw CEM-PILOT-020 to CEM-PILOT-023. No map or repository data
changes.

## Open evidence

The fetch from `proxy.golang.org` and the GitHub runner `sudo unshare --net` path are `NOT_RUN`.
No published pseudo-version or executable digest is recorded. The claim that a darwin
cross-compile equals a native linux/amd64 build is inferred from Go's reproducible-build design,
not observed.
