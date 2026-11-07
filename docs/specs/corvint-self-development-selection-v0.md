# Corvint Self-Development Selection Correctness V0

Owner: Russell Lewis
Date: 2026-11-03
Requirement prefix: `ASD-V0`
Intent status: per requirement, each experimental under the standing lane auto-accept (2026-11-03)
Delivery status: experimental
Authoritative inputs: `docs/specs/corvint-self-development-v0.md` (parent contract: lane flow,
`script/heavy.sh` and `script/batch-precheck.sh`), native tickets V1-3968, V1-4358, V1-3804 and
V1-4245, `AGENTS.md` invariants 2 and 8. The parent's Requirements section holds 254 of the
256 obligations allowed by OCM, leaving no room for four more, so lane selbugs (2026-11-03) defines ASD-V0-2400 to ASD-V0-2403 here
under the parent's `ASD-V0` prefix; see build-log entry `2026-11-03-selbugs-selection-correctness`.
The heavycap charter of 2026-11-07 adds ASD-V0-2477/2478; its evidence is in
`docs/build-log/2026-11-07-heavycap.md`.

## Agent digest
- Claim: Lane admission bounds runtime concurrency and admits source edits conservatively while preserving narrow test-only selections.
- Status: per requirement, each experimental under the standing lane auto-accept (2026-11-03); delivery experimental.
- Exists: `new_package_base`, `selection_manifest_path`, `darwin_local` in `script/heavy.sh`; `SELECT_ALLOWED` in `script/batch-precheck.sh`.
- Blocked on: the parent spec's gates.
- Read next: Requirements; the parent's heavy.sh sections in `docs/LANE-ORCHESTRATION.md`.

## User and current state

A lane agent runs Go tests through `script/heavy.sh` and checks a batch with
`script/batch-precheck.sh`. Each requirement removes a refusal or a full run that no selection
needs, and each keeps a negative control that still refuses or runs everything.

## Requirements

- `ASD-V0-2400`: (experimental under the standing lane auto-accept, 2026-11-03; V1-3968 heavy.sh refused a brand-new package as `whole-reason-missing`) `script/heavy.sh` MUST admit a whole-package `go test` with no `HEAVY_WHOLE_REASON` when every package argument is a `./REL` directory holding a Go file of which no Go file directly under it exists at the merge base of `HEAD` and `origin/main`, logging `whole reason=new-package:<12 hex of the base>`; `./...`, `..`, `-C`, a package that exists at the base without a qualifying source edit (ASD-V0-2478), a mix with an unchanged package, and an unresolvable `origin/main` or merge base MUST still be refused, and a failing admitted run MUST return its own status.
- `ASD-V0-2401`: (experimental under the standing lane auto-accept, 2026-11-03; V1-4358 heavy.sh suggested a `corvint test --manifest` path under the Git directory, which `corvint test` refuses) The manifest path of `script/heavy.sh`'s `rerun with Corvint selection` line MUST be `${TMPDIR:-/tmp}/corvint-heavy-selection-UID-CKSUM.json` (`CKSUM` the `cksum` of the physical working directory), never under the Git directory or the worktree; when `TMPDIR` resolves inside the worktree, its Git directory or the common Git directory it MUST be under `/tmp`, including through a symbolic link.
- `ASD-V0-2402`: (experimental under the standing lane auto-accept, 2026-11-03; V1-3804 a first batch precheck never narrowed, since every real plan carries an unknown outside `FRONTIER_OK`; amends `ASD-V0-912`) `script/batch-precheck.sh`'s selection MUST admit exactly the unknowns CI's deciding selector narrows with (`gateselect.AllowedUnknown`: a `LANGUAGE_FRONTIER` that is not a `go:` frontier other than `go:build-constraint-variants` or `go:nested-module-frontier`, an `UNOWNED_DIRTY_PATH`, a `NO_SELECTABLE_TEST`) and a well-formed `UNSELECTED_BINARY_CONSUMER` (a relative path with no `.` or `..` element, backslash or control character); any other unknown MUST still run every drift test, naming it.
- `ASD-V0-2403`: (deferred 2026-11-04 by the orchestrator, review round e; formerly experimental under the standing lane auto-accept, 2026-11-03; V1-4245 heavy.sh kept every package with any darwin-only file wholly on the Mac) `script/heavy.sh` MUST NOT split a `go test` command between a remote and this host: on a Darwin host, every command whose packages hold any darwin-only file (V1-2552) MUST stay wholly local (`ASD-V0-934`), including one whose darwin-only files are all `_test.go` files and whose other files carry no build constraint, platform name element, non-Go source, platform keyword or uncovered import. The split this requirement specified through review round d is retired and its code removed. Measured over the 42 packages of this repository that hold darwin-only files, it split 0 (build-log entry `2026-11-04-selbugs-review-round-d`). It is also unsound: no static scan of a package's own files proves that the Linux half runs every common test as Darwin would, since a test that both platforms build can branch on the platform at run time through a subprocess (`exec.Command("uname", "-s")`), an imported helper's own check or any other probe the scan does not read, and the split command would then pass while the same test fails on Darwin. A new split needs its own requirement and a proof of equivalence; until then this rule only pins that every darwin-only command stays local.

- `ASD-V0-2477`: (accepted under the owner's standing auto-accept (2026-10-11); heavycap charter 2026-11-07; V1-5040; refines ASD-V0-014)
  Every local heavy.sh command, including admit-only, MUST inherit GOMAXPROCS equal to the
  validated slot share `par` unless the caller already set it. An environment or argv override
  MUST remain unchanged. The Go build parallelism and remote runtime policy remain separate.
- `ASD-V0-2478`: (accepted under the owner's standing auto-accept (2026-10-11); heavycap charter 2026-11-07; V1-5007; refines ASD-V0-2400)
  A non-test Go source change MUST admit its package's reaching tests without a manual whole
  reason; absent line-level selection evidence it MUST admit the whole package. Admission MUST
  include staged, unstaged, committed-since-merge-base and untracked source edits, and MUST NOT
  derive whole-package authority from a test-only edit. Existing narrow test commands remain
  admitted. Every package argument must independently qualify; unresolved bases and globs do not.
  Automatic source-change admission MUST run the whole package without test exclusions, including
  filters inherited through GOFLAGS or GOENV. An explicit -skip needs a HEAD-bound manifest
  selecting that exact exclusion; a whole-unit manifest alone MUST NOT authorize exclusions.
  Provably nonrestrictive -run patterns MUST retain automatic and whole-unit manifest admission;
  the final repeated -run value is effective. Restrictive inherited filters conservatively require
  an explicit command for manifest validation. Validation MUST preserve all environment assignments
  affecting Go defaults, and MUST refuse unsupported env options rather than discard them.
  Parallelism setup MUST resolve the child's Go defaults and validate the final effective filters.
  Offloaded execution MUST retain the sender's validated GOFLAGS despite receiver-only defaults.
  A pre-commit caller using heavy.sh MUST retain an existing test broken by a non-test edit.

## Non-goals

No change to the CI deciding selector, to `corvint test` or to the darwin evidence rule
(`ASD-V0-934`). No split of a darwin-only command between hosts (`ASD-V0-2403` is deferred).

## Failure modes

A brand-new package moved from another directory runs whole, as its selection would. A darwin-only
command runs wholly on the Mac even when only its test files are darwin-only, which costs local
time but never hides a failure. A stale `origin/main`
measures newness and source edits against an old base; source admission may conservatively run
more tests. No test-only edit is enough to prove whole-package necessity.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| ASD-V0-2400 | `new_package_base` and its call in `selection_policy` of `script/heavy.sh` | `script/lane-scripts_test.sh` group `tools 63`: new package admitted and logged, committed new package admitted, `./old`, a mix, `./...` refused, failing new package exits 1, no `origin/main` refused |
| ASD-V0-2401 | `selection_manifest_path` and its call in `rerun_lines` of `script/heavy.sh` | group `tools 63` (path outside Git dir and worktree; `/tmp` when `TMPDIR` is in the worktree, a linked worktree's Git or common Git directory, or a symlink to either); the amended `ASD-V0-730` assertion of group `policy 27` |
| ASD-V0-2402 | `SELECT_ALLOWED` and its use in `select_plan` of `script/batch-precheck.sh` | `script/batch-precheck_test.sh` ASD-V0-2402 cases: CI-admitted unknowns narrow a doc-only diff, the same unknowns with real golden drift select and fail it, `UNINDEXED_SOURCE_PATH` and a malformed consumer run everything |
| ASD-V0-2403 | the `darwin_local` call in the offload branch of `script/heavy.sh` (the split's functions removed) | group `offload 9`: on a Darwin host a package whose darwin-only files are all test files stays wholly local, runs the unchanged command once and passes its failure through; `./mac` (darwin source), `-run` and a mix stay local; an other test file constrained `//go:build !linux`, `// +build !linux` or by a `_linux` name, or an assembly file, a blank, named, dot, cgo or plain import in the darwin test file, a second package argument whose `//go:build !linux` test fails, a common test gated by `runtime.GOOS` with `Skip`, an aliased `runtime` import's `GOARCH`, `os.Getenv("GOOS")`, `testenv`, `SkipNow` or `Skipf`, a package helper naming `runtime.GOOS`, and a common test that runs `uname -s` and fails on Darwin each keep the command local with its Darwin-only failure seen; the before and after runs are in build-log entry `2026-11-04-selbugs-review-round-e` |
| ASD-V0-2477 | local admission in `script/heavy.sh` | `TestASDV02477LocalRuntimeShare` in tools/test-commands-check: default, caller and argv fault controls, admit-only |
| ASD-V0-2478 | `changed_source_base`, `source_tests_unfiltered`, `bound_parallelism`, `offload`, `serve` and `manifest_admits` in `script/heavy.sh` | `TestASDV02478SourceEditRunsExistingTests` in tools/test-commands-check: existing failing test, staged and committed source, unchanged/test-only/glob/mixed refusals, direct and inherited exclusion refusals including env unsetting, local GOENV=off with wrapper-only exclusions, nonrestrictive run patterns through automatic and manifest admission, exact manifest exclusion retains the broken test; `TestASDV02478OffloadKeepsValidatedFlags`: receiver-only skip/run/GOENV exclusions and explicit empty GOFLAGS retain the broken test |

## External integration boundary

The heavycap checkout contains neither a tracked pre-commit entry point nor an installed Git
pre-commit hook. ASD-V0-2478 is verified through the actual heavy.sh runner, including a caller
working in the package directory. The separate pre-commit integration of V1-5007 remains
NOT_PRODUCED until its owning hook is available; this is not a claim that a narrower external
hook has been repaired.

## Rollback

Roll back `ASD-V0-2400` to `ASD-V0-2402` by reverting their functions in `script/heavy.sh` and
`script/batch-precheck.sh` and the named test groups; heavy.sh then refuses a new package and
suggests the Git-directory manifest again, and batch-precheck narrows only on `FRONTIER_OK`.
`ASD-V0-2403` needs no rollback: it keeps the base behavior, where every darwin-only command
stays local. No product, state or wire format changes.

For ASD-V0-2477/2478, revert the local runtime default and changed-source admission to restore
prior policy. No JSON, error code, exit code or core wire contract changes.
