# Design: remove the source that 1.0 no longer ships

Change name: `source-removal`. Read with `proposal.md`, `tasks.md`, `specs/source-removal/spec.md` and `path-verdicts.md` in this folder. Intent status: proposed (only the owner can accept it).

## Owner decisions applied (2026-11-09)

Each line names the owner decision, recorded as "owner decision Rn (2026-11-09)", and where it is
applied in this change.

- R14: the provider binaries `corvint-go-test-provider`, `corvint-js-test-provider`,
  `corvint-ruby-test-provider` are retired after G23 lands, each folded into `corvint`, and not shipped as separate 1.0 artifacts. The fold belongs
  to the G23 runner-contract change; no batch here removes them (`proposal.md` Capabilities;
  `path-verdicts.md` rows).
- R28/R29: `corvint context` ships redesigned and non-blocking, with no claim of measured benefit;
  the measurement runs in 1.0.x. B7 keeps what the redesigned context reuses and does not wait for a
  measurement (RMV-V1-023). The companion decision states the invariant 4 wording that follows.
- R32: a change no runner owns, or a removed or unavailable runner, runs all tests with a named
  reason; `UNOWNED_DIRTY_PATH` comes off the allowlist. This settles the RMV-V1-014 mechanism as
  (b) and closes the open item on a removed runner below.
- R33: the VS Code extension moves to the new public repository `beamfall/corvint-vscode`
  (RMV-V1-012, B4). Orphaned code is deleted and tracked in a table (RMV-V1-031); `internal/docviews`
  joins B1.
- R34: one `docs/build-log/YYYY-MM-DD-<slug>.md` entry per removal batch holds its verdicts, orphan
  table and proof (RMV-V1-003, RMV-V1-030, RMV-V1-031); the companion decision stays short.
- R36: `make windows-vet` runs once per removal batch, before that batch's push; this exception
  covers removal lanes only (RMV-V1-003 item 2).
- Owner plan line: removals finish by 2026-12-01 (RMV-V1-001), except B8b and B11, whose gates fall
  later.
- Decided 2026-11-09 under R42 (final decision round):
  - D11: four CI paths are kept for 1.0, each until a named condition ends (RMV-V1-035);
    `greptile-gate-v0.md` stays an active spec and B8b does not archive it.
  - D14 and D15: B11's `.github`-only PR deletes four workflow files, trims
    `ci-control-plane.yml` (two jobs deleted, status still required) and leaves
    `greptile-gate.yml` untouched (RMV-V1-028). The live ruleset is NOT_VERIFIED.
  - E1: the rule for frozen corpora and retained evidence, and the settled rows (RMV-V1-036).
  - E5: B8b lands by 2026-12-10 or moves to 1.0.x, and removes only files with no reader in
    product code, CI or a gate target (RMV-V1-001).
  - E6: self-use is a progress note on the change's ticket; the `docs/DOGFOOD.md` stub stays
    until re-decided after B6 (RMV-V1-019).
  - E9: the lane group moves to a private operator repository in B8b; `heavy.sh` stays for 1.0
    as tooling that is not shipped (RMV-V1-025).
  - M6: `conformance/use-cases-v0` leaves in B8 with its two lints (RMV-V1-019).
  - N11: the test milestones after B6b and B8a are accepted; between milestones a batch runs
    compile and vet only.
  - Plan-text finding: owner decision R14 retires `cmd/corvint-test-runner` after G23; the
    `path-verdicts.md` row says so and no row for it is `undetermined`.

## Failure modes

- A removal deletes a package that a kept package or test imports. The audit was wrong in this way
  once already (`corvint affected` calls the language providers through
  `internal/langplugin/langplugin.go:20-36`). Two more importers the audit missed were found while
  writing this change: `internal/appflows/playwright_attribution_test.go` imports
  `internal/playwrightruns`, and `internal/cem/gitauth/diff.go` imports `internal/cem/patch`.
  Guard: RMV-V1-004 and RMV-V1-005.
- A batch deletes or adds a package without editing `tools/import-ratchet/areas.json` or
  `baseline.json`, and the ratchet test fails with STALE_AREA_PREFIX, UNMAPPED_PACKAGE or
  STALE_BASELINE_EDGE after the batch lands. Guard: RMV-V1-003 item 5.
- The CEM stable-result gate is left with a dangling `internal/cem/verify` import after the verbs
  go. Guard: RMV-V1-019 and RMV-V1-020 (verify, sim and the gate chain leave together in B6b).
- A removed language provider makes `corvint test` silently skip tests for that language instead
  of running them. This is the present code path (see "Open item: a repository that declares a
  removed runner"). Guard: RMV-V1-014, with owner decision R32 (2026-11-09).
- Code orphaned by a batch is left in place and the next batch's importer check misses it. Guard:
  RMV-V1-031 (delete in the same batch, list in the batch's orphan table).
- A path that no batch lists is removed, or kept, without a recorded reason. Guard: RMV-V1-031
  (`path-verdicts.md`; an `undetermined` row is never removed).
- `extensions/vscode` is deleted before its frozen copy exists in `beamfall/corvint-vscode`.
  Guard: RMV-V1-012.
- A `.github` change lands with a code change and the narrowing or split checks run FULL or refuse
  the range. Guard: RMV-V1-002.
- A required status check names a job that a `.github` batch deletes, and every PR blocks waiting
  for it. Guard: RMV-V1-002 (check the ruleset list first).
- A history path (build-log, decisions, retired specs, `benchmarks/results`, `.agent-evidence`,
  `.corvint/changes`) keeps naming a deleted path and the zero-hit check cannot pass. Guard:
  RMV-V1-003 defines the history exclusion list.
- A G25 revival needs code that a batch removed. Guard: RMV-V1-023 holds those paths until the
  revival proposals decide.
- Lane tooling is removed while lanes still build with it. Guard: RMV-V1-025 waits for the layering
  merge (about 2026-12-08).

## Batches

Each batch lists: paths removed; files edited to keep the build compiling; tests rewritten first;
proof (always RMV-V1-003 plus the extra checks named); rollback (always revert the batch merge
commit, restoring from its parent, plus the named recovery commits).

### B0: `.github`-only repair of dangling references (RMV-V1-002, RMV-V1-006)

Gate: lands right after the cut PR.

Paths removed or edited, all under `.github/`:
- `workflows/ci.yml:282`, a comment naming `tools/qualification`, which does not exist and never
  appears in this history (no add or delete commit found; VERIFIED absent).
- `workflows/ci.yml:2266-2375`, the `local-authority-deps` job naming `tools/local-authority`,
  deleted in 389f59ba8 (VERIFIED absent).
- `workflows/go-macos.yml:57` (comment) and `:285-287`, the same two directories.
- `workflows/release-gates.yml:90`, `make companion-release-gate`; no such target exists, only
  `cem-release-gate` at `Makefile:307` (VERIFIED). Remove the step, because G8 removes the CEM
  release gate.
- `workflows/intake-containment.yml:44`, `go test ./internal/intake ./cmd/corvint-intake`; both
  deleted in 408ea405b (VERIFIED). Remove the workflow.
- `workflows/spec-toolchain-conformance.yml`, which runs `script/spec-toolchain-conformance.sh`;
  that script calls `corvint ocm prepare` at `:14,200,238,262,288`, and the OCM commands were removed
  in 389f59ba8 (`cmd/corvint/ocm*.go`). Remove the workflow here; the script goes in B1.
- `workflows/ci.yml:2927` onward, the `cce-live` job; `make cce-live` (`Makefile:1470`) now only
  echoes that it was retired with decision 0593 (VERIFIED). Remove the job.
- `workflows/cem-release-gate.yml` (G8). Remove.
- `workflows/vscode-extension.yml` (G22). Remove.
- `cishards/package-costs.json`, `cishards/walltime_test.go` and other `cishards` entries for paths
  B1 to B6 delete. Whether a stale entry for a missing package fails a CI step is NOT_VERIFIED; if
  it does not, these entries may also be removed in a later `.github` batch after the code goes.

Edits to keep the build compiling: `.github/cishards/*_test.go` that list removed jobs or binaries.
Tests rewritten first: `.github/cishards/walltime_test.go` analyzer rows (NOT_VERIFIED which rows).
Proof: RMV-V1-003 items 3 and 4 for the removed workflow names; `actionlint` if the repository
runs it (NOT_VERIFIED); the ruleset's required-check list read before merge.
Rollback: revert; workflow files are restored from the parent commit.

### B1: dead leaves and dangling scripts (RMV-V1-007)

Gate: after B0.

Paths removed (Go import check VERIFIED for each):
- `internal/breakagemap` (no importer).
- `internal/cliconformance` (no importer).
- `internal/unplannedread` and `internal/scopelease` (no importer).
- `internal/worklistadapter`, `internal/worksource`, `internal/workqueue`,
  `conformance/work-queue-v0`, `docs/worklist.json`, `.corvint/work-queue-policy.json` (chain:
  `worksource` is imported only by `worklistadapter`; `workqueue` only by `work-queue-v0`,
  `worklistadapter` and `worksource`).
- `internal/tracerecordrepo`, `internal/trace`, `internal/tracerepopaths` (`trace` and
  `tracerepopaths` are imported only by `tracerecordrepo`).
- `protocol/step`, `protocol/delta` (no reference outside themselves and docs history).
- `benchmarks/selfuse-batch`.
- `script/spec-toolchain-conformance.sh` (calls removed `corvint ocm`).
- `internal/docviews`: no importer (VERIFIED, import grep in integ 9ec3a29d0); orphaned code
  removed under owner decision R33 (2026-11-09), which amends decision 0594 item 3 through the
  companion decision.

Files edited: `tools/import-ratchet/areas.json` (remove the `shared` prefixes `internal/cliconformance`,
`scopelease`, `trace`, `tracerecordrepo`, `tracerepopaths`, `unplannedread`, `worklistadapter`,
`workqueue`, `worksource`, the `tests` prefix `internal/breakagemap`, and the `docs` prefix
`internal/docviews`; VERIFIED present at `9ec3a29d0`) and `baseline.json` if an edge names them (none does at `9ec3a29d0`);
`tools/ci-forward-row/testdata/{shadow,selection}` fixtures; `.corvint/test-read-scopes.json` and
the other `.corvint/*.json` files that name these paths; `tools/declaration-compat-check` (names
`.corvint/work-queue-policy.json`); `conformance/use-cases-v0/receipts/UC-TASK-ORIENTATION`
(names `docs/worklist.json`); `benchmarks/taskcontext-lookup` and `benchmarks/daily-loop-v0/runs`
(name `selfuse-batch`; NOT_VERIFIED whether these are pinned by preregistration digests, in which
case they are history and join the exclusion list).
Tests rewritten first: none found by the import check.
Also edited for `internal/docviews`: the `tools/affected-edit-oracle/testdata/corpus.json` and
`tools/ci-forward-row/testdata/outcomes/shard-3.json` fixtures that name it (integ 9ec3a29d0).
Not in B1: `internal/testrunner/golang`,
`internal/appflows/e2erunner`, `internal/liveverify/jsresolve`, `internal/liveverify/pyresolve`
(RMV-V1-011).
Rollback: revert; recovery for the OCM command the script needed is 389f59ba8^.

### B2: test-quality cut (RMV-V1-008, RMV-V1-009; G24)

Gate: after B1.

Paths removed:
- `cmd/corvint/test_plan.go`, `test_plan_pagestate.go`, `test_plan_test.go`, `internal/testplan`
  (`testplan` is imported only by those three files; VERIFIED).
- `cmd/corvint/runs*.go` (the six `runs` files, including tests), `internal/playwrightruns`.
  `playwrightruns` is imported by `cmd/corvint/runs*.go`, `cmd/corvint/tests.go` and
  `internal/appflows/playwright_attribution_test.go` (VERIFIED; the last one is a correction to the
  audit).
- `cmd/corvint/test_validity*.go`, the `lint`, `duplicates` and `validity` cases in
  `cmd/corvint/tests.go:318-323` and their files (`tests_duplicates_decisions_test.go` and the lint
  helpers; exact file list NOT_VERIFIED).
- `cmd/corvint-test-validity-mcp`, `internal/mcp/testvaliditybridge` (imported by
  `cmd/corvint-test-validity-mcp`, `internal/mcp/cli/groups.go`, `internal/mcp/cli/unavailable.go`
  and `tools/capabilities-check/machine.go`; VERIFIED).
- `internal/testsource/validity*.go` (46 files; VERIFIED count).
- `internal/tcq` code used only inside `tcq`. `tcq` is imported by `internal/appflows/query.go`,
  `runevidence.go`, `internal/jstestprovider/freshness.go`, `playwright.go`, `projection.go`,
  `internal/rubytestprovider/document.go` and `internal/testvalidity/projection.go` (VERIFIED);
  the identifiers they use stay. Which identifiers are unused is NOT_VERIFIED.

Files edited: `cmd/corvint/main.go` (dispatch), `cmd/corvint/tests.go`,
`internal/mcp/cli/groups.go`, `internal/mcp/cli/unavailable.go`,
`tools/capabilities-check/machine.go`, `tools/release-binary/binaries.json:20`,
`docs/CAPABILITIES.md`, help text, `internal/testvaliditydoc/read.go` moved to
`internal/pinnedread` (which already exists) with its callers;
`tools/import-ratchet/areas.json` (remove the `tests` prefixes `internal/testplan`,
`internal/playwrightruns`, `internal/mcp/testvaliditybridge` and the `cmd/corvint-test-validity-mcp`
prefix; VERIFIED present at `9ec3a29d0`); `tools/import-ratchet/baseline.json` (its edges
`internal/appmap -> internal/testvaliditydoc` and `internal/doccorpus -> internal/testvaliditydoc`
change if the `read.go` move changes those imports); `tools/import-ratchet/flows_layer_test.go`
(drop `flowsTestplan` from the keyed map and the reader list; at `9ec3a29d0` the constant is on
line 20 and the reader loop on lines 64 to 68); the CMS-V0-915 trace row in
`docs/specs/core-bundled-apps-v0.md` (line 170 at `9ec3a29d0`, which names `internal/testplan` as a
reader). `cmd/corvint` imports `internal/appflows` and `internal/appmap`, and
`internal/testsource` imports `internal/appmap`, at `9ec3a29d0` (VERIFIED), so both remaining
readers pass.
Tests rewritten first: `internal/appflows/playwright_attribution_test.go`; `cmd/corvint/tests_test.go`
and `tests_help_test.go` where they cover removed subcommands; `tools/capabilities-check` tests.
Extra proof: provider output byte-identical on the providers' existing golden tests.
Rollback: revert.

### B3: runner protocol (RMV-V1-010, RMV-V1-011; G23)

Gate: after B2.

Paths removed: `internal/runnerprotocol` (imported only by `tools/runner-plugin` and
`tools/runner-reference`; VERIFIED), `internal/runnerpolicy` (no importer; VERIFIED),
`internal/runnerqualification` (imported only by `runnerpolicy` and `tools/runner-qualification`;
VERIFIED), `tools/runner-plugin`, `tools/runner-qualification`, `tools/runner-reference` (no Go
importer; VERIFIED).
Files edited: none in `internal/testrunner/native` (`runnerprotocol/reference.go` imports
`native`, not the reverse; VERIFIED), `internal/testrunner/registry/runner_plugin.go` if it names the protocol
(NOT_VERIFIED), `.corvint/*.json`, `tools/import-ratchet/areas.json` (remove the `tests` prefixes
`internal/runnerpolicy`, `internal/runnerprotocol` and `internal/runnerqualification`; VERIFIED
present at `9ec3a29d0`), the 0592 runner-selection spec (status lines for RSP-V0-025 MODIFIED and RSP-V0-026..032 and RSP-V0-047 REMOVED).
Tests rewritten first: none found by the import check.
Rollback: revert.

### B4: VS Code extension and G22 extras (RMV-V1-012, RMV-V1-013)

Gate: after B3. `extensions/vscode` waits until its frozen, unpublished copy has been pushed to the
new public repository `beamfall/corvint-vscode` (owner decision R33 (2026-11-09)); the rest may
land.

Paths removed: `extensions/vscode`, `conformance/vscode-extension-v0`, `tools/lsp-editors`,
`script/measure-lsp-quality.py`, `script/qualify-lsp-editors.py`, `script/qualify-lsp.py`,
`examples/evidence-provider`, `docs/EXTERNAL-EVIDENCE-PROVIDERS.md`,
`protocol/integrations/speckit`, `conformance/cli-parity-v0` (all present; VERIFIED).
Files edited: `Makefile` (remove `vscode-check-install-test` at `:1312`, `host-adapter-test` at
`:1323`, and their `shell-test` and `GATE_STEPS` entries), `internal/gokernel/repository.go:390-391`
(comment cites `conformance/cli-parity-v0/snapshot.go:35`), `.corvint/test-read-scope-literals.json`,
`.corvint/test-read-scopes.json`, `.corvint/test-selection.json`, benchmarks that name
`cli-parity-v0` (NOT_VERIFIED whether pinned history), `docs/EXTENSION-ECOSYSTEM.md` and
`docs/specs/deployment-neutral-index-platform-v0.md` (status lines),
`tools/import-ratchet/areas.json` and `baseline.json` for any removed path they map (none of the
B4 paths is mapped at `9ec3a29d0`; re-checked in the batch). `.github/cishards` entries go in a
`.github` batch.
Tests rewritten first: any Go test reading `conformance/cli-parity-v0` (NOT_VERIFIED).
Rollback: revert; the moved extension also lives in `beamfall/corvint-vscode`.

### B5: languages and analyzers (RMV-V1-014 to RMV-V1-018; G26)

Gate: RMV-V1-014 has landed and its fixture test passes. This is a milestone batch.

Paths removed:
- `cmd/corvint-analyzer-{c-jni,dotnet,go,html-css,java,js,kotlin-android,objective-c,python,ruby,rust,shader,shell,sql-native,structured-data,swift}` (16, present; VERIFIED).
- The 14 `internal/analyzer*` packages. Each has no importer outside the analyzer binaries,
  except `internal/analyzerpython` and `internal/analyzerrust`, which
  `internal/closureguard/candidates_test.go` imports (VERIFIED).
- `internal/analyzerenvelope` (imported only by `analyzerruby` and `analyzershell`; VERIFIED).
- `experimental/analyzers/sqlnative` (imported only by `cmd/corvint-analyzer-sql-native`;
  VERIFIED).
- `internal/liveverify/affected/{cfamily,dotnet,java,kotlin,rust,swift}`. Importers:
  `internal/langplugin/langplugin.go:20-36`, `internal/liveverify/affected/conformance_test.go`,
  and each other (`java` from `kotlin.go`; `swift` from `swiftimpact` and `swfix_test.go`)
  (VERIFIED).
- `internal/testrunner/registry/{csharp_bodies,dotnet_artifacts,dotnet_evidence,dotnet_loaded,dotnet_mutation,dotnet_plugin,jvm_evidence,jvm_inventory,jvm_plugin,rust_evidence,rust_plugin}.go`
  with their tests; `internal/testrunner/swiftselection` (no importer; VERIFIED);
  `internal/testrunner/mobile` and `internal/testrunner/sql` (imported only by `registry.go`;
  VERIFIED).
- `internal/swiftimpact` (importers `cmd/corvint/swift_impact.go`,
  `cmd/corvint-mcp/swift_impact.go`, `cmd/corvint-test-validity-mcp/swift_impact.go` (gone in B2),
  `internal/contextindex/swiftpm_reader_link_test.go`; VERIFIED).
- `internal/csharpregions` (importers `cmd/corvint-test-runner/csharp_regions.go`,
  `registry/csharp_regions_test.go`, and the B3-removed `tools/runner-*`; VERIFIED).
- `internal/nativelinecov` (importers: registry `dotnet_evidence.go`, `jvm_evidence.go`,
  `jvm_plugin_test.go`, `rust_evidence.go`, `swiftselection/swift_evidence.go`; VERIFIED).
- `internal/rustsyntax` (importers `analyzerrust`, `registry/rust_evidence.go`; VERIFIED).
- `internal/rubysyntax` (importers `analyzerruby`, `liveverify/rbmutate`; VERIFIED).
- `internal/shaderdialect` (importer `internal/contextindex/shader_dialect.go`; VERIFIED).
- `cmd/corvint-go-toolchain-receipt`, `internal/toolchainreceipt` (only that binary; VERIFIED).
- `internal/liveverify/pymutate`, `internal/liveverify/rbmutate` (no Go importer; named as string
  paths in `internal/langplugin/decl/decl.go:127-128`; VERIFIED).
- The sql-native ratchet and the analyzer-schema, analyzer-inputs and analyzer ratchet lints
  (paths NOT_VERIFIED; to be listed from the Makefile in the batch).
- The `native` split: the parts of `internal/testrunner/native` used only by `nativelinecov`,
  `runnerprotocol`, `dotnet_plugin.go`, `rust_plugin.go` (VERIFIED importers); go-test stays.

Files edited:
- `internal/langplugin/langplugin.go` (imports at `:20-36`; `Providers()` registrations at
  `:130-226` for cfamily, dotnet, java, kotlin, rust and swift, and every `SubPlugin` `Analyzers`
  name).
- `internal/langplugin/decl/decl.go:122-136` (language paths, including `sql`, `shader`,
  `html-css`, `structured-data`, `pymutate`, `rbmutate`; `liveverify/mutate` at `:124` stays).
- `internal/testrunner/registry/registry.go` (removed runner names; `runnernames.go` lists
  `cargo-test`, `dotnet-*`, `gradle-junit`, `junit-platform`, `maven-surefire`, `nextest`).
- `cmd/corvint/swift_impact.go` and `cmd/corvint-mcp/swift_impact.go` (removed with their
  dispatch), `cmd/corvint-test-runner/csharp_regions.go`.
- `internal/contextindex/shader_dialect.go` and `swiftpm_reader_link_test.go`.
- Analyzer names in `cmd/corvint-cli/main.go:18`, `internal/cliapp/help.go:52,64,87`,
  `internal/cliprofile/profile.go:234`, `internal/releasecandidate/apps_test.go`,
  `internal/testsupport/analyzerbridgecli.go`, `tools/beamfall-shadow/*`,
  `tools/capabilities-check/main_test.go`, `conformance/cli-errors-v0`.
- `tools/release-binary/binaries.json:26-41,43`.
- `tools/import-ratchet/areas.json`: delete the `analyzers` area whole (its 31 prefixes: the 16
  `cmd/corvint-analyzer-*`, `experimental/analyzers` and the 14 `internal/analyzer*` directories;
  VERIFIED at `9ec3a29d0`), leaving four layers: shared (0); tests and docs (1); release (2).
  Rewrite the manifest's rule string, which cites decision 0594 item 8 and names analyzers, to the
  four layers (companion decision item 8). Also remove the `shared` prefixes
  `internal/rubysyntax`, `internal/rustsyntax`, `internal/shaderdialect`, the `tests` prefixes
  `internal/csharpregions`, `internal/nativelinecov`, `internal/swiftimpact`, and the `release`
  prefixes `cmd/corvint-go-toolchain-receipt` and `internal/toolchainreceipt` (VERIFIED present).
  No baseline edge names an analyzer package at `9ec3a29d0`.
- `docs/CAPABILITIES.md:311` and the language specs (status lines).
- `.github/cishards/walltime_test.go` analyzer rows go in a `.github` batch.

Tests rewritten first: `internal/closureguard/candidates_test.go`,
`internal/liveverify/affected/conformance_test.go`, `internal/contextindex/swiftpm_reader_link_test.go`,
`internal/releasecandidate/apps_test.go`, `tools/capabilities-check/main_test.go`.
Extra proof: the RMV-V1-014 fixture test; `corvint affected --base` on a change touching a `.rs`
file reports UNKNOWN; the shell and Go providers' tests pass unchanged (they import no analyzer,
`nativelinecov` or `mutate` package; VERIFIED).
Rollback: revert. The analyzers are not recoverable from an earlier removal commit; the parent of
B5's merge is the recovery point.

### B6: CEM, OCM and dogfood (RMV-V1-019 to RMV-V1-022; G8)

Gate: the companion decision is accepted. B6a (move) lands before B6b (removal).

B6a, move only: `internal/cem/{wire,gitauth,gitrun,cemcode,coverprofile,patch}` to
`internal/{wire,gitauth,gitrun,cemcode,coverprofile,patch}` (RMV-V1-020). Every importer is
rewritten mechanically (60, 47, 43 and 21 files for the first four; VERIFIED counts). Files edited:
`tools/import-ratchet/areas.json` (the six new prefixes in the `shared` area; the `internal/cem`
prefix stays until B6b) and `baseline.json` if an edge names a moved path (none does at
`9ec3a29d0`). Tests rewritten first: `internal/cem/wire/stable_producer_only_test.go`, which
imports `internal/cem/verify` (VERIFIED); it is rewritten without `verify` or removed.

B6b, paths removed: `cmd/corvint/cem*.go` (8 files, of which only `cem_action_help.go` is not a
test; VERIFIED), `internal/cem/{cli,mdreport,publish,sim,verify,witnesspaths,workflow}` (`mdreport`,
`publish` and `witnesspaths` have no outside importer; `sim` is imported only by `verify`; `verify`
outside `internal/cem` only by `internal/corerelease/core_stable_result.go` and
`tools/cem-release-suites/main.go`; `cli` is imported by `cmd/corvint/main.go`; `workflow` by
`cmd/corvint/cem_stable_root_test.go`; VERIFIED). There is no `internal/cem` root package; the only
root-path hit is a prefix string in `cmd/corvint/cem_test.go`, removed with the verbs. Also removed:
`tools/cem-release-suites` (imports `internal/corerelease` and `internal/cem/verify`; VERIFIED),
`internal/corerelease/core_stable_result.go` and its test, the `corerelease` files that exist only
for the CEM stable gate (`core_stable_gate_outcome.go` and `core_stable_readiness.go` name the
`cem-release-gate-outcome/1` and `cem-release-readiness/1` profiles; their remaining callers are
listed under RMV-V1-004 in the batch), and the `releasecandidate` CEM readiness gate (the
`gate/cem-release-gate` row and `cemGate` handling in `internal/releasecandidate/readiness.go`,
with `readiness_cem_gate_test.go`; `candidate.go` and `readiness.go` import `corerelease`;
VERIFIED),
`interop/cem01-go`, `protocol/cem-1.0`, `protocol/cem-completeness-1`, `Makefile:253-307`
(`cem-stable-regen*`, `cem-release-suites`, `cem-release-gate`), `script/install-cem-verifiers.sh`,
`script/dogfood-merge-base.sh`, `script/dogfood-merge-base_test.sh`, `Makefile:1204`
(`dogfood-merge-base-test`), `.corvint/changes`, `.corvint/deferred-intents`, `.agent-evidence`,
`artifacts/cem-release-27e9d98`; `docs/DOGFOOD.md` becomes a retirement stub.

Files edited: `tools/import-ratchet/areas.json` (remove the `internal/cem` prefix);
`internal/releasecandidate/readiness.go` and `candidate.go`; `cmd/corvint/main.go` (dispatch), `Makefile:196-235` (`cross-vet` and `go-test` name
`interop/cem01-go`) and `windows-vet` (about `Makefile:1432`), `internal/testrunner/golang/plugin_test.go:455,505`
(names `interop/cem01-go`), `script/proof-tour.sh` (calls `corvint cem`), `LICENSING.md:21-23`,
`AGENTS.md` per the companion decision, `docs/SELF-DEVELOPMENT.md` and `docs/AGENT-ROUTES.md`
where they route to CEM (NOT_VERIFIED which rows), `docs/CAPABILITIES.md`, the CEM specs (status
lines).
Tests rewritten first: `internal/testrunner/golang/plugin_test.go`;
`internal/releasecandidate` tests that build a candidate with a CEM gate row;
`cmd/corvint/cem_stable_root_test.go` is removed with the verbs; any test reading `.corvint/changes`
or `.agent-evidence` (NOT_VERIFIED).
Extra proof: the RMV-V1-003 item 1 check drops `interop/cem01-go` from this batch on; the history
exclusion list loses `.agent-evidence/` and `.corvint/changes/`.
Milestone: B6b is a milestone batch; the milestone checks run on the head that has both B6a and
B6b. B6a needs none of its own: it only moves packages, its proof is the RMV-V1-003 build and
test run, and B6b follows it directly. (Review removal-arch finding 12; proposed here, not an owner
decision.)
Rollback: revert B6b, then B6a. For CEM verbs removed earlier and restored, recovery commit
b03184699 ("Restore corvint cem create and verify commands"); for OCM, 389f59ba8^.

### B7: G25 "still removed" (RMV-V1-023)

Gate: the five revival proposals are committed and the `watch` measurement is retained. This is a
milestone batch.

Paths removed: listed in RMV-V1-023, minus what the revival proposals reuse. `contextindex` has
129 importer files and `gokernel` 133 (VERIFIED), so this batch removes files inside those packages,
not the packages.
Files edited: NOT_VERIFIED until the revival proposals exist.
Rollback: revert.

### B8: lints and lane tooling (RMV-V1-024, RMV-V1-025; G27)

B8a, now: remove lane and batch steps from `GATE_STEPS`, `doc-gates` and `shell-test`; fold and
remove the three lints; remove the listed lints.
B8b, after the layering merge (about 2026-12-08): remove the lane scripts, merge drivers
(`tools/merge-drivers`), `tools/flow-guide-check`, `tools/requirement-id-collisions`,
`docs/LANE-ORCHESTRATION.md` (stub left), `.claude/skills/corvint-lane-flow`. These paths are
present (VERIFIED); their callers are NOT_VERIFIED. The removed lane group is moved to a private
operator repository, not deleted (E9). B8b lands by 2026-12-10 or moves to 1.0.x, and removes
only files with no reader in product code, CI or a gate target (E5). `heavy.sh` stays for 1.0.
B8 also removes `conformance/use-cases-v0` with its two lints (M6).
Held back from B8b: `script/github-split-check.sh` and its test stay until the B11 cut-over pull
request has merged and leave in B11's script removal pull request (RMV-V1-032). The
requirement-ID scripts under `script/` stay until the last change folder that calls them is
archived (RMV-V1-033); `narrowing-measurement/tasks.md` step 1 calls
`script/requirement-id-reservations.sh reserve`.
Milestone: B8a is a milestone batch, because it edits `GATE_STEPS`, `doc-gates` and `shell-test`,
which are the gate itself; the milestone checks (`corvint affected --base` output and a green CI
run) show the reduced gate still runs. B8b needs none of its own: it removes files that no gate
step names after B8a, and its RMV-V1-003 proof covers it. (Review removal-arch finding 12;
accepted 2026-11-09 under R42, N11, together with the milestone after B6b.)
Rollback: revert.

### B9: spec archive in place (RMV-V1-026)

Runs inside each batch that retires a spec, plus one sweep for the 24 live lane and process specs
the panel counted (count NOT_VERIFIED). No file moves.

### B10: Tasks specs (RMV-V1-027)

Gate: `beamfall/corvint-tasks` PR #4 merged.

### B11: action cut-over (RMV-V1-028)

Gate: 30 shadow PRs matched, by 2026-12-15. A `.github`-only PR first, then the script removal PR.
This is a milestone batch. The `.github`-only PR deletes `pr-tests-qualification.yml`,
`ci-forward-bar.yml`, `ci-route-health.yml` and `flake-report.yml`, trims `ci-control-plane.yml`
(the `main-miss` and `forward-rows` jobs go; the `control-plane` and `merge-group` jobs and the
required status stay) and leaves `greptile-gate.yml`, `merge-queue-cleanup.yml`,
`cache-prune.yml` and `.github/testconfine/` untouched (D11, D14, D15). If the gate is not met,
all of it stays for 1.0 (G18).

## Open item: a repository that declares a removed runner (G26 asked for this check)

Checked in code at `9ec3a29d0`:
- A repository declares runner commands in `.github/cishards/test-commands.json`
  (`internal/sessiontest/declared.go:26-27`). Any plain language name with an argv is accepted
  (`declared.go:242-250`); the name is not checked against the language providers.
- `corvint affected`: a dirty path no provider claims becomes UNKNOWN with reason
  `UNOWNED_DIRTY_PATH` (`internal/liveverify/affected/select.go:61-64`, `:303`) and stays in the
  plan's unknown list. This part fails closed, as G26 says.
- `corvint test`: `internal/gateselect/allowlist.go:29` returns true for `UNOWNED_DIRTY_PATH` (the
  comment at `:21` says it is read only by the Go package rules), and
  `internal/sessiontest/plan.go:329-330` skips allowlisted unknowns, so no FULL is forced.
  The dirty-language set comes only from `language.Owns(dirty)` over the registered providers
  (`cmd/corvint/test.go:719-728`), and declared languages run only under FULL
  (`plan.go:378-383`). After the Rust or Swift provider is removed, a `.rs` or `.swift` change under
  `corvint test` runs only the Go readers the Go rules select and does not run the declared `rust`
  or `swift` command. That is a silent miss and contradicts G26's "removal fails closed".
- `corvint-test-runner` given a removed runner name fails with "unsupported runner"
  (`internal/testrunner/registry/registry.go:92,103`).
- `docs/CAPABILITIES.md:311` says an unowned path widens to UNKNOWN; that holds for `affected`, not
  for the FULL decision in `test`.

RMV-V1-014 blocks B5 on this. Resolved by owner decision R32 (2026-11-09): mechanism (b). A change
no runner owns, or a removed or unavailable runner, runs all tests with a named reason, and
`UNOWNED_DIRTY_PATH` comes off the allowlist. The behaviour is specified by RNC-V1-004 (change
`runner-contract`) and NCM-V1-001 (change `non-code-map`); B5 waits until it has landed and the
RMV-V1-014 fixture test passes. Paths NCM-V1-001 classes as declared documentation still add no
test.

## Other open items

1. RMV-V1-014 mechanism: resolved, (b), by owner decision R32 (2026-11-09).
2. Jest path through `corvint test`: end-to-end check and ticket (G23).
3. `internal/liveverify/mutate`: own ticket (G26, RMV-V1-017).
4. VS Code repository name and visibility (G22): resolved, public `beamfall/corvint-vscode`, by
   owner decision R33 (2026-11-09).
5. `internal/docviews`: resolved, removed in B1 as orphaned code, by owner decision R33
   (2026-11-09); the companion decision amends decision 0594 item 3.
6. Whether `conformance/tcq-0` is CEM-only (RMV-V1-019). `conformance/use-cases-v0` is resolved
   (final round, M6): it is removed in B8 with its two lints, because the use-case receipt
   contract is superseded by the qualification of REL-V1-012.
7. Whether stale `.github/cishards` entries for missing packages fail CI (B0).
8. The `undetermined` rows in `path-verdicts.md` (RMV-V1-031): resolved. Decided 2026-11-09
   under R42, D11: `.github/testconfine/`, `greptile-gate.yml`, `merge-queue-cleanup.yml` and
   `cache-prune.yml` are kept for 1.0 (RMV-V1-035). Decided 2026-11-09 under R42, E1: a frozen
   corpus nothing reads is removed with its last commit recorded, and evidence an active spec
   cites is kept (RMV-V1-036). Which of B6 and B7 takes each corpus is NOT_VERIFIED.
9. The reason name for a removed or unavailable runner differs between changes: `runner-removed`
   in RNC-V1-004 and `runner-unavailable` in NCM-V1-001. This change uses `runner-removed`,
   the name in the option owner decision R32 (2026-11-09) chose; NCM-V1-001 now uses it too. `runner-unavailable` remains only as the pytest and Ruby refusal for a missing interpreter.

## Verification table

| Claim | Status | Evidence |
|---|---|---|
| `tools/local-authority` absent; named in `ci.yml` and `go-macos.yml` | VERIFIED | deleted in 389f59ba8; `ci.yml:2266-2375`, `go-macos.yml:285-287` |
| `tools/qualification` absent; never in history | VERIFIED | `ci.yml:282`, `go-macos.yml:57`; no add/delete commit |
| `companion-release-gate` target missing | VERIFIED | `release-gates.yml:90`; only `Makefile:307` `cem-release-gate` |
| `script/spec-toolchain-conformance.sh` calls removed `corvint ocm` | VERIFIED | `:14,200,238,262,288`; OCM removed in 389f59ba8 |
| `intake-containment.yml` tests deleted packages | VERIFIED | `:44`; deleted in 408ea405b |
| Language providers are called by `corvint affected` | VERIFIED | `internal/langplugin/langplugin.go:20-36,130-226` |
| `playwrightruns` has a test importer in `appflows` | VERIFIED | `internal/appflows/playwright_attribution_test.go` |
| `cem/patch` is needed by `cem/gitauth` | VERIFIED | `internal/cem/gitauth/diff.go` |
| `cem/verify` is used outside `internal/cem` only by the CEM stable-result gate | VERIFIED | `internal/corerelease/core_stable_result.go`, `tools/cem-release-suites/main.go`; `verify` imports `sim` in 5 files |
| no `internal/cem` root package | VERIFIED | only a prefix string in `cmd/corvint/cem_test.go` |
| `releasecandidate` carries a CEM readiness gate row | VERIFIED | `internal/releasecandidate/readiness.go` `gate/cem-release-gate`; `readiness_cem_gate_test.go` |
| `flows_layer_test.go` pins `internal/testplan` as a CMS-V0-915 reader | VERIFIED | `tools/import-ratchet/flows_layer_test.go`; `core-bundled-apps-v0.md` trace row |
| `areas.json` has five layers, the analyzers area holding 31 prefixes; no baseline edge names a removed package | VERIFIED | `tools/import-ratchet/areas.json`, `baseline.json` (19 edges) |
| analyzer packages: only `analyzerpython`, `analyzerrust` imported outside binaries | VERIFIED | `internal/closureguard/candidates_test.go` |
| shell and Go providers import no removed package | VERIFIED | import grep |
| `docviews`, `breakagemap`, `cliconformance`, `unplannedread`, `scopelease`, `runnerpolicy`, `swiftselection`, `pymutate`, `rbmutate` have no importer | VERIFIED | import grep |
| work-queue and trace chains are leaves | VERIFIED | import grep |
| `corvint test` fails closed for removed languages | NOT TRUE today | see open item above |
| G24 file counts (about 60 deleted, 15 edited) | NOT_VERIFIED | panel estimate |
| Unused `tcq` identifiers | NOT_VERIFIED | |
| Snapshot cut size (B7) | NOT_VERIFIED | no `deadcode` run |
| `.corvint/runner-plugins.json` read by any code | NOT_VERIFIED | file absent |
| Callers of lane tooling | NOT_VERIFIED | |

## Rollback

Revert the batch's merge commit (`git revert -m 1 <merge>`); every removed path returns from the
parent commit. Batches are independent after B0 except B6b on B6a and B5 on RMV-V1-014, so revert
in reverse order when several must go. Recovery commits for code removed earlier, all on the
integration history and not yet on `origin/main` until the cut PR lands: 389f59ba8 (process command
families, OCM, `tools/local-authority`), 408ea405b (hosted surfaces, intake), 841a58cbd (host hook
adapters and MCP plugin wiring), b03184699 (CEM create and verify restored). Nothing published or
persisted outside the repository depends on the removed binaries, except the analyzer and
`corvint-test-validity-mcp` binaries in earlier release candidates, which stay downloadable from
those releases.
