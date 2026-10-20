# V1-1981 Tasks linux/amd64 archive (CAL-V0-320)

Decision: the standalone Tasks archive builder (`corvint-companion-release -tasks-only`,
CAL-V0-042) admits `linux/amd64` as a cross-built target. `internal/tasks` and `cmd/corvint-tasks`
have no cgo and no darwin-only build-tag gap: `CGO_ENABLED=0 GOOS=linux go vet` passes.
Because the build host is darwin/arm64, the archive is never executed there, so the version/help
smoke is skipped and the manifest says `reproducible-cross-build-only-target-runtime-not-observed`.
The full bundle stays darwin/arm64 only.

Measured (darwin/arm64 host, go1.27.2 from the module cache on PATH because the tool pins it; the
host default is go1.27.3 and is refused by design; heavy.sh slot, load band not recorded), at
commit 82c924ce07b9 (pre-amend tree; the tool's own two-build and two-assembly agreement passed):
`corvint-companion-release -tasks-only -target linux/amd64 ...` produced
`corvint-tasks_linux_amd64.tar.gz` sha256 `6b8cdd1c...d550`, `SHA256SUMS` line verified with
`shasum -c`, extracted `bin/corvint-tasks` is `ELF 64-bit LSB executable, x86-64, statically
linked` (21,848,116 bytes), inner `SHA256SUMS` has 13 entries. `GOOS=linux go vet` of
`./cmd/corvint-tasks/... ./internal/tasks/...` passes. `go test ./internal/companionrelease` passes.

NOT_OBSERVED: acceptance criterion 2 (a fresh Linux host downloads, verifies and runs
`corvint-tasks version` and `roadmap` read-only). Command for an owner or CI Linux run is in
`docs/INSTALL.md` (Standalone Tasks archive). No release, tag or upload was made.

Proposed (not applied, `.github/` is owner-gated): a release workflow job that runs the
`-tasks-only -target linux/amd64` command on a macOS or Linux runner and uploads
`corvint-tasks_linux_amd64.tar.gz` and its `SHA256SUMS` line to the `tasks-dev-*` release, then a
`ubuntu-latest` job that runs the INSTALL.md verify/version/roadmap commands before publishing.
