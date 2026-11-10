# Source removal: path verdicts

Status: proposed (experimental until the owner accepts the source-removal change). Requirement:
RMV-V1-031. This table is the input to the per-batch build-log entries required by RMV-V1-003
(owner decision R34, 2026-11-09). Each batch's entry copies its rows, adds the batch's proof, and
replaces any `undetermined` row it settles.

## How the checks were run

Every check ran read-only in the integ worktree at commit 9ec3a29d0 (integ 9ec3a29d0). For each path
the basename (for `internal/*`, the import path `github.com/Beamfall/corvint/<path>`) was searched with
`git grep -F`. The check column uses these abbreviations:

- `mk=N,M`: `Makefile` lines that name the path; `mk=-`: none.
- `gh=`: live `.github` files that name it (`cishards/package-costs.json`, `cishards/walltime_test.go`
  and `linecov/qualification.json` are inventories and were excluded); `gh=-`: none.
- `st=`: other `script/` or `tools/` files that name it; `st=-`: none.
- `ot=`: other files, excluding `docs/`, `.corvint/`, `*.md`, `benchmarks/results` and
  `.agent-evidence`; `ot=-`: none. `ot=fixtures` means only test fixtures or ticket JSON name it.
- `imp=`: non-test Go importers outside the package (internal packages only).

Verdicts: `keep`; `remove in Bn` (batch names as in design.md); `move to <repo>`; `listed in Bn` (the
path is already named in that batch's list in design.md); `undetermined` (what is missing is stated;
no verdict is guessed). A path named in a design.md batch list or requirement counts as having a row.

Paths removed with their batch also lose their `.corvint/test-read-scopes.json`,
`.corvint/test-read-scope-literals.json`, `.corvint/test-selection.json` and
`.github/cishards/package-costs.json` entries in the same batch.

## Top-level directories

| Path | Verdict | Reason | Check |
| --- | --- | --- | --- |
| `.agent-evidence` | listed in B6b | history of lane and CEM work; B6b removes it with the CEM | spec.md RMV-V1-019 names it |
| `.claude` | keep | project skills (`.claude/skills`) | named 3 times in this change; no removal |
| `.corvint` | keep | product state; batches edit its read-scope and selection files | named 40 times; edits only |
| `.github` | keep | B0 and B11 edit it in `.github`-only PRs | RMV-V1-002 |
| `.taskman` | keep | the native Tasks store of this repository; B10 moves Tasks specs, not the store | not named in this change |
| `artifacts` | listed in B6b | `cem-release-27e9d98` and `runner-{plan,receipt}-*.json` are CEM release inputs | `artifacts/runner-` named by `internal/cem/verify/stable_receipt_auth_test.go`, `internal/cem/wire/stable_auth_test.go`, `interop/cem01-go/testdata/multi-runner/EXPECTED.json` |
| `assets` | keep | `assets/brand` holds the Corvint lockups and marks (`assets/brand/README.md`) | not named in this change; no code caller |
| `benchmarks` | see Benchmarks below | | |
| `cmd` | see Commands below | | |
| `conformance` | see Conformance below | | |
| `docs` | keep | product documentation and history; B9/B10 edit specs in place | RMV-V1-026, RMV-V1-027 |
| `evidence` | undetermined | `evidence/issues-167-175` has no code caller; missing: an owner rule for retained issue evidence | `git grep -F issues-167-175`: `.corvint/test-selection.json` only |
| `examples` | keep `examples/documentation-corpus`; `examples/evidence-provider` listed in B4 | the corpus feeds the docs product tests | `internal/doccorpus/adapter_test.go` |
| `experimental` | listed in B5 | `experimental/analyzers` | design.md B5 |
| `extensions` | listed in B4 | `extensions/vscode` moves to beamfall/corvint-vscode (owner decision R33) | RMV-V1-012 |
| `flows` | see Flow files below | | |
| `integrations` | keep | Claude Code and Codex integration packages are product surface | `cmd/corvint/adopt_setup.go` reads both |
| `internal` | see Internal packages below | | |
| `interop` | `interop/cem01-go` listed in B6b; `interop/spec-toolchains` keep | spec-toolchain conformance stays with the spec product | `mk=548`; `gh=workflows/spec-toolchain-conformance.yml`; `st=script/spec-toolchain-latest.sh` |
| `protocol` | `protocol/cem-*` listed in B6b; `protocol/integrations/speckit` listed in B4; rest keep | the Apache-2.0 protocol boundary stays (LICENSING.md) | RMV-V1-019, RMV-V1-013 |
| `script` | see Scripts below | | |
| `testing` | undetermined | `testing/context-retrieval-goldens.json` is a context evaluation corpus; missing: whether the 1.0.x context measurement (owner decisions R28/R29) uses it | `ot=benchmarks/eval-split-v0.json,benchmarks/manifest.json` |
| `tests` | keep | `tests/README.md` records which current checks replaced the retired Python drivers | no code caller; history pointer |
| `tools` | see Tools below | | |

## Commands (`cmd/*`) not named in a batch

`cmd/corvint-analyzer-*` (16) are listed in B5; `cmd/corvint-test-validity-mcp` in B2;
`cmd/corvint-go-toolchain-receipt` in B6b; `cmd/corvint-cli`, `cmd/corvint-mcp` are edited in B5.

| Path | Verdict | Reason | Check |
| --- | --- | --- | --- |
| `cmd/corvint` | keep | the product binary | |
| `cmd/corvint-docs` | keep | docs product command | `mk=619` (capability catalog) |
| `cmd/corvint-docs-mcp` | keep | docs product stdio MCP server | imports `internal/companioncli`, `internal/procprofile` |
| `cmd/corvint-update` | keep | the optional verified updater the owner uses | `mk=623` |
| `cmd/corvint-readiness-record` | keep | builds or verifies the stable readiness record (release path) | `internal/capcatalog/trees/corvint-readiness-record.json` |
| `cmd/corvint-release-candidate` | keep | assembles qualified release candidates | `internal/capcatalog/trees/corvint-release-candidate.json` |
| `cmd/corvint-release-install` | keep | installs one verified host core archive | `internal/capcatalog/trees/corvint-release-install.json` |
| `cmd/corvint-web-flows` | keep | experimental browser companion used by the web-flows gate | `.corvint/runtime-edges.json`; `gh=workflows/ci.yml` via `tools/web-flows` |
| `cmd/corvint-go-test-provider` | keep until G23, then fold into `corvint` | owner decision R14: retired after the built-in contract lands; not a separate 1.0 artifact | `mk=620`; `gh=workflows/vscode-extension.yml` |
| `cmd/corvint-js-test-provider` | keep until G23, then fold into `corvint` | owner decision R14 | `mk=621` |
| `cmd/corvint-ruby-test-provider` | keep until G23, then fold into `corvint` | owner decision R14 | `mk=622` |
| `cmd/corvint-test-runner` | undetermined | experimental test-runner companion over `internal/testrunner/cli`, a product row in `tools/release-binary/binaries.json`; owner decision R14 names provider binaries only; missing: whether R14 or the G23 contract retires it | named in design.md B5 (`csharp_regions.go`); `binaries.json` row 22 |

## Internal packages (`internal/*`) not named in a batch

The 11 `internal/analyzer*` rows below are covered by the B5 wildcard `internal/analyzer*`.

| Path | Verdict | Reason | Check |
| --- | --- | --- | --- |
| `internal/analyzerdotnet`, `analyzergo`, `analyzerhtmlcss`, `analyzerjs`, `analyzerkotlinandroid`, `analyzernativebridge`, `analyzerruby`, `analyzershader`, `analyzershell`, `analyzerstructured`, `analyzerswift` | listed in B5 | only importers are `cmd/corvint-analyzer-*` | `imp=` one or three `cmd/corvint-analyzer-*` each |
| `internal/archivepath` | keep | used by the updater | `imp=internal/update,tools/compat-trial` |
| `internal/browsercov` | keep | Playwright coverage | `imp=internal/appflows/e2erunner,internal/testrunner/registry` |
| `internal/capcatalog` | keep | capability catalog | `imp=cmd/corvint,internal/cliapp,...` (5) |
| `internal/clierror` | keep | shared CLI errors | `imp=` 14 |
| `internal/clitree` | keep | shared CLI tree | `imp=` 14 |
| `internal/command/flows` | keep | docs flows command | `cmd/corvint-docs/flows.go` |
| `internal/companioncli` | keep | docs and MCP companions | `imp=` 8 |
| `internal/diagnostic` | keep | diagnostic contract; `script/diagnostic-coverage.count` is its ratchet | `imp=cmd/corvint,cmd/corvint-docs,internal/contextindex` |
| `internal/doccitations`, `doccompiler`, `docmaintain`, `docsenvelope`, `docstools` | keep | docs product | `imp=cmd/corvint-docs` (each) |
| `internal/execfile` | keep | test helper for kept packages | no non-test importer; test files in 71 directories import it, including `cmd/corvint` |
| `internal/flowcoverage`, `internal/flowdocs`, `internal/observedstale`, `internal/requestpath` | keep | docs flows | `imp=internal/command/flows` or `cmd/corvint-docs` |
| `internal/genesis`, `jobinputs`, `testreuse`, `plansnapshot`, `testevidencedoc` | keep | `corvint` features | `imp=cmd/corvint` |
| `internal/gitancestry`, `gitstatus`, `groupreap`, `jsonend`, `procgroup`, `procprofile`, `rootalias`, `runtimeenv`, `secretscreen`, `versionline`, `workspace` | keep | shared infrastructure | `imp=` 5 to 24 each, including `cmd/corvint` |
| `internal/gitprobe` | keep | test helper for kept packages | test imports from `internal/gitstatus`, `internal/jstestprovider` |
| `internal/gotestrun` | keep until G23, then fold with its provider | owner decision R14 | `imp=cmd/corvint-go-test-provider` |
| `internal/jslinecov`, `internal/linecov` | keep | line coverage for the blocking runners | `imp=` 4 and 11 |
| `internal/lineanchor`, `projectpath`, `repopath`, `projectprofile`, `taskprojection`, `untrackedallowance`, `webdialect`, `stepnegation` | keep | used by kept context, docs or selection packages | `imp=` 1 to 5 each |
| `internal/mergedrivers` | remove in B8b | only importer is `tools/merge-drivers` (B8b) | `imp=tools/merge-drivers` |
| `internal/observations` | keep | writer of the bounded self-observation ledger (invariant 4) | `imp=cmd/corvint,...` (4) |
| `internal/proctrace` | keep | runner registry | `imp=internal/testrunner/registry` |
| `internal/pythongrammar`, `internal/pythonsyntax` | keep | pytest is a blocking runner | `imp=internal/contextindex,internal/liveverify/pyresolve,...` |
| `internal/releasegate` | keep | release hostile-regression gate | `st=script/check-hostile-regressions.sh` |
| `internal/repoenvelope` | keep | MCP bridge | `imp=internal/mcp/*` (4) |
| `internal/rubyautoload`, `rubydialect`, `rubylinecov`, `rubyprofile` | keep | RSpec and Minitest are blocking runners; `rubyprofile` is the Ruby profile conformance test | `imp=internal/contextindex,internal/liveverify/affected/ruby,...`; `rubyprofile` is test-only |
| `internal/runhygiene` | keep | appflows, JS provider, Playwright runs | `imp=` 3 |
| `internal/specindex` | keep | spec index gate | `mk=384` |
| `internal/testplatform` | keep | test support | `imp=internal/testsupport` |
| `internal/update` | keep | updater | `imp=cmd/corvint-update` |
| `internal/workflowshell` | remove in B11 | only test importers are `tools/ci-forward-row` and `tools/ci-reuse-plan` | `git grep`: `tools/ci-forward-row/narrowing_test.go`, `tools/ci-reuse-plan/self_hosted_runner_test.go` |
| `internal/docviews` | remove in B1 | orphaned code (owner decision R33); amends 0594 item 3 | zero importers (VERIFIED, design.md verification table); now in the B1 list |

## Tools (`tools/*`) not named in a batch

| Path | Verdict | Reason | Check |
| --- | --- | --- | --- |
| `tools/affected-edit-oracle` | remove in B1 | no caller | `mk=- gh=- st=- ot=-` |
| `tools/affected-zero-unknown-replay` | keep | AZU-V0-009 frozen merge-tree replay of selection safety | `st=script/affected-zero-unknown-replay.sh` |
| `tools/cem-structural-closure` | remove in B6b | digests the Go parser closure for CEM structural predicates | `ot=protocol/cem-1.0/structural-corpus/CORPUS.json` |
| `tools/check-analyzer-go.sh` | remove in B5 | analyzer check | `ot=` perf raw output only |
| `tools/ci-flake-report` | remove in B11 | bespoke CI report | `gh=flake-report/report.sh,workflows/flake-report.yml` |
| `tools/ci-nightly-signal` | keep | tests the live `.github/nightly-signal` release-gates script | `tools/ci-nightly-signal/doc.go` |
| `tools/ci-reuse-plan` | remove in B11 | bespoke CI selection | `gh=ci-cadence/classify.sh,forward-rows/github-guard.sh,workflows/*` |
| `tools/ci-reuse-replay` | remove in B1 | Python replay with no caller | `mk=- gh=- st=- ot=-` |
| `tools/ci-shard-costs` | remove in B11 | bespoke CI sharding | `gh=workflows/ci.yml` |
| `tools/ci-test-slices` | remove in B11 | bespoke CI slicing | `gh=cishards/slices.go,cishards/slices_test.go` |
| `tools/cli-discovery-gen` | keep | capability catalog generator | `mk=624,626` |
| `tools/compat-trial` | undetermined | preregistered trial harness; missing: whether frozen evaluations stay (see Benchmarks) | `st=` fixtures; `ot=benchmarks/compat-trial-v0.json` |
| `tools/conflict-markers-check` | keep | repository hygiene gate | `mk=68,1153,1474` |
| `tools/corvint-pr-tests` | remove in B11 | bespoke CI selection | `gh=cishards/test-commands.json,forward-rows/narrow.jq` |
| `tools/cw-trial` | undetermined | agent trial harness; same missing rule | `st=` fixtures and `script/measure-context-packet.py` |
| `tools/docs-blinded-task` | keep | operator harness for SDD-V0-014/015 (docs product evaluation) | `ot=benchmarks/docs-blinded-maintainer-v0/tasks.json`; imports `internal/docmaintain` |
| `tools/docs-ci-plan` | remove in B11 | bespoke docs-only CI plan | `gh=ci-cadence/classify.sh,qualifications/docs-only.json` |
| `tools/fixture-maintenance-check` | remove in B1 | no caller outside `tools/ci-forward-row` fixtures | `st=tools/ci-forward-row/testdata/*` only |
| `tools/gate-affected-select` | remove in B11 | bespoke selection | `mk=150,986,1006`; `gh=forward-rows/mq-narrow.sh` |
| `tools/gotest-evidence` | remove in B1 | only named by the `tools/affected-edit-oracle` corpus (B1) | `st=tools/affected-edit-oracle/testdata/corpus.json` |
| `tools/heading-nav-bench` | undetermined | benchmark harness; same missing rule | `st=` fixtures only |
| `tools/live-test-run` | keep | live test target | `mk=528,532`; `gh=cishards/live_test.go` |
| `tools/native-bridge-pre-review.sh` | remove in B5 | analyzer native-bridge review | `ot=` perf raw output only |
| `tools/paired-trial` | undetermined | paired agent-trial analysis (AHI-217); same missing rule | `st=tools/cw-trial/gate.go` |
| `tools/pi-tui-fixture` | remove in B1 | PTY driver for Pi tests; no Pi integration exists under `integrations/` | `st=tools/ci-forward-row/testdata/*` only |
| `tools/plan-delta` | remove in B1 | no caller | `mk=- gh=- st=- ot=-` |
| `tools/plugin-dirs` | keep | plugin build target | `mk=1345,1352` |
| `tools/retrieval-bench` | undetermined | context retrieval experiment; missing: the 1.0.x context measurement design (owner decisions R28/R29) | `st=` fixtures and line-citation inventories |
| `tools/run-measure` | remove in B8b | lane-host timing helper (ASD-V0-570) | `ot=.gitignore` only |
| `tools/selection-baseline` | remove in B11 | bespoke selection baseline | `mk=986-1003`; `gh=ci-cadence/classify.sh,ci-cadence/run.sh,workflows/ci.yml` |
| `tools/shell-test-select` | remove in B11 | bespoke shell-test selection | `mk=1014,1022,1024`; `gh=workflows/ci.yml` |
| `tools/str-parity` | undetermined | STR-V0-013 parity check of the session test runner; missing: whether G23 keeps the session runner contract | `st=tools/ci-forward-row/testdata/*` |
| `tools/test-commands-check`, `tools/test-commands-run` | keep | test command contract | `mk=19,120,585` and `mk=18,173,586` |
| `tools/test-exec-writes-check` | keep | test hygiene gate | `mk=121,726` |
| `tools/test-read-scopes-check` | keep | read-scope gate | `mk=122,571` |
| `tools/test-shards` | keep | used by `corvint test` sharding | `mk=1335,1340`; `ot=cmd/corvint/session_test_shards_unix_test.go` |
| `tools/tiers-replay` | remove in B11 | bespoke CI tiers | `gh=forward-rows/github-guard.sh` |
| `tools/typed-reach-measure` | remove in B1 | one-off AFP-V0-2378 measurement; no caller | `mk=- gh=- st=- ot=-` |
| `tools/unbounded-readers` | keep | doc gate in the CI list | `mk=126,564` |
| `tools/web-flows` | keep | web-flows gate | `gh=workflows/ci.yml` |

## Scripts (`script/*`) not named in a batch

| Path | Verdict | Reason | Check |
| --- | --- | --- | --- |
| `affected-zero-unknown-replay.sh`, `affected-zero-unknown-merges.txt` | keep | driver and input of `tools/affected-zero-unknown-replay` | `st=tools/affected-zero-unknown-replay/main.go` |
| `agent-preflight.sh`, `agent-preflight_test.sh` | remove in B8b | lane preflight | `mk=1287` (test target only) |
| `atm-fixture-store.sh` | undetermined | Tasks fixture store for demos; missing: whether `conformance/interactive-alpha` and `benchmarks/daily-loop-v0` stay after B10 | `ot=benchmarks/daily-loop-v0/corpus.json,conformance/interactive-alpha/*` |
| `batch-assemble*`, `batch-precheck*`, `batch-regen.sh`, `batch-review-packet.sh` | remove in B8b | batch landing tooling | `mk=937-965,1034` |
| `capability-catalog_test.sh` | keep | tests the capability catalog | `mk=613,630` |
| `capability-screen.sh` | remove in B8b | friction-ticket screen | `st=script/lane-host-tools_test.sh` only |
| `changed-checks.sh`, `changed-checks_test.sh` | keep | changed-files mode of three doc gates | `mk=824,1492` |
| `check-ai-coauthor-trailers.sh`, `_test.sh` | keep | CI doc-gates job runs it on pull requests (CIN-V0-006) | `mk=968-984,1029` |
| `check-analyzer-python-offline-build*`, `check-analyzer-python-ratchets*` | remove in B5 | analyzer lints (RMV-V1-024) | `mk=795,799` |
| `check-binary-blobs.sh` | keep | repository hygiene | `mk=687` |
| `check-ci-least-privilege*`, `ci-least-privilege-exceptions.tsv` | keep | CI permission gate | `mk=536,541,545` |
| `check-decision-numbers*`, `check-dr-numbers.sh` | keep | decision record gates | `mk=441,445,1275` |
| `check-diagnostic-coverage.sh`, `diagnostic-coverage.count` | keep | diagnostic ratchet read by `internal/diagnostic/gate_test.go` | `mk=1038,1041` |
| `check-doc-gate-targets.sh` | keep | doc gate list check | `mk=358` |
| `check-error-code-ownership*`, `error-code-ownership.allowlist` | keep | error code gate | `mk=807,842,846` |
| `check-go-format*` | keep | format gate | `mk=504,508` |
| `check-host-package-versions*`, `host-package-version*` | keep | integration package version gate | `mk=642-648,740,744` |
| `check-hostile-regressions*` | keep | release gate | `mk=1052,1056`; `gh=workflows/release-gates.yml` |
| `check-install-lifecycle*` | keep | release gate | `mk=1045`; `gh=workflows/release-gates.yml` |
| `check-lane-script-tests.sh` | remove in B8b | dogfood seal preflight for lanes | `st=` one lane file |
| `check-license-headers*` | keep | license gate | `mk=493,497` |
| `check-line-citations*`, `line-citations-resolve.sh`, `line-citation-drift-waivers.txt`, `line-citation-legacy-documents.txt`, `line-citation-unpinned-inventory.txt` | keep | citation gate and its inputs | `mk=514,517,521,658` |
| `check-release-artifact-reproducibility*` | keep | release gate | `mk=803` |
| `check-requirement-definitions*`, `requirement-definitions-outside-section.txt` | keep | spec gate | `mk=414,417,420` |
| `check-spec-claims*`, `check-spec-readme.sh` | keep | spec gates | `mk=331,334,363` |
| `check-sql-native-ratchets*` | remove in B5 | analyzer ratchet | `mk=1064,1071` |
| `check-test-script-refs.sh` | keep | test script reference gate | `mk=1270` |
| `check-traceability-tests*` | keep | spec traceability gate | `mk=425,429` |
| `citeperf-measure.py` | remove in B1 | one-off V1-4851/4852 measurement; no caller | `mk=- gh=- st=- ot=-` |
| `codex-review.sh`, `codex-review_test.sh` | remove in B8b | lane review wrapper | `mk=1291` (test target only) |
| `core-n1-replay-default_test.sh`, `core-n1-source*`, `select-previous-release*` | keep | N-1 release compatibility | `mk=1046-1048,1077,1086`; `gh=workflows/release-gates.yml` |
| `doc-gate-timed.sh`, `doc-gates-summary.sh` | keep | run `make doc-gates`, which stays | `mk=1186,1192,1200` |
| `gate-affected-corvint.sh`, `_test.sh` | keep | fast tier that runs `corvint test` from this checkout | `mk=176,183,189` |
| `gate-affected.sh`, `gate-affected_test.sh` | remove in B11 | bespoke selection kept as rollback; RMV-V1-028 conditions apply | `mk=180,188`; `gh=cishards/test-commands.json` |
| `gen-documentation-paths*` | keep | docs index generator | `mk=662,666,670` |
| `gen-spec-requirements*`, `spec-requirements-*` | keep | `make spec-requirements` | `mk=343-399` |
| `github-split-check.sh`, `_test.sh` | remove in B8b | batch push check | `st=script/batch-precheck.sh`; `mk=1256` |
| `go-archive-gate-injection_test.sh`, `go-archive-gate-injection-interrupt_test.sh` | remove in B11 | tests of `go-archive-gate` (B11) | `mk=230,231` |
| `go-cache-trim*` | remove in B8b | lane host cache trim | `mk=885,889` |
| `go-test-shard.sh` | undetermined | DTL-V0-007 package partition; missing: a runner caller (only a fixture test names it) | `st=tools/test-commands-check/dogtools_test.go` |
| `goos-filename-allowlist.txt` | remove in B8a | input of `check-goos-filenames`, folded into `cross-vet` (RMV-V1-024) | `mk=861` |
| `heavy-groups.sh` | remove in B8b | lane heavy grouping | `st=script/lane-scripts_test.sh` only |
| `install-local-bin.sh` | keep | local install target | `mk=1501` |
| `known-flakes.txt` | remove in B8b | input of `pr-landing-watch.sh` | `st=script/pr-landing-watch.sh` |
| `lane-*` (26 files, including `lane-handback.sh`, `lane-screen.pl`, `lane-transcript-metrics*.py`) | remove in B8b | lane tooling (RMV-V1-025) | `mk=1253-1281` test targets only; `lane-handback.sh` has no caller |
| `measure-context-packet.py`, `_test.py` | undetermined | context packet measurement; missing: the 1.0.x context measurement design (owner decisions R28/R29) | `st=` its own test |
| `measure-worktree-index-share.sh` | undetermined | measures linked-worktree index sharing; missing: the `watch`/`index` measurement in RMV-V1-023 | `mk=- gh=- st=- ot=-` |
| `merge-regenerate-specs.sh`, `merge-use-case-pins.sh` | remove in B8b | merge drivers | `ot=.gitattributes` |
| `no-python-runtime-dependency_test.sh` | keep | GOC-V0-008 assertion | `mk=779` |
| `pr-landing-watch.sh` | remove in B8b | lane landing watch | `st=` lane files |
| `pre-push-windows-vet*` | keep | `make windows-vet` twin | `mk=912,916` |
| `process_group.py` | remove in B4 | imported only by `qualify-lsp.py`, `measure-lsp-quality.py` (B4) and `qualify-checkpoint-handoff.py` (B1) | `git grep process_group -- script tools` |
| `proof-tour-test.sh` | remove in B6b | tests `script/proof-tour.sh`, which B6b removes | file header |
| `qualify-checkpoint-handoff.py` | remove in B1 | experimental FPK-V0-020..026 pilot; no caller | `ot=.taskman/tickets/V1-0473.json` only |
| `register-shell-test*` | keep | shell-test registration | `mk=1127,1480` |
| `release-checklist`, `release-checklist_test.sh` | keep | release checklist | `mk=106,746-748`; `gh=workflows/release-gates.yml` |
| `repin.sh`, `repin_test.sh` | keep; B5 edits it | one command for mechanical repins; B5 drops its analyzer-schema step | file header (DCG-V0-801) |
| `require-full-history.sh` | remove in B8b | batch and lane guard | `st=script/batch-precheck.sh,script/lane-setup.sh,...` |
| `requirement-id-reservations*`, `requirement-ids-free.sh`, `requirement-next-id*` | remove in B8b | requirement-ID reservation tooling (RMV-V1-025) | `mk=398,399,1216-1242` |
| `retained-data-digest.sh` | remove in B8b | named only by a lane test | `st=script/lane-scripts_test.sh` |
| `review-head-check*` | remove in B8b | lane review check | `mk=1295` (test target only) |
| `runtime-environment_test.sh` | remove in B1 | already removed on the cut branch (commit 245e4ab8a) | `mk=1303` at integ |
| `scratch-sweep*` | remove in B8b | lane scratch cleanup | `mk=896,900` |
| `seed-planning-store*` | undetermined | seeds a Tasks store from a plan; missing: whether `conformance/interactive-alpha` stays after B10 | `mk=1309`; `ot=conformance/interactive-alpha/core-planning-proof.mjs` |
| `selection-contract-check.sh` | remove in B11 | replays CI and local parity over the selection baseline | `mk=1011` |
| `spec-acceptance-audit.sh` | keep | spec gate | `mk=407` |
| `spec-coverage-audit` | undetermined | Go spec coverage tool with no caller; missing: whether the spec health classes (owner decision R37) use it | `st=` fixtures only |
| `spec-toolchain-latest*` | keep | spec toolchain check | `mk=556,562` |
| `ssh-heredoc-check.sh` | remove in B8b | lane host check | `st=` one lane file |
| `test-rg-fallback*` | keep | sourced by kept shell tests | `mk=910` |
| `web-flows-gate` | keep | web-flows gate | `gh=workflows/ci.yml` |
| `wsl-keepalive-check.sh`, `wsl-keepalive.ps1` | remove in B8b | lane host keepalive | `st=` lane files |

## Benchmarks and conformance not named in a batch

| Path | Verdict | Reason | Check |
| --- | --- | --- | --- |
| `benchmarks/blind-v2`, `blind-v3`, `blind-v4`, `blind-v6-dev`, `blind-v7`, `cases`, `cem2-discovery-replay-v0`, `daily-loop-v1`, `expert-audit-followup`, `parity-fixture-index`, `parity-snapshot-discovery`, `untouched-repository-v1` | undetermined | frozen evaluation data with no code caller; missing: an owner rule for frozen evaluation corpora (keep, or remove with Git history as the record) | `mk=-`; no Go, shell, mjs or workflow caller (`untouched-repository-v1` is named only by `script/check-license-headers.sh`) |
| `benchmarks/docs-blinded-maintainer-v0` | keep | input of `tools/docs-blinded-task` | `ot=` |
| `benchmarks/selection-baseline` | remove in B11 | corpus of `tools/selection-baseline` | `mk=987`; `st=script/batch-precheck.sh,tools/gate-ledger/main.go` |
| `benchmarks/snapshot-reader` | undetermined | isolated reader-format experiment (AT-04) in the root module; missing: the `index` decision in RMV-V1-023 | `gh=cishards/live_test.go` names it as a module fixture |
| `conformance/go-live-test-v0` | keep | Go provider conformance; folds with the provider (owner decision R14) | `cmd/corvint-go-test-provider/main_test.go` |
| `conformance/go-query-start-v0` | keep | context receipt budget input | `internal/contextindex/receipt_budget_test.go` |
| `conformance/human-documentation-compiler-v0` | keep | docs product conformance | 3 Go files |
| `conformance/interactive-alpha` | undetermined | node harness; names Tasks fixtures and `extensions/vscode/test/installed/run.mjs`; missing: whether it stays after B4 and B10 | `mk=1331` |
| `conformance/mcp-2026-07-28` | keep | MCP protocol conformance | 5 Go files |
| `conformance/mcp-test-validity-v0`, `flows/installed-test-validity-mcp.json` | remove in B2 | test-validity MCP (G24) | `ot=` |
| `conformance/perf-v0` | keep | perf budget conformance (RMV-V1-029) | `st=script/release-checklist_test.sh` |
| `conformance/release-artifact-v0` | keep | release gate | `gh=workflows/release-gates.yml` |
| `conformance/ruby-profile-v0` | keep | Ruby profile fixture | `internal/rubyprofile/profile_test.go` |
| `conformance/test-validity-v0` | keep | vectors read by `internal/testvalidity/vectors_test.go` | B2 must recheck if `internal/testvalidity` leaves |
| `conformance/tcq-0`, `conformance/use-cases-v0` | listed in B6 (check) | RMV-V1-019 open item | |
| `flows/installed-go-live-tests.json`, `flows/installed-js-live-tests.json`, `flows/web-flows-browser-gate.json` | keep | installed live-test and web-flows flows | |
