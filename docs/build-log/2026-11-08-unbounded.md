# V1-5180: bounded readers and the remaining unbounded floor

Owner direction: 2026-11-08. Base: `447bdcbd5963683f5f2b4934a45b8f48fc6742a3`.
Contract: AFP-V0-3360 through AFP-V0-3364 in
`docs/specs/affected-plan-selection-v0.md`. This is scoped lane evidence, not a full gate,
release qualification, or native ticket-completion receipt. No Tasks command, subagent,
network write, driver computed-invocation edit or record-reader implementation edit was used.

## Result and proof boundary

Two fixture-only dashboard test units now use the existing Go read-scope declaration.
Their common blocker was `internal/dashboard/repository.openDirectory`: it opened `/`
for reading while walking to a fixture. It now pins the first component directly, rejects
symlinks/non-directories, compares the opened descriptor with before/after observations,
and retains the descriptor-relative no-follow walk for subsequent components. Explicit
root reads still require permission. This is a general opener correction, not a test-name
or build-log exception.

The repository test package reads its own subtree and `go.mod` for its standard-library-only
process helper. The conformance package additionally reads the secret-screen parity corpus
and builds `cmd/corvint-dashboard-snapshot`; its declaration lists that command's complete
non-standard package closure plus `go.mod` and `go.sum`. The closure regression fails if a
new dependency is not declared. Both packages passed complete tests under Linux Landlock
ABI 4 with Go 1.27.3, as well as native macOS tests. No scope grants the Git object store.

The ledger reporter `script/dogfood-change-runs.sh` only mentions its writer in a header
comment. Its existing foreign-caller entry had nevertheless made it inherit the writer's
execution chain to `heavy.sh`. Moving that one reference to `mentions` removes the false
execution dependency. The reporter still reads only the Git common directory's run ledger;
the real writer retains its runtime dependencies. The reporter remains selected by its
independent computed-make frontier, so this correction changes its witness, not its selection. A regression checks both directions and
the caller/mention completeness gate.

No test read scope was applied to `go run`, no unbounded reason was erased globally, and no
new record-path exclusion was added. Existing Go declarations also drive Landlock confinement:
declaring a test that reads the real `.git` would make its oracle fail or skip. Merely finding
a small working-tree read set is insufficient to make that declaration sound.

## Audit of all 43 baseline witnesses

Witnesses were obtained first from `UNBOUNDED_READER` in `internal/liveverify/affected/select.go`
and `Graph.UnboundedChain`, then checked against source. `graph.go` follows runtime root
locators and declared callers but excludes build-only edges; `golang/unbounded.go` identifies
root lookups and escaping literals; `shell/readscopes.go` refuses a declaration over an
unresolved source chain. A package's test scope does not bound its command at runtime.

The make rows below have the prefix `driver:make:Makefile#`. “Retain” distinguishes what the
source reads from what this lane can prove with the existing declarations; it does not claim
that every retained unit actually reads build-log contents.

| Unit | Actual reads and source evidence | Disposition |
|---|---|---|
| `analyzer-schema-check` | `script/analyzer-schema.sh` and `analyzer-inputs.sh` read the analyzer pins, input directories and Git base; the output path runs a command through `heavy.sh`. | Retain: environment-selected execution is unresolved. |
| `analyzer-schema-test` | `script/analyzer-schema_test.sh` copies the analyzer scripts into a temporary repository and uses stub output/repin/heavy commands. | Retain: the existing scope validator follows the copied script's execution chain to the environment source. |
| `capabilities-check` | `tools/capabilities-check/main.go`, `Collect`, `ScanLiterals`, `Check` and `scanBinaries` read catalog/README, command and internal source, and existence-check documentation links. | Retain: neither the test-only declaration nor Go import paths prove the invoked tool's complete runtime reads. |
| `cem-stable-regen` | `tools/cem-stable-regen.run`, `regenerate`, `loadCapture` and `parseFile`: protocol corpus plus caller-supplied schema/capture/value files. | Retain: input filenames are caller-controlled. |
| `cem-stable-regen-any-of` | Same tool, `--any-of --record`: protocol corpus plus the caller's record and the paths it supplies. | Retain. |
| `cem-stable-regen-check` | `check`, `manifestProblems` and `pointerProblems` in `tools/cem-stable-regen`: protocol manifests, tuples and referenced pointer targets. | Retain: this invocation lacks a proven runtime-scope declaration; the package's test scope cannot stand in for it. |
| `cem-stable-regen-replace` | Same tool, `--replace --record` plus optional capture/value files. | Retain: caller-controlled files. |
| `cem-stable-regen-widen` | Same tool, `--widen --record`. | Retain: caller-controlled record. |
| `codex-review-test` | `script/codex-review_test.sh` runs the real review wrapper against a temporary Git fixture with stub Codex/heavy commands. | Retain: invocation-specific stub proof is not represented by the current shell scope mechanism. |
| `conflict-markers-check` | `tools/conflict-markers-check.tracked` uses unfiltered `git ls-files -z`; `run` reads each tracked file. | True whole-tree reader; retain for every path. |
| `decision-numbers-check` | `script/check-decision-numbers.sh` lists `docs/decisions`; Make also runs prerequisite `analyzer-schema-check`. | Retain the prerequisite's unresolved execution closure. |
| `declaration-compat-check` | `tools/declaration-compat-check.tracked`, `check`, `atPin`: tracked `.corvint` declarations, CI workflow pins and the corresponding immutable Git blobs. | Retain: no runtime-command scope; `.git` is not granted by the test declaration. |
| `dogfood-bin-reuse-test` | `script/dogfood-bin-reuse_test.sh` copies wrapper/helper scripts via `dogfood-fixture-scripts.sh` and runs them in a fixture with stub Go/Core executables. | Retain: current shell scope admission follows the copied wrappers to `heavy.sh`. |
| `dogfood-change` | `script/dogfood-change.sh` runs the change workflow, analyzer check, builds and repository context/index operations. | Retain: repository-wide evidence plus unresolved runtime inputs. |
| `dogfood-change-runs` | `script/dogfood-change-runs.sh` obtains `--git-common-dir` and reads `corvint/dogfood-change-runs-all.tsv` with `awk`; its only writer reference is a comment. | Bounded non-executing mention; corrected the existing declaration. |
| `dogfood-prechange` | `script/dogfood-prechange.sh` and the Core pre-change workflow collect repository context and immutable change evidence. | Retain: whole-repository evidence and unresolved execution. |
| `go-test` | `tools/test-commands-run` executes the repository's declared test commands, including whole-repository readers. | Retain: the union is not a build-only read set. |
| `go-version-pins-check` | `TestASDV0880GoVersionPinsAgree` in `tools/capabilities-check`: `go.mod`, `AGENTS.md`, `README.md`, `Makefile`, `.github/workflows/*.yml`; Make wraps the tests with `heavy.sh`. | Retain the unresolved wrapper; the check itself is a bounded file set. |
| `import-direction-check` | `internal/tasks/boundary_test.go: TestImportDirection` walks admitted repository directories and parses `.go` imports; Make uses the heavy wrapper. | Retain the unresolved wrapper; non-Go build-log contents are not parsed by the check. |
| `mcp-tools-list-goldens` | Make runs selected `internal/mcp/cli` fixture tests and reads status of their golden directory. The CLI package has runtime-rooted execution. | Retain: the planner does not infer the selected test functions' entire runtime closure. |
| `repin` | `script/repin.sh` runs analyzer repinning and `check-line-citations.sh --repin`, which reads tracked citation documents and referenced source paths. | Retain: tracked-document/reference enumeration plus the analyzer execution closure. |
| `selection-contract-check` | `script/selection-contract-check.sh` builds the baseline tool and both selectors, decides applicability, and replays the pinned corpus; Make uses `heavy.sh`. | Retain the unresolved wrapper and repository applicability analysis. |
| `shell-foreign-callers-check` | `TestRepositoryShellForeignCallersComplete_TSP_V0_005` and `shell.foreignCallersComplete` read the caller declaration and source/Make/YAML candidate files; Make uses `heavy.sh`. | Retain the unresolved wrapper; the scanner's own candidate file set is bounded by type. |
| `shell-test-registration-check` | `tools/test-commands-check` registration tests read the Makefile/command declaration and glob `script/*_test.sh`; Make uses `heavy.sh`. | Retain the unresolved wrapper. |
| `spec-requirements` | `script/gen-spec-requirements.sh` reads staged `docs/specs` inputs, frozen status/capacity inventories and helper source. The observed driver edge still reaches `heavy.sh`. | Retain that edge; changing driver invocation/certification belongs to the other lane. |
| `spec-requirements-check` | Same generator plus README/worktree checks and `internal/specindex`/`internal/lrfrepo` tests. | Retain the heavy execution closure. |
| `tasks-build` | Make's `go build` compiles Go inputs, module files and embeds, and obtains VCS build metadata. It produces an executable. | Retain: current AFP-V0-2951 deliberately propagates the produced binary's possible use by dependent steps; proving/discarding that edge needs invocation/output analysis, not a test scope. |
| `tasks-test` | Make tests `cmd/corvint-tasks/...`, `internal/tasks/...` and vets them. Those tests include real repository readers, unlike vet alone. | Retain: command/test runtime reads remain unresolved; record-entry readers are owned by the other lane. |
| `test-commands-check` | `tools/test-commands-check` reads the declaration and Makefile/includes and runs make dry-runs (`runMake`); parse-time Make commands still execute during a dry-run. | Retain: a test scope is not a bound on arbitrary Make evaluation. |
| `unbounded-readers-check` | `tools/unbounded-readers.run` builds the repository graph and checks declarations, the recorded floor and record-directory readers. | Whole-repository inventory; retain. |

Go unit prefix: `go:github.com/Beamfall/corvint/`.

| Unit | Actual reads and source evidence | Disposition |
|---|---|---|
| `conformance/dashboard-snapshot-v0` | `trace_corpus_test.go` reads secret-screen parity data; the `trace_producer*` and `trace_v2_test.go` tests build the producer and run it over temporary fixtures. | Declared build closure plus parity data; complete confined run passed. |
| `internal/analyzerhtmlcss` | `TestTypedFactOrderingWorstCaseRatchet` reads the fixed parent commit's `internal/analyzerhtmlcss/analyzer.go`; `causalParentRepository` searches the real object database and ancestor checkpoint repositories. | Retain: a read scope cannot grant `.git`; hiding the access would skip the historical oracle. |
| `internal/dashboard/repository` | `authority_integration_test.go`, trace and process tests construct temporary repositories/files; `process_unix_test.go` builds its own standard-library-only helper. | Own subtree plus `go.mod`; root opener fixed and complete confined run passed. |
| `internal/doccorpus/selfcorpus` | `self_test.go` calls `doccorpus.Inventory`; `inventoryScopes` in `internal/doccorpus/compile.go` builds the entire immutable index before filtering the requested source. | True whole-tree inventory; retain. |
| `internal/postmergeworkflow` | Fixture tests, builds of the workflow command, real `HEAD`/tree bindings in `native_test.go`, and a shared clone of the real object store in `historical_test.go`. Linux traversal already uses `O_PATH`; the old recorded root-read reason alone is no longer a complete diagnosis. | Retain: actual Git metadata/object reads remain outside the read-scope grant. |
| `tools/test-exec-writes-check` | `run` enumerates tracked `*_test.go` and reads those sources; `TestRepository` invokes it on this checkout. | Filtered file-set reader, retained because its actual `.git` read is not confineable by the existing scope. |
| `tools/test-read-scopes-check` | `tracked` lists all tracked names; the real-repository parallel test checks declarations/literal existence and lists build/import closures. New tracked names can change which literals count as real reads. | True tracked-tree membership reader; retain. |

Shell unit prefix: `shell:`. These four are fixture-oriented, but the current declaration
cannot safely discharge their invocation-specific execution chains merely from directory names.

| Unit | Source and retained chain |
|---|---|
| `script/analyzer-schema_test.sh` | Copies analyzer scripts and stubs output/repin/heavy behavior; retained `heavy.sh -> analyzer-schema.sh -> analyzer-schema_test.sh`. |
| `script/codex-review_test.sh` | Runs `codex-review.sh` with Codex/heavy stubs; retained `heavy.sh -> codex-review.sh -> codex-review_test.sh`. |
| `script/dogfood-bin-reuse_test.sh` | Copies dogfood wrappers/helpers, uses a stub Go toolchain; retained wrapper chain to `heavy.sh`. |
| `script/lane-host-tools_test.sh` | Builds temporary repositories, HOME and SSH/tool stubs; exercises host/check/delegation wrappers; retained `heavy.sh -> lane-host-heavy.sh -> lane-host-check.sh -> lane-host-tools_test.sh`. |

Playwright unit prefix: `typescript:playwright:`.

| Unit | Source and retained runtime edge |
|---|---|
| `conformance/interactive-alpha/fixture/e2e.spec.js` | The tiny browser assertion is not the complete process: `.corvint/runtime-edges.json` binds it to the console and dashboard producer used by the installed interactive-alpha proof. `internal/console` reads repository state. Retain. |
| `tools/web-flows/test/fixture/flows.spec.ts` | `tools/web-flows/test/e2e.mjs` copies the fixture and runs it through the Core observer; the declared runtime edge reaches `cmd/corvint-web-flows` and `internal/jstestprovider` repository attestation. Retain; no TypeScript test read-scope contract is present. |

The source-confirmed whole-tree/membership set includes `conflict-markers-check`,
`internal/doccorpus/selfcorpus`, `tools/test-read-scopes-check`, `unbounded-readers-check`,
the repository-context part of `dogfood-change`/`dogfood-prechange`, and the union run by
`go-test`. `repin` can read any tracked citation target. This does not label the filtered Go
source checkers as readers of every Markdown file.

## Measurements and verification

Baseline probe: a copy of an existing CEM at
`.corvint/changes/5180518051805180518051805180518051805180.cem.json` and
`docs/build-log/2026-11-09-probe.md`, committed together in a detached scratch worktree.
The prebuilt `corvint-main` identifies the exact base above. `corvint-unbounded` was built from
the candidate working tree with Go 1.27.3; its version correctly reports `modified` because
it was built before the implementation commit. Candidate measurement and final check
results are recorded below after the probe. No claim of a clean branch-bound receipt is made.

Baseline `summary.line`:
`scope=UNKNOWN dirty=2 selected=240 excluded=659 unknown=5 LANGUAGE_FRONTIER=5`.

Computed reasons, measured with `driverUnitsSource(FSSource(os.DirFS(repoRoot)), reasons)`:
173 sites in both trees; counts are per reason and therefore overlap. The temporary measurement
test was removed.

| Reason | Count |
|---|---:|
| `shell:assignment` | 98 |
| `make:evaluated-uncertified` | 70 |
| `shell:local-referenced` | 46 |
| `shell:builtin-set` | 20 |
| `shell:line-continuation` | 19 |
| `shell:unsupported-syntax` | 16 |
| `shell:local-command-word` | 12 |
| `shell:loop-variable` | 12 |
| `shell:computed-command-word` | 7 |
| `make:recipe-ambiguous` | 7 |
| `shell:interpreter-inline` | 5 |
| `shell:builtin-export`, `shell:heredoc-body` | 4 each |
| `shell:heredoc-substitution`, `go:list-unproven-output`, `shell:builtin-read` | 2 each |
| `shell:arithmetic-substitution`, `shell:builtin-shopt`, `shell:github-env-file`, `workflow:github-env-file` | 1 each |

The fail-before proof temporarily restored both original read declarations and the original
opener, ran `TestUnboundedDashboardScopes_AFPV03360`, and restored the candidate in a `finally`
block. Exit 1 named exactly the two dashboard units selected as `UNBOUNDED_READER` for the
build-log addition. The passing path retains their real source/fixture/build dependencies.

Local Linux qualification used the official `go1.27.3.linux-arm64.tar.gz`, size 67,076,044,
SHA-256 `a06e99309dbcbdca1a8ccd2282c3ee146e55930886e26b4f1193269587165f49`, checked against
the Go release manifest before execution. Source and toolchain were streamed into an ephemeral
local Colima container with networking disabled; `test-confine -probe` reported ABI 4. The
container was removed. Initial harness setup failures (a missing test-only source import and
an inherited scratch index in nested Git fixtures) were corrected before the passing runs;
neither is counted as product evidence.

The installed Core is `1.0.0-rc.3.dev`, build 137, at the base. The read-only updater check
could not prove process-group cleanup under this sandbox, so current release freshness is
`UNKNOWN`. Tasks was not queried, by owner direction. Both initial and final `make dogfood-change` attempts were refused
when creating the lane's Git evidence directory. Initial staging was refused on `index.lock`;
a later retry succeeded. Early validation used a separate scratch index; fixture-spawning
test subprocesses explicitly did not inherit it. Final affected/driver tests use the real index. Independent review is `NOT_RUN`: the owner prohibited subagents. These
limitations do not weaken the remaining unbounded-reader selections.

## Final candidate results

Candidate snapshot: `64fd15eb0`; two-record probe: `944cb98d1`. The snapshot differs from
the delivered code only in this result narrative. Both measurements use the same record paths.

Candidate `summary.line`: `scope=UNKNOWN dirty=2 selected=238 excluded=661 unknown=5 LANGUAGE_FRONTIER=5`.

| Unit prefix | Witness kind | Baseline | Candidate |
|---|---|---:|---:|
| `driver` | `DEPENDENCY_PATH` | 83 | 83 |
| `driver` | `FRONTIER_SITE` | 80 | 81 |
| `driver` | `UNBOUNDED_READER` | 30 | 29 |
| `go` | `BINARY_EXEC` | 5 | 5 |
| `go` | `DECLARED_READ_SCOPE` | 2 | 2 |
| `go` | `DEPENDENCY_PATH` | 2 | 2 |
| `go` | `PATH_LITERAL_READER` | 9 | 9 |
| `go` | `UNBOUNDED_READER` | 7 | 5 |
| `shell` | `DEPENDENCY_PATH` | 9 | 9 |
| `shell` | `PATH_LITERAL_READER` | 7 | 7 |
| `shell` | `UNBOUNDED_READER` | 4 | 4 |
| `typescript` | `UNBOUNDED_READER` | 2 | 2 |

Selected units fall from 240 to 238; `UNBOUNDED_READER` witnesses fall from 43 to 40.
The ledger reporter changes from `UNBOUNDED_READER` to `FRONTIER_SITE`; the two dashboard
units are excluded. All five existing language frontiers remain. The computed histogram above
is unchanged because this lane changes no driver inference.

Validation (Go 1.27.3, one package per command):

- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'UnboundedDashboard|UnboundedLedger|RepositoryBuiltReader|RepositoryShellForeignCallersComplete|Driver' ./internal/liveverify/affected/`.
- PASS: complete native tests of `internal/dashboard/repository` and
  `conformance/dashboard-snapshot-v0`; both complete confined Linux runs described above.
- PASS: `go vet` for `internal/liveverify/affected`, `internal/dashboard/repository` and
  `conformance/dashboard-snapshot-v0`; Windows vet for the two changed Go packages.
- PASS: `tools/unbounded-readers` tests, including rejection of an undeclared new reader and
  refusal to hide an unresolved source with a shell read scope.
- PASS: `make spec-requirements`, `make spec-requirements-check`, `make changed-checks`,
  `test-read-scopes-check`, `unbounded-readers-check` and `git diff --check`.
  The ratchet retains its pre-existing advisory that `internal/tasks/cli` is listed but not
  detected as unbounded. That entry is not narrowed by this lane.
- FAIL as expected before the fix: `TestUnboundedDashboardScopes_AFPV03360`, with exactly
  both dashboard units selected as `UNBOUNDED_READER` for the build-log path.

The first added caller explanation made the declaration 65,810 bytes, above the unchanged
65,536-byte bound. The validator correctly refused it. Shortening only the touched explanation
keeps the complete declaration at 65,511 bytes; its completeness test passes. A second test
expectation incorrectly assumed the reporter had no independent frontier; it now checks the
removed unbounded chain and preserves the separate computed-make selection.

`make gate`, independent review, CEM bind/check/seal and native ticket completion are
`NOT_RUN`/`NOT_PRODUCED`: the scoped owner request excludes the full gate, subagents and Tasks,
and the Git evidence directory remains unwritable. No push or network write was attempted.
