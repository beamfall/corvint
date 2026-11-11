# V1-4801 and V1-4827: the pinned dead-function ratchet, and deleting what it lists

Intent proposed (owner acceptance pending); lane `deadcode`, base
`4c3c203336a6cc4a9772af39b1a771f34236138e`, OpenSpec change
`docs/specs/openspec/changes/deadcode-ratchet/`, contract `docs/specs/deadcode-ratchet-v0.md`
(DCR-V0-001 to DCR-V0-008).

## What changed

`make deadcode` runs `go run ./tools/deadcode-ratchet -version $(DEADCODE_VERSION)`. The tool
installs `golang.org/x/tools/cmd/deadcode@v0.52.0` for the host (GOOS and GOARCH unset, working
directory outside the module) into an `os.MkdirTemp` directory it removes on exit, checks the
binary's build information names that module and version, and runs it with `-test -json` and
`CGO_ENABLED=0` for linux, darwin and windows on amd64 and arm64, one after another, over `./...`
plus the four `.github` packages that `./...` skips. Each run's JSON stays under
`$(git rev-parse --absolute-git-dir)/deadcode/`. The report is the set of (import path, analyzer
name) identities all six runs list; the verdict compares it with
`tools/deadcode-ratchet/baseline.json` (`NEW_DEAD_FUNCTION`, `STALE_BASELINE_ENTRY`).
`make deadcode-baseline` is the only writer of the baseline. The tool uses the standard library
only; `go.mod` has no `go.sum` and neither changed.

## Measurements (WSL lane host, 2026-11-11)

| Tree | linux/amd64 | linux/arm64, darwin | windows | In all six | `make deadcode` |
|---|---|---|---|---|---|
| base 4c3c20333 (V1-4801 recorded 139 on 2026-11-06) | 152 | 155 | 208 | 149 | 69.0 s, exit 0 |
| after the deletion | 38 | 41 | 94 | 35 | 53.1 s, exit 0 |

Peak resident memory of one run is 4.6 GB (`/usr/bin/time`); each run takes 8 to 12 s.

Controls on the ratchet commit `a75decf64`:

- A scratch commit adding `func DeadcodeProbe() int` to `internal/clierror`: `make deadcode`
  printed `NEW_DEAD_FUNCTION github.com/Beamfall/corvint/internal/clierror.DeadcodeProbe at
  internal/clierror/zz_deadcode_probe.go:6:6` and exited 2 (the tool exited 1); the commit was
  then dropped.
- A file `zz_deadcode_probe_windows_arm64.go` with a type error: five runs reported, then
  `configuration windows/arm64: analyzer: exit status 1; no verdict`, exit 1.
- `GOPROXY=off go run ./tools/deadcode-ratchet -version v0.51.0`: `cannot install
  golang.org/x/tools/cmd/deadcode@v0.51.0 ... no other analyzer is run`, exit 1; `-version latest`
  is refused before anything runs.
- Without the explicit `.github` packages, linux/amd64 reports 19 more functions, all in
  `.github/cishards` and `.github/testconfine` and reached only from their `cmd` mains.
- The deletion made exactly the 114 deleted entries `STALE_BASELINE_ENTRY` and reported no new
  function.

## Deletion (V1-4827)

The list is the 149-entry report at the base. 114 functions were deleted: 1,614 Go lines removed
across 73 files (one more line is a gofmt realignment), 87 imports left unused removed with them,
and four files left with no declaration and no package documentation deleted. No constant,
variable or type was removed.

Citation rule applied for DCR-V0-007: a function counts as cited when a code span in
`docs/specs/**` or a top-level `docs/*.md` file is exactly a reference to it (`Name`,
`pkg.Name`, `T.M`, `(*T).M`, with or without call parentheses) and, when the name is one ordinary
word (`Verify`, `Judge`, `Serve`), the same line also names its package or file. Old build-log and
decision entries do not count. Every hit was then read; spans that name a different package's
identifier of the same word (`git add`, `ctx.Done()`, `t.Run`) do not count.

The 35 kept entries and their reasons are in the baseline. By reason:

| Reason | Count |
|---|---|
| cited by a spec | 16 |
| counted as a covered site by `TestDiagnosticCoverageRatchet` | 11 |
| left for a concurrent lane (`internal/liveverify/affected/**`) | 4 |
| called by `ValidateQueryAuthorityStart`, which a spec cites | 2 |
| declared under `.github/**`, which this change may not edit | 1 |
| fixture data read by `tools/beamfall-shadow` | 1 |

No listed function is declared under `protocol/**`, in a generated file, under
`internal/docviews/**`, or is named by a `//go:linkname`, `//export` or assembly file.

Removed-declaration check (V1-4827 criterion 3): a scratch program parses every package the
change touched, at HEAD and in the index, and lists each top-level declaration present before and
absent after. Result: `removed declarations: 114 (all funcs listed in HEAD baseline: true);
added: 0`, exit 0. A control that also removed the live `clierror.Validate` failed with `NOT IN
LIST: func github.com/Beamfall/corvint/internal/clierror Validate`, exit 1.

## Proof (DCR-V0-008)

All with `GOTOOLCHAIN=local` on the final tree:

- `CGO_ENABLED=0 GOOS=o GOARCH=a go build` and `go vet` of `./...` plus the four `.github`
  packages for the six configurations: exit 0 each.
- `go vet -tags T` for `race`, `live` and `beamfall_appstore`: exit 0. `corvint_development`:
  exit 1 on `internal/contextindex/taskcontext_uncertainty_development_test.go` (`not enough
  arguments in call to relationRows`), the same error at the base; reported, not fixed.
- `go vet` for freebsd, openbsd, netbsd and solaris on amd64 (the `cross-vet` set): exit 0.
- `go -C interop/cem01-go vet ./...`: exit 0; that module imports nothing of the main module.
- Tests of the 43 packages that lost a function, one `script/heavy.sh` command each, and
  `cmd/corvint` as `script/go-test-shard.sh ./cmd/corvint/ 1..8 8`: all pass
  (`internal/cliconformance` and `internal/worklistadapter` have no test files).
- `internal/diagnostic` (reads `cmd/corvint` refusal sites), `make diagnostic-coverage-check`,
  `make error-code-ownership-check`, `make capabilities-check`, `script/analyzer-schema.sh check
  --output`: pass.
- Line citations the deletion moved: `--repin` moved 17 pinned citations; 37 unpinned legacy
  citations into `doccompiler/build.go`, `docmaintain/docmaintain.go`,
  `testvalidity/projection.go` and `liveverify/provider/provider.go` were re-read, pinned with
  `--hash` and dropped from `script/line-citation-unpinned-inventory.txt`.

## Findings

- The diagnostic-coverage ratchet counts each `diagnostic.Refusal` literal in the tree, reachable
  or not; 12 of its 27 counted sites sit in 11 unreachable `cmd/corvint` refusal constructors.
  Deleting them needs an owner decision to lower `script/diagnostic-coverage.count`.
- The deletion left package-level names without references: `cmd/corvint` `readResult`,
  `workBoundBuild`; `conformance/cli-parity-v0` `maximumGitArchiveBytes`; `internal/contextindex`
  `LexicalCounter`, `localPromptCancelContext`; `internal/doccorpus` `stableReadAcceptCase`,
  `stableReadFixture`, `stableReadPacket`, `stableReadTupleRoot`; `internal/testsupport`
  `fakeGoplsDefinitionsEnv`, `fakeGoplsServeEnv`; `internal/worklistadapter` `PolicyPath`,
  `Scratch`, `Worklist`, `WorklistProfile`, `mappings`. `internal/worklistadapter` is now types
  and constants only.
- V1-4827 criterion 1 (zero baseline functions) is not met: 35 remain, each with its reason.
