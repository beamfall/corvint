# 2026-11-03 lane floor: two floor packages declared; typed-reach test-function narrowing

- Lane `floor`, branch `claude/floor`, base `6f12f4704235dc8f9bea19fc12995bfb68c746c3` (origin/main).
- Tickets: V1-0643, V1-2744. Requirements: AFP-V0-2360 to AFP-V0-2363 and AFP-V0-2370 to
  AFP-V0-2378 in `docs/specs/affected-plan-selection-v0.md`. The parent `affected-plan-v0.md` is at
  its 256-obligation limit, so they live in the companion spec under the parent's prefix.
- Host: wsl-lanes, go1.27.3 linux/amd64, Landlock ABI 7.

## V1-0643: companionrelease and release-artifact-v0 leave the UNBOUNDED_READER floor

Owner direction (2026-11-03) replaces the V1-2292 acceptance of both packages as floor members.
Each package's tests now run Git only against disposable fixture repositories, and each package is
declared in `.corvint/test-read-scopes.json` (AFP-V0-023). Both rows are gone from
`.corvint/unbounded-readers.json`.

### What the fixtures prove

**internal/companionrelease**
- `currentSourceFixture` is built once per test binary. It copies the Tasks export subset, plus
  `.gitattributes`, `.gitignore` and `extensions/vscode/`, from the working tree, applies the
  repository's own ignore rules and commits with `closedGitEnv`, so 2,066 files are exported.
- The unchanged `exportSource`, `tasksExport` and offline-build assertions run over it.
- One new validity check: the offline test fails if `tasksExport` drops nothing.
- Root builds pass `-buildvcs=false`.
- 34 declared entries: `cmd/` command directories, `internal/`, `.github/cishards/`, `go.mod`, the
  notices, `extensions/vscode/`, and the files the runtime-tuple and core-stable tests read.

**conformance/release-artifact-v0**
- Destination validation and the cancelled archive build use disposable repositories.
- Fixture repositories are created from a temporary parent, never with the checkout as Git's
  working directory.
- 16 declared entries. `.agent-evidence/` and `benchmarks/results/` are declared whole, so the
  historical-version sweep over every `.json` keeps its strength.

### What is given up

The three current-export tests no longer check the whole tree against the export bounds
(entries, bytes, symlinks, case folds); only the copied subset is checked. Production
`exportSource` still enforces those bounds on the real checkout at release time.

### Decision: the darwin host archive proof becomes an explicit opt-in (AFP-V0-2362)

`TestCorvintHostArchivePartialProof` (darwin/arm64 only) exports and builds the checkout's `HEAD`
through `.git`, which no declaration can grant, and building it from a fixture would mean copying
and declaring the whole `cmd/corvint` closure. It now runs only with
`CORVINT_HOST_ARCHIVE_PROOF=1`, which the `release-gates` job env sets; that workflow change is a
separate `.github` commit. Keeping the package on the floor for one darwin-only test was rejected,
because it would cost every change on every host.

### Evidence

| Check | companionrelease | release-artifact-v0 |
|---|---|---|
| Unconfined run, before → after | pass, 132.1 s → 124.7 s; 304 pass / 5 skip both | pass, 15.3 s → 14.2 s; 102 pass / 1 skip both |
| Confined (`go test -exec test-confine`, `CORVINT_CONFINE_ROOT` set) | pass, 149.1 s | pass, 13.5 s (17.8 s after the opt-in edit) |
| Confinement is real | `test-confine /bin/cat ../../.git` denied | removing the `Makefile` entry fails `TestGateExcludesCompanionReleaseGate` with `permission denied` |
| Unrelated path (`corvint affected --worktree --path scratchfloor-*/u.txt`) | not selected | not selected |
| Negative control (path inside a declaration) | `.github/cishards/cmd` → `DECLARED_READ_SCOPE`; `LICENSING.md`, `extensions/vscode/package.json` → `PATH_LITERAL_READER` | `.agent-evidence/ctx-w58/SHA256SUMS` → `DECLARED_READ_SCOPE`; `Makefile`, `LICENSE` → `PATH_LITERAL_READER` |
| Fault injection (selected, then fails) | blank import of `internal/companioncli` in `cmd/corvint-tasks/main.go`: `BINARY_EXEC`; `TestCurrentTasksSourceArchiveBuildsOffline` fails (`no required module provides package`) | current version string written into `.agent-evidence/ctx-w58/local-outcome.json`: `DECLARED_READ_SCOPE`; `historical_evidence_version_test.go:65` fails |

On a trivial unrelated path, the floor went from 29 to 27 units. The remaining
`UNBOUNDED_READER` members each keep their recorded reason; making them hermetic is outside this
lane (Friction below).

### Per-merge saving, measured

Replay: the 60 most recent first-parent merges of origin/main, each merge's changed paths planned
with this lane's `corvint affected --worktree --paths-from`, on the current graph with the new
declarations. Before, both packages were floor members, so they were selected on 60 of 60.

| Package | Selected after | Package time | Saved per merge |
|---|---|---|---|
| internal/companionrelease | 47 of 60 | 124.7 s | 13/60 × 124.7 ≈ 27.0 s |
| conformance/release-artifact-v0 | 37 of 60 | 14.2 s | 23/60 × 14.2 ≈ 5.4 s |

Total: about 32 package-seconds per merge.

Most merges touch `internal/` or `cmd/`, which companionrelease's root builds really read: 43 of
the 60 touch `cmd/`, `internal/`, `go.mod` or `go.sum`. That dependency is honest, so the rest of
companionrelease's cost belongs to the declaration, not to the floor.

## V1-2744: typed call-graph test-function narrowing (AFP-V0-2370 to AFP-V0-2378)

### Design decisions (decided here; the ticket body is NOT_OBSERVED)

V1-2744 and V1-2708 are not in this checkout's `.taskman` snapshot, and neither is on origin/main.
The V1-2708 repro definitions are therefore NOT_OBSERVED, and the repros are real first-parent
commits on origin/main that change a dependency of the allow-listed packages.

1. **No external dependency.** The repository has none, so the graph is built on `go/types` from
   the standard library. Module packages of the host test binary are type-checked from source;
   standard-library packages are checked from GOROOT without bodies.
2. **Conversions, not CHA.** The graph has an edge for every call, every function value taken, and
   every method of every module type structurally inside a value converted to an interface. This
   covers standard-library and reflective callers, such as `fmt` calling `String`.
3. **Package-variable initializers hold values, they do not call them.** A dispatch table built
   at initialization (the cause of afr-w31's "always-run reaches everything") reaches its handlers
   only through code that reads the variable.
4. **Two things are excluded from always-run code:**
   - the child-only `TestMain` branches (condition on `os.Args` or the environment, ending in
     `os.Exit`);
   - `init` assignments into a package variable, which are held.
5. **Cross-test state** is handled by a fixpoint over the package variables kept tests write, plus
   environment and signal pseudo variables. Any other process-wide setter in a kept test keeps
   the package whole.

Every refusal is named in the slice reason.

### Measurements

All rows are after both review rounds below, and all use the same build: this lane's binary.

| Range (origin/main) | Package | `typed-reach-measure` | `corvint test --dry-run` |
|---|---|---|---|
| `6a2c582ef^..6a2c582ef` (`internal/tasks/wire/value.go`) | internal/tasks/store | 648 kept, 18 dropped of 666; 4.8 s | **narrowed**: `-run` of 648; reason `selected: the tests reaching internal/tasks/wire/value.go (typed reach: 18 of 666 test functions cannot reach it)`; 9.4 s whole plan |
| `c38c32203^..c38c32203` (`internal/tasks/intent`) | internal/tasks/store | 656 kept, 20 dropped of 676; 6.2 s | whole: `the package names internal/tasks/intent/export_test.go as a path` |
| `d1ccab3b6^..d1ccab3b6` (`internal/tasks/dispatch/config.go`) | internal/tasks/store | 648 kept, 18 dropped of 666; 7.2 s | whole: `the package names docs/specs/corvint-tasks-dispatch-signals-v0.md as a path` |
| all of the above | internal/tasks/cli, cmd/corvint | n/a | whole: unbounded reader (AFP-V0-025), or an `IMPORT_GRAPH_READER` / `PATH_LITERAL_READER` witness |

Kept reasons on `6a2c582ef`:
- 553 `REACHES_CHANGE`
- 23 `READS_STATE_A_KEPT_TEST_WRITES`
- 72 `STARTS_PROCESS`

The conservative mass is:
- tests that start any process, git included;
- tests that read memo or test-hook package variables a kept test writes.

### Recall against observed outcomes (fault-injection oracle, AFP-V0-2378)

`tools/typed-reach-measure -oracle -oracle-dropped-only -oracle-also <4 kept tests>` builds the
store test binary at head and again with `panic("typed-reach fault")` at the entry of every
changed function. It runs each listed test function alone in both binaries, and classifies each
outcome from `-test.v` output.

| Range | Faulted functions | Ran | Observers (outcome changed) | Dropped observers (missed) |
|---|---|---|---|---|
| `6a2c582ef` (final model) | `wire.CheckSortedUnique`, `wire.clip` | 18 dropped + 4 kept | 4: the 4 kept controls, pass → fail | **0** |
| `6a2c582ef` (after round 1) | the same | 24 dropped + 4 kept | 4: the 4 kept controls, pass → fail | **0** |
| `c38c32203` (after round 1) | 9 `intent` functions | 26 dropped + 4 kept | 4: the 4 kept controls, pass → fail | **0** |
| `14e055d69` (before the review fixes) | 67 (none linked by store tests) | 29 dropped | 0 | **0** |

A missed observer can only be a dropped test, so no unit that observes the change was left out.
The kept controls show the fault is observable. The observer count over all kept tests is
NOT_PRODUCED: about 650 store tests run alone twice is longer than this lane's budget, and it
cannot change the missed count.

### Independent review (`script/codex-review.sh`, reviewed head `a5e422f56`)

The review found eight soundness gaps, a fixture-strength regression and an oracle bug. Each
soundness gap is fixed and has a regression in `TestAFPV02374ReviewRegressions`:

1. **An import change** with unchanged declaration text. Now a change, or an initialization
   change.
2. **Conversions through multi-value returns, assignments and calls.** Now recorded.
3. **Exec and setter functions taken as values.** Now keep their effects; an `os/exec.Cmd`
   literal counts as a process start.
4. **The constant-git exemption.** Removed: aliases, hooks and configuration can run module code.
5. **Writes through a local copy of a package slice or map.** Now writes.
6. **`sync.Pool` and `sync.Once.Do`.** Now writes.
7. **The child-only TestMain branch.** Now needs `os.Args[N] == "C"`.
8. **Files at a constant path.** Now process state. Paths derived from a package variable write
   that variable.

The oracle bug is fixed too: it now reads `-test.v`, so a skip is no longer a pass.

### Re-review (reviewed head `90c65cd6a`)

The re-review confirmed four fixes complete: imports, tuples, git, and the oracle. It found five
remaining gaps, all now fixed with regressions in `TestAFPV02374ReReviewRegressions`:

1. **Reader functions taken as values.** They now read their pseudo variable.
2. **A write through a copy of a struct holding a pointer.** Structs and arrays holding references
   now count as references.
3. **A bound `once.Do` value.** It now writes its receiver.
4. **A TestMain condition that runs in the parent.** The condition is now always-run code.
5. **A shared file named through a local variable.** Files are now a pseudo variable unless the
   path is provably derived from `TempDir`, `MkdirTemp` or `CreateTemp` in the same body.

It found no gap in `cmd/corvint/test_typed_slices.go` itself.

**Fixture strength (P2), recorded and not fixed.** The companionrelease fixture applies ignore
rules. That equals `HEAD`'s tracked set only while no tracked file matches an ignore rule;
`git ls-files -ci --exclude-standard` is empty on 2026-11-03. A gate that keeps it empty would
make this hold; it is proposed in the Friction list.

### Soundness cases in tests

- Negative control: `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction`. Changing `lib.B`
  instead of `lib.A` flips the kept test, and TestMain's child-only branch, which also calls B,
  does not make the package whole.
- End to end: `TestAFPV02370DryRunNarrowsAnAllowListedDependent` (`cmd/corvint`) covers an edit of
  A, an edit of B, and an added `init`, which keeps the package whole.
- Fault injection on the model: with the conversion edge disabled,
  `TestAFPV02371ConversionsAndHeldValuesCarryReach` fails; restored, it passes.

### Not met, stated

- `cmd/corvint` and `internal/tasks/cli` do not narrow: they are unbounded readers, and narrowing
  an unbounded reader by test function would drop tests that read a dirty path outside the model.
- Wall-clock saving of a narrowed store run is NOT_PRODUCED. The selection drops 25 of 666 test
  functions, and their durations were not measured in isolation.
