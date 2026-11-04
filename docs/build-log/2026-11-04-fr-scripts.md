<!-- SPDX-License-Identifier: Apache-2.0 -->
# Script and gate friction: batch precheck, dogfood cache test, heavy.sh, walks, selection

Date: 2026-11-04. Lane fr-scripts, base `6c33df33`. Tickets: V1-4525, V1-4524, V1-4513,
V1-4511, V1-4532, V1-4531, V1-4512, V1-4528, V1-4515, V1-4526, V1-4523. Requirements:
ASD-V0-2472 to ASD-V0-2476 in `docs/specs/lane-shell-portability-v0.md`, AFP-V0-2540 to
AFP-V0-2542 in `docs/specs/affected-plan-selection-v0.md`, GOC-V0-062 in
`docs/specs/go-only-cutover-v0.md` and WQO-V0-971 in `docs/specs/work-queue-observation-v0.md`.
`corvint-self-development-v0.md` and `affected-plan-v0.md` are at the 256-obligation limit, so
their requirements went to the existing companions. V1-4508 and V1-4533 target code that is not
at this base and are left to a later lane.

## Decisions

- **The CI-only batch-check step is pinned, not mirrored (ASD-V0-2472, V1-4525).** The doc-gates
  step named `(CIN-V0-2304` needs the pull request's base and head, `RUNNER_TEMP` and a
  `claude/batch-*` ref, so no local step can run it; its twin is `make dogfood-batch-check`. The
  sync check accepts it only while its trimmed body equals `CI_ONLY_BATCH_CHECK`, so a changed
  body, or the same body under another name, still fails closed.
- **Shell test reports count cases (ASD-V0-2473, ASD-V0-2474).** `batch-precheck_test.sh` folds a
  multi-line message onto one line, and `BATCH_PRECHECK_TEST_GROUPS` runs named groups with the
  groups whose fixtures they reuse; an unknown label exits 2. `dogfood-cache_test.sh` re-runs
  itself under bash when started by `sh`, refuses a re-run bash that is still in POSIX mode
  rather than looping, and prints a FAIL line with the last output when a case stops it. Its
  closeout cases now pass the fixture's real base, which the spec capacity step reads.
- **heavy.sh keeps go.env flags (ASD-V0-2475, V1-4513).** An exported `GOFLAGS` replaces the
  value `go env` reads from GOENV or `$GOROOT/go.env`, so with none in the environment the `-p`
  bound starts from `go env GOFLAGS`. A value that already sets `-p` is left unexported.
- **SkipDir only for directories (ASD-V0-2476, WQO-V0-971, V1-4511).** `fs.SkipDir` returned for
  a file skips the rest of its parent directory. `verifyMaterializedTree` did this for a linked
  worktree's `.git` file, so every later root entry went unverified; the post-merge launcher had
  the same shape. `tools/walk-skipdir-check` is a doc gate that accepts `SkipDir` only under a
  known-directory guard or for a walk error. It resolves each file's import names, so an alias
  or a dot import of `path/filepath` or `io/fs` is still checked.
- **Dropped declarations name the right file (AFP-V0-2540, AFP-V0-2541, V1-4532).** When a
  package leaves `.corvint/test-read-scopes.json` its tests become unbounded readers. The refusal
  named only `.corvint/unbounded-readers.json`, so lanes edited the wrong file.
  `unbounded-readers-check` now adds a `likely cause` line, and `test-read-scopes-check` fails
  for the dropped package itself. Both read HEAD's blob through `git show`, and without Git or a
  commit they add nothing. The reader is duplicated because two `main` packages cannot share it.
- **JSON language names are data (GOC-V0-062, V1-4531).** The no-Python scan was red at the base
  on `"python"` in language lists (`FRONTIER_OTHER_LANGS`) and on `"detail":"python:REASON"`. The
  mask blanks only those two forms, so `python3`, a JSON argv and an image tag still fail. A
  language list is recognized by its field or variable name (`LANGS`, `languages` and the like),
  not by its neighbours, so the argv `["python","go"]` still fails.
- **A leaf package pattern is no unbounded read (AFP-V0-2542, V1-4512).** Rule (d) of AFP-V0-012
  made any test token ending in `/...` an unbounded read. A `./...` or `./NAME/...` whose subtree
  holds no other package directory and no nested `go.mod` reaches no more than `"."`, which rule
  (d) already accepts, and a data file below it already selects the package. Both analyzers apply
  the rule: the planner's Go plugin and `internal/gateselect`, which mirror rule (d). The
  nested-module reader (`nested.go`) is left unchanged, which is conservative. The rule counts
  directories, not files, so a package added below a leaf makes the read unbounded again. The
  gateselect scan memo caches tokens, not verdicts, and the planner's graph cache restats every
  listed directory, so neither can keep a stale verdict. The ticket's example, a `WalkDir` over
  `"."`, is not unbounded under rule (d), so the fix targets the `/...` form. The repository's
  recorded unbounded set is unchanged by the rule. The premise is that the command runs in the
  package directory, so a package whose Go files set `Dir` to anything but `"."`, call `Chdir`
  or pass `-C` or a `cd` word keeps rule (d) for its leaf patterns.
- **No product change for V1-4526 and V1-4523.** The `internal/mcp/cli` and `internal/tasks/cli`
  tests that need a directory outside any Git repository now skip when the temp directory sits
  below a `.git` entry, and the skip names that directory. The guard is duplicated in each package so that
  `internal/testsupport`, with over a hundred importers, does not widen selection. The
  journal-absent refusal of V1-4523 already names the CTS-V0-821, CTS-V0-680 and ASD-V0-2421
  remedies at the base.
- **Documentation only (V1-4528, V1-4515).** Two duplicate lines from a receipt repin are removed;
  no gate catches adjacent duplicate lines. `docs/AGENT-ROUTES.md` and `AGENTS.md` give the code
  search pathspec that excludes tracked change evidence: at the base an unscoped
  `git grep -n check-line-citations` printed 997 KB, against 65 KB with the pathspec.

## Evidence

Each behavior change has a regression test that fails at `6c33df33` (or with the change
reverted) and passes after it; the traceability rows of each spec name them. For AFP-V0-2542,
with the exemption disabled `TestLeafPackagePatternParity_AFPV02542` reports `datum`, `fixtured`
and `leaf` as unbounded, and `TestAFPV2542LeafPackagePatternIsNoUnboundedRead` fails.

## Fix round 1

An independent review found three gaps, each now closed with a regression that fails on the
sealed head `6ebc5518`:

- AFP-V0-2542 assumed a leaf pattern runs in the package directory. A test that finds the root
  with `runtime.Caller`, sets `cmd.Dir` there and runs `go build ./...` was exempted by both
  analyzers. Each now records a directory move per file (`directoryMove`) and puts the package's
  leaf reads back under rule (d) when any file moves. The gateselect scan memo stores the flag,
  so a warm index agrees with a cold one. `tools/unbounded-readers` sets `cmd.Dir` in its tests,
  so its AFP-V0-2542 test now builds the fixture's `./...` from two literals rather than carry it.
- GOC-V0-062 masked `"python"` next to any other language name, so the argv `["python","go"]`
  hid an interpreter call. The mask now needs a named language-list field.
- ASD-V0-2476 matched `filepath.WalkDir` and `fs.SkipDir` by literal qualifier, so an aliased or
  dot import skipped the check, and a different package imported as `filepath` was checked. It
  now resolves the file's imports.
