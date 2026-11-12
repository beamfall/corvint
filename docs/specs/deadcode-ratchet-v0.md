# Dead-Function Ratchet V0

Owner: Russell Lewis
Date: 2026-11-11
Requirement prefix: `DCR-V0`
Intent status: proposed
Delivery status: experimental (ratchet and deletion on the lane branch; 35 functions kept with reasons)
Authoritative inputs: tickets V1-4801 (the report and its baseline) and V1-4827 (delete the listed functions), steps of the 2026-11-06 expert-panel restructuring plan; OpenSpec change `docs/specs/openspec/changes/deadcode-ratchet/` (proposal, design, spec delta and tasks); owner decision R33 (`docs/specs/PRODUCT-PLAN-1.0.md` checklist row O42: orphaned code is deleted and tracked).

## Agent digest
- Claim: `make deadcode` runs a pinned dead-code analyzer for six GOOS/GOARCH configurations and fails unless the functions all six report equal a shrink-only baseline.
- Status: proposed / experimental (ratchet and deletion on the lane branch; 35 functions kept with reasons)
- Exists: Makefile targets `deadcode` and `deadcode-baseline`, `tools/deadcode-ratchet/` (the runner, the intersection and verdict, `baseline.json`, hermetic tests on `testdata` fixtures); the baseline held 149 entries at 4c3c20333 and holds 35 after the deletion of 114 functions (V1-4827), each kept entry with its reason.
- Blocked on: owner acceptance of the intent; independent review; an owner decision on the 35 kept entries (V1-4827 asks for zero).
- Read next: Requirements, then Failure modes.

## Problem and current state

The 2026-11-06 panel measured 139 functions (about 1.4k lines) that no binary and no test can
reach. Nothing stopped that number from growing: `go vet` does not report an unused exported
function or an unused method, and no Makefile target ran a reachability analysis. The number was
also stale by 2026-11-11: decision 0594 and the orphan deletions removed whole packages after it
was taken.

`golang.org/x/tools/cmd/deadcode` builds a call graph by rapid type analysis from every `main`
package and, with `-test`, from every test executable, and prints the functions no root reaches.
Its answer holds for one GOOS/GOARCH/tag configuration only. This contract runs it for six
configurations, keeps the functions every run reports, and compares them with a checked-in
baseline that can only shrink, the way `tools/import-ratchet` treats a wrong-way import. At
origin/main 4c3c20333 (2026-11-11) the six runs report 152 (linux/amd64), 155 (linux/arm64, both
darwin) and 208 (both windows) functions; 149 are in all six. The deletion removed 114 of them
(1,614 Go lines); 35 stay in the baseline with a reason: 16 cited by a spec (DCR-V0-007), 2 called
by a cited function, 11 counted by the diagnostic-coverage ratchet test, 4 in paths a concurrent
lane is editing, 1 under `.github/**` and 1 fixture.

## Requirements

- `DCR-V0-001`: (proposed 2026-11-11; delivery experimental; V1-4801) Pinned analyzer.
  `make deadcode` MUST install `golang.org/x/tools/cmd/deadcode` at one exact version, named in
  one Makefile variable (`DEADCODE_VERSION`), built for the host, into a scratch directory that the
  target removes when it ends. The target MUST NOT use `@latest`, MUST NOT use an analyzer found on
  `PATH`, and MUST NOT change `go.mod` or `go.sum`. When the version cannot be fetched or verified
  the target MUST fail, name the version and run no other analyzer.
- `DCR-V0-002`: (proposed 2026-11-11; delivery experimental; V1-4801) Six configurations over every
  package. The target MUST run the analyzer with `-test` and `CGO_ENABLED=0` once for each of
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` and
  `windows/arm64`, one after another. Each run MUST analyse every package of the main module,
  including packages under directories whose names begin with a dot (which `./...` skips). When a
  run fails, or loads no package, the target MUST fail and name the configuration; it MUST NOT
  compute a report from fewer than six runs. Every run uses `CGO_ENABLED=0` and the default build
  tags, so a function whose only callers are in cgo files or in files behind a build tag (`race`,
  `live` or another custom tag) is reported as dead. Under DCR-V0-007 such a function is kept only
  when its removal breaks a build or vet of DCR-V0-008.
- `DCR-V0-003`: (proposed 2026-11-11; delivery experimental; V1-4801) Identity and intersection. A
  function's identity MUST be its package import path and the analyzer's name for it (`F`, or
  `T.M` for a method), without a file or line. The report MUST be the set of identities that every
  one of the six runs reports. Functions in generated files MUST NOT be reported.
- `DCR-V0-004`: (proposed 2026-11-11; delivery experimental; V1-4801) Strict, sorted baseline.
  `tools/deadcode-ratchet/baseline.json` MUST carry the profile `corvint-deadcode-baseline/0` and a
  list of entries, each with `package`, `func` and `reason`, sorted by package then function, with
  no duplicate. The tool MUST refuse a baseline that has an unknown field, a wrong profile, a
  duplicate entry or an unsorted list, naming the first offending entry. A test MUST assert the
  number of entries in the committed baseline.
- `DCR-V0-005`: (proposed 2026-11-11; delivery experimental; V1-4801) Shrink-only verdict. The tool
  MUST exit 0 when the report equals the baseline. It MUST exit non-zero and name each function
  that is in the report and not in the baseline, with a declaration position
  (`NEW_DEAD_FUNCTION`). It MUST exit non-zero and name each baseline entry that is not in the
  report, saying to delete the entry (`STALE_BASELINE_ENTRY`). `make deadcode` MUST NOT rewrite
  the baseline; only the separate `make deadcode-baseline` does.
- `DCR-V0-006`: (proposed 2026-11-11; delivery experimental; V1-4801) Hermetic proof. The tool's
  tests MUST run without the analyzer, without loading the module and without the network. They
  MUST cover: a function reported by one run only, a new function, a stale entry, a refused
  baseline of each kind in DCR-V0-004, a missing run, and the committed baseline's count.
- `DCR-V0-007`: (proposed 2026-11-11; delivery experimental; V1-4827) Fail-closed deletion. A
  function MAY be deleted only when the report regenerated at the change's base lists it. A listed
  function MUST be kept, with a non-empty baseline `reason`, when it is declared under
  `protocol/**`, carries or is the target of `//go:linkname` or `//export`, is named by an assembly
  file, is cited by name under `docs/specs/**` or in a top-level `docs/*.md` file, or when its
  removal breaks a build or vet of DCR-V0-008. No other declaration MAY be removed, except an
  import left unused.
- `DCR-V0-008`: (proposed 2026-11-11; delivery experimental; V1-4827) Deletion proof. After the
  deletion, `go build` and `go vet` MUST pass for every package of the module in each of the six
  configurations and under each custom build tag present in the tree, `go vet` MUST pass in
  `interop/cem01-go`, and the tests of every package that lost a function MUST pass.
  `make deadcode` MUST then exit 0 against the updated baseline.

## Non-goals

Code that only tests reach (`-test` treats it as live); packages linked into no binary (decision
R33 and the `source-removal` change own those); a CI job (workflow files change in their own pull
request); functions that exist on one operating system only, which the intersection never lists;
unused constants, variables, types and struct fields; any change to `go.mod`, `go.sum`,
`protocol/**`, `.github/**` or the module `interop/cem01-go`. `tools/deadcode-ratchet` is a
development tool: no product package imports it and it uses the standard library only.

## Failure modes

- A configuration that fails to load or type-check fails the target with the configuration's name
  and no verdict; a missing run would shrink the report and make the baseline look stale.
- A version that cannot be fetched or verified fails the target. The installed binary's build
  information must name `golang.org/x/tools` at `DEADCODE_VERSION`. There is no fallback to
  another version or to a tool on `PATH`.
- The analyzer misses a use it cannot see: a `//go:linkname` target, an assembly reference, or a
  caller under a build tag outside the six configurations. The ratchet then only carries a harmless
  baseline entry; DCR-V0-007 keeps such functions out of the deletion and DCR-V0-008 vets every
  custom build tag.
- A function declared once per operating system (`f` in `x_linux.go`, `x_darwin.go` and
  `x_windows.go`) is reported only when every variant is unreachable.
- Each run is a whole-program load: about 9 to 11 s and 4.6 GB peak resident memory on the WSL
  lane host, 53 to 69 s for the six runs (2026-11-11). The target is not part of `go test` or
  `GATE_STEPS`.
- A test outside a package can count a dead function as live evidence: the diagnostic-coverage
  ratchet (`script/diagnostic-coverage.count`) counts each `diagnostic.Refusal` literal, reachable
  or not, so deleting an unreachable refusal constructor fails it. Such functions are kept with
  that reason rather than lowering the count.

## Acceptance evidence and traceability

| Requirement | Evidence |
|---|---|
| DCR-V0-001 | `make deadcode` run of 2026-11-11 (exit 0; `git diff go.mod` empty; no `corvint-deadcode-*` scratch left); `GOPROXY=off go run ./tools/deadcode-ratchet -version v0.51.0` exits 1 naming the version; `TestValidVersion` |
| DCR-V0-002 | a `windows/arm64`-only type error makes the run exit 1 naming `windows/arm64` with no verdict; without the `.github` package patterns linux/amd64 reports 19 more functions, all in `.github/cishards` and `.github/testconfine`; `TestIntersectRefusesMissingRun` |
| DCR-V0-003 | `TestReportExcludesFunctionDeadOnOneOS`, `TestMovedFunctionKeepsIdentity` |
| DCR-V0-004 | `TestDecodeBaselineRefusals`, `TestEncodeBaselineRoundTrip`, `TestCommittedBaselineCount` |
| DCR-V0-005 | `TestVerdictAcceptsMatchingBaseline`, `TestVerdictNamesNewFunction`, `TestVerdictNamesStaleEntry`; `make deadcode` on a scratch commit that adds one unreachable function exits non-zero naming it |
| DCR-V0-006 | `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 ./tools/deadcode-ratchet/` (all tests, 0.004 s, fixtures embedded from `testdata`) |
| DCR-V0-007 | the removed-declaration check in `docs/build-log/2026-11-11-deadcode-ratchet.md` (114 removed declarations, every one a function of the base report; a control that also removes a live function fails naming it); `TestCommittedBaselineReasons` |
| DCR-V0-008 | `go build` and `go vet` for the six configurations, `go vet -tags` for `race`, `live`, `beamfall_appstore` and `corvint_development` (red at the base, unchanged), `go vet` in `interop/cem01-go`, the tests of the 43 packages that lost a function, and `make deadcode` exit 0 on the 35-entry baseline, all recorded in `docs/build-log/2026-11-11-deadcode-ratchet.md` |

## Rollback

Revert the deletion commit to restore the functions; the ratchet commit stands alone. Revert the
ratchet commit to remove the targets, the tool, the baseline and this spec's index rows. Nothing
persists outside the repository except each run's analyzer JSON under the Git directory
(`$(git rev-parse --absolute-git-dir)/deadcode/`); the analyzer binary lives in a scratch directory
the tool removes.
