# V1-5473: a tag release is an appliable core set (lane rel1115)

Experimental, base `1a02c5f39`, branch `claude/rel1115`. Tickets V1-5473 (delivered here) and
V1-5237 (already fixed on the base; no change).

## V1-5237: release archives omit corvint-docs

Already fixed on the base by the packdocs lane (`2026-11-09-packdocs.md`, `ARTIFACT-BIN-V0-508..512`)
and the tag pipeline (`7e432eafb`, V1-5236): `release-binary` builds one archive per product binary,
including `corvint-docs`, `corvint-mcp` and `corvint-docs-mcp`; `internal/releasecandidate/verify.go` and
`script/go-archive-gate` agree on the five `corvint` archives by recorded design (`ARTIFACT-BIN-V0-509`);
`TestInstalledCoreSetRunsCorvintDocs` and the workflow's installed-set smoke run `corvint docs --help`
on extracted archives. Reconfirmed on this branch by the live run below (`corvint docs --help` came
from the real archives).

## V1-5473: the release does not publish an appliable core set

**Premise on the base.** Half stale: the tag pipeline, which landed a day after the ticket was filed,
already builds and uploads the nine app archives (three apps on darwin/arm64, linux/amd64 and
linux/arm64). The real gap is elsewhere. `corvint-update apply --component core` requires the asset
`verification-report.json`, the `corvint.release-go-archive-report.v2` report of the `corvint` archive
gate, and binds each `corvint` archive to it before any app step. The pipeline never ran the gate, so a
tag release had no report. Reproduced: the real `release` output of this branch with the report removed,
served to the updater, refuses `missing release asset verification-report.json`.

**Design.** Two producers build `corvint` archives: the gate (runbook step 5, two cold builds per
target) and `release-binary`. Both build with the same flags (`-trimpath`, `CGO_ENABLED=0`, the
`main.build` and `main.sourceCommit` stamps, a clean commit with VCS info) through the shared
`archivebuild` encoders, so their archives should be byte-identical. Measured here: the three shipped
`corvint` archives from the two producers at `2e8f94330` are identical (`cmp`). So the pipeline runs
the gate and `release-binary` binds it instead of replacing one producer:

- `ARTIFACT-BIN-V0-525`: `release` writes the release `SHA256SUMS` itself; before, a workflow shell
  line assembled it.
- `ARTIFACT-BIN-V0-526`: `release --core-gate GATE_DIR`, required exactly when the tag builds
  `corvint`, checks that the report passed at the built commit and toolchain and that each built
  `corvint` archive is the gate's retained archive (report row and gate file, byte for byte), then
  copies the report unchanged into the output. The reader is stdlib-only, so `ARTIFACT-BIN-V0-006`'s
  import rule still holds.
- `.github/workflows/release-binaries.yml` (its own commit): the build job provisions the host tools,
  runs the gate when `release-binary select` names `corvint`, passes `--core-gate`, drops the shell
  checksum line, and the smoke also installs each MCP server and checks its version line. Build-job
  timeout 60 to 120 minutes (the gate took 233 s on a 32-core host; hosted runners are slower).

Rejected: having the updater accept `release-manifest.json` as the Core qualification (that changes
`UPD-V0` and drops the gate's two-build reproducibility evidence), and publishing the gate's archives
over `release-binary`'s (two files of one name, and the manifest would record the other producer's
digests).

## Evidence

- `TestReleaseCoreTagBindsTheGateReportAndWritesTheChecksums` (`tools/release-binary`): missing
  and unused gate refuse before building; revision, verdict, reasons, schema, toolchain,
  archive digest, binary digest, zero and two report targets, and a changed gate archive each refuse
  `core-gate-mismatch` with the output left as found; the success copies the report unchanged and
  writes one sorted `SHA256SUMS` row per archive.
- Live, this host, at `2e8f94330`: the gate (`rc=0`, 233 s, verdict PASS, 5 targets), then
  `release --tag core/v1.0.0-rc.3 --core-gate` (`rc=0`, 172 s) wrote 12 archives, `SHA256SUMS`
  (`sha256sum --check --strict` OK) and the report (byte-equal to the gate's).
  `TestUPDV0920ApplyInstallsATagReleaseOutput` with `CORVINT_UPDATE_RELEASE_DIR` set to that output
  passed: one apply from build 192 installed `corvint-docs`, `corvint-docs-mcp` and `corvint-mcp`
  and replaced `corvint`, each at the manifest's binary digest, through the real platform inspection,
  version probes and smoke. Without the report the same run refuses (above).

Not run: a hosted tag run of the workflow (needs a pushed tag); `make gate`.

Rollback: revert the three commits; `release` then writes no `SHA256SUMS` or report and the workflow's
shell line returns.
