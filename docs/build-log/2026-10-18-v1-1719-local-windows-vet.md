# V1-1719: local twin of the CI Windows vet step

Owner input: ticket V1-1719 (no GitHub issue). Windows-only `go vet` failures were found only by
the `go-static` step "Windows vet including test files (AFP-V0-042)": V1-1092, and PR #1089 on
2026-10-18, where `internal/tasks/store/lease_bounded_audit_test.go` called helpers defined in a
unix-constrained file. Each cost a CI round. AFP-V0-054 was added to
`docs/specs/affected-plan-v0.md`; it is accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). No CAL-V0 ID was added. No `.github/` file changed.

## Design

- The CI step runs `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...` on the root module and on
  `interop/cem01-go`, under the `go-static` job env (`GOTOOLCHAIN=local`, `GOWORK=off`, an empty
  `GOFLAGS`, so no build tags). The new Makefile target `windows-vet` runs those two commands under
  that environment.
- `make cross-vet` already ran the Windows vet, but only inside `make gate`, which lanes do not run.
  So the gap was discovery, not tooling. `AGENTS.md` Verify now declares `make windows-vet` in its
  own fence, with a sentence scoping it to a change that touches `.go`, `go.mod` or `go.sum`.
  Unchanged `corvint affected` code reads that declaration (AFP-V0-009, decision 0531) and lists it
  as a `conditional` check. No product code changed, so other repositories see no new advice.
- The target vets whole modules, not touched packages. That is the CI command itself. A subset
  would miss a break in a package whose files did not change, and would need a resolver CI does not
  use. Cost on this host: 19 s for both modules with a partly warm cache (5 s warm and 45 s cold in
  build log 2026-10-10-windows-test-vet).

## Evidence

- `TestAFPV0054WindowsVetTargetMatchesTheCIStep`, `TestAFPV0054AffectedListsWindowsVet` and
  `TestAFPV0054WindowsVetFlagsUnixOnlyTestHelper` in `cmd/corvint/affected_windows_vet_test.go`
  pass. The fixture has `holder_unix_test.go` (`//go:build unix`) defining `waitForHolder` and
  `audit_test.go` calling it. Native `go vet` passes; the recipe's root-module command fails with
  `undefined: waitForHolder`.
- Mutations, each restored afterwards:
  - `GOOS=linux` in `WINDOWS_VET_ENV` fails the parity test and the fixture test.
  - `GOFLAGS=-tags=unix` fails the parity test and the fixture test.
  - Dropping the `interop/cem01-go` line fails the parity test.
  - Rewording the scoping sentence so it no longer names `make windows-vet` fails the affected test.
- `corvint affected --base d35b0ee33e0efda8468db03a22956d0b629bf4d8` (installed 1.0.0-rc.3 build
  634) on this tree lists `conditional make windows-vet`, with the scoping sentence as its reason.
- `make windows-vet` on this tree: exit 0.

Hosted behaviour is unchanged; the AFP-V0-042 step still runs in CI.

Rollback: delete the target, the `AGENTS.md` paragraph and fence, the tests and AFP-V0-054.

## Review fix

Codex review of PR #1098 (head `5f51f755f8b69350128a36d17b0cde32609597ca`) raised three P2
findings, all fixed; AFP-V0-054 (1) is amended in place under the owner's standing auto-accept
(2026-10-11).

- Saved Go settings: an empty `GOFLAGS` falls back to a value saved by `go env -w`, so a saved
  `-tags=local` could change the vetted files. `WINDOWS_VET_ENV` now sets `GOENV=off`. The CI step
  does not rely on a go env file: the runner has none, and the job sets everything through `env`.
  `TestAFPV0054WindowsVetIgnoresSavedGoEnv` points `GOENV` at a file saving
  `GOFLAGS=-tags=local`; the control (empty `GOFLAGS` only) reports the saved tag, and each recipe
  command reports none.
- Toolchain: `GOTOOLCHAIN=local` selects the installed Go without pinning it. `windows-vet` now
  depends on `go-version` (`go1.27.2`, the version CI's `setup-go` pins); the parity test checks
  the prerequisite, the `go-version` recipe and the `setup-go` pin.
- Step env: the parity parser read only the job env. It now merges the Windows vet step's own `env`
  over the job env. `TestAFPV0054StepEnvOverridesJobEnv` inserts `GOFLAGS: "-tags=special"` into
  that step's env and requires the merged value and a parity mismatch.

Mutations, each restored: dropping `GOENV=off` fails the parity and saved-env tests; dropping the
`go-version` prerequisite fails the parity test; skipping the step-env merge fails the step-env
test.
