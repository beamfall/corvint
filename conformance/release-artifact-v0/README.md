# Corvint release artifact reproducibility gate V0

This conformance package has two separate local gates. The unchanged default command implements the
loose-binary reproducibility half of `GPK-V0-018`. The `archive` command implements accepted
`ARTIFACT-GO-V0-001..007`: it performs two raw-commit exports, two cold builds and two canonical
assemblies for each target, then passes the results through a strict request to a separately compiled
offline verifier process.
Neither command publishes, uploads, signs, tags, or promotes anything, and neither writes a build
or candidate output into the repository.

`manifest.json` is the profile authority. The checker never infers a target, a build flag, an
environment value, or a legal-file digest. The runner is implemented entirely with the Go standard
library and invokes only `go` and `git`.

## What is proved

- **One exact clean commit.** The gate refuses a dirty worktree before building anything, and then
  re-proves the binding from inside each artifact: the embedded `vcs.revision` must equal `HEAD` and
  `vcs.modified` must be `false`. A binary whose provenance does not name the scanned commit fails.
- **One exact toolchain.** `go env GOVERSION` must equal `go1.27.1` exactly, `go.mod` must carry the
  exact `go 1.27.1` directive, and it must carry **zero** `toolchain` directives, because Go 1.27
  normalizes an equal directive away. Every `go` invocation the gate makes runs under
  `GOTOOLCHAIN=local`, so selecting or downloading another toolchain fails rather than succeeding
  silently.
- **Offline, CGO-free, trimmed builds.** `CGO_ENABLED=0`, `-trimpath`, `GOFLAGS=-mod=readonly`,
  `GOPROXY=off`, `GOSUMDB=off`. The build environment is a closed allowlist, so an inherited
  `GOFLAGS` or `GOPROXY` cannot change the profile. Each artifact's embedded settings are re-checked
  for `CGO_ENABLED=0`, `-trimpath=true`, and the expected `GOOS`/`GOARCH`.
- **Two same-profile builds are byte-identical.** This is the core claim. Each of the two builds gets
  its **own cold `GOCACHE`** and its own `TMPDIR`. A shared cache would prove only that the cache was
  reused; a cold cache forces genuine recompilation, so the comparison is a determinism test rather
  than a cache-hit test. A mismatch reports the first differing byte offset, never a bare assertion.
- **One checksum per exact artifact.** `SHA256SUMS` is written beside the build outputs.
- **No undeclared runtime dependency.** `go.mod` must declare zero module requirements and each
  artifact's embedded dependency list must be empty.
- **Unchanged legal files.** `LICENSE`, `LICENSE-APACHE-2.0`, `LICENSING.md`, and `PROVENANCE.md` are
  pinned by digest in the manifest, per `GPK-V0-020`, which forbids the migration to alter them.

## What is not proved

- **Native execution on four of five targets.** The smoke test runs only where the host can
  genuinely execute the artifact. Every other target reports smoke status `NOT_RUN` with its reason.
  `GPK-V0-018` states that cross-compilation is not execution evidence, so the native conformance,
  filesystem-safety, process-cleanup, and read-nonmutation matrices remain owed on each operating
  system that has not run them. `NOT_RUN` is visible evidence of an unexecuted check; it is never a
  skip and never a pass.
- **The release gate as a whole.** A `PASS` here is a build-reproducibility verdict only. The
  `pendingEvidence` field and the `pending-evidence=` field of the `SUMMARY` line name the evidence
  that is still owed — currently the W10 performance protocol (`GPK-V0-016..017`). The release gate
  cannot be declared satisfied while that list is non-empty.
- **Publication and authentication.** The archive command closes the exact-notice byte check in
  `GPK-V0-020` for its named revision, but creates no signature, identity, attestation, SBOM, tag,
  upload, publication, promotion, cross-builder claim, native cross-target execution, or legal
  conclusion. The checksum is an integrity observation, not publisher authentication. See the
  dated assessment under `results/2026-08-31-go-archive/`.
- **Cross-builder reproducibility.** Like `release-artifact-integrity-v0`, the claim is
  same-profile. A second controlled builder remains a separate gate.

## Binary name

The artifact is `corvint`, built from `./cmd/corvint`. `GPK-V0-015` forbids installing the
candidate as `corvint` and `GPK-V0-023` gates that name on evidence that does not exist. The manifest
validator refuses `binaryName: "corvint"` outright, so the gate cannot be pointed at the forbidden
name by a manifest edit.

## Run

The output directory must be outside the repository; the gate refuses an in-repository output so a
run can never leave a binary where it could be committed.

```sh
script/check-release-artifact-reproducibility.sh
```

or directly:

```sh
GOTOOLCHAIN=local go run ./conformance/release-artifact-v0 \
  --output /tmp/corvint-release-artifact \
  --report /tmp/corvint-release-artifact/report.json
```

The separate archive gate requires an existing real invoking-user-owned `0700` parent and an absent
destination. It keeps all scratch and staging directories beneath that trusted external parent,
fsyncs the completed staged files and directory, rechecks the exact clean commit/tree, and atomically
renames the stage into place:

```sh
parent=$(mktemp -d /private/tmp/corvint-release-parent.XXXXXX)
GOTOOLCHAIN=local go run ./conformance/release-artifact-v0 archive \
  --revision HEAD --output "$parent/corvint-go-release"
```

The retained directory contains exactly five archives, archive-level `SHA256SUMS`, and
`verification-report.json`. `archive-report.schema.json` and `archive-witness.schema.json` freeze
the compact, path-free V1 schemas as conformance detail. The report derives its archive and total
byte ceilings from the verified loose binaries, exact legal blobs, checksum bytes, and a fixed
1 MiB framing allowance; decoded content uses the exact derived content ceiling. The separately
compiled verifier also enforces immutable 64 MiB archive and decoded-content ceilings, regardless
of request values. Reports are capped at 1 MiB and private witnesses at 64 KiB.
`make go-archive-gate` runs the full five-target profile under a bounded temporary parent, and the
canonical `make gate` includes it.

`archive-status` strictly reads the owner-only `0600` private witness: unknown/duplicate/trailing
JSON, stale revision/tree, wrong target order, invalid hashes/sizes, or a noncanonical encoding can
never become `PASS`. `script/release-checklist` uses that operation without repairing any output.

The offline verifier executable lives in `archive-verifier`; its verification logic lives in
`archiveverify`, and the two sides share data-only `archivewire`. Source-level tests forbid the
verifier process and library from importing the assembler, process execution, or network packages.
The verifier independently parses the exact manifest/legal authority, reads and hashes both loose
builds and both assemblies under derived limits, reconstructs canonical container bytes, and checks
raw framing, fixed metadata, closed inventory/order, checksums, target build information, and the
double-build/double-assembly evidence before emitting a strict result.

Exit codes for both gates are `0` verdict `PASS`, `1` verdict `FAIL`, and `2` usage or environment
error. The default loose-gate typed reasons are closed: `toolchain-mismatch`,
`gomod-directive-mismatch`, `gomod-toolchain-directive-present`, `dirty-worktree`,
`output-inside-repository`, `build-failed`, `not-byte-identical`, `buildinfo-mismatch`,
`undeclared-runtime-dependency`, `legal-file-missing`, `legal-file-altered`, `smoke-failed`.

Loose-gate evidence lives in `evidence/` and dated `results/` directories. Archive build outputs and
candidate reports remain external; only bounded dated assessments of their exact digests and verdict
are committed. The repaired exact-revision assessment is
`results/2026-08-31-go-archive/gpk-v0-020-notices-assessment-7638cf3.md`.
