# GPK-V0-020 assessment: binary archives and the applicable notices

Date: 2026-08-29 · Commit: `c4b3b255a4c0ef9b6b4736ea8436faf136a51a7c` · Work item: W11

`GPK-V0-020` (`docs/specs/go-production-kernel-migration-v0.md:401-405`) carries three obligations.
This assessment separates the two that are verified today from the one that is not.

## 1. Unchanged legal files — VERIFIED

`LICENSE`, `LICENSING.md`, and `PROVENANCE.md` are pinned by SHA-256 in
`conformance/release-artifact-v0/manifest.json` and re-hashed on every run by `checkLegalFiles`
(`conformance/release-artifact-v0/gate.go:186-207`), which fails `legal-file-missing` or
`legal-file-altered`. All three PASS in today's run.

## 2. Path-based split-license boundary not altered — VERIFIED, indirectly

The boundary is not a compiled artifact; it is the path enumeration written in `LICENSING.md`
("Apache-2.0 interoperability layer": `conformance/**`, `interop/**`, `examples/**`, and the listed
`docs/` protocol files; everything else AGPL-3.0-or-later). Because the
gate pins `LICENSING.md` byte-exactly, any edit to that enumeration fails the gate. The migration's
new Go paths (`cmd/corvint/**`, `internal/**`) fall on the product side by path, and the gate's own
code lives under `conformance/**`, which is the correct Apache-2.0 side. No boundary change is
present at this commit.

## 3. "Binary archives carry the exact applicable notices" — NOT VERIFIED for the Go artifact

**This is a genuine gap, and it is a gap in the world, not only in the checker: no binary archive
exists to carry notices.**

Evidence, all at commit `c4b3b25`:

- The reproducibility gate emits **loose binaries plus a checksum file only**. `buildTargets`
  writes `build-a/<name>`, `build-b/<name>` and `SHA256SUMS`
  (`conformance/release-artifact-v0/gate.go:235-245`, `build.go:writeChecksums`). Nothing is
  packaged.
- No archive code exists in the gate package:
  `rg -n 'archive/tar|archive/zip|\.tar|\.zip' conformance/release-artifact-v0/` returns no matches.
- The gate's `manifest.json` has **no archive section at all** — no member list, no layout, no
  notice paths. `Manifest` (`manifest.go:24-32`) declares `toolchain`, `profile`, `targets`,
  `legalFiles`, `smoke`, `pendingEvidence` and nothing else, and `decodeStrictJSON` rejects unknown
  fields, so an archive declaration cannot be added by manifest edit alone.
- The checker's closed reason set contains no archive-related reason
  (`conformance/release-artifact-v0/README.md:79-82`): `toolchain-mismatch`,
  `gomod-directive-mismatch`, `gomod-toolchain-directive-present`, `dirty-worktree`,
  `output-inside-repository`, `build-failed`, `not-byte-identical`, `buildinfo-mismatch`,
  `undeclared-runtime-dependency`, `legal-file-missing`, `legal-file-altered`, `smoke-failed`.
- The tool the plan reserves for this job does not exist. `docs/plans/go-production-kernel-migration.md:27`
  lists `tools/release_go_artifact.py` as the "local Go binary archive/checksum/reproducibility
  checker"; `ls tools/release_go_artifact.py` → no such file. `tools/` contains only
  `release_artifact.py`, which is the **Python wheel** checker.
- CI builds Go for the targets but packages nothing: `.github/workflows/ci.yml:80,114` run
  `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` and discard the output.
- `internal/releasegate/archive.go` is **not** an assembler. `internal/releasegate` is the offline
  evidence scanner behind `cmd/corvint-release-gate`, whose own header states it "never builds or
  publishes artifacts" (`cmd/corvint-release-gate/main.go:1-2`); `archive.go` unpacks and inspects
  existing tar/zip content for findings. It produces no release archive and checks no notices.

By contrast, the clause **is** discharged on the Python side, which is what makes the Go absence a
real asymmetry rather than a clause nobody implements: `tools/release_artifact.py` requires the
wheel to contain `<dist-info>/licenses/LICENSE` and `<dist-info>/licenses/LICENSING.md` in its
exact member inventory (lines 474-480), compares both against the source bytes and fails
`wheel-license-byte-mismatch` / `wheel-licensing-map-byte-mismatch` (lines 488-491), and pins
`License-Expression` and `License-File` in `METADATA` (lines 502-505).

### Verdict, and why it was not closed here

`GPK-V0-020` is **PARTIAL** at this commit: obligations 1 and 2 PASS; "binary archives carry the
exact applicable notices" is `UNVERIFIED` for the Go artifact, because no Go binary archive is
produced anywhere in the repository.

It was reported rather than closed, for three reasons:

1. **It is not small.** Closing it means choosing an archive format and layout per target
   (`tar.gz` for darwin/linux, `zip` for windows), deciding the member set and the archive name,
   extending `manifest.json` with an archive section plus new closed failure reasons, and — because
   this gate's entire claim is byte-identity — proving the *archives* are reproducible too. Tar and
   zip both carry mtimes, uid/gid/uname, ordering, and gzip header fields that are non-deterministic
   by default. A half-proved archive-determinism claim would be worse than a named gap.
2. **The archive shape is an owner/publication decision, not an inference.** `GPK-V0-020` says
   archives carry the exact applicable notices; it does not say what the archive is. Picking the
   layout inside a reproducibility work item would fabricate a publication contract.
3. **W11's stated scope is builds, and publication is explicitly out of scope** for this gate — the
   README already disclaims signature, attestation, SBOM, and upload
   (`conformance/release-artifact-v0/README.md`, "What is not proved"). Archive assembly belongs
   with publication.

### What closing it would require

A separate work item that: defines the per-target archive (format, name, member layout) in
`manifest.json`; assembles it deterministically (fixed member order, zeroed mtime/uid/gid/uname,
no gzip mtime/OS byte); includes the binary plus the exact `LICENSE`, `LICENSING.md`, and
`PROVENANCE.md` bytes already pinned in the manifest; verifies each member against its pinned digest
with new closed reasons (e.g. `archive-member-inventory`, `archive-notice-byte-mismatch`,
`archive-not-byte-identical`); and proves two same-profile assemblies are byte-identical, so the
archive inherits the same standard of proof as the binary it wraps.

Until that exists, `GPK-V0-020` must not be reported as fully satisfied for Go binaries.
