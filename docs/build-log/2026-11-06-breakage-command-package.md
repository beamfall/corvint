# Breakage command package (V1-4751, V1-4747 slice 1)

The V1-4747 scope review proposed feature command packages with colocated tests and named the
small `breakage` command as the first bounded seam. The review authorized no wholesale
reorganization; this entry records the only move made.

## Scope

- `internal/command/breakage` now owns the `breakage` and `breakage manifest` handlers and their
  private flag types behind one `Run(ctx, root, args, out, diagnostic) int` entry.
- `cmd/corvint/breakage.go` keeps `parseBreakageInvocation` (the `--root` preamble, root
  normalization and argument errors), and `runBreakage` is a one-line wrapper, so `main.go`
  dispatch, help registration and `require-clean` handling are untouched.
- Handler-level tests moved with the package (`TestBreakageCLIRejectsMissingAndAmbiguousBindings`,
  `TestBreakageCommandHelperProcess`, `TestBreakageCLIComplete`) and one new package-local test
  covers `breakage manifest` usage refusals. Root keeps `TestBreakageManifestCLIPinsNamedSources`
  (dispatch end to end) and `TestWorkflowHelpKeepsBooleanFlagsScoped` (help).

Non-goals: no flag, help, diagnostic, exit-code or wire change; no other command moved; no
generic command framework. Total LOC and binary functionality are unchanged; no speedup is claimed.

## Evidence

Base `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`, Go 1.27.3, Darwin.

- Byte parity: 18 fixed invocations (help forms, `--root` preamble errors, usage, binding and
  manifest refusals) gave identical stdout, stderr and exit code from base and candidate
  binaries; a success path (`breakage manifest` then `breakage` over this repository) gave
  identical manifest, report and stderr bytes.
- `go list -deps ./internal/command/breakage` contains no `cmd/` package.
- Focused tests via `script/heavy.sh`: `./internal/command/breakage` all pass;
  `./cmd/corvint -run '^(TestBreakage|TestWorkflowHelpKeepsBooleanFlagsScoped|TestRequireClean|TestHelp)'`
  pass. CI test slices list the moved names under `cmd/corvint`; an anchored named slice simply
  matches fewer tests and the new package runs in its own default slice, so no `.github` change.

## Rollback

Revert the commit: the handlers and tests return to `cmd/corvint/breakage*.go`. There is no
persistent state, schema or wire contract to unwind.
