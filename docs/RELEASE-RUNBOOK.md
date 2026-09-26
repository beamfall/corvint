# Reproducible release and recovery runbook

Ordered commands from a clean clone to a tagged release, then the rollback of a bad release.
Every step is local evidence until step 10; nothing here authorizes signing, upload or promotion,
which stay owner calls under [Public Release V0](specs/public-release-v0.md). The checks are
governed by [Stable operations V0](specs/stable-operations-v0.md) (`SOP-V0-001` to `SOP-V0-012`);
see also [archive integrity](specs/release-artifact-integrity-v0.md), the
[changelog](RELEASE-NOTES.md), [security/support policy](../SECURITY.md) and [INSTALL](INSTALL.md).

Replace `FULL_COMMIT` with the 40-hex commit being released and `X.Y.Z` with its version. Keep all
output directories outside the checkout; the reproducibility script refuses one inside it.

1. Clean clone at the exact commit; a dirty or shallow tree is not a release source.

   ```sh
   git clone https://github.com/Beamfall/corvint.git corvint-release
   cd corvint-release
   git checkout --detach FULL_COMMIT
   test -z "$(git status --porcelain)"
   ```

2. Toolchain identity. The build is reproducible only for this Go; record `go version` and
   `git --version` with the evidence.

   ```sh
   test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.1"
   ```

3. Read-only readiness report. No row may be `FAIL`; a `FAIL` row names the clause to fix before
   continuing. Before promotion the `tag`, `publication` and `promotion` rows are `NOT_RUN` by
   design, `native-performance` stays `NOT_RUN` (`GOC-V0-005`, decision 0420), and `full-gate`
   and `go-archive` stay `NOT_RUN` until step 4 records their witnesses.

   ```sh
   script/release-checklist
   ```

4. The full gate on the exact commit. Run it once, alone on the host; commit nothing while it
   runs. Its receipt is the gate evidence for this commit. Then rerun the report in
   pre-promotion mode: it exits 0 only when `native-runtime`, `go-archive` and `full-gate` are
   `PASS` and no row is `FAIL` (`ARTIFACT-RDY-V0-001`).

   ```sh
   make gate
   script/release-checklist --pre-promotion
   ```

5. Produce the release archives into a new private directory. The producer writes
   `corvint_<goos>_<goarch>.tar.gz` for darwin/linux × amd64/arm64, `SHA256SUMS` and
   `verification-report.json`; retain the whole directory. It also writes
   `corvint_windows_amd64.zip`, which is not a qualified target. The output's parent must already
   exist as a real 0700 directory, or the producer refuses.

   ```sh
   mkdir -m 0700 /abs/release/X.Y.Z
   GOTOOLCHAIN=local go run ./conformance/release-artifact-v0 archive \
     --revision FULL_COMMIT --output /abs/release/X.Y.Z/core
   ```

6. Checksum manifest. Verify it independently of the producer and record the digests in the
   release evidence; publish `SHA256SUMS` beside the archives.

   ```sh
   (cd /abs/release/X.Y.Z/core && shasum -a 256 -c SHA256SUMS)
   ```

7. Reproducibility. The gate builds every declared target twice in one profile and requires
   byte-identical outputs; its `report.json` is retained with the release.

   ```sh
   CORVINT_RELEASE_ARTIFACT_OUTPUT=/abs/release/X.Y.Z/repro \
     script/check-release-artifact-reproducibility.sh
   ```

8. Install lifecycle on this host's archive: verified install, first index and read, upgrade into
   a second store, rollback, uninstall with `.corvint` retained, backup/restore, and corrupted-snapshot
   recovery. To exercise a real N-1 upgrade, install the previous release first (its archive as
   `CORVINT_LIFECYCLE_ARCHIVE`, or its `corvint` as `CORVINT_LIFECYCLE_BINARY`) and set
   `CORVINT_LIFECYCLE_UPGRADE_BINARY` to this release's `corvint`, the executable the script installs
   as the upgrade. The upgrade's packet is compared to the one it builds from a cold index and
   reported `packet=identical` or `packet=changed` (decision 0360); without
   `CORVINT_LIFECYCLE_UPGRADE_BINARY` the upgrade is reported `same-bytes`. `make install-lifecycle-test`
   and `make hostile-regressions-test` check the two scripts themselves. Then the hostile
   regression matrix; both must end `status=PASS` / `check-hostile-regressions: PASS`. The `tee`
   pipelines run under `bash -o pipefail` and keep stderr, so a failing gate fails the command and its
   whole output is retained (V1-0375). The log path is an argument to `bash`, never part of the
   `-c` script. Repeat step 8 on each supported host with its own archive; a host not run is
   `NOT_RUN`, never implied.

   ```sh
   CORVINT_LIFECYCLE_ARCHIVE=/abs/release/X.Y.Z/core/corvint_$(go env GOOS)_$(go env GOARCH).tar.gz \
     CORVINT_LIFECYCLE_REPORT=/abs/release/X.Y.Z/lifecycle-$(go env GOOS)-$(go env GOARCH).txt \
     script/check-install-lifecycle.sh
   # N-1: W.V.U is the previous release; the upgrade is this release's corvint, extracted from its archive.
   CORVINT_LIFECYCLE_ARCHIVE=/abs/release/W.V.U/core/corvint_$(go env GOOS)_$(go env GOARCH).tar.gz \
     CORVINT_LIFECYCLE_UPGRADE_BINARY=/abs/extracted/X.Y.Z/corvint \
     CORVINT_LIFECYCLE_REPORT=/abs/release/X.Y.Z/lifecycle-n1-$(go env GOOS)-$(go env GOARCH).txt \
     script/check-install-lifecycle.sh
   bash -o pipefail -c 'script/check-hostile-regressions.sh 2>&1 | tee "$1"' _ /abs/release/X.Y.Z/hostile-regressions.txt
   ```

   Then the CCF-V1-007 N-1 replay (proposed, decision 0398): every frozen Core mode also runs under
   the previous release tag, which must still carry each member, identifier and registered value this
   commit emits. It must end `ok`; retain its output.

   ```sh
   bash -o pipefail -c 'make core-n1-replay CORE_N1_TAG=vW.V.U 2>&1 | tee "$1"' _ /abs/release/X.Y.Z/core-n1-replay.txt
   ```

   The optional companion and installed qualification keep their own gates:
   `script/corvint-companion-release-gate` and `script/public-release-check`; run them when the
   release includes those tuples and retain their reports. For companion bundle `/2`, run
   `script/public-release-check` with `CORVINT_PUBLIC_RELEASE_QUALIFICATION=core` and set
   `CORVINT_NODE_SHA256`, `CORVINT_NPM_SHA256`, `CORVINT_GO_AUTHORITY_BUNDLE` and
   `CORVINT_GO_AUTHORITY_SHA256` alongside the ten common settings (PUB-V0-020).

9. Changelog entry. Add the `## X.Y.Z` section at the top of `docs/RELEASE-NOTES.md` naming the
   version, `Corvint X.Y.Z (build N)` and `FULL_COMMIT`, where `N` is
   `git rev-list --count --first-parent FULL_COMMIT`. If `VERSION` and `const version` in
   `cmd/corvint/main.go` change, that is a separate earlier commit that steps 1–8 already ran on;
   the notes commit is the only commit after the gate, its only parent is `FULL_COMMIT`, and it
   changes only Markdown under `docs/` (decision 0380).

   ```sh
   git rev-list --count --first-parent FULL_COMMIT
   git add docs/RELEASE-NOTES.md && git commit -m "docs: release notes for X.Y.Z"
   ```

10. Tag and publish (owner action). Merge first, then tag: before tagging, verify `FULL_COMMIT` is
    on `origin/main`'s first-parent chain, so its stamped build number (step 9) matches what a fresh
    clone of `origin/main` reproduces and cannot collide with a later, unrelated `origin/main` commit
    at the same first-parent count (decision 0375, `PUB-V0-021`).

    ```sh
    git fetch origin
    git merge-base --is-ancestor FULL_COMMIT origin/main
    bash -o pipefail -c 'git rev-list --first-parent origin/main | grep -x FULL_COMMIT'
    ```

    The tag points at the notes commit. `script/release-checklist --pre-promotion`, run from
    `FULL_COMMIT`, passes its tag row only for that shape (`ARTIFACT-RDY-V0-003`). The archives, `SHA256SUMS`,
    `verification-report.json`, the reproducibility `report.json`, the lifecycle reports and the
    hostile-regression report are the release evidence. Pushing the tag and uploading assets are
    outward actions that need the owner's explicit go.

    ```sh
    git tag -a vX.Y.Z -m "Corvint X.Y.Z"
    git push origin vX.Y.Z
    ```

## Rollback of a bad release

Public tags and published assets are never moved, deleted or rewritten. To roll back:

1. Add a `## X.Y.Z withdrawn` line at the top of `docs/RELEASE-NOTES.md` naming the defect and the
   release users should run instead; commit it.
2. Users select the previous retained version by absolute path, as [INSTALL](INSTALL.md) describes;
   the installer never replaces a version in place, so the previous store is intact.
3. Fix on main, then run this runbook from step 1 for `X.Y.Z+1`. Do not re-tag `vX.Y.Z`.
4. If the defect is a security issue, follow [SECURITY.md](../SECURITY.md): the withdrawn release
   is outside the support window the moment its successor is published.

## Existing security evidence and remaining qualification

| Boundary | Existing regression/evidence | Remaining release obligation |
|---|---|---|
| Candidate integrity and inventory | `internal/releasecandidate` checksum/source/version/inventory tests; runbook step 6 | Exact future closed artifact verification |
| Upgrade, rollback, reinstall, removal | `TestPUBV0025RecoveryLifecycle`; `script/check-install-lifecycle.sh` (SOP-V0-001..006) | Native runs on every supported host with the previous release as the upgrade source |
| Store symlinks, case aliases and overlap | `TestPUBV0025HostileStore`, no-replace promotion tests | Exclusive store ownership; concurrent malicious renames unqualified |
| Probe output, cancellation and descendants | `TestPUBV0026ProbeFailureCleansInstall`, `TestPUBV0026InterruptedInstallReapsDescendant`, `internal/procgroup` | Escaped descendants remain outside owned-group proof |
| Hostile repositories, paths, symlinks, case folds, bounded output, time, interruption, secrets, corrupt derived state | `script/check-hostile-regressions.sh` (SOP-V0-007..009), 29 rows | `memory-resident` (whole-process and git child memory) is NOT_COVERED; case-fold rows are NOT_RUN on a case-sensitive filesystem |
| Hostile archives and repository exports | `internal/companionrelease` unsafe path, casefold, source count/byte tests; `go-archive-gate-injection-test` | Capability-specific exact artifact/security gates; injection gate is opt-in |
| Secrets and evidence | `internal/secretscreen`, `internal/trace`, local-completion screening | Review retained release evidence before sharing; fixtures are not a comprehensive audit |
