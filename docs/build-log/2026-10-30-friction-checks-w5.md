# Friction lane checks-w5: batch-precheck, doc checks and lane hygiene

Lane K=3 on unraid-lanes, base `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`, tickets V1-3122,
V1-3124, V1-3128, V1-3132, V1-3134, V1-3135, V1-3136, V1-3137, V1-3142, V1-3143, V1-3153, V1-3156,
V1-3170, V1-3171, V1-3174, V1-3175, V1-3224. The full ticket text file was not on this host and
the Tasks store here is a snapshot without these tickets, so each ticket was read from its digest
title and the code.

## Problem

Nine of the seventeen tickets were already delivered before the 1.0 snapshot: the requirements,
code and tests that name them are on main (table under Outcome). The other eight were open:

- V1-3153: the `CAP-V0-047` dispatch scan keyed argument slices by name alone, so a helper whose
  `argv` or `args` slice builds a child `git` or `go` command line demanded tree entries for the
  child's words and flags.
- V1-3170: `make host-package-version-bump PKG=pi` left the `corvint-pi-capabilities/0` tuple in
  `docs/specs/pi-workflow-v0.md` behind; only the `ASD-V0-044` node test caught it, later.
- V1-3171: the `DCW-V0-388` block ended with `DCW-V0-387`'s rollback line, and `DCW-V0-387` had none.
- V1-3174: `.corvint/documentation-paths.json` had no generator or freshness check.
- V1-3175: `TestDogfoodFinishRunsFromBinaryInForeignRepository` flaked with `fork/exec ...: text
  file busy`: a parallel test's fork can hold the just-copied binary's write descriptor.
- V1-3156, V1-3132, V1-3224: the lane guide did not tell lanes where scratch logs and helper
  binaries go, nor to keep closeout and seal in the foreground.

## Decisions

- `CAP-V0-200`: exclude, per function body, a slice the body hands to `exec.Command`,
  `exec.CommandContext`, `syscall.Exec` or a `Cmd`'s `Args` (directly, sliced, spread or through
  `append`), or builds from a `[]string{...}` literal. Names bound from `os.Args`, a selector's
  `Args` or an `Args()` call are never excluded, so self re-execution stays scanned. Before and
  after, the scan of all three committed command trees is identical (612 lines, diffed).
- `AHI-522`: a new `tuple:FIELD` location kind in `script/host-package-version.sh` rather than a
  general regex location, so the Pi spec's tuple is bumped and verified with the other six Pi
  locations; zero or two tuple objects refuse as "no single value", like `lit:`.
- `ASD-V0-105`: the declaration is derived (every tracked top-level `docs/*.md` plus three
  directories); the generator reproduced the committed file byte for byte. The check joins
  `DOC_GATE_TARGETS`, which CI's `doc-gates` job already runs through `make doc-gates`, so no
  `.github/` file changes.
- `ASD-V0-106`: retry only the test runner's start, on the existing `workTextBusyRetries`
  delays, with a plain `Start()`; reusing `workStartAdapter` would route through `workStartLive`,
  a global the prereap tests swap.
- `ASD-V0-107`, `ASD-V0-108`: guide text in `docs/LANE-ORCHESTRATION.md` sections 4 and 6.
- V1-3171: text fix only; no requirement changes meaning.

## Adversarial refusal pass (docs/LANE-ORCHESTRATION.md section 9)

`CAP-V0-200` (a narrowing rule of the dispatch scan):

| Category | Case and test |
|---|---|
| Malformed input | An unparsable file still fails `ScanDirs` (unchanged); a body-less `FuncDecl` adds no scope. |
| Aliasing | `a := argv` then exec of `a` excludes only `a`; `argv` stays scanned (conservative, stated in the spec). |
| Spread/variadic | `argv...`, `args[1:]...` and `append([]string{"-C", dir}, argv...)...`: `TestCAPV0047ChildArgvIsNotDispatch` (`appended-child`, `context-child`). |
| Quoting | Words are AST string literals unquoted by `strconv`; unchanged. Not applicable. |
| Historical vs worktree | The scan reads the files it is given; unchanged. Not applicable. |
| Unobserved state | Own command line re-executed (`os.Args[1:]`, `flag.Args()`) stays scanned: the same test (`reexec-verb`, `flag-args-verb`); without the guard those words vanish. |
| Concurrency | Pure function of the parsed files. Not applicable. |

`AHI-522` (`tuple:`): malformed, two tuples refuse in verify and bump (case 4c); a drifted tuple
is named (case 4b); quoting: the value is matched as a JSON string with no quote or newline, like
`lit:`; historical reads, unobserved state and concurrency do not apply (one worktree file, and
bump rewrites through a temporary file and `mv`).

`ASD-V0-105`: malformed, a hand edit and a missing file fail `--check`; quoting, a name with a
space, quotes and a non-ASCII byte is declared raw (NUL-separated listing; Git would C-quote it);
historical vs worktree, the listing reads the index, so an untracked guide is not declared until
staged (case 2); concurrency, the write goes through a temporary file and `mv`.

## Evidence

All on unraid-lanes (Linux 6.18.38-Unraid, go1.27.3), installed `corvint` build 6 at the base.

- `corvint affected --base c5d5f08`: `scope=UNKNOWN dirty=10 selected=93 excluded=473 unknown=87
  LANGUAGE_FRONTIER=86`; the focused tests below were run instead of the 93 units.
- `go test -run 'TestCAPV00|_V1_3137$|TestChangeDCWV0386|TestDCWV0387|TestDCWV0388'` over
  `internal/clitree`, `tools/capabilities-check`, `tools/test-read-scopes-check` and
  `internal/dogfoodflow`: PASS. `TestCAPV0047ChildArgvIsNotDispatch` fails with the change
  stashed (`flags [--exec-child --no-pager --real-flag]`).
- `go test -run '^(TestPortableRunRetriesTextFileBusy|TestDogfoodFinishRunsFromBinaryInForeignRepository|TestDogfoodDailyPathRunsFromBinaryInForeignRepository|TestTraceGitignoreFinishThenChangeCheckSeal)$' ./cmd/corvint`:
  PASS; with the retry disabled the new test fails `fork/exec .../bin/corvint: text file busy`.
- `make doc-gates` (now with `documentation-paths-check`; includes `use-case-receipts-test`,
  `traceability-tests-test`, `test-read-scopes-check`, `line-citations-check`): rc 0, 19 s.
- `make documentation-paths-test` (5 cases), `host-package-versions-test` (16 cases),
  `spec-requirements-test`, `traceability-tests-test`, `line-citations-test`: PASS.
- `make batch-precheck-test`: section 1b needs `node`, which this host lacks; a temporary copy
  without section 1b passed every other section in 69 s (groups 2, 3, 7 and section 8).
- `make windows-vet`: rc 0.

## NOT_RUN

- `make batch-precheck-test` section 1b (the Pi tuple node test) and `make host-adapter-node-test`:
  no `node` on this host.
- `make gate` and whole-package tests: lane policy.

## Rollback

Each requirement carries its own rollback in its spec. Together: revert this lane's commits; the
documentation-paths declaration file itself is unchanged.

## Outcome

| Ticket | Disposition |
|---|---|
| V1-3122 | already delivered: `ASD-V0-046` rerun and `FLAKY` row; batch-precheck-test section 8 PASS here |
| V1-3124 | already delivered: `UCV0-019` and `DCG-V0-047`; `use-case-receipts-test` (9 cases) and `line-citations-test` PASS |
| V1-3128 | already delivered: `DCW-V0-352` names each unstaged spec (reproduced here with an unstaged edit); `spec-requirements-test` PASS |
| V1-3132 | already delivered in the tool (`DCW-V0-386`, `TestChangeDCWV0386NotesUntrackedBuildOutputs` PASS); lane guide bullet added (`ASD-V0-108`) |
| V1-3134 | already delivered: `ASD-V0-050`; batch-precheck-test group 7 PASS |
| V1-3135 | already delivered: `ASD-V0-052`, section 9; this entry uses it |
| V1-3136 | already delivered: `ASD-V0-051`; batch-precheck-test group 7 PASS |
| V1-3137 | already delivered: `AFP-V0-828`; `TestTempRootOnly*_V1_3137` PASS |
| V1-3142 | already delivered: `ASD-V0-047`; batch-precheck-test section 3 serial probe PASS |
| V1-3143 | already delivered: `TTG-V0-011`; `traceability-tests-test` PASS |
| V1-3153 | delivered: `CAP-V0-200` |
| V1-3156 | delivered: `ASD-V0-108` |
| V1-3170 | delivered: `AHI-522` |
| V1-3171 | delivered: rollback lines corrected |
| V1-3174 | delivered: `ASD-V0-105` |
| V1-3175 | delivered: `ASD-V0-106` |
| V1-3224 | delivered: `ASD-V0-107` |

The dogfood result and its `NOT_PRODUCED` reasons are in the sealed CEM for this branch.
