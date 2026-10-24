# Windows production path logic (V1-2488)

Date: 2026-10-23. Ticket: V1-2488. Base: `origin/main` `368b80d4`. Spec: `docs/specs/affected-plan-v0.md`
`AFP-V0-415`, `AFP-V0-416`. Follows `2026-10-23-windows-native-test-qualification.md` (V1-2441).

## Decision

The V1-2441 native Windows run classed 38 packages as path or logic defects, plus 7 Tasks
packages where `PathText` rejects `C:\`. The ticket file was not in the checked-out task store, so
this lane worked the items named in its brief (projectpath, gitstatus, specindex, flowdocs) and
re-read every other package in that row. No Windows host was available: each cause below is the
first failure the code reaches when read, not an observed log line, and the Windows rerun is
`NEEDS-HOST`.

- Fixed in production, because the code is meant to run on Windows and mishandled its syntax:
  `projectpath` (slash input and link targets split only on `\`), `flowdocs.openDirectory`
  (opened `\C:` instead of `C:\`, the cemcandidate defect V1-2441 fixed), `sharedpath` (the
  alternates guard refused the drive colon of Git for Windows' `C:/.../objects`), and
  `breakagemap` (`--show-toplevel` prints `C:/x`, compared bytewise with `C:\x`).
- Fixed in test logic of the same class: JSON fixtures that pasted raw OS paths (invalid `\U`
  escapes), a Go source fixture with an unquoted path, roots without a volume (`/repo`, `\repo`
  are not absolute on Windows), the INDEX key trimmed before `ToSlash`, the POSIX not-exist text,
  and a `git` fixture `exec.LookPath` cannot match without a `PATHEXT` extension.
- Kept as refusals (AFP-V0-416): every `//go:build !darwin && !linux` stub and `procgroup`'s
  `process-platform-unsupported`. Tasks `PathText` stays a POSIX absolute path: admitting `C:\`
  changes the persisted head and manifest wire format and waits on V1-2485.
- Rejected: memoising `gitstatus.Executable` per `PATHEXT`. `internal/gitstatus` is an
  IDX-SNAP-V0-017 analyzer input, so the change forced an analyzer schema bump (audit pins and
  use-case receipts) for a cache-key edge case outside the ticket; `TestAnalyzerSchemaInputs`
  failed with it and passes without it. The fixture fix alone resolves item (2).

## Per-package root cause and class

Classes: P = production path defect, fixed; T = test-logic path defect, fixed; R = deliberate
fail-closed refusal (AFP-V0-416); H = test helper `.exe` or POSIX tool (V1-2490); A = POSIX
assumption in a test (V1-2491); U = unknown without the log.

| Package | Class | Root cause (first failure reached) |
|---|---|---|
| `internal/projectpath` | P | `resolve` trimmed only `\`, so `missing/child` and a `missing/child` link target split into an empty component |
| `internal/flowdocs` | P, R | `openDirectory` dropped the volume; reads then refuse in `open_other.go` (no `O_NOFOLLOW`) |
| `internal/sharedpath` | P, H | `privateMergeRepository` refused `:` anywhere in the store; `behind_test.go` merge driver passes an unquoted Windows path to `sh` |
| `internal/breakagemap` | R, P | `read.go` runs Git through `procgroup` (refuses); behind it, the top-level comparison ignored `C:/` spelling |
| `internal/gitstatus` | T | fixture named `git`, which `exec.LookPath` never matches on Windows |
| `internal/specindex` | T | `TrimPrefix("../../")` before `ToSlash` on a `..\..\docs\specs\x.md` glob match |
| `internal/liveverify/gotest` | T | `"Path":"`+dir+`"` JSON fixtures |
| `internal/playwrightruns` | T | `"path":"`+p+`"` JSON baseline fixture |
| `internal/analyzergo` | T | marker path pasted into a Go string literal |
| `internal/dashboard/adapters` | T | root `\repo` has no volume |
| `internal/postmergemetrics` | T | root `/repo` has no volume (`git_test.go` is darwin/linux-only) |
| `tools/ci-shard-costs` | T | expected `no such file`; Windows says `The system cannot find the file specified.` |
| `cmd/corvint-corpus-parity` | R | `testvaliditydoc/read_other.go` |
| `cmd/corvint-corpus-republish` | R | `postmergeconnector/files_other.go` |
| `cmd/corvint-docs-mcp` | R | `gitstatus/read_other.go`, `contextindex/readfile_unsupported.go` |
| `conformance/dashboard-snapshot-v0` | R, H | `process_group_other.go`; tests build `dashboard` without `.exe` |
| `conformance/mcp-test-validity-v0` | R, A | `testvaliditydoc/read_other.go`; `os.Symlink` case |
| `internal/analyzercap` | R | `analyzerexec/platform_other.go` leaves an empty host platform |
| `internal/analyzernativebridge` | H | candidate binaries built without `.exe`; `os.Symlink` |
| `internal/authoritystore` | R | `repository_directory_other.go` |
| `internal/behaviorfalsify` | R, H | `procgroup`; `#!/bin/sh` hooks |
| `internal/criterionexperiment` | A | expects Git mode `100755` (`core.filemode=false` on Windows) and a symlink |
| `internal/dashboard/repository` | R | `open_nofollow_windows.go`, `process_windows.go` |
| `internal/delta` | R | `input_other.go` |
| `internal/depsource` | H | `git` symlinked without `.exe` |
| `internal/gitnotes` | R | `cem/gitauth/stable_boundary_other.go` |
| `internal/intake` | R | `link_other.go` |
| `internal/localcompletion` | R, H | `procgroup`; `true`, `false`, `sh -c` check commands |
| `internal/lspstdio` | R | `workspace_guard_other.go` |
| `internal/mcp/docsbridge` | R | `gitstatus/read_other.go` |
| `internal/mcp/testvaliditybridge` | R | `testvaliditydoc/read_other.go` |
| `internal/postmergeconnector` | R | `files_other.go` |
| `internal/receiptbundle` | R | `stable_open_other.go`; five tests lack `requireStableRuntime` |
| `internal/releasegate` | R | `platform_windows.go`; the test override reaches only `platform_unix.go` |
| `internal/testevidence` | A | `Geteuid` is -1 on Windows, so the `chmod 0500` test runs; `os.Symlink` |
| `internal/upgradechanges` | R, H | `refs_other.go`; `#!/bin/sh` `git` shims without `.exe` |
| `tools/cem-trial` | R, H | `procgroup`; `corvint` without `.exe`, `fake-agent.sh` |
| `tools/capabilities-check` | U | no path, exec, mode or tag defect found by reading; needs the log |
| `internal/tasks/{archive,authority,cli,intent,journal,mutation,snapshot}` | R | `wire.ParsePathText` requires a leading `/` (Tasks wire §2) |

## Evidence

- Focused tests on Linux (go1.27.2): the twelve changed packages pass, except
  `internal/gitstatus` `TestStatusRefusesUnsupportedMetadataBeforeLiveStatus/reftable` (host git
  2.43 lacks `--ref-format=reftable`) and `internal/postmergemetrics` `TestMetricsGitProcessBounds`
  (process-group quiescence). Both fail identically on clean `origin/main` here.
- `corvint affected --base 368b80d4` selected 109 units; the Go units ran, with the only failures
  also failing on clean `origin/main` in this container (contextindex golden bytes, appmap and
  doccorpus Node producers, dashboard/repository and release-artifact process groups,
  doccorpus/selfcorpus, tasks/cli `merge-tree` output). `cmd/corvint` ran its breakage tests only.
- `make windows-vet` passed; `GOOS=windows go test -c` compiles the `_windows_test.go` files.

## Limits

- No native Windows run. Every class above is from reading code (`NOT_OBSERVED` on a host).
- `tools/capabilities-check` stays unexplained.
- Not fixed here: helper `.exe`/POSIX-tool defects (V1-2490), refusal skips (V1-2486), POSIX
  assumptions (V1-2491), and `gitstatus.Executable`'s cache key ignoring `PATHEXT`.
