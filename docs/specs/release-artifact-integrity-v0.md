# Release Artifact Integrity V0

- Owner: Russell Lewis
- Date: 2026-08-23
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: `AGENTS.md`, `pyproject.toml`, `PROVENANCE.md`, `LICENSE`, `LICENSING.md`, and the
  repository-owner's AGPL-3.0 product / Apache-2.0 interoperability license and public-release decision

## Agent digest
- Claim: Local gates verify deterministic Go archive artifacts; archive integrity authorizes no signing, tagging, publication, or promotion.
- Status: proposed overall/experimental; ARTIFACT-GO-V0-001..007 accepted and implemented 2026-08-31; ARTIFACT-GO-V0-008 accepted and No signing selected for the 0.4.0a4 alpha only (decision 0108, 2026-09-12); ARTIFACT-GO-V0-009 accepted (decision 0142, 2026-09-12: the gates keep refusing untracked paths); `tools/release_artifact.py`, `requirements/release-build.txt`, and `release-wheel-witness` removed by `54735d98` (2026-09-11)
- Exists: deterministic local Go-archive checks, a read-only release-readiness checklist, and its publication receipt reader (`ARTIFACT-RDY-V0-006..009`, decision 0141). No wheel checker exists.
- Blocked on: owner-only tag, publication, promotion, and signing decisions; missing evidence stays `NOT_RUN`.
- Read next: Requirements (`ARTIFACT-V0-001..010` dispositions); User and job; Verified current state; Release-readiness checklist.

## User and job

A Corvint maintainer needs to know that the installable wheel was built from one exact Git revision,
contains exactly the intended runtime and license bytes, installs without an index or dependencies,
and has not changed between verification steps. This slice is a local and pull-request gate. It does
not publish or authenticate a release.

## Verified current state

At Git commit `7790e1f7aed4360a2bdbec71e76eaed67e6fad1a`, two wheel builds from separate
committed-tree exports with `SOURCE_DATE_EPOCH=1787492542` produced identical bytes within each
builder profile. The exploratory Python 3.9.6 profile produced SHA-256
`f8cbe64384d526acc8bc305c9da9d9efdaafffa075e96e823c1ed378270577f7`; the locked
Python 3.14.7 profile produced
`cbfc83919e0c028d745bdef5fb685069e50e6e447d27321ca0d56f7c15df4395`. This
cross-builder difference is why the builder is pinned and why this contract makes only a same-profile
reproducibility claim. A fresh no-index, no-dependency installation imported all 16 modules,
returned `Corvint 0.4.0a0`, and served a real read-only query.

Two sdists built from that same revision and epoch did not produce identical bytes. They also
included the test suite while omitting the conformance, interoperability, specification, and
provenance assets needed for a deliberate source-release policy. No sdist is in scope.

## Requirements

- `ARTIFACT-V0-001`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `ARTIFACT-GO-V0-001`) The checker MUST resolve one exact Git commit, record its tree and commit
  timestamp, and reconstruct two independent source trees directly from that commit's raw tree and
  blob objects. Ambient Git environment, replacement refs, worktree modifications, info attributes,
  ignored files, and untracked files MUST NOT alter either source tree. The Python wheel this
  guarded is retired; `ARTIFACT-GO-V0-001` requires the same exact-commit resolution and the same
  closed-environment/raw-export resistance to ambient Git state, now for the Go archive builder,
  implemented in `conformance/release-artifact-v0/archive_run.go` (`GIT_NO_REPLACE_OBJECTS=1`,
  closed `GIT_CONFIG_*`/`HOME`, `ls-tree --full-tree` raw export, dirty-worktree refusal). This is
  full coverage of the same obligation under a different artifact, not a retirement.
- `ARTIFACT-V0-002`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `ARTIFACT-GO-V0-001`) The release-builder interpreter and every Python build package MUST have one
  exact accepted version. Build packages MUST be installed from a hash-locked requirements file.
  The checker MUST invoke no installer or dependency resolver and MUST invoke the exact exported
  backend in isolated interpreter mode with build isolation disabled only after validating the
  installed versions. This is not an operating-system network sandbox for backend code. The Python
  interpreter/build-package pinning this described no longer exists; `ARTIFACT-GO-V0-001` pins the
  analogous builder identity instead (`go1.27.1`/`GOTOOLCHAIN=local`) and its manifest
  (`conformance/release-artifact-v0/manifest.json`) sets `GOPROXY=off`, `GOSUMDB=off`, and
  `GOFLAGS=-mod=readonly` — the offline, no-installer, locked-dependency analog of a hash-locked
  requirements file. Coverage is translated, not identical: Go has no "build backend/isolation
  mode" concept, so that specific clause has no literal successor and needs none — the ambient-
  toolchain risk it guarded against is closed by the `GOTOOLCHAIN=local` pin instead.
- `ARTIFACT-V0-003`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `ARTIFACT-GO-V0-002`) Each build MUST produce exactly one regular `py3-none-any` wheel and no
  other artifact. A source distribution or surplus output MUST fail the check. `ARTIFACT-GO-V0-002`
  requires exactly one archive per manifest target with fixed naming and a closed member inventory,
  the same "exactly one output, no other artifact" obligation applied to five platform archives
  instead of one wheel. Full coverage.
- `ARTIFACT-V0-004`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `ARTIFACT-GO-V0-004`, jointly with `ARTIFACT-GO-V0-002` and `ARTIFACT-GO-V0-003`) The wheel's
  standard `RECORD` file is the per-member manifest. Before decompression, the checker MUST reject
  duplicate, absolute, non-canonical, traversal, backslash, encrypted, special-type, missing,
  surplus, or oversized members; disagreement between local and central flags, compression, CRC,
  sizes, or names; unsupported general-purpose flags; ZIP archive or member comments; extra fields;
  and leading or trailing bytes outside the canonical container. Every runtime module and the
  combined license and split-license map MUST match the exported source byte-for-byte; metadata,
  console entry point, pure-wheel tag, and top-level module inventory MUST match the exported
  source and contract. `ARTIFACT-GO-V0-004` requires the same closed set of rejected member forms
  (absolute, dot, traversal, duplicate, case-fold-colliding, link, device, sparse, encrypted,
  nested-archive) and canonical container metadata for tar/ZIP; `ARTIFACT-GO-V0-002` fixes the
  member inventory and order, and `ARTIFACT-GO-V0-003` binds `SHA256SUMS` to member bytes. Full
  coverage of the same membership-integrity obligation, translated from wheel/`RECORD` to
  tar.gz/ZIP.
- `ARTIFACT-V0-005`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `ARTIFACT-GO-V0-005` and `ARTIFACT-GO-V0-007`) The two independently exported builds MUST be
  byte-identical. A digest or size difference or any later verification failure MUST leave no
  output. The destination parent MUST be an owner-only directory owned by the invoking user. Only a
  fully checked wheel and checksum MAY be atomically promoted to a previously nonexistent
  destination inside that trusted parent. `ARTIFACT-GO-V0-005` requires the same double-build/
  double-assembly byte-identical property (cold caches, separate temporary directories, no
  candidate retained on mismatch); `ARTIFACT-GO-V0-007` requires the same owner-only,
  previously-nonexistent-destination atomic promotion. Full coverage.
- `ARTIFACT-V0-006`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `ARTIFACT-GO-V0-003`) The checker MUST emit one canonical `SHA256SUMS` row for the retained wheel
  and verify it before and after installation. A one-byte artifact change MUST fail. The checksum
  is an integrity observation, not publisher authentication. `ARTIFACT-GO-V0-003` requires the same
  canonical `SHA256SUMS` row, bound to both the archive member bytes and the loose binary, and a
  mutation fails closed. The "integrity, not authentication" disclaimer for archives is stated in
  `ARTIFACT-GO-V0-008`, accepted by decision 0108; with the checksum mechanism already implemented
  and accepted, this is full coverage of the operative obligation.
- `ARTIFACT-V0-007`: (superseded 2026-09-12 by decision 0088; obligation now carried by
  `GPK-V0-018` in `docs/specs/go-production-kernel-migration-v0.md`, not by an
  `ARTIFACT-GO-V0-0NN` row) The retained wheel MUST install into a fresh environment with no index,
  dependencies, or cache. Every shipped module MUST import from that environment, `corvint --version`
  MUST return the packaged version, and one real read-only query against a temporary Git repository
  MUST succeed without changing its worktree status. No requirement in the `ARTIFACT-GO-V0-0NN`
  family states this smoke obligation, even though `conformance/release-artifact-v0/smoke.go` (used
  by `conformance/release-artifact-v0/gate.go`) implements it for the Go archive: it runs
  `--version` and a real read-only query against a fixture. The requirement text that actually
  names this — "build and smoke-test darwin and linux on amd64/arm64 and Windows on amd64 ...
  verify `corvint --version` and a real read-only query" — is `GPK-V0-018`. Full coverage of the same
  obligation, translated from "every module imports" (Python) to "the single static binary runs",
  but the successor lives in a different spec than the other nine rows in this family.
- `ARTIFACT-V0-008`: Pull-request CI MUST grant only `contents: read`, disable persisted checkout
  credentials, use full-commit action pins, set bounded timeouts, and expose no secret, release,
  publishing, signing, or artifact-upload step to untrusted pull-request code. Still live:
  `.github/workflows/ci.yml` is unaffected by decision 0088 and, confirmed 2026-09-12, still
  declares `permissions: contents: read` (line 12) and `persist-credentials: false` (lines 22, 56,
  80) with 40-character action pins. This requirement never depended on the wheel. The Python test
  suite that verified it statically was deleted with the rest of `tests/test_release_artifact.py`
  in `54735d98`; `script/check-ci-least-privilege.sh` (Makefile target `ci-least-privilege-check`,
  a `gate` prerequisite) is the replacement, added 2026-09-12. It reads the workflow from the Git
  index, not the worktree, so it measures what a fresh clone contains;
  `script/check-ci-least-privilege_test.sh` (`ci-least-privilege-test`, also a `gate` prerequisite)
  pins both directions and holds a checkout step to it whichever key the step opens with.
- `ARTIFACT-V0-009`: `docs/extraction-manifest.json` MUST NOT be represented as release-manifest
  verification. It records original Beamfall source hashes but has no destination mapping and
  requires the source repository to verify; that work remains part of the provenance decision.
  Still live: `docs/extraction-manifest.json` still exists at this revision and this obligation
  never depended on the wheel or on Python packaging.
- `ARTIFACT-V0-010`: (retired 2026-09-12 by decision 0088) Corvint MUST NOT build or publish an sdist
  until its exact inclusion policy, reproducibility, and mandatory legal files have been accepted
  and tested. An sdist is a Python source-distribution artifact; the Go archive family has no sdist
  analog (Go source is the Git tree itself, not a separately packaged distribution), and the wheel
  and its packaging pipeline (`tools/release_artifact.py`, `requirements/release-build.txt`,
  `release-wheel-witness`) were removed by `54735d98`. Nothing can ever satisfy or violate an
  sdist-inclusion policy that has no packaging pipeline behind it. This stable ID is retained for
  the retirement record and MUST NOT be reused for a successor.
- `ARTIFACT-GO-V0-001`: the archive builder MUST consume the exact clean commit, target matrix,
  `go1.27.1` / `GOTOOLCHAIN=local` selection, `CGO_ENABLED=0`, `-trimpath`, offline module settings,
  binary names, and legal-file digests already pinned by
  `conformance/release-artifact-v0/manifest.json`. It MUST reject drift instead of selecting a
  toolchain, target, dependency, file, or notice from ambient state.
- `ARTIFACT-GO-V0-002`: one archive MUST be produced per manifest target. Darwin and Linux use
  `corvint_<goos>_<goarch>.tar.gz`; Windows uses `corvint_windows_amd64.zip`. Each container has
  one root directory matching the archive basename without `.tar.gz` or `.zip`, then exactly the
  target binary, `LICENSE`, `LICENSE-APACHE-2.0`, `LICENSING.md`, `PROVENANCE.md`, and
  `SHA256SUMS`, in bytewise path order. No other member, directory entry, comment, extra field,
  leading byte, or trailing byte is permitted.
- `ARTIFACT-GO-V0-003`: `SHA256SUMS` MUST contain exactly one LF-terminated row for the binary,
  using lowercase SHA-256, two ASCII spaces, and its root-relative basename. Verification MUST bind
  that row to both the archive member bytes and the corresponding byte-identical loose binary from
  the existing reproducibility gate.
- `ARTIFACT-GO-V0-004`: archive metadata is canonical rather than inherited. Regular-file modes are
  `0755` for the binary and `0644` otherwise; uid/gid are zero; user/group names and comments are
  empty; member order is fixed; timestamps are zero for ustar/gzip and the ZIP minimum DOS instant;
  tar uses ustar plus gzip with fixed header fields; ZIP uses stored members with no data
  descriptors or extra fields. Path separators are `/`, and absolute, dot, traversal, duplicate,
  case-fold-colliding, link, device, sparse, encrypted, and nested-archive members fail closed.
- `ARTIFACT-GO-V0-005`: every target binary MUST be built twice with separate cold caches, and each
  corresponding archive MUST be assembled twice in separate temporary directories. Both binary
  pairs and both archive pairs MUST be byte-identical. The checker records SHA-256 and byte count
  for every retained binary and archive; a mismatch retains no candidate output.
- `ARTIFACT-GO-V0-006`: verification MUST be offline and independent of the assembler. From archive
  bytes plus the pinned manifest and exact commit, it rechecks the closed member inventory,
  container framing and metadata, every legal-file byte, `SHA256SUMS`, the binary digest/build info,
  target coordinates, and the double-build/double-assembly report. It invokes no artifact, installer,
  package manager, network client, signing service, or publisher.
- `ARTIFACT-GO-V0-007`: all build and candidate output stays outside the repository. Only after every
  target passes may the checker atomically retain the archives, an LF-sorted archive-level
  `SHA256SUMS`, and a canonical verification report in a previously nonexistent owner-only output
  directory. It also records the bounded private status witness at
  `$(git rev-parse --absolute-git-dir)/corvint/release-go-archive-report.json`, with exact schema,
  revision, archive digests, and `PASS|FAIL`; failure to record it does not relabel the run. A
  read-only status operation may inspect those outputs but never repair them.

Current archive identity is exactly `corvint` (`corvint.exe` on Windows), module
`github.com/Beamfall/corvint`, source package `./cmd/corvint`. Current reports and witnesses
use `corvint.release-go-archive-report.v2` and `corvint.release-go-archive-witness.v2`. Readers MUST
retain closed v1 Corvint tuples unchanged, dispatch by schema version, and reject mixed identities.
Legacy evidence cannot qualify new source. Publication readers accept only the `corvint`
destination and archive names; preparation creates no publication receipt. A host-only
archive proof is partial and MUST NOT emit the five-target PASS report or private success witness.

## Non-goals and simpler baseline

The simpler baseline is one wheel built from the current checkout followed by `pip install`. It
does not detect dirty-tree contamination, dependency drift, nondeterminism, missing wheel members,
or a corrupt archive.

This slice records but does not independently adjudicate the owner's license and provenance
decision. It does not publish, upload, sign, attest, create an SBOM, promise cross-platform
reproducibility, or create a custom package-manifest protocol. It adds no runtime dependencies.

## Trust boundary and limits

The checker protects against ambient Git redirection, replacement refs, attributes-based export
changes, accidental worktree inclusion, mutable build-package selection, source-directory module
shadowing, action-tag retargeting, same-builder nondeterminism, forged runtime/license bytes,
malformed wheel membership, `RECORD` mismatch, partial publication, and artifact mutation between
build and install. It assumes the selected repository's object store, exact Python distribution,
hash-locked package bytes, pinned action commits, Git executable, and host are trusted.

The output parent is also a trust boundary: it must be owner-only and owned by the invoking user.
Corvint does not claim to defeat a malicious same-UID process that can mutate that directory; the
preexisting-destination check prevents accidental replacement, while final promotion is atomic.

The co-delivered checksum is not a signature. The same-runner rebuild is not an independent or
cross-platform reproducible-build claim. `--no-index` prevents package-index use by the invoked
installer; neither that flag nor `--no-isolation` is an operating-system network sandbox for build
backend code. Resource limits are 16 MiB per wheel, 128 members, 8 MiB per member, and 32 MiB total
uncompressed content. Every failure is fail-closed.

## Acceptance and testing

The implementation is accepted for this experimental slice when:

1. focused unit tests fabricate and reject duplicate names, traversal, encryption, special members,
   forged source/license bytes, bad `RECORD` hashes, checksum mutation, surplus build output, and
   non-identical rebuilds, invalid metadata versions or fields, missing members, archive limits,
   absolute/backslash/non-canonical paths, and CRC corruption;
2. committed-tree tests prove hostile Git environment, replacement refs, info attributes, dirty
   files, and untracked files cannot change exported bytes;
3. the checked-in workflow test proves every action reference is a 40-character commit and the
   workflow has the required least-privilege, exact-runtime, conformance, formatting, cross-build,
   cancellation, and fresh-builder boundaries — `script/check-ci-least-privilege.sh` (Makefile
   target `ci-least-privilege-check`, a `gate` prerequisite) covers the least-privilege share of
   this: exact `contents: read` workflow permissions and no job-level block granting more, `persist-credentials: false` on every
   `actions/checkout` step, and a full 40-character commit SHA on every `uses:` reference;
4. `NOT_PRODUCED` (retired 2026-09-12 by decision 0088): `make release-wheel-witness`,
   `tools/release_artifact.py`, and `requirements/release-build.txt` were removed by `54735d98` and
   no longer exist in the tree, so this step cannot run, produces no
   `<git-dir>/corvint/release-wheel-report.json`, and `script/release-checklist` reads no such file —
   its `native-performance`/`GOC-V0-005` row is an unconditional `NOT_RUN` with no witness file at
   all. See the `## Requirements` section above for the current disposition of each `ARTIFACT-V0`
   obligation this step formerly tested: each is superseded by a named `ARTIFACT-GO-V0-0NN`
   requirement (separately accepted; see `## Accepted amendment: Go binary archive profile`),
   still live, or retired; and
5. the `make gate` target (`Makefile:33@97034f12`) remains green. There is no separate Python gate: decision
   0088 (`docs/decisions/0088-go-only-cutover-and-oracle-retirement-2026-09-11.md`) retired the
   Python oracle and packaging, and the Makefile defines no python target (`grep -n -i python
   Makefile` returns nothing).

## Rollout, rollback, and maintenance

The pull-request workflow runs the checker but retains no remotely downloadable artifact. Rollback
is removal of the workflow job, checker, tests, and build lock; package runtime behavior is
unchanged. Build-tool and action updates occur only in reviewed changes that update exact versions,
hashes, and pins together and rerun the artifact check.

Adding a module, dependency, package, legal file, platform tag, or sdist requires an explicit spec
update and adversarial fixture. Publication authorization is recorded; artifact authentication and
an independent rebuild remain separate gates.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `ARTIFACT-V0-001` | (superseded by `ARTIFACT-GO-V0-001`, decision 0088) `tools/release_artifact.py` removed by `54735d98`; raw committed-tree reconstruction now `conformance/release-artifact-v0/archive_run.go` | historical: hostile environment, replace-ref, attributes, dirty/untracked export tests (deleted with `tests/test_release_artifact.py`); current: `ARTIFACT-GO-V0-001` manifest-drift/raw-export/hostile-environment tests |
| `ARTIFACT-V0-002` | (superseded by `ARTIFACT-GO-V0-001`, decision 0088) `requirements/release-build.txt` removed by `54735d98`; exact builder preflight now the Go toolchain/offline-module pins in `conformance/release-artifact-v0/manifest.json` | historical: lock and mismatch tests; CI install with `--require-hashes` (deleted); current: `ARTIFACT-GO-V0-001` manifest-drift tests |
| `ARTIFACT-V0-003` | (superseded by `ARTIFACT-GO-V0-002`, decision 0088) one-wheel output check removed with `tools/release_artifact.py`; now the fixed five-target/six-member archive inventory | historical: surplus-output and wheel-tag tests (deleted); current: `ARTIFACT-GO-V0-002` raw tar.gz/ZIP inventory and full five-target gate |
| `ARTIFACT-V0-004` | (superseded by `ARTIFACT-GO-V0-004`, jointly `ARTIFACT-GO-V0-002`/`-003`, decision 0088) wheel/`RECORD` verifier removed by `54735d98`; now the canonical archive container/member verifier | historical: valid and adversarial wheel fixtures (deleted); current: `ARTIFACT-GO-V0-004` raw framing, metadata, path, special-type, collision, and resource fixtures |
| `ARTIFACT-V0-005` | (superseded by `ARTIFACT-GO-V0-005` and `ARTIFACT-GO-V0-007`, decision 0088) independent-build comparison and private atomic promotion removed with the wheel checker; now the archive double-build/double-assembly and atomic-promotion path | historical: mismatch, smoke-failure cleanup, symlink-output, and real local checks (deleted); current: `ARTIFACT-GO-V0-005` descriptor-derived pair separation and five real byte-identical build/assembly pairs; `ARTIFACT-GO-V0-007` racing-destination/collision/cleanup/cancellation tests |
| `ARTIFACT-V0-006` | (superseded by `ARTIFACT-GO-V0-003`, decision 0088) canonical `SHA256SUMS` writer/verifier removed with the wheel checker; now the archive `SHA256SUMS` writer/binder | historical: one-byte mutation test and real local check (deleted); current: `ARTIFACT-GO-V0-003` hostile loose-gate report/byte mismatch and checksum/member mutation tests; the "not authentication" disclaimer is stated in `ARTIFACT-GO-V0-008`, accepted by decision 0108 |
| `ARTIFACT-V0-007` | (superseded by `GPK-V0-018` in `docs/specs/go-production-kernel-migration-v0.md`, decision 0088 — not an `ARTIFACT-GO-V0-0NN` row) fresh-environment smoke path removed with the wheel checker; now `conformance/release-artifact-v0/smoke.go` (`--version` plus real read-only query) invoked from `conformance/release-artifact-v0/gate.go` | historical: real local and CI artifact check (deleted); current: `GPK-V0-018`'s build-and-smoke-test requirement; `TestQueryMustResolveExpectedIntentAndLeaveRepositoryUnchanged` and `TestSummaryCountsNotRunSmokeSeparately` in `conformance/release-artifact-v0/gate_test.go` |
| `ARTIFACT-V0-008` | `.github/workflows/ci.yml` (unaffected by decision 0088; live) | static least-privilege workflow test — the original test lived in the deleted `tests/test_release_artifact.py`; replaced 2026-09-12 by `script/check-ci-least-privilege.sh` (Makefile target `ci-least-privilege-check`, a `gate` prerequisite) |
| `ARTIFACT-V0-009` | explicit exclusion in this contract; `docs/extraction-manifest.json` still present (unaffected by decision 0088; live) | no extraction-manifest claim in checker output |
| `ARTIFACT-V0-010` | (retired 2026-09-12, decision 0088) no successor; sdist has no Go-archive analog | historical: no-sdist static and surplus-output tests (deleted with `tools/release_artifact.py`/`tests/test_release_artifact.py`); none applicable going forward |

## Accepted amendment: Go binary archive profile

Amendment status: **accepted 2026-08-31 by repository-owner instruction "accept both"; see
`docs/decisions/0010-query-and-go-archive-acceptance-2026-08-31.md`**. Delivery status:
**implemented**. This acceptance authorizes implementation and local verification
of `ARTIFACT-GO-V0-001..007` only. It does not authorize signing, tagging, publication, promotion, or
any item-45 option. `GPK-V0-020`'s binary-archive-notice obligation is `PASS` at the exact revision
recorded by the current assessment; that evidence is not a legal conclusion or release authority.

### Archive-integrity exclusion — accepted 2026-09-12 (decision 0108)

- `ARTIFACT-GO-V0-008`: Archive checksums prove integrity only. This profile creates no signature,
  identity, attestation, SBOM, tag, upload, publication, promotion, cross-builder reproducibility,
  native execution, or legal conclusion. Those states remain separately authorized and reported.

### Untracked paths stay refused — accepted 2026-09-12 (decision 0142)

- `ARTIFACT-GO-V0-009`: The untracked-path allowance of decision 0142 MUST NOT apply to the loose
  gate or the archive builder. `go build` stamps `vcs.modified` from `git status --porcelain`,
  which lists every untracked path Git does not ignore. The gate requires `vcs.modified` to be
  `false`, so no such path is disjoint from the gated binary. Failure modes: any listed untracked
  path fails the loose gate as `dirty-worktree`. It fails the archive builder as `git-state-invalid`
  ("worktree is not clean"), and, when it appears after capture, as `git-state-changed`
  (`conformance/release-artifact-v0/archive_run.go:422`). A path excluded by ignore rules is never
  listed, leaves the stamp `false`, and does not refuse.

Acceptance requires adversarial fixtures for every forbidden member/container form, notice or
checksum mutation, target/build-info mismatch, cold-cache and temporary-directory independence,
binary/archive nondeterminism, partial-output cleanup, offline denial, and byte-identical repeated
assembly for all five target rows. Rollback is deletion of the unaccepted assembler/checker and its
outside-repository outputs; the existing loose-binary and Python-wheel gates remain unchanged.

### Go archive implementation traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `ARTIFACT-GO-V0-001` | strict manifest validation, closed Git/tool environments, two raw commit exports | manifest drift, raw-export/VCS identity, dirty-state and hostile-environment tests |
| `ARTIFACT-GO-V0-002` | `archivebuild`; fixed five-target naming and six-member inventory | raw tar.gz/ZIP inventory and full five-target gate |
| `ARTIFACT-GO-V0-003` | canonical inner checksum generation plus byte/report binding to the existing loose reproducibility gate | hostile loose-gate report/byte mismatch and checksum/member mutation tests; `TestIgnoredLiveSourcePassesLooseGateButArchiveBindingRefuses` (a `.go` source hidden by a committed `.gitignore` and by `.git/info/exclude` changes the live-root loose binary while its per-target report still passes with `vcs.modified=false`; the raw-commit archive build omits it and the binding refuses with `loose-gate-report-mismatch`, decision 0158) |
| `ARTIFACT-GO-V0-004` | canonical ustar/gzip and raw stored ZIP encoders; independent exact-byte reconstruction | raw framing, metadata, path, special-type, collision, and resource fixtures |
| `ARTIFACT-GO-V0-005` | existing loose-gate pair plus separate raw-export builds, cold caches, build directories, and assembly directories | descriptor-derived pair separation and five real byte-identical build/assembly pairs |
| `ARTIFACT-GO-V0-006` | separately compiled `archive-verifier` with stable no-follow reads, immutable caps, and no assembler/execution/network import | process/import boundary, strict request/result, resource, build-info/target/legal/checksum/raw-container rejection matrix |
| `ARTIFACT-GO-V0-007` | external `0700` scratch, descriptor-rooted stage, fsync, atomic no-replace promotion, path-free report, private witness | racing-destination/collision/cleanup/cancellation tests; strict status/checklist validation |
| `ARTIFACT-GO-V0-009` | unchanged empty-status checks in `checkCommit` (`gate.go`) and `resolveCleanState` (`archive_run.go`) | `TestCleanStateAllowsIgnoredButRefusesAnyUntrackedPath`, `TestFinalGitRecheckRejectsDirtyStagedUntrackedAndHeadMovement` |
| `ARTIFACT-GO-V0-008` | no signing, identity, attestation, SBOM, tag, upload or publication code in `conformance/release-artifact-v0`; tag, publication and promotion are separate `script/release-checklist` rows | `archive-report.schema.json` is a closed object with no signature, identity or publication member; `script/release-checklist_test.sh` |

The frozen conformance-only wire schemas are `archive-report.schema.json` and
`archive-witness.schema.json`. The first complete real-run assessment and repaired exact-revision
assessment are `conformance/release-artifact-v0/results/2026-08-31-go-archive/gpk-v0-020-notices-assessment.md`
and `conformance/release-artifact-v0/results/2026-08-31-go-archive/gpk-v0-020-notices-assessment-7638cf3.md`.

## Proposed release-readiness reporting profile

This reporting profile is proposed and non-authorizing. It defines the item-44 checklist without
turning observation into execution.

- `ARTIFACT-RDY-V0-001`: `script/release-checklist` MUST be deterministic and read-only. It prints
  exactly the native-runtime, native-performance, Go-archive, full-gate, tag, publication, and
  promotion rows, in that order, with `PASS`, `FAIL`, or `NOT_RUN`, the owning clause, and one
  bounded reason. Per decision 0088's Go-only cutover, native-runtime and native-performance replace the retired Packet 5
  and wheel-integrity rows. Go-archive, full-gate, tag, and promotion `PASS` require a current-revision
  machine-readable witness. Publication `PASS` requires the receipt binding of
  `ARTIFACT-RDY-V0-006..009`, whose witness is bound to the receipt's recorded release revision, not
  to `HEAD`, because a receipt committed after the tag can never name its own commit (decision 0141).
  A missing, stale, ambiguous, or merely prose witness is `NOT_RUN`; a present terminal negative
  witness is `FAIL`. native-runtime is not witness-based: it is
  `PASS` or `FAIL` from a static check of the current tree for legacy Python packaging. native-performance
  is unconditionally `NOT_RUN` until a native performance measurement exists (`GOC-V0-005`); no witness
  applies to it. It exits zero only when all seven rows are `PASS`, one for any incomplete/failed
  checklist, and two for invalid invocation/environment. Failure to read required local Git state
  is an invalid environment: the checklist MUST diagnose it and MUST NOT emit a `PASS` for the
  dependent row. Before outward action, tag, publication, and promotion remain `NOT_RUN`.
  The expected prepublication exit one is a lifecycle observation, not a verdict that applicable
  exact-target native, artifact, CEM, OCM, or public-release prerequisites failed; it is not a
  substitute for those gates.
- `ARTIFACT-RDY-V0-002`: native-runtime is owned by `GOC-V0-001`; native-performance is owned by
  `GOC-V0-005`; Go archive build and verification by `ARTIFACT-GO-V0-001..007`; full-gate by
  `GOC-V0-010`, whose private receipt binds a complete `make gate` to HEAD and the archive witness.
  Per decision 0088's Go-only cutover, the native-runtime and native-performance rows replace the retired Packet 5 (`GPK-V0-017`, decisions 0037/0016/0085)
  and wheel-integrity (`ARTIFACT-V0-001..010`) rows. `GOC-V0-005` forbids running the cancelled Packet 5
  paired retry or relabeling its historical result, so native-performance carries no witness, revision,
  or dependency-closure binding. native-runtime is a static current-tree check for the absence of the
  legacy Python engine, wheel entry point, or wheel release job; it is not a built witness either.
- `ARTIFACT-RDY-V0-003`: a tag row reports only an exact release tag already pointing at the
  candidate revision. The tag is resolved under `refs/tags/` only: a branch or other ref spelled
  like it is not a tag. It never creates, moves, signs, or deletes a tag; no exact tag is `NOT_RUN`.
  Per `ARTIFACT-RDY-V0-001`, only the ordinary "the ref does not resolve" outcome of the tag read is
  `NOT_RUN`; any other failure of that read is an invalid environment and exits 2 with a diagnostic,
  never a folded-in `NOT_RUN`.
- `ARTIFACT-RDY-V0-004`: publication is `PASS` only with a repository-owner-approved receipt that
  binds destination, immutable artifact digests, tag, and revision, as read by the receipt reader of
  `ARTIFACT-RDY-V0-006..009`. The checklist never uploads or queries a remote service; absence is
  `NOT_RUN`.
- `ARTIFACT-RDY-V0-005`: promotion is `PASS` only with a repository-owner acceptance record naming
  the exact per-command evidence required by `GPK-V0-023`, `GPK-V0-026`, and `GPK-V0-042`. The
  checklist never changes a default, compatibility label, package name, or promotion state.

### Publication receipt reader (decision 0141)

Accepted by `docs/decisions/0141-publication-receipt-binds-to-the-tag-revision-2026-09-12.md`;
delivery experimental. The decision settles the conflict between `ARTIFACT-RDY-V0-001`'s
current-revision witness rule and decision 0109's receipt, which is committed after the tag. The
reader is `release-artifact-v0 publication-status`, which the checklist invokes only when a receipt
is committed at `HEAD`.

- `ARTIFACT-RDY-V0-006`: the receipt is the blob at `docs/releases/<tag>/publication-receipt.json`
  in `HEAD`'s commit, where `<tag>` is `v` plus `VERSION`. The reader takes it from the commit
  object, never from the worktree or index. It MUST be a regular non-executable blob of at most
  64 KiB. It MUST be canonical compact JSON with one final LF, carrying exactly `schema`
  (`corvint.release-publication-receipt.v1`), `approval` (`repository-owner`), `decision`, `destination`,
  `tag`, `revision`, `tree`, `publisherIdentity` (`NOT_VERIFIED`, decision 0108) and `files`, in that
  order. `decision` MUST name a `docs/decisions/NNNN-*.md` record committed as a regular blob in the
  same `HEAD` tree. `destination` MUST be `https://github.com/Beamfall/corvint/releases/tag/<tag>`
  (decision 0109) with `corvint_*` archive names. Other
  archive identities fail closed; this local reader does not assert repository ownership. `tag` MUST equal the expected tag, and `revision` and `tree` MUST be full object
  names. `files` holds 1 to 16 `{name, sha256}` rows, strictly sorted by name, with exactly one
  `SHA256SUMS` and at least one Go archive name. No receipt is `NOT_RUN`. Any violation is `FAIL`.
- `ARTIFACT-RDY-V0-007`: the release revision is the receipt's `revision`, not `HEAD`. `PASS`
  requires three things. The exact ref `refs/tags/<tag>` must exist and peel to `revision`. `tree`
  must equal that commit's tree. `revision` must be an ancestor of `HEAD`. A missing tag, a tag
  naming another commit, or a `revision` that is not an ancestor of `HEAD` is stale, which is
  `NOT_RUN`. A `tree` that differs from its commit's tree is `FAIL`. Ancestry MUST use immutable
  commit parents: the closed Git runner pins `GIT_GRAFT_FILE` to the platform null device and
  disables graft deprecation advice as well as replacement objects. Repository `info/grafts` and
  ambient graft files MUST NOT invent or remove ancestry, and remain byte-identical after reads.
- `ARTIFACT-RDY-V0-008`: every attached digest MUST be witnessed locally by the private archive
  witness (`ARTIFACT-GO-V0-007`) bound to the receipt's `revision` and `tree`. Each archive row must
  equal that witness's digest for the same name. The `SHA256SUMS` row must equal the SHA-256 of the
  canonical checksum file rebuilt from the witness rows: `<sha256>  <name>` plus LF, for all five
  archives in witness order. The row is `NOT_RUN` when the witness is absent, bound to another
  revision or tree, or silent about an attached file. That covers the companion bundle, which has no
  local witness. The row is `FAIL` when any witnessed digest differs, when the witness at that
  revision is `FAIL`, or when the witness is malformed. A digest contradiction outranks an unwitnessed
  file.
- `ARTIFACT-RDY-V0-009`: the reader is read-only and offline. It invokes only local `git`, with the
  closed environment of `closedGit` and optional locks off. It changes no worktree, index, ref, or
  Git-directory byte, and it never contacts the destination. The checklist maps reader output
  exactly: `PASS` to `PASS`; `STALE` and `UNWITNESSED` to `NOT_RUN`; `MISMATCH`, `INVALID`, and any
  failure to run the reader to `FAIL`. A committed receipt is never `PASS` unless the reader returns
  `PASS`. Failure to list `HEAD`'s receipt path is an invalid environment, which exits 2.

Failure modes. None of these can pass. A prose-only receipt, or a receipt edited only in the
worktree or index, is `NOT_RUN`. A tag moved, deleted, or recreated at another commit after the
receipt is `NOT_RUN`. So is a checklist run on a branch that does not descend from the release, or a
fresh clone without the private witness. A hand-edited digest, a receipt naming a `FAIL` archive run,
a noncanonical or extended receipt, or a missing decision record is `FAIL`. A broken Go toolchain or
Git read at the reader is `FAIL`, or exit 2 for the checklist's own Git read.

Non-goals. The reader does not create, move, sign, or delete a tag. It does not upload, query the
release URL, or show that the remote release exists or holds these bytes. It does not verify
publisher identity, and it does not write, repair, or template a receipt. It does not witness the
companion bundle, and it does not change the go-archive, tag, or promotion rows, which still judge
`HEAD`. So a run at the receipt-bearing descendant shows tag `FAIL` and go-archive `NOT_RUN` by
design, and decision 0109's readiness bar stays judged at the frozen candidate.

Acceptance evidence. The four focused tests below pass, and so does `script/release-checklist_test.sh`,
including its committed-but-unreadable receipt row. Rollback: revert
`conformance/release-artifact-v0/publication_status.go`, its test, the `publication-status` dispatch,
and the checklist's publication block to the prior unconditional `NOT_RUN`. Restore
`ARTIFACT-RDY-V0-001/004` and supersede decision 0141. No tag, receipt, or release is touched.

#### Publication receipt reader traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `ARTIFACT-RDY-V0-006` | `committedBlob`, `decodePublicationReceipt` in `conformance/release-artifact-v0/publication_status.go` | `TestPublicationReceiptStrictFormRefusesMalformedReceipts` |
| `ARTIFACT-RDY-V0-007` | `publicationBinding`, `ancestryBinding` | `TestPublicationReceiptBindsToTheRecordedTagRevisionNotHead` |
| `ARTIFACT-RDY-V0-008` | `publicationWitnessStatus`, `witnessedPublicationDigests` | `TestPublicationReceiptDigestsMustMatchTheRecordedRevisionWitness` |
| `ARTIFACT-RDY-V0-009` | closed `git` reads; publication block of `script/release-checklist` | `TestPublicationStatusIsReadOnly`; `script/release-checklist_test.sh` receipt rows |

## Signing options — alpha selection

Decision 0108 selected **No signing** for the historical `0.4.0a4` candidate. Decision 0328 expressly
selects **No signing** for the current `0.5.0a2` alpha prerelease. Any later prerelease or stable
release needs a new selection. No agent may generate, import, store, rotate, request, or use
a signing key or identity, and no signature is a publication or promotion authorization.

| Option | Identity source and tool | Verifier checks | Trade-off |
|---|---|---|---|
| Sigstore keyless bundle | exact repository, GitHub Actions workflow path/ref, and OIDC issuer; `cosign` produces a detached blob signature and Sigstore bundle | archive SHA-256; Fulcio chain; exact issuer and certificate subject/SAN; Rekor signed-entry timestamp, inclusion proof, and checkpoint from the bundle; accepted trusted-root set and signing time | short-lived identity and offline bundle verification, but signing depends on external OIDC/Fulcio/Rekor services |
| Owner-controlled hardware/KMS key | owner-selected HSM/KMS identity with its public-key digest recorded in a separately accepted decision; `cosign` signs the archive digest through that provider | archive SHA-256; detached signature; exact pinned public-key digest and algorithm; optional separately required timestamp/transparency proof | verification can remain offline with pinned public material, but key custody, rotation, recovery, and provider policy become owner obligations |
| No signing | no identity or signing tool; retain archive `SHA256SUMS` only | byte integrity and reproducibility only; the release notes and any publication receipt MUST state publisher identity as `NOT_VERIFIED`; `archive-verifier`'s frozen report has no identity member, so its `PASS` asserts bytes only | preserves the local boundary but provides no publisher authentication and cannot be described as signed |

## Promotion and unresolved decisions

The license/provenance decision is closed. Promotion beyond experimental requires an independent
rebuild on another controlled builder. Sdist contents, release authentication, trusted publishing,
attestations, SBOM format, and cross-platform reproducibility remain unresolved and out of scope.
The Go archive profile `ARTIFACT-GO-V0-001..007` is accepted for local assembler and checker
implementation. Decision 0108 selects No signing for the alpha only; a signing option must be
selected before any signing implementation begins. Decision 0109 fixes the alpha's publication set,
its receipt form and "no promotion"; tagging, publication, and promotion themselves remain
separately owner-gated. Decision 0141 specifies the receipt reader (`ARTIFACT-RDY-V0-006..009`), so the
publication row stays `NOT_RUN` until the owner commits a receipt that binds.

### Local unavailable-state marker audit — 2026-09-12

This audit classifies the 16 occurrences above without creating a release artifact or changing a
contractual status value. Counts: (a) 11, (b) 0, (c) 5.

Commands observed on 2026-09-12:

- `RAI-C1`: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/release-artifact-v0 -run '^TestPublication(ReceiptStrictFormRefusesMalformedReceipts|ReceiptBindsToTheRecordedTagRevisionNotHead|ReceiptDigestsMustMatchTheRecordedRevisionWitness|StatusIsReadOnly)$'` — PASS (exit 0).
- `RAI-C2`: `GOTOOLCHAIN=local script/release-checklist_test.sh` — PASS (exit 0). This test uses only an isolated temporary fixture repository; it creates, moves, signs, tags, or publishes no Corvint release artifact.

| Marker | Class | 2026-09-12 result or precise local boundary |
|---|---|---|
| R01, digest blocked-on summary | (c) | Exact tagging, publication, promotion, and future signing selection are repository-owner/outward actions; none is authorized or evidenced by this local audit. |
| R02, retired wheel step/native-performance row | (c) | Decision 0088 removed the wheel witness and `GOC-V0-005` has no current native-performance measurement; the accepted contract forbids relabeling the cancelled Packet 5 retry. |
| R03, checklist closed row vocabulary/order | (a) | PASS, exit 0 (`RAI-C2`); the script test checks all six ordered rows and their closed local statuses. |
| R04, missing/stale/ambiguous witness mapping | (a) | PASS, exit 0 (`RAI-C1`, `RAI-C2`); absent, stale, malformed, and unwitnessed fixtures remain incomplete while terminal contradictions fail. |
| R05, native-performance without measurement | (c) | A test confirms honest reporting, but no current `GOC-V0-005` measurement exists and the historical cancelled retry cannot be rerun or promoted. |
| R06, no exact candidate tag | (a) | PASS, exit 0 (`RAI-C2`); an isolated fixture with no exact tag reports the required incomplete state without changing Corvint refs. |
| R07, publication receipt absent | (a) | PASS, exit 0 (`RAI-C2`); the isolated no-receipt fixture exercises the absence mapping offline. |
| R08, `ARTIFACT-RDY-V0-006` receipt absent | (a) | PASS, exit 0 (`RAI-C1`); the focused strict-form test observes `publicationAbsent` at the release revision and rejects malformed committed receipts. |
| R09, `ARTIFACT-RDY-V0-007` stale tag/revision | (a) | PASS, exit 0 (`RAI-C1`); moved, deleted, wrong-tree, and unrelated-history fixtures exercise stale versus invalid results. |
| R10, `ARTIFACT-RDY-V0-008` witness absent/misaligned | (a) | PASS, exit 0 (`RAI-C1`); absent, other-revision, companion, mismatch, and failed-witness cases are exercised. |
| R11, `ARTIFACT-RDY-V0-009` reader/checklist mapping | (a) | PASS, exit 0 (`RAI-C1`, `RAI-C2`); focused reader outcomes and checklist translation are covered locally and offline. |
| R12, prose-only or worktree/index-only receipt | (a) | PASS, exit 0 (`RAI-C2`); an uncommitted receipt is ignored, while a committed unreadable receipt fails rather than passing. |
| R13, stale tag/unrelated branch/missing private witness | (a) | PASS, exit 0 (`RAI-C1`); moved/deleted tags, unrelated history, and an absent witness are explicit focused fixtures. |
| R14, receipt-bearing descendant row combination | (a) | PASS, exit 0 (`RAI-C2`); after the fixture advances beyond its exact tag, tag fails and Go archive remains unwitnessed. |
| R15, rollback prescription | (c) | This is a hypothetical rollback state, not current acceptance evidence; exercising it would revert production/spec behavior and is outside this evidence-only task. |
| R16, owner-bound publication row | (c) | A passing row requires the owner to commit the binding receipt after an actual publication; this task forbids publication and cannot invent that receipt. |
