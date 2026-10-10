# Canonical-create fixture identity controls

Date: 2026-10-10

## Context

V1-0667 retains evidence for CEM-V1-001 and CEM-V1-007. Fixture-local identity
was delivered by `1a355ea5`: `canonicalCreateRepo` in
`internal/cem/gitauth/canonical_create_test.go` sets `user.name` and `user.email`.

## Evidence

On `origin/main` at `29e1f653486fca13216606683d3a5c2b05cacd36`, both
`TestCanonicalDiffWithCreateDestinationsProvesBaseEntries` and
`TestCanonicalDiffWithCreateDestinationsWithholdsProofOnProvenanceFailure` pass
with `GIT_AUTHOR_*`, `GIT_COMMITTER_*` and `EMAIL` unset, `HOME` an empty directory,
`GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_SYSTEM=/dev/null`, and
`user.useConfigOnly=true` via `GIT_CONFIG_COUNT=1`,
`GIT_CONFIG_KEY_0=user.useConfigOnly`, `GIT_CONFIG_VALUE_0=true`:

- Actual Darwin arm64: Apple Git 2.54.0, Go 1.27.1; `go test` passed in 9.6s.
- Linux amd64: the test binary was cross-built on the host using Go 1.27.1 with
  `GOOS=linux GOARCH=amd64`, then run under x86_64 emulation in golang image
  `b475798fb161` (Go 1.27.1 linux/amd64, Git 2.47.3, `uname -m` = `x86_64`)
  on a Docker Desktop arm64 host. `git config --get user.name` returned 1 (unset).

Running the Go toolchain itself under emulation crashed with a runtime SIGSEGV
(unexpected fault address). The passing control used the cross-built test binary,
not `go test` in the container. The container was removed afterwards.

## Qualification Limits

Linux amd64 execution was emulated on arm64; it was not a hosted Linux amd64
runner. The hosted run remains `NOT_RUN`. The original-failure control on
pre-`1a355ea5` source was not re-run here; prior evidence remains in
[the October 2 repair entry](2026-10-02-cem-linux-retirement-repair.md).
No global identity workaround or skip was used.

This entry records evidence only: no source change and no new requirement.
