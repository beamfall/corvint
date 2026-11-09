# V1-5178: shared script data readers

Owner request: a data edit must select only tests it can affect; uncertain readers stay
selected. Base: `447bdcbd5963683f5f2b4934a45b8f48fc6742a3` (`origin/main`), branch
`claude/v1-5178-tsv-readers`. Requirements: AFP-V0-3330 through AFP-V0-3334; the requested
3330..3339 block had no matches before editing (`git grep`, exit 1).

## Diagnosis and bounded change

The supplied `aff157.json` contains twelve dirty paths, not an isolated TSV change. Its
24 requested witnesses were traced using the planner's `Select`, `pathTokens`,
`newPathToken.namesAt`, `Unit.ReadScope` and `Unit.Imports` in temporary diagnostic tests.
Go literal origins were checked by running `golang.FilePathTokens` on individual AST
string literals. Temporary diagnostic code was removed. Original diagnostic output is
retained in the task scratchpad as `debug-5178.log` and `debug-final-5178.log`.

In the following table, shell names mean `script/NAME_test.sh`; local files mean
`script/FILE`. D means `DECLARED_READ_SCOPE`, L means `PATH_LITERAL_READER`.
Every D row originally has `.corvint/shell-read-scopes.json`, entry `script/`, via length
1. This is the adapter's **implicit own directory**, not an explicit `script/` JSON row.
Every L row originally has via length 1: the token is on the unit itself, not inherited
from an unexecuted script. Existing external `reads` entries are retained unchanged.

| Unit | Before: exact cause and original source evidence | After / audited local reads |
|---|---|---|
| capability-catalog | D; test line 26 runs the checkout Makefile with a fixture generator | Excluded; no sibling inputs |
| check-install-lifecycle | D; line 21 names its lifecycle wrapper; lines 23–24 build Core | Excluded; `check-install-lifecycle.sh` |
| check-sql-native-ratchets | D; line 51 reads the wrapper into a fixture | Excluded; `check-sql-native-ratchets.sh` |
| core-n1-replay-default | D; line 15 copies the release selector | Excluded; `select-previous-release.sh` |
| dogfood-merge-base | D; lines 17 and 70 name/copy the merge-base wrapper | Excluded; `dogfood-merge-base.sh` |
| gate-affected-corvint | D; lines 28–30 execute the wrapper; line 38 builds Core | Excluded; `gate-affected-corvint.sh` |
| gate-affected | D; lines 35 and 106 execute the wrapper against the fixture | Excluded; `gate-affected.sh` |
| gate-receipt | D; line 20 copies the wrapper; lines 200–220 parse the checkout Makefile, whose line 1197 conditionally runs the merge-base helper | Excluded; `gate-receipt`, `dogfood-merge-base.sh` |
| qualify-tasks-basic | D; line 22 binds the qualification wrapper; line 32 builds Tasks | Excluded; `qualify-tasks-basic.sh` |
| repin | D; line 10 runs `make -n`; wrapper text is expected output, not execution | Excluded; no sibling inputs |
| spec-requirements-conflict | D; lines 41–42 copy exactly three scripts into the fixture | Excluded; `gen-spec-requirements.sh`, `repin-use-case-receipts.sh`, `spec-requirements-resolve.sh` |
| vscode-check-install | D; line 22 copies only the extension and validity fixtures | Excluded; no sibling inputs |
| check-ci-least-privilege | L; exact `script/ci-least-privilege-exceptions.tsv` at test lines 135–136; executes `check-ci-least-privilege.sh` | Retained L |
| dogfood-bin-reuse | L; `/script`, `/script/`; fixture-copy destination at line 15 and chmod glob at line 63 | Retained: dynamic fixture copier is not bounded by this patch |
| dogfood-bind-range | L; `/script` at fixture-copy call, line 24 | Retained: opaque copier argument and legacy own-directory scope |
| release-env-compat | L; `/script/` from `$root/script/$wrapper`, lines 15 and 24 | Retained: computed wrapper reads not proven by this patch |
| review-head-check | L; `/script/` from `/bin/cp` destination at line 90 and chmod glob at line 92 | Literal removed; retained `DEPENDENCY_PATH` (3 units) |
| runtime-environment | L; `/script/` from `$root/script/$helper`, line 27 | Retained: computed helper plus invoked dogfood tests |
| test-rg-fallback | L; `/script/` is the truncated `script/*_test.sh` glob at line 15 | Excluded; matching shell files still select it |
| internal/analyzerhtmlcss | L; `script` and `/script` from HTML grammar (`parse.go` lines 37–43, 71–77) and HTML fixtures | Retained: no package-wide read proof or narrower declaration |
| internal/opencodequalification | L; `script` in `provider_test.go` line 205's `t.Run` name | Literal removed; retained `BINARY_EXEC`: dogfood-change → dogfoodflow → Core → qualification |
| internal/stepverify | L; `script` in `step_test.go` line 339's `t.Fatal("script executed")` | Excluded |
| tools/flow-guide-check | L; `script`, `script/`, including `main.go` lines 334 and 467's walk/glob; explicit Go scope `script/` | Retained: broad declared tree remains authoritative |
| tools/test-commands-check | L; `script`, `script/`, including `closeout_recipe_test.go` line 45 and `dogtools_test.go` joins; explicit Go scope `script/` | Retained: broad declared tree remains authoritative |

The wrappers above either execute in temporary repositories, operate on copied inputs,
or build the already-declared Go closure. No narrowed local list includes a recursive
checkout-directory enumeration. The enumerating fixture in the regression deliberately
omits `localReads` and retains its entire directory. Explicit declarations are project
claims, not automatic proof of arbitrary shell behavior; omitted/null declarations,
invalid declarations, unknown computed reads and unsupported wildcard syntax stay broad.

Implementation adds optional audited `localReads` and applies the resulting complete
shell scope consistently to literal, glob, declared and transitive readers. It recognizes
path-qualified copy utilities' output operands and preserves simple path-shaped wildcard
patterns rather than their truncated directory tokens. Non-path `*/*` case patterns do
not become readers. Go narrows only direct diagnostics on resolved `*testing.T` parameters
and `Run` names whose callback does not observe or escape its test handle. Real reads,
custom/shadowed receivers, handle escapes and name-derived paths remain indexed. This
changes no Go package subtree rule and contains no TSV-specific exclusion.

## Controlled measurement

The isolated baseline was built from unmodified origin/main to `corvint-5178-before`.
The candidate is `corvint-5178`, rebuilt in the committed disposable snapshot to bind
its embedded source identity. The earlier main-worktree build emitted an advisory
`self-binary-stale` notice because its embedded commit preceded the unstaged patch;
that measurement is superseded by the snapshot build. Both paths are in the parent task scratchpad. Each plan
was made in the same disposable detached worktree, with a single committed
`# V1-5178 selection measurement` line appended to the TSV (its existing comment syntax
is `#`), using `<bin> --root <scratch-worktree> affected --base HEAD~1 --format json`.
The main worktree cannot stage because its private index is protected, so the candidate
source and declaration snapshot was committed only in the disposable measurement tree.

| Build | Baseline of TSV-only range | summary.line |
|---|---|---|
| origin/main | `447bdcbd5963683f5f2b4934a45b8f48fc6742a3` | `scope=UNKNOWN dirty=1 selected=248 excluded=647 unknown=75 LANGUAGE_FRONTIER=75` |
| candidate | `bf17ba07f07b130e784120b675175a236e95b953` | `scope=UNKNOWN dirty=1 selected=234 excluded=661 unknown=75 LANGUAGE_FRONTIER=75` |

Fourteen units leave the selected set: the twelve audited shell scopes, test-rg-fallback,
and internal/stepverify. No units are added. The true TSV reader remains. These are plan
unit counts, including driver targets, not a claim that only 234 executable tests run.
The 75 uncertainty entries remain; no FULL-fallback waiver or universal precision claim.
A run with a staged documentation edit (dirty=2) was rejected and retained as
`measurement-after-dirty-rejected-5178.json`; the final run verified a clean worktree
and reports worktree=0. The final binary emitted no stale-source warning.
The complete selected unit IDs and witness kinds are retained as `selected-before-5178.txt`
and `selected-after-5178.txt`; full original receipts are `measurement-before-5178.json`
and `measurement-after-5178.json` in the parent scratchpad.

## Changed files

- `.corvint/shell-read-scopes.json`
- `internal/liveverify/affected/readers.go` and `tsv_readers_test.go`
- `internal/liveverify/affected/shell/shell.go`, `readscopes.go`, `readscopes_test.go`
- `internal/liveverify/affected/golang/golang.go`, `testmetadata.go`, `testmetadata_test.go`
- `docs/specs/affected-plan-selection-v0.md` and generated `REQUIREMENTS.tsv`
- This build-log entry.

## Verification and limits

Toolchain: `go1.27.3`. Corvint context and affected reads were used. Installed Core is
`1.0.0-rc.3.dev`, build 137 at the base commit. The official Core update check returned
freshness UNKNOWN because process-cleanup verification was denied by the sandbox; no
update was installed. Tasks, network writes and subagents were excluded by the owner.
The existing model was retained for this bounded diagnosis/build; no routing fanout.

- PASS: affected package with `-count=1 -timeout 30m -run 'TestShell.*AFPV0333|TestRepositoryShellDataReaders_AFPV03330|ReadScope|Shell'` through `script/heavy.sh`.
- PASS: shell package with `-count=1 -timeout 30m -run 'ReadScope|Shell'` through `script/heavy.sh`.
- PASS: new Go `TestTestMetadataReaders_AFPV03334`; the wider Go run also exercised literal/runtime-rooted reader tests.
- BLOCKED: the required wider Go `ReadScope` run fails in five existing Git-fixture test groups with `process-cleanup-failed ... operation not permitted`. Repeating `TestReadScopeEditSelectsOnlyChangedDeclarations_AFPV0050` with original production code reproduces that sandbox failure.
- PASS: full `./tools/test-read-scopes-check/` package, count 1 and timeout 30m, through the heavy wrapper with the owner's explicit whole-package reason.
- PASS: `GOTOOLCHAIN=local go vet` on affected, affected/shell and affected/golang, one package per command; command build; `git diff --check`.
- PASS in the staged disposable snapshot: `make spec-requirements`, `make spec-requirements-check`, and `make changed-checks BASE=447bdcbd5963683f5f2b4934a45b8f48fc6742a3`. Generated REQUIREMENTS.tsv was copied back. The main worktree's generator refuses its unstaged spec; its index-reading checks alone cannot certify these unstaged edits. The unbounded-reader check reports a non-failing stale `internal/tasks/cli` entry; no unrelated queue/declaration change was made.
- Fail-before proof: temporarily restored all changed production files from HEAD, ran the new tests, then restored the patch in a finally block. Data-scope, qualified-copy and glob tests fail on the named false selections; the Go metadata test fails for diagnostic, name and aliased-testing sinks. Restored regressions pass. Logs: `fail-before-final-{affected,metadata}-5178.log` and `test-restored-5178.log`.
- Dogfood CEM/seal: NOT_PRODUCED; `make dogfood-change` cannot create the private Git evidence directory (`Operation not permitted`). Independent subagents were forbidden; direct diff/counterexample review was used.
- Main-branch staging/commit: blocked by private-worktree `index.lock` access. Preserve all edits; no push, Tasks call or network write.

Rollback: remove the new `localReads` JSON members and restore the old reader/token rules
as one change. There is no state migration. The broader declarations and unresolved
readers intentionally remain selected until separately justified bounds exist.
