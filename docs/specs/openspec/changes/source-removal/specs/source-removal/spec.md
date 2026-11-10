# Spec delta: source-removal

Change name: `source-removal`. Capability: `source-removal` (prefix `RMV-V1`; the owning spec `docs/specs/source-removal-v1.md` is written when this change is accepted). Intent status: proposed.

## ADDED Requirements

### Requirement: RMV-V1-001 Removal order (G27)

The removal SHALL land in this order: the pending cut PR unchanged (its source branch is the
`claude/bigcut-keepdocs` cut, which carries commits 389f59ba8, 408ea405b and 841a58cbd); then
the `.github`-only batch B0; then removal batches B1 to B7 as the first lanes on the integration
branch, before any directory move; then lane tooling (B8); then the spec and Tasks archive batches
(B9, B10); then the action cut-over (B11) after the shadow run. A batch whose gate (named in its
section) is not met SHALL wait; it SHALL NOT be folded into another batch. Batches B0 to B8a, B9
and B10 SHALL finish by 2026-12-01, before the 2026-12-15 freeze (owner plan line, 2026-11-09);
B8b follows the layering merge (about 2026-12-08) and B11 follows the shadow run (by 2026-12-15),
as their gates say.

B8b and B11 are the two named exceptions to the 2026-12-01 date (decided 2026-11-09 under R42,
E5). B8b SHALL land by 2026-12-10 or move to 1.0.x. B8b SHALL remove only files that have no
reader in product code, in CI or in a gate target, and SHALL NOT remove `github-split-check`
(RMV-V1-032). B11 follows G18 and RMV-V1-028.

#### Scenario: B8b misses its date
- GIVEN 2026-12-10 has passed and B8b has not landed
- WHEN B8b is proposed
- THEN it becomes a 1.0.x change and the lane files stay in the 1.0 tree

#### Scenario: a batch before its gate
- GIVEN B5 (languages) is ready but RMV-V1-014 has not landed
- WHEN the batch is proposed for merge
- THEN it is refused and stays on its branch

### Requirement: RMV-V1-002 `.github` changes are separate (G22, G27)

Every change under `.github/` SHALL land in a `.github`-only batch, and no other batch SHALL touch
`.github/`. Before a `.github` batch that deletes or renames a job lands, the repository ruleset's
required-check list SHALL be read and any listed job removed from it in the same window, by the
owner if the ruleset needs owner rights.

#### Scenario: required check names a deleted job
- GIVEN the ruleset requires `cem-release-gate (ubuntu-24.04)`
- WHEN the batch deleting `cem-release-gate.yml` is prepared
- THEN the ruleset change is named in the batch and the batch does not merge before it

### Requirement: RMV-V1-003 Per-batch proof (G27)

Each batch SHALL record, on its exact head, in that batch's one
`docs/build-log/YYYY-MM-DD-<slug>.md` entry (owner decision R34 (2026-11-09); no numbered decision
per batch):
1. `GOTOOLCHAIN=local go vet ./...` in the root module, and in `interop/cem01-go` until B6 removes it;
2. `make windows-vet` once per batch, before that batch's push, when the batch touches a `.go`,
   `go.mod` or `go.sum` file (owner decision R36 (2026-11-09); this exception to the every-push
   rule covers removal lanes only);
3. `make -n gate doc-gates shell-test release-binaries-check` exiting 0 (targets at
   `Makefile:135`, `:1199`, `:1363`, `:754`), which proves no make prerequisite names a removed
   target or file;
4. `git grep -n -F <path>` returning zero hits for each deleted path, outside this history
   exclusion list: `docs/build-log/`, `docs/decisions/`, spec files carrying a retirement status
   line, `benchmarks/results/`, `docs/specs/openspec/changes/` (active and archived change folders,
   which are the removal's own plan and record), `.agent-evidence/` and `.corvint/changes/` (the
   last two only until B6 removes them);
5. `GOTOOLCHAIN=local go test -count=1 ./tools/import-ratchet/...` passing, with
   `tools/import-ratchet/areas.json` and `baseline.json` edited in the same batch: every prefix
   naming a deleted package is removed (the ratchet reports STALE_AREA_PREFIX otherwise), every
   new package path gets a prefix (UNMAPPED_PACKAGE otherwise), and every baseline edge naming a
   deleted package is removed (STALE_BASELINE_EDGE otherwise). The ratchet runs only as a Go test,
   so items 1 to 4 do not catch these; it takes seconds.
Milestones (after B0, after B5, after B7, after B11) SHALL also record `corvint affected --base
<batch base>` and a green CI run. A batch whose proof is missing is not done.

#### Scenario: a stale layer prefix
- GIVEN B3 deletes `internal/runnerprotocol`, which `areas.json` maps to the `tests` area
- WHEN the batch runs `go test ./tools/import-ratchet/...` without editing `areas.json`
- THEN the test fails with STALE_AREA_PREFIX and the batch is not done

#### Scenario: a remaining reference
- GIVEN B3 deletes `tools/runner-plugin`
- WHEN `git grep -F tools/runner-plugin` finds `.corvint/test-read-scopes.json`
- THEN the batch is not done until that file is edited

### Requirement: RMV-V1-004 Importer claims are re-checked on the batch base

Each batch SHALL re-run the import-path `git grep` for every package it deletes, on its own base,
and SHALL list every importer found. An importer not in this proposal SHALL be either rewritten in
the batch (with the reason recorded) or SHALL stop the batch. A claim marked NOT_VERIFIED in this
proposal SHALL be checked before the batch starts.

#### Scenario: a new importer appears
- GIVEN a lane merged after this proposal added an import of `internal/tcq` code G24 removes
- WHEN the batch re-checks importers
- THEN the importer is listed and the batch stops or rewrites it

### Requirement: RMV-V1-005 Tests are rewritten before deletion

Tests in kept packages that import a removed package SHALL be rewritten or removed in a commit that
precedes the deletion commit in the same batch, and that commit SHALL pass `go vet` on its own.

#### Scenario: closureguard
- GIVEN `internal/closureguard/candidates_test.go` imports `internal/analyzerpython` and
  `internal/analyzerrust`
- WHEN B5 runs
- THEN the test is rewritten first and the analyzer packages are deleted in a later commit

### Requirement: RMV-V1-006 Dangling references are repaired first (G27, audit B4)

B0 SHALL remove or repair every CI reference to code that no longer exists, listed under B0. After
B0, no workflow SHALL run a `make` target, `go test` package or `corvint` subcommand that does not
exist.

#### Scenario: release-gates
- GIVEN `.github/workflows/release-gates.yml:90` runs `make companion-release-gate`
- WHEN B0 lands
- THEN the step is removed or repointed and `make -n` of whatever it calls succeeds

### Requirement: RMV-V1-007 Dead leaves are removed (audit tables; order from G27)

B1 SHALL remove the packages and files listed under B1, each of which has no Go importer outside
its own chain (VERIFIED). The owner accepts this list only through this proposal; no owner answer
names these paths one by one.

#### Scenario: chain removal
- GIVEN `internal/worksource` is imported only by `internal/worklistadapter`
- WHEN B1 removes the chain
- THEN both go in the same commit and nothing else imports either

### Requirement: RMV-V1-008 Test-quality cut (G24)

B2 SHALL remove `corvint test-plan`, `corvint tests lint`, `corvint tests duplicates`,
`internal/testplan`, `corvint runs`, `internal/playwrightruns`, the `test-validity` and
`tests validity` verbs, `cmd/corvint-test-validity-mcp` (and its `binaries.json` row),
`internal/mcp/testvaliditybridge`, the `test-validity` MCP group, the rule set in
`internal/testsource/validity*.go`, and the `internal/tcq` code nothing outside `tcq` uses. Because `internal/testplan` is one of the three
tests-layer flow readers that `tools/import-ratchet/flows_layer_test.go` pins for CMS-V0-915
(decision 0595), B2 SHALL drop it from that test's reader set and amend the CMS-V0-915 trace row
in `docs/specs/core-bundled-apps-v0.md` in the same batch; the remaining readers, `cmd/corvint` and
`internal/testsource`, SHALL still import `internal/appflows` or `internal/appmap` after B2.

#### Scenario: flow reader set after B2
- GIVEN B2 has removed `internal/testplan`
- WHEN `go test ./tools/import-ratchet/...` runs
- THEN `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs` passes with two readers and reports
  no FLOW_READER_UNREACHED

#### Scenario: verb gone
- GIVEN B2 has landed
- WHEN a user runs `corvint test-validity`
- THEN Corvint reports an unknown command with its normal unknown-command code

### Requirement: RMV-V1-009 Test-quality keeps (G24)

B2 SHALL keep `internal/testvalidity` projection types and state constants, `tcq` `Flaky` and
result constants, the `internal/testvaliditydoc` receipt decoder (moving `read.go` into the existing
`internal/pinnedread`), `internal/testevidence`, and `corvint test-evidence`, `corvint flows
test-evidence` and their MCP group, labelled preview. Provider output formats SHALL NOT change.

#### Scenario: provider output
- GIVEN the JS, Ruby and Go test providers' output before B2
- WHEN B2 lands
- THEN their output for the same input is byte-identical

### Requirement: RMV-V1-010 Runner protocol cut (G23)

B3 SHALL remove `internal/runnerprotocol`, `internal/runnerpolicy`, `internal/runnerqualification`,
`tools/runner-plugin`, `tools/runner-qualification` and `tools/runner-reference`, and SHALL keep
`registry.RunnerPlugin`, `internal/runnerselection` and `internal/testrunner`.

#### Scenario: no external runner
- GIVEN B3 has landed
- WHEN a repository has a `.corvint/runner-plugins.json`
- THEN Corvint does not load it (the file does not exist in Corvint today; NOT_VERIFIED whether
  any code reads it, to be checked in B3)

### Requirement: RMV-V1-011 The single compiled-in contract is decided separately (G23)

B3 SHALL NOT change how Vitest, Playwright, Jest, pytest, RSpec or Minitest are run. The single
compiled-in contract for all seven runners SHALL come from its own committed proposal. Packages that
have no importer today but may serve that contract (`internal/testrunner/golang`,
`internal/appflows/e2erunner`, `internal/liveverify/jsresolve`, `internal/liveverify/pyresolve`)
SHALL stay until that proposal decides. A ticket SHALL be filed for an end-to-end check of the Jest
path through `corvint test`.

#### Scenario: idle package
- GIVEN `internal/liveverify/pyresolve` has no importer (VERIFIED)
- WHEN B1 or B3 runs
- THEN it is not removed

### Requirement: RMV-V1-012 VS Code extension moves out (G22, owner decision R33 (2026-11-09))

B4 SHALL remove `extensions/vscode`, `conformance/vscode-extension-v0`, `tools/lsp-editors`,
`script/measure-lsp-quality.py`, `script/qualify-lsp-editors.py`, `script/qualify-lsp.py` and the
Makefile targets `host-adapter-test` (`Makefile:1323`) and `vscode-check-install-test`
(`Makefile:1312`); B0 or a later `.github` batch removes `.github/workflows/vscode-extension.yml`.
The extension moves to the new public repository `beamfall/corvint-vscode`, whose creation the
owner approved (owner decision R33 (2026-11-09)). The removal of `extensions/vscode` SHALL wait
until the frozen, unpublished copy has been pushed there; the batch's build-log entry SHALL name
the copy's commit in `beamfall/corvint-vscode`. Creating the repository and pushing the copy are
outward actions done by the owner or under the standing publication rules, not by this change.

#### Scenario: frozen copy not yet pushed
- GIVEN `beamfall/corvint-vscode` does not yet hold the frozen copy
- WHEN B4 is prepared
- THEN `extensions/vscode` stays and the rest of B4 may land

### Requirement: RMV-V1-013 Third-party evidence and parity extras (G22 panel, folded here)

B4 SHALL keep `internal/extevidence` as an internal format; remove `examples/evidence-provider`,
`docs/EXTERNAL-EVIDENCE-PROVIDERS.md`, `protocol/integrations/speckit` and
`conformance/cli-parity-v0`; repoint the comment at `internal/gokernel/repository.go:390-391`;
and archive `docs/EXTENSION-ECOSYSTEM.md` and `docs/specs/deployment-neutral-index-platform-v0.md`
in place. These items were recommended by the panel and not asked separately; the owner accepts
them only through this proposal.

#### Scenario: cli-parity fixtures
- GIVEN `conformance/cli-parity-v0` is named in `.corvint/test-read-scopes.json`,
  `.corvint/test-read-scope-literals.json`, `.corvint/test-selection.json` and
  `.github/cishards/package-costs.json`
- WHEN B4 lands
- THEN the `.corvint` entries go in B4 and the `.github/cishards` entries go in a `.github` batch

### Requirement: RMV-V1-014 Removed languages fail closed under `corvint test` (G26, owner decision R32 (2026-11-09))

Before B5 removes any language provider, a change that no runner owns, or a change for a removed
or unavailable runner, SHALL make `corvint test` run ALL tests with a named reason, and SHALL make
`corvint affected` report UNKNOWN. The reason is `runner-removed` for a removed or unavailable
runner and `UNOWNED_DIRTY_PATH` for a path no runner owns (owner decision R32 (2026-11-09), the
recommended option of the question it answered). The mechanism is the owner's choice (b):
`UNOWNED_DIRTY_PATH` comes off the allowlist in `AllowedUnknown` (`internal/gateselect/allowlist.go`,
the `case "UNOWNED_DIRTY_PATH", "NO_SELECTABLE_TEST": return true` branch, integ 9ec3a29d0), so the
session planner no longer skips it. Mechanism (a), a retired-language ownership entry in
`internal/langplugin`, is not built. The behaviour itself is owned by RNC-V1-004 (change
`runner-contract`) and NCM-V1-001 (change `non-code-map`); paths that NCM-V1-001 classes as
declared documentation keep adding no test. This requirement only blocks B5 until that behaviour has
landed. A test SHALL prove it on a fixture repository that declares a `rust` command in
`.github/cishards/test-commands.json` and changes a `.rs` file.

#### Scenario: Rust change after removal
- GIVEN a fixture repository declares `rust` in `test-commands.json` and the Rust provider is removed
- WHEN a change touches `src/lib.rs`
- THEN `corvint test` runs all tests and the plan names `UNOWNED_DIRTY_PATH` and the path

#### Scenario: Swift change after removal
- GIVEN beamfall-apple declares `swift` in `test-commands.json` and the Swift provider is removed
- WHEN a change touches `Sources/App/View.swift`
- THEN `corvint test` runs all tests, including the declared Swift command, and the plan names the
  reason

### Requirement: RMV-V1-015 Languages and analyzers beyond the seven runners (G26)

B5 SHALL remove all 16 `cmd/corvint-analyzer-*` binaries, the 14 `internal/analyzer*` packages,
`internal/analyzerenvelope`, `experimental/analyzers/sqlnative`, the `cfamily`, `dotnet`, `java`,
`kotlin`, `rust` and `swift` providers under `internal/liveverify/affected`, their registry and
testrunner plugin files, `internal/swiftimpact`, `internal/csharpregions`,
`internal/nativelinecov`, `internal/rustsyntax`, `internal/shaderdialect`, `internal/rubysyntax`,
the sql-native ratchet, `cmd/corvint-go-toolchain-receipt` with `internal/toolchainreceipt`,
`internal/liveverify/pymutate` and `internal/liveverify/rbmutate`. It SHALL keep the shell provider
and Go. B5 SHALL also delete the `analyzers` area from `tools/import-ratchet/areas.json`, leaving
four layers (shared; tests and docs; release), and rewrite the manifest's rule string to match
(companion decision item 8, amending decision 0594 item 8).

#### Scenario: four layers after B5
- GIVEN B5 has landed
- WHEN `go test ./tools/import-ratchet/...` runs
- THEN it passes with the areas shared, tests, docs and release, and no prefix names an analyzer

#### Scenario: analyzer binary gone
- GIVEN B5 has landed
- WHEN `make -n release-binaries-check` runs
- THEN no `corvint-analyzer-*` name appears

### Requirement: RMV-V1-016 `testrunner/native` keeps go-test (G26)

B5 SHALL split `internal/testrunner/native` so the go-test part stays and the parts used only by
.NET, Rust and runner-protocol code go.

#### Scenario: Go runner unaffected
- GIVEN the go-test selection tests before B5
- WHEN B5 lands
- THEN they pass unchanged

### Requirement: RMV-V1-017 `liveverify/mutate` is ticketed, not removed here (G26)

After B5, `internal/liveverify/mutate` has no importer (its importers `pymutate`, `rbmutate` and
`testrunner/golang` are removed or idle). B5 SHALL file a ticket for it and SHALL NOT remove it.

#### Scenario: orphan
- GIVEN B5 has landed
- WHEN the importer check runs for `internal/liveverify/mutate`
- THEN a ticket exists and the package is still present

### Requirement: RMV-V1-018 Retired language specs and CAPABILITIES (G26)

B5 SHALL add retirement status lines to the specs whose only implementation it deletes and SHALL
amend `docs/CAPABILITIES.md` (which says "Eleven language providers" at `:311`).

#### Scenario: capability catalog
- GIVEN B5 has landed
- WHEN `capabilities-check` runs
- THEN no removed binary, verb or provider is listed as available

### Requirement: RMV-V1-019 CEM, OCM and dogfood removal (G8)

B6 SHALL remove the `corvint cem` verbs (`cmd/corvint/cem*.go`), the `internal/cem` subpackages that
are not shared (`mdreport`, `publish`, `sim`, `witnesspaths`, `verify`, `workflow`, `cli`; there
is no `internal/cem` root package), the CEM stable-result release gate (`tools/cem-release-suites`,
`internal/corerelease/core_stable_result.go` and the other `corerelease` parts that serve only that
gate, and the `releasecandidate` CEM readiness gate), `interop/cem01-go`, `protocol/cem-1.0`, `protocol/cem-completeness-1`, the
`cem-*` Makefile targets (`Makefile:253-307`), `script/install-cem-verifiers.sh`, the dogfood
scripts and target (`script/dogfood-merge-base*.sh`, `Makefile:1204`), `docs/DOGFOOD.md` (left as a
retirement stub), `.corvint/changes`, `.corvint/deferred-intents`, `.agent-evidence` and
`artifacts/cem-release-27e9d98`. `conformance/tcq-0` SHALL be checked for CEM-only use in B6;
whether it is CEM-only is NOT_VERIFIED.

`conformance/use-cases-v0` does not leave in B6 (final round, M6): "The use-case receipt contract
is superseded by the per-product end-to-end qualification of REL-V1-012;
`conformance/use-cases-v0` is removed in B8 together with its two lints, and no 1.0 claim cites a
use-case receipt." The two lints are `repin-use-case-receipts` and `use-case-receipts-check`
(RMV-V1-024). The name of the spec that governs the receipt contract is UNVERIFIED; the B8
build-log entry SHALL name it. Earlier wording, superseded by M6: `conformance/use-cases-v0` was
checked for CEM-only use in B6.

Self-use after B6 (decided 2026-11-09 under R42, E6): self-use of Corvint on a change SHALL be
recorded as a progress note on the change's ticket, or in the PR body when the change has no
ticket, and friction SHALL be filed as tickets. The `docs/DOGFOOD.md` retirement stub SHALL stay
until the owner re-decides it after B6.

#### Scenario: use-case receipts leave with their lints
- GIVEN B6 has landed and B8 is prepared
- WHEN B8 removes `repin-use-case-receipts` and `use-case-receipts-check`
- THEN the same batch removes `conformance/use-cases-v0`

#### Scenario: self-use record after B6
- GIVEN B6 has landed and a change has a ticket
- WHEN Corvint is used on that change
- THEN the use is a progress note on the ticket and no CEM file is written

#### Scenario: verb gone
- GIVEN B6 has landed
- WHEN a user runs `corvint cem verify`
- THEN Corvint reports an unknown command

### Requirement: RMV-V1-020 Shared packages move before B6 (G8, verified importers)

Before B6 deletes `internal/cem`, the packages kept code needs SHALL move to the shared layer with
no behaviour change: `internal/cem/wire` (60 outside importer files), `gitauth` (47), `gitrun` (43),
`cemcode` (21), `coverprofile` (imported by `internal/liveverify/gorunner`), and `patch` (imported
by `gitauth/diff.go`). The target paths are `internal/wire`, `internal/gitauth`, `internal/gitrun`,
`internal/cemcode`, `internal/coverprofile` and `internal/patch` (none exists at `9ec3a29d0`;
VERIFIED), each added as a prefix of the `shared` area (layer 0) in `areas.json`, because `wire`'s
importers span the tests, docs and release layers. `internal/cem/verify` and `internal/cem/sim` SHALL
NOT move: `verify` imports `sim` (5 files), and its only use outside `internal/cem` is the CEM
stable-result gate (`internal/corerelease/core_stable_result.go`, imported by
`internal/releasecandidate` and `tools/cem-release-suites`), which B6b removes whole (RMV-V1-019).
`internal/cem/wire/stable_producer_only_test.go` imports `verify`, so the move SHALL rewrite or
remove that test. Counts and importers are VERIFIED at `9ec3a29d0`.

#### Scenario: move only
- GIVEN the move commit
- WHEN `go vet ./...` and the moved packages' tests run
- THEN both pass and no exported identifier changed

### Requirement: RMV-V1-021 Superseded tickets close (G8)

After the companion decision is accepted, V1-0014 and V1-1344 to V1-1348 SHALL close as superseded
through the Tasks completion workflow, citing the decision. They SHALL NOT be closed before
acceptance.

#### Scenario: decision still proposed
- GIVEN the decision status is proposed
- WHEN closure is attempted
- THEN the tickets stay open

### Requirement: RMV-V1-022 Licence notes follow the removals

When B6 removes `interop/cem01-go`, `protocol/cem-1.0` and `protocol/cem-completeness-1`, it SHALL
update `LICENSING.md` "Notes on retained files" (`:21-23` name the `LICENSE-APACHE-2.0` copies in
those directories). The AGPL SPDX notes at `:26-31` for the two preregistered benchmark harnesses
SHALL NOT change.

#### Scenario: license pins
- GIVEN B6 removes the directories
- WHEN `check-legal-file-pins` runs
- THEN it passes with the updated notes

### Requirement: RMV-V1-023 G25 "still removed" set waits for the revival proposals (G25)

B7 SHALL remove the context ranking files in `internal/contextindex` (`lexrank.go`, `ppr.go`,
`slot_weights.go`, `span_rank.go`, `taskcontext*.go` and their tests), the separate impact engine in
`internal/contextindex`, the feature regex heuristics, and the snapshot path
(`internal/gokernel/harness.go`, `adapter_codes.go`, `cmd/corvint/index_snapshot.go`) if `watch`
does not need it. B7 SHALL start only after the `impact`, `review`, `context`, feature-fold and
`index` revival proposals are committed and say what they reuse, and after the `watch` 1.0 s
measurement says whether `index` is needed. `gokernel.HandleEvent`/`HandleEventContext` has no
caller outside `internal/gokernel` (VERIFIED); the size of the snapshot cut is NOT_VERIFIED (no
`deadcode` run). `corvint context` ships redesigned and non-blocking, with no claim of measured
benefit, and its measurement runs in 1.0.x (owner decision R28/R29 (2026-11-09)); B7 SHALL keep
every file the redesigned context reuses and SHALL NOT wait for that measurement.

#### Scenario: revival reuses a file
- GIVEN the context revival proposal reuses `taskcontext.go` for declared links
- WHEN B7 is prepared
- THEN that file is kept and the reason is recorded

### Requirement: RMV-V1-024 Lints (G27)

B8 SHALL fold `check-goos-filenames`, `check-mkfifo-build-tags` and `check-windows-main` into
`cross-vet` (`Makefile:202`) and `windows-vet` (`Makefile:1430`), then remove them; remove
`long-lines-check`, `check-stray-backups`, `tracked-ignored-check`, `check-fixed-tmp-paths`,
`check-eol-policy`, `check-bash32` (once remaining shipped scripts are `#!/bin/sh`),
`check-instruction-conflicts`, `agent-grep.sh`, `test-run-pattern`, `walk-skipdir-check`,
`py-signal-lint`, `repin-use-case-receipts` and `use-case-receipts-check`; and keep the
product-protecting lints the panel listed. The analyzer-schema, analyzer-inputs and analyzer ratchet
lints SHALL go with B5, because G26 removes the analyzers the panel kept them for. Matching
`GATE_STEPS`, `DOC_GATE_TARGETS` and `shell-test` entries change in the same non-`.github` batch.
`gate-ledger`, `gate-receipt`, `go-archive-gate` and `adopter-ci-advisory` are replaced only at
B11.

#### Scenario: fold proven
- GIVEN a file named `x_windows.go` without a build constraint
- WHEN `make cross-vet` runs after the fold
- THEN it fails on the same defect the removed lint caught

### Requirement: RMV-V1-025 Lane tooling leaves after the layering merge (G27)

Lane and batch steps SHALL leave `GATE_STEPS`, `doc-gates` and `shell-test` in B8's first part,
which can land at once. The lane scripts, merge drivers, `flow-guide-check`, requirement-ID
reservation tooling, `docs/LANE-ORCHESTRATION.md` and the `corvint-lane-flow` skill SHALL leave only
after the layering integration branch merges (about 2026-12-08), with a retirement stub at
`docs/LANE-ORCHESTRATION.md`. RMV-V1-032 and RMV-V1-033 hold back two groups of files from B8's
second part.

Decided 2026-11-09 under R42, E9: the lane group that leaves in B8b SHALL be moved to a private
operator repository, not deleted; the B8b build-log entry SHALL name that repository and the
commit that received the files. `heavy.sh` SHALL stay for 1.0 as tooling that is not shipped, and
SHALL leave with the gate rewrite or in 1.0.x. B8b follows the date and reader rules of
RMV-V1-001 (E5).

#### Scenario: lane files moved, not lost
- GIVEN B8b is prepared
- WHEN its build-log entry is written
- THEN it names the private operator repository commit that holds the lane files

#### Scenario: early removal
- GIVEN the layering branch has not merged
- WHEN B8's second part is proposed
- THEN it waits

### Requirement: RMV-V1-026 Retired specs are archived in place (G27)

Each spec whose only implementation a batch deletes SHALL get a status line naming the superseding
decision and a retirement base, in the same batch. Spec files SHALL NOT be moved or deleted.

#### Scenario: requirement history
- GIVEN B2 retires a test-validity spec
- WHEN `REQUIREMENTS.tsv` is regenerated
- THEN the requirement IDs remain as history rows

### Requirement: RMV-V1-027 Tasks specs leave after corvint-tasks PR #4 (G27)

B10 SHALL delete the Tasks specs from this repository only after `beamfall/corvint-tasks` PR #4
merges, in one docs change that regenerates the index and points live links at the corvint-tasks
URL. History documents keep their old links.

#### Scenario: PR not merged
- GIVEN corvint-tasks PR #4 is open
- WHEN B10 is proposed
- THEN it waits

### Requirement: RMV-V1-028 Action cut-over (G18, G27)

B11 SHALL replace the bespoke CI selection scripts and the four gate scripts named in RMV-V1-024
with the corvint action only after the action has matched them on 30 PRs with no would-be miss. If
that has not happened by 2026-12-15, the scripts stay for 1.0. B11 is two PRs: a `.github`-only PR,
then a non-`.github` PR that removes the scripts.

The first PR, the `.github`-only cut-over (decided 2026-11-09 under R42, D15), SHALL:

1. delete four workflow files: `pr-tests-qualification.yml`, `ci-forward-bar.yml`,
   `ci-route-health.yml` and `flake-report.yml`;
2. trim `ci-control-plane.yml`, not delete it: the `control-plane` and `merge-group` jobs stay,
   the `main-miss` and `forward-rows` jobs are deleted, and the `ci-control-plane` status stays
   required;
3. leave `greptile-gate.yml` untouched (D14) and leave the other paths RMV-V1-035 keeps.

No file `main-miss.yml` exists; `main-miss` is a job of `ci-control-plane.yml`. The cut-over
therefore touches five workflow files and removes two jobs. The required statuses follow
GHA-V1-031. The live ruleset is NOT_VERIFIED and the integ tree was not read for this decision
(medium confidence); the B11 lane SHALL read both before it opens the PR. If the 30-PR shadow
match is not reached by 2026-12-15, every file and job named here stays for 1.0 (G18).

#### Scenario: control plane trimmed
- GIVEN the shadow run has matched on 30 PRs
- WHEN the `.github`-only cut-over PR merges
- THEN `ci-control-plane.yml` still has the `control-plane` and `merge-group` jobs
- AND the `ci-control-plane` status is still required

#### Scenario: shadow not matched
- GIVEN 2026-12-15 has passed with 22 matched PRs
- WHEN B11 is proposed
- THEN it becomes a 1.0.x change

### Requirement: RMV-V1-029 perf-budget is kept (G27)

No batch SHALL remove `internal/perfbudget`, `perf-budget.yml` or `make perf-budget`
(`Makefile:1444`). Tight limits are enforced on a quiet host, not on hosted runners.

#### Scenario: removal touches perfbudget
- GIVEN a batch's diff deletes a `budgets.json` row for a kept command
- WHEN the batch is reviewed
- THEN it is refused

### Requirement: RMV-V1-030 Rollback by Git (G27)

Each batch SHALL be one merge commit (or, for B11, two) whose revert restores the removed paths.
The batch's build-log entry (owner decision R34 (2026-11-09)) SHALL name its parent commit and,
where the code was already removed once, the recovery commit.

#### Scenario: revert
- GIVEN B3 merged as commit X
- WHEN B3 must be undone
- THEN `git revert -m 1 X` restores `tools/runner-*` and the build passes

### Requirement: RMV-V1-031 Every unlisted path has a verdict; orphans are deleted and tracked (owner decisions R33, R34 (2026-11-09))

`path-verdicts.md` in this change SHALL give every top-level path, and every `cmd/`, `internal/`,
`tools/`, `script/`, `benchmarks/` and `conformance/` entry that no batch in `design.md` lists, one
verdict (`keep`, `remove in Bn`, `move to <repo>`, `listed in Bn`, or `undetermined`), a one-line
reason and the read-only check that supports it in integ 9ec3a29d0. An `undetermined` row SHALL
state what is missing and SHALL NOT be removed until a batch settles it. Code left with no importer
or caller by a batch (orphaned code) SHALL be deleted in that batch and listed in an orphan table in
the batch's one `docs/build-log/YYYY-MM-DD-<slug>.md` entry, with the path, the importer or caller
whose removal orphaned it, and the zero-hit check (owner decision R33 (2026-11-09)). Each batch's
entry SHALL copy its `path-verdicts.md` rows and record the verdict it applied; no numbered
decision is written per batch (owner decision R34 (2026-11-09)).

#### Scenario: an undetermined path
- GIVEN `tools/retrieval-bench` is `undetermined` because the 1.0.x context measurement design is
  missing
- WHEN B7 is prepared
- THEN the path stays and the batch entry repeats what is missing

#### Scenario: a path orphaned by a batch
- GIVEN B5 removes the last importer of a package that `path-verdicts.md` marks `keep`
- WHEN the batch's importer check (RMV-V1-004) finds no remaining importer
- THEN the package is deleted in B5 and appears in the orphan table of B5's build-log entry

### Requirement: RMV-V1-032 The `.github` split check outlives the action cut-over (review ci-agents, split-check sequencing)

`script/github-split-check.sh`, `script/github-split-check_test.sh` and the Makefile target that
runs the test SHALL stay until the B11 `.github`-only cut-over pull request has merged. They SHALL
leave in B11's script removal pull request, not in B8's second part. B8's second part removes
`script/batch-precheck.sh`, the only caller (integ 9ec3a29d0, `script/batch-precheck.sh:850`);
from then until the cut-over the check is run by hand before a push that touches `.github/`.

#### Scenario: B8b lands before the cut-over
- GIVEN B8's second part is prepared and the B11 cut-over pull request has not merged
- WHEN the batch's path list is checked
- THEN `script/github-split-check.sh` and its test are not in it, and they are still present after
  the batch merges

#### Scenario: after the cut-over
- GIVEN the B11 `.github`-only cut-over pull request has merged
- WHEN B11's script removal pull request is prepared
- THEN it removes `script/github-split-check.sh`, its test and the Makefile line that runs it

### Requirement: RMV-V1-033 Requirement-ID reservation tooling stays while a change folder calls it (review tools-script audit)

`script/requirement-id-reservations.sh`, its test, `script/requirement-ids-free.sh`,
`script/requirement-next-id*`, `script/requirement-id-collisions.sh`, its test and the Makefile
targets that run them SHALL stay until the last change folder under
`docs/specs/openspec/changes/` whose `tasks.md` or `proposal.md` calls one of them is archived.
`script/requirement-id-collisions.sh` stays with them because the `requirement-ids-free` target
runs it beside the reservation check (integ 9ec3a29d0, `Makefile:1217`, `Makefile:1218`). The Go
tool `tools/requirement-id-collisions` SHALL still leave in B8's second part. The batch that
removes the scripts SHALL record the zero-hit search of the change folders in its build-log entry.

#### Scenario: a change folder still reserves a prefix
- GIVEN `narrowing-measurement/tasks.md` step 1 calls `script/requirement-id-reservations.sh
  reserve` and that change is not archived
- WHEN B8's second part is prepared
- THEN the reservation scripts and `script/requirement-id-collisions.sh` are not in its path list

#### Scenario: the last caller is archived
- GIVEN no file under `docs/specs/openspec/changes/` outside `archive/` names
  `requirement-id-reservations`, `requirement-ids-free` or `requirement-next-id`
- WHEN the next removal batch is prepared
- THEN it removes the scripts and their Makefile targets and records the zero-hit search

### Requirement: RMV-V1-034 One Go attribution after SLF-V1-007 (selection verdicts, finding 8)

After SLF-V1-007 (change `selection-hardening`) has landed, the second Go attribution that
`corvint test` computes for itself SHALL be removed, so that the one function SLF-V1-007 names
decides the Go selection for `corvint affected`, `corvint test` and the MCP plan tool. At integ
9ec3a29d0 that second attribution is `testRunInput` and `testFillPlan`
(`cmd/corvint/test.go:589`, `cmd/corvint/test.go:660`), which call
`gateselect.SelectQualifiedIndexed` (`cmd/corvint/test.go:736`). The removal SHALL NOT land before
SLF-V1-007 and is not part of batches B0 to B11.

#### Scenario: SLF-V1-007 has not landed
- GIVEN the single mapping function of SLF-V1-007 is not on `main`
- WHEN the removal is proposed
- THEN it waits, and `corvint test` keeps its own attribution

#### Scenario: after the removal
- GIVEN SLF-V1-007 has landed and the second attribution is removed
- WHEN `corvint affected` and `corvint test` plan the same Go change
- THEN both print the same Go selection, and `cmd/corvint/test.go` no longer calls
  `gateselect.SelectQualifiedIndexed`

### Requirement: RMV-V1-035 Four CI paths are kept for 1.0 (decided 2026-11-09 under R42, D11)

No batch from B0 to B11 SHALL remove the four paths below. Each stays until its condition ends,
and the owner re-decides it then.

| Path | Kept until |
| --- | --- |
| `.github/testconfine/` | the action proves the same read-scope confinement |
| `.github/workflows/greptile-gate.yml` | the owner removes the required `greptile-gate` status |
| `.github/workflows/merge-queue-cleanup.yml` | the merge queue is no longer used |
| `.github/workflows/cache-prune.yml` | CodeQL stops sharing the cache quota |

`cache-prune.yml` SHALL keep its recorded least-privilege exception. `greptile-gate-v0.md` SHALL
stay an active spec while its workflow is kept. B11 SHALL leave `greptile-gate.yml` untouched
(D14).

#### Scenario: a batch deletes a kept workflow
- GIVEN a batch whose diff deletes `.github/workflows/greptile-gate.yml`
- WHEN the batch is reviewed
- THEN it is refused, because the `main` ruleset still requires the `greptile-gate` status

#### Scenario: a condition ends
- GIVEN the merge queue is no longer used
- WHEN removal of `merge-queue-cleanup.yml` is proposed
- THEN the owner decides it as a new change, outside batches B0 to B11

### Requirement: RMV-V1-036 Frozen corpora, retained evidence and the settled rows

Decided 2026-11-09 under R42, E1. A frozen corpus that nothing reads SHALL be removed, and the batch's build-log entry SHALL record
the corpus's last commit. Evidence that an active spec cites SHALL be kept. The rows below
replace the matching `undetermined` rows of `path-verdicts.md`.

| Paths | Verdict | Batch |
| --- | --- | --- |
| `tools/compat-trial`, `cw-trial`, `paired-trial`, `heading-nav-bench`, `retrieval-bench` | remove | B7 |
| `tools/str-parity` | remove once the compiled-in runner contract (G23) lands | after G23 |
| `script/measure-context-packet.py` and its test, `testing/context-retrieval-goldens.json` | remove | B7 |
| `script/measure-worktree-index-share.sh`, `benchmarks/snapshot-reader` | follow RMV-V1-023 | B7 |
| `benchmarks/blind-v2`, `blind-v3`, `blind-v4`, `blind-v6-dev`, `blind-v7`, `cases`, `cem2-discovery-replay-v0`, `daily-loop-v1`, `expert-audit-followup`, `parity-fixture-index`, `parity-snapshot-discovery` | remove, last commit recorded | B6 or B7 |
| `benchmarks/untouched-repository-v1` | keep until change `proof-repositories` replaces that record and `LICENSING.md` lines 26 to 31 are edited in the same change | later |
| `evidence/issues-167-175` | keep | none |
| `conformance/interactive-alpha` and its test | keep | none |
| `script/seed-planning-store*`, `script/atm-fixture-store.sh`, `core-planning-proof.mjs`, `roadmap-proof.mjs` | move to `beamfall/corvint-tasks` | B10 |
| `cmd/corvint-test-runner` | retire with the provider binaries (owner decision R14) | after G23 |
| `script/go-test-shard.sh` | remove with the lane tooling | B8b |
| `script/spec-coverage-audit` | remove | B2 |

Which of B6 and B7 removes each of the eleven corpora is NOT_VERIFIED; the batch lane SHALL
re-check readers on the batch base (RMV-V1-004) and name the batch in its build-log entry.

#### Scenario: a corpus with no reader
- GIVEN `benchmarks/blind-v3` has no reader in the Makefile, workflows, scripts or Go code
- WHEN its batch removes it
- THEN the batch's build-log entry records the last commit that held the corpus

#### Scenario: evidence an active spec cites
- GIVEN `evidence/issues-167-175` is cited by an active spec
- WHEN a batch proposes to remove it
- THEN the removal is refused and the directory stays

#### Scenario: a corpus gains a reader
- GIVEN a 1.0.x measurement design that reads `benchmarks/cases` is committed before its batch
- WHEN the batch is prepared
- THEN that corpus is kept and the reason is recorded

### Requirement: RMV-V1-037 The `e2e-safe` profile and the appmap navigation verdict leave 1.0

Decided 2026-11-09 under R42, A4. The `--selection-profile e2e-safe` value of
`corvint affected --provider FILE` and the appmap navigation verdict SHALL leave the public 1.0
surface: command help, user documents and MCP tool descriptions. Their code SHALL be deleted by
2026-12-01, or in the first 1.0.x release if the batch misses that date. The batch lane SHALL
re-check readers on the batch base (RMV-V1-004) and name the batch in its build-log entry.

This supersedes the Core classification of the profile in decision 0385 (AFU-V1-019 to
AFU-V1-024) and in the `corvint impact`, `affected` row of
`corvint-1.0-product-and-release-v1.md`. The supersession takes
effect only when the owner accepts the plan record that lists it (plan checklist O66). Until
then the accepted text governs and no lane removes the profile.

#### Scenario: the plan record is not yet accepted
- GIVEN the owner has not accepted the record that lists checklist row O66
- WHEN a removal batch is prepared
- THEN the `e2e-safe` profile and the navigation verdict are left in place

#### Scenario: after acceptance
- GIVEN the record is accepted and the batch has removed the profile
- WHEN a user runs `corvint affected --selection-profile e2e-safe`
- THEN the command refuses the unknown value and names the supported way to select flow tests

## MODIFIED Requirements

### Requirement: RSP-V0-025 Runner pieces are compiled in (G23)

Source: decision 0592 (owner 2026-11-07), superseded for the out-of-process form by G23 (owner
answer 2026-11-09) through `docs/decisions/PROPOSED-accept-product-plan-1.0.md`. Spec:
`docs/specs/runner-selection-plugins-v0.md`.

A runner's pieces SHALL be provided compiled-in, implementing the in-process runner-plugin
interface (`registry.RunnerPlugin`, `internal/testrunner/registry/runner_plugin.go`, integ
9ec3a29d0). The out-of-process form, a `corvint-runner-<name>` executable
speaking `corvint-runner/1`, is removed. The selector, the store and the conformance kit treat
every compiled-in runner alike. The previous text allowed both forms.

#### Scenario: an out-of-process runner is declared
- GIVEN a repository's `.corvint/runner-plugins.json` names a `corvint-runner/1` plugin
- WHEN `corvint test` plans a change after B3
- THEN no plugin process starts, and a runner with no compiled-in implementation runs all tests
  with reason `runner-removed` (RNC-V1-004)

## REMOVED Requirements

### Requirement: RSP-V0-026 to RSP-V0-032 and RSP-V0-047 The `corvint-runner/1` out-of-process protocol

**Reason**: G23 keeps one compiled-in runner contract; decision 0592's out-of-process form is
superseded (`docs/decisions/PROPOSED-accept-product-plan-1.0.md`). B3 removes `tools/runner-*`,
`internal/runnerprotocol`, `internal/runnerpolicy` and `internal/runnerqualification`
(RMV-V1-010). The
removed IDs are RSP-V0-026 (JSON-lines framing), RSP-V0-027 (methods), RSP-V0-028
(`.corvint/runner-plugins.json` discovery), RSP-V0-029 (process limits), RSP-V0-030 (plugin output
is evidence), RSP-V0-031 (plugin failure handling, including the `runner-adapter-unavailable` FAIL),
RSP-V0-032 (versioning) and RSP-V0-047 (JSON Schema and version negotiation).
**Migration**: runners are compiled in (RSP-V0-025 as modified above, and change `runner-contract`).
A removed or unavailable runner runs all tests with reason `runner-removed` (RNC-V1-004, owner
decision R32 (2026-11-09)) instead of RSP-V0-031's FULL-or-FAIL rule. The IDs stay in
`docs/specs/runner-selection-plugins-v0.md` as retired history rows with a status line added in B3
(RMV-V1-026); none is reused.

#### Scenario: protocol IDs after B3
- GIVEN B3 has merged
- WHEN `make spec-requirements` regenerates `docs/specs/REQUIREMENTS.tsv`
- THEN RSP-V0-026 to RSP-V0-032 and RSP-V0-047 are listed as retired and no code cites them

### Requirement: Spec families whose only code a removal batch deletes

**Reason**: each spec below is archived in place in the batch that deletes its code (G27,
RMV-V1-026). The list is taken from `docs/specs/openspec/changes/spec-set-cleanup/specs-verdicts.md`
(rows with verdict "archive in place" and a batch from B1 to B8b) as written on 2026-11-09; that file
stays the owner of each spec's verdict, and a change there changes this list. Each batch's
build-log entry SHALL confirm the exact requirement IDs it retires.
**Migration**: none of these capabilities ships in 1.0. Every ID stays as a retired history row in
`REQUIREMENTS.tsv`; no ID is reused or renumbered. Where a prefix is shared with a kept spec
(`ASD-V0` with `corvint-self-development-v0.md`), only the requirements in the archived file retire.
Not in this list: specs whose code is already gone and workflow documents (spec-set-cleanup batch
C1), MERGE folds (C2), Tasks specs moved to `beamfall/corvint-tasks` (B10), `SKH-V0` (owned by the
change that rewrites `corvint how`), and rows whose verdict is still `undetermined` in
specs-verdicts.md. `greptile-gate-v0.md` (GRG-V0) is also not in this list: its workflow is kept
for 1.0 (RMV-V1-035), so the spec stays active and B8b does not archive it (decided 2026-11-09
under R42, D11).

| Batch | Spec file under `docs/specs/` | Prefix |
| --- | --- | --- |
| B1 | `cross-repository-breakage-map-v0.md` | BKM-V0 |
| B1 | `spec-toolchain-adapters-v0.md` | STA-V0 |
| B1 | `unplanned-read-events-v0.md` | URE-V0 |
| B2 | `mcp-test-validity-profile-v0.md` | MTV-V0 |
| B2 | `test-claim-qualification-v0.md` | TCQ-V0 |
| B2 | `test-consolidation-planner-v0.md` | TCN-V0 |
| B4 | `deployment-neutral-index-platform-v0.md` | DNIP |
| B4 | `lsp-editor-qualification-v0.md` | LEQ-V0 |
| B4 | `lsp-qualification-evaluation-policy-v0.json` | (no requirement IDs) |
| B4 | `lsp-quality-platform-v0.md` | LQP-V0 |
| B4 | `vscode-extension-v0.md` | VSC-V0 |
| B5 | `analyzer-candidate-profiles.md` | ACP |
| B5 | `analyzer-python-native-candidate.md` | PNC |
| B5 | `dotnet-affected-selection-v0.md` | DNAS-V0 |
| B5 | `java-affected-test-v0.md` | JAT-V0 |
| B5 | `kotlin-android-candidate-v0.md` | KA-V0 |
| B5 | `kotlin-jvm-affected-test-v0.md` | KAT-V0 |
| B5 | `native-jvm-bridge-analyzer-candidates-v0.md` | NJB |
| B5 | `rust-affected-test-understanding-v0.md` | RUST-AFFECTED-V0 |
| B5 | `rust-analyzer-candidate-v0.md` | RAC |
| B5 | `shader-glsl-metal-analyzer-candidate-v0.md` | SHA-V0 |
| B5 | `sql-native-ratchet-gate-v0.md` | SNR-V0 |
| B5 | `structured-data-analyzer-candidate-v1.md` | SDA |
| B5 | `swift-affected-test-understanding-v0.md` | SATS-V0 |
| B5 | `swift-apple-analyzer-candidate-v0.md` | SAC |
| B6 | `aggregate-local-outcome-v0.md` | ALO-V0 |
| B6 | `cem-2.0-intent.md` | CEM2-V0 |
| B6 | `cem-additive-profiles-v0.md` | CAX-V0 |
| B6 | `cem-completeness-profile-v0.md` | CCP-V0 |
| B6 | `cem-pilot-kit.md` | CEM-PILOT |
| B6 | `cem-stable-v1.md` | CEM-V1 |
| B6 | `cem2-discovery-replay-v0.md` | VRP-V0 |
| B6 | `local-completion-policy-v0.md` | LCP-V0 |
| B6 | `native-cem-adapter.md` | (no requirement IDs) |
| B6 | `proof-tour-v0.md` | PT-V0 |
| B7 | `agent-instruction-doctor-v0.md` | IID-V0 |
| B7 | `context-ranking-v0.md` | CRK-V0 |
| B7 | `expanded-range-impact-v0.md` | ERI-V0 |
| B7 | `non-go-impact-v0.md` | NGI-V0 |
| B7 | `repository-guidance-v0.md` | RGV-V0 |
| B7 | `snapshot-batch-v0.md` | SBQ-V0 |
| B7 | `task-context-packet-v0.md` | TCP-V0 |
| B8a | `eol-policy-gate-v0.md` | EPG-V0 |
| B8b | `batch-integration-checks-v0.md` | ASD-V0 |
| B8b | `ci-control-plane-early-post-v0.md` | CCEP-V0 |
| B8b | `corvint-self-development-companion-v0.md` | CSD-V0 |
| B8b | `corvint-self-development-gates-v0.md` | ASC-V0 |
| B8b | `corvint-self-development-selection-v0.md` | ASD-V0 |
| B8b | `lane-check-tooling-v0.md` | ASD-V0 |
| B8b | `lane-dogfood-tooling-v0.md` | DTL-V0 |
| B8b | `lane-shell-portability-v0.md` | ASD-V0 |
| B8b | `lane-verification-tooling-v0.md` | LVT-V0 |
| B8b | `shared-path-change-policy-v0.md` | SPC-V0 |

#### Scenario: a family retired with its batch
- GIVEN B5 removes `internal/liveverify/affected/swift`
- WHEN the batch lands
- THEN `swift-affected-test-understanding-v0.md` carries a retirement status line and its
  `SATS-V0` IDs remain in `REQUIREMENTS.tsv` as history rows
