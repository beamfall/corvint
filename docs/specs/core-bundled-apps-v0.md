# Core and Bundled Apps V0

Owner: Russell Lewis
Date: 2026-11-06
Requirement prefix: `CMS-V0`
Intent status: accepted (owner, 2026-11-06)
Delivery status: in-progress (docs edges cut and flows commands forwarded, 2026-11-09)
Authoritative inputs:
- decision 0591;
- `AGENTS.md` invariant 7 as amended by 0591;
- decision 0081;
- decision 0595 (`corvint flows` and `corvint how` move to the docs app);
- `core-compatibility-freeze-v1.md` (Core verb list);
- restructure tickets labelled `restructure-2026-11` (V1-4800..4829, V1-4898..4909).

## Agent digest
- Claim: Corvint splits into a core plus bundled apps and plugins, each measured for LLM fit, test isolation, plugin cost, speed and memory.
- Status: accepted (owner, 2026-11-06) / in-progress (docs edges cut and flows commands forwarded, 2026-11-09)
- Exists: one root module; `corvint docs` and `--corpus=FILE` are exec-only shims onto the `corvint-docs` binary, and `internal/mcp/docsbridge` reaches it through the `docs-tool` envelope (V1-4907, 2026-11-09); `corvint flows` and `corvint how` are exec-only shims onto `corvint-docs` too (decision 0595, `CMS-V0-912..916`), with `flows test-evidence` native in `corvint`; `corvint` still links the shared flow readers (`internal/appflows`, `internal/appmap`, `internal/flowcoverage`) for test selection; Tasks left with decision 0594.
- Blocked on: baseline measurements (`CMS-V0-005`, `-006`, `-008`, `-010`, `-011`) before the docs module split (V1-4901) and the remaining extractions.
- Read next: Requirements; Units and edges to cut; Acceptance and evidence.

## User job and verified baseline

There are two users:

- An agent or developer changing one area. They need to read, test and ship that area without
  loading or linking the rest.
- An operator. They need the default install to stay local, fast and governed by one release.

At base `1a401eb5ee68ff4a0541534b33a5d6538793e9ea`:

- `go.mod` declares one root module.
- `cmd/corvint` imports `internal/doccorpus` 34 times, `internal/appflows` 9 times, and
  `internal/taskman` 3 times.
- `internal/command/breakage` imports `appflows`.
- `internal/mcp/tasksbridge`, `internal/mcp/tasksread`, `internal/mcp/docsbridge` and `internal/mcp/bridge` (via `flowcoverage`) link
  the apps into the adapters.

## Units and edges to cut

| Unit | Contents (indicative) | Edges to cut before extracting |
| --- | --- | --- |
| Core `corvint` | index, query/context, impact, `affected`/test selection, review, CEM/dogfood evidence, `corvint-test-runner` | n/a (root module) |
| Kernel (leaf packages) | git runner, `gitprobe`, `gitstatus`, language ID, `execfile`, `procgroup`, `secretscreen`, `analyzerenvelope`, `langplugin`, `protocol/**` | upward imports (V1-4809) |
| Tasks `corvint-tasks` | `internal/tasks/**`, `taskman`, `taskprojection`, `taskbarrier`, `localcompletion`, `scopelease`, `workqueue`, `worksource`, `corvint-work-queue` | `tasks`→`contextindex` (V1-4810); `cmd/corvint`→`taskman`; `internal/mcp/tasksbridge`, `internal/mcp/tasksread` |
| Docs `corvint-docs` | doccorpus, doccompiler, docmaintain, doccitations, docstools, `cmd/corvint-docs`, `corvint-docs-mcp` (decision 0594 deleted corpus index/serve/republish and `corvint-corpus-*`) | cut (V1-4907): `cmd/corvint` forwards `docs` and `--corpus=FILE` to `corvint-docs`; `internal/mcp/docsbridge` runs its `docs-tool` verb (`internal/docsenvelope`); still open: `internal/mcp/toolgroup` and `internal/mcp/cli` → `doccorpus`/`corpusbridge` for the corpus tools |
| Flows `corvint-web-flows` | appflows, appmap, funclink, browsercov, flowcoverage, flowdocs (the E2E proof lab left with decision 0593) | cut (decision 0595): `cmd/corvint`→`internal/command/flows`, the `flows` and `how` commands now run in `corvint-docs`; still open: `cmd/corvint`→`appflows` (kept on purpose while the readers are `shared`, `CMS-V0-915`); `command/breakage`→`appflows`; `internal/mcp/bridge`→`flowcoverage` |
| Adapters | `corvint-mcp`, `corvint-lsp`, test-validity MCP | bridges call apps through envelopes |
| Plugins | analyzers; test providers only if measured (V1-4824) | registries (V1-4818); `corvint-plugin/1` (V1-4819) |
| Release/update (in manifest) | `corvint-update`, `corvint-release-install` | none |
| Developer tooling (not in manifest, under `tools/`) | release-candidate (Core-only), readiness-record, go-toolchain-receipt (decision 0594 deleted public-release-check, companion-release and cem-experiments) | none |

## Requirements

- `CMS-V0-001`: Each bundled app MUST build and test from its own module, importing only:
  - kernel packages;
  - published protocol packages;
  - its own packages.

  An import ratchet MUST fail on any new cross-unit edge. A write ratchet MUST fail any app package
  that opens a path under `.corvint/` evidence, the index or trace stores for writing.
- `CMS-V0-002`: A release MUST have one version and one checksummed manifest, and every entry MUST
  cite its accepted decision. The release check MUST refuse an entry that cites none.
  `corvint-update` MUST install and roll back the whole set. Every executable MUST refuse to run
  beside a mismatched version, with a named error. A test MUST interrupt an install and show that
  the set either refuses with that error or is restored.
- `CMS-V0-003`: No Core verb in `core-compatibility-freeze-v1` may forward to a bundled app. A test
  MUST run every Core verb with no bundled-app binary on PATH. A non-Core `corvint <sub>` kept for
  compatibility MUST be an exec-only shim that passes no authority and reports a missing app as a
  named `NOT_INSTALLED` gap.
- `CMS-V0-004`: Each extraction MUST be one PR or stack. It MUST keep CLI names, flags and wire
  formats byte-identical, or ship a recorded migration. It MUST keep frozen-eval parity and stay
  within the existing hot-path benchmark budget.
- `CMS-V0-005`: `corvint affected` MUST treat an edit to a kernel or protocol package as affecting
  every app that uses it. The frozen selection replay set MUST be measured before the first
  extraction and after each one.
  - Pass: no recall loss.
  - Pass: precision and CI test-plan size within 5% of baseline.
- `CMS-V0-006`: Install size and cold-start time of the default set MUST be recorded at each
  extraction. Growth beyond 10% needs an owner note.
- `CMS-V0-007`: Bundled-app modules MUST:
  - live in this repository;
  - require the root module through an in-repo `replace` in both release and development builds;
  - never be published or tagged on their own.

  The release build MUST NOT depend on `go.work`. A module boundary MUST NOT cross a licence
  boundary in `LICENSING.md`.
- `CMS-V0-008`: Each unit MUST have an `AGENTS.md` route naming its purpose, entry points, envelopes
  and test command. A check MUST flag production packages over about 8k lines that have no split
  note. For representative single-unit tickets, the `corvint context` pack MUST shrink against
  baseline, with no recall loss on the frozen context eval.
- `CMS-V0-009`: Each unit's tests MUST run alone with no cross-unit fixture paths. Shared fixtures
  live only in kernel or `protocol/**` testdata. For an app-only edit, `affected` MUST select only
  that unit plus its envelope contract tests.
- `CMS-V0-010`: A plugin boundary MUST be accepted only when its per-run overhead, with batched
  calls served by one process per run, stays within 5% of `index` and `affected` wall time on the
  frozen repositories. Otherwise the implementation stays linked behind its registry.
- `CMS-V0-011`: Cold start, peak RSS and binary size for `corvint affected`, `context` and
  `index --if-stale` MUST be recorded at baseline and after each extraction.
  - Pass: none regresses beyond noise.
  - Pass: the core binary is smaller after the docs, flows and Tasks extractions.
- `CMS-V0-912`: (owner decision 0595, 2026-11-09) The `flows` command group and its `how` alias
  MUST run in `corvint-docs`: `corvint-docs flows ...` and `corvint-docs how ...` take the argument
  vectors `corvint flows ...` and `corvint how ...` took and write the same stdout, exit codes and
  refusals. Their help spells the binary `corvint-docs`, and every registered `flows` subcommand
  MUST be a node of the `corvint-docs` command tree.
- `CMS-V0-913`: `corvint flows ...` and `corvint how ...` MUST be exec-only shims onto
  `corvint-docs` under `CMS-V0-003`, with the `corvint docs` shim's behaviour: the argument vector
  is forwarded unchanged, the child's bytes and exit code are returned, no authority is passed, a
  missing `corvint-docs` is the named `NOT_INSTALLED` gap (code `not-installed`, exit 2), help is
  answered natively and names `corvint-docs`, and `--require-clean` is refused by the shim.
- `CMS-V0-914`: `flows test-evidence` MUST stay native in `corvint`, because it joins flow links
  to test evidence from the tests layer, which the docs layer may not import. `corvint-docs flows
  test-evidence` MUST refuse with exit 2 and a message naming `corvint flows test-evidence`.
- `CMS-V0-915`: The tests layer MUST read flows in process, through packages the import manifest
  keys `shared` (`internal/appflows`, `internal/appmap`, `internal/flowcoverage`). No tests-layer
  package may import a docs-layer package, or run `corvint-docs`, to read a flow; the code -> flow
  -> E2E test selection path MUST work with no `corvint-docs` installed.
- `CMS-V0-916`: Moving the commands MUST add no cross-layer import edge. The baseline
  `tools/import-ratchet/baseline.json` only shrinks: the edge `cmd/corvint ->
  internal/command/flows` is gone and MUST NOT return, and no baselined edge may start at
  `cmd/corvint` or end at `internal/command/flows` or `cmd/corvint-docs`.
- `CMS-V0-930`: (proposed 2026-11-12; delivery experimental; V1-5466) The `not-installed` refusal
  of `corvint docs`, `corvint flows`, `corvint how`, the `corvint-docs --corpus` core read and the
  MCP docs bridge MUST name the exact install command
  `corvint-update apply --allow-network --component core --bin-dir DIR`, with `DIR` the running
  executable's directory after symlink resolution, single-quoted when it needs shell quoting, and
  without `--bin-dir` when that directory cannot be read (`UPD-V0-920`). It MUST also point at the
  manual core-set archive install in `docs/INSTALL.md`. When the missing executable is `corvint`
  itself (the `corvint-docs --corpus` read), the refusal MUST name only that manual install,
  because the updater replaces an installed `corvint` and cannot place a first one. Where the
  command would refuse, it MUST name only the manual install and say why: no `corvint` is beside
  the running executable, or the `corvint` there is unnumbered (build 0 or none, or no answer to
  `--version`), which the updater does not replace (`UPD-V0-010`). The MCP not-installed `next`
  step MUST be the same hint for the server's own directory. The
  `version-mismatch` refusal MUST name the same command. The code and exit status of both refusals
  are unchanged.

## Non-goals, trust boundary and failure modes

Out of scope:

- `.so` plugins;
- a plugin marketplace, remote fetch or third-party trust model;
- any new language, daemon, service, network path or database;
- a second repository or a separately versioned app;
- any change to what "companion" means in PUB-V0 and CCF-V1.

Bundled apps hold no authority beyond the core's and are untrusted callers of its envelopes.

| Failure | Guard |
| --- | --- |
| The manifest grows without a decision | `CMS-V0-002` |
| Version skew, or a partial install or rollback | `CMS-V0-002` |
| Code duplicated across modules | kernel packages; ratchet (`CMS-V0-001`); deadcode report (V1-4801) |
| Cross-module cycles | ratchet (`CMS-V0-001`) |
| CI growth | derived shard lists (V1-4802); `CMS-V0-005` |
| An app writes core state | write ratchet (`CMS-V0-001`) |
| Core depends on an app through forwarding | `CMS-V0-003` |
| A feature is lost from `corvint` | `CMS-V0-004`; `NOT_INSTALLED` shims |
| A plugin slows hot paths | `CMS-V0-010` |
| Memory or start-time growth | `CMS-V0-006`, `CMS-V0-011` |

Missing measurements stay `NOT_OBSERVED` and block the extraction that needs them. They are never
assumed to pass.

## Acceptance and evidence

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| `CMS-V0-001` | import and write ratchet (V1-4800) | ratchet test with a seeded cross-unit edge and a seeded `.corvint/` write |
| `CMS-V0-002` | release manifest and `corvint-update`; the beside-version refusal in `internal/docsenvelope` (`CheckBeside`), `cmd/corvint/docs_shim.go` and `cmd/corvint-docs/corpus_integration.go`, and the one-version install probe in `internal/releasecandidate` (`ARTIFACT-BIN-V0-510..512`, V1-5237) | manifest refusal test (`release-binary`, `ARTIFACT-BIN-V0-004`); `TestDocsShimRefusesSiblingAtAnotherVersion`, `TestCoreReadRefusesSiblingCoreAtAnotherVersion`, `TestARTIFACTBINV0511InstallPlacesAppsBesideCorvintAtOneVersion`; the `corvint-update` whole-set install, rollback and interrupted-install restore in `internal/update` (`UPD-V0-920` to `UPD-V0-922`, V1-5466): `TestUPDV0920ApplyInstallsTheWholeCoreSet`, `TestUPDV0921RollbackAndInterruptedSetRestore`, `TestUPDV0922MissingAppRefusesAndEqualBuildCompletesTheSet`; a published release carrying the app archives is NOT_RUN until the release workflow publishes them |
| `CMS-V0-003` | Core verb dispatcher | no-app PATH test over every Core verb |
| `CMS-V0-004` | each extraction PR | CLI/wire golden diff; frozen eval parity; benchmarks |
| `CMS-V0-005` | `affected` module mapping | replay before and after, recorded in the build log |
| `CMS-V0-006`, `CMS-V0-011` | release and benchmark harness | recorded size, cold start and RSS per extraction |
| `CMS-V0-007` | module files and release build | release build with `GOWORK=off` |
| `CMS-V0-008` | per-unit `AGENTS.md`; size check | context-pack size and context-eval recall |
| `CMS-V0-009` | per-unit test layout | isolated `go test`; `affected` app-only edit fixture |
| `CMS-V0-010` | `corvint-plugin/1` host | overhead benchmark per plugin boundary |
| `CMS-V0-912` | `cmd/corvint-docs/flows.go` (`flowInvocation`, `runFlows`), `cmd/corvint-docs/help.go`, `internal/command/flows` | `TestCMSV0912FlowsAndHowParityWithCorvintDocs` (over fixture repositories, `flows map`, `flows export`, `how`, `flows appmap how` and four refusals print the same stdout, stderr and exit code three ways: corvint's dispatcher forwarding to a built `corvint-docs` process, that `corvint-docs` binary run directly, and the handlers called in process as `corvint` dispatched them before the move); `TestCMSV0912FlowsHelpDoesNotRequireRepository`; `TestCMSV0912EveryFlowsSubcommandIsInTheCommandTree`; `TestSKHV0810HowAlias` |
| `CMS-V0-913` | `cmd/corvint/flows_shim.go` (`runFlowsShim`), `forwardToDocs` in `cmd/corvint/docs_shim.go` | `TestCMSV0913FlowsShimForwardsToCorvintDocs` (a stand-in `corvint-docs` must receive the exact argument vector, so a native implementation in `corvint` fails it); `TestCMSV0913FlowsShimHelpIsNativeAndNamesCorvintDocs`; `TestCMSV0913FlowsShimReportsNotInstalled`; `TestCMSV0913FlowsShimRefusesRequireClean` |
| `CMS-V0-914` | `cmd/corvint/flows_test_evidence.go` (`runFlowsTestEvidence`); `refuseFlowsTestEvidence` in `internal/command/flows` | `TestRSPV0051FlowProjection`; `TestCMSV0914TestEvidenceRefusesWithThePointerToCorvint`; `TestCMSV0914TestEvidenceRefusalIsReachable` |
| `CMS-V0-915` | `tools/import-ratchet/areas.json` (`shared` prefixes `internal/appflows`, `internal/appmap`, `internal/flowcoverage`) | `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs` (area keys; no tests-layer production import of `internal/command/flows`; `cmd/corvint`, `internal/testplan` and `internal/testsource` import a shared flow reader); `TestImportRatchet` (`IRT-V0-004`) for every other tests -> docs edge |
| `CMS-V0-930` | `internal/docsenvelope/envelope.go` (`NotInstalledMessage`, `InstallHint`, `installHint`, `InstallCommand`), `version.go` (`CheckBeside`), `internal/mcp/toolgroup/errordetail.go` (`Detail`) | `TestCMSV0930NotInstalledNamesTheInstallCommand` (no `corvint` beside the test executable names only the manual install; a numbered `corvint` gets the command with its directory; an unnumbered, build-0 or silent one and a missing `corvint` name only the manual install; quoting); `TestDocsShimReportsNotInstalled` asserts the manual install on `corvint docs`, `flows` and `how` with no `corvint` beside, and the command with `--bin-dir` once a numbered `corvint` stand-in is beside; `TestDocsShimRefusesSiblingAtAnotherVersion` asserts the hint on `version-mismatch`. The MCP `next` step is not asserted by a test |
| `CMS-V0-916` | `tools/import-ratchet/baseline.json` (18 -> 17 edges) | `TestImportRatchet` (`IRT-V0-003`, `IRT-V0-004`, `IRT-V0-005`); the baseline scan and the seeded `cmd/corvint -> internal/command/flows` control in `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs` |

## Compatibility and rollback

- Each extraction is one PR. Reverting it re-links the unit into `corvint`, and no wire change is
  needed (`CMS-V0-004`).
- Reverting decision 0591 needs a new superseding decision.
- Decision 0595 rolls back by reverting its change: `corvint` dispatches `flows` and `how`
  natively again and the edge `cmd/corvint -> internal/command/flows` returns to the baseline. No
  wire format, flag or stored state changed.

## Remaining decisions

None for intent. Each manifest addition still needs its own decision. A plugin boundary for test
providers depends on V1-4824's measurements.
