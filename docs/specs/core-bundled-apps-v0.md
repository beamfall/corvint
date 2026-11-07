# Core and Bundled Apps V0

Owner: Russell Lewis
Date: 2026-11-06
Requirement prefix: `CMS-V0`
Intent status: accepted (owner, 2026-11-06)
Delivery status: not-started
Authoritative inputs:
- decision 0591;
- `AGENTS.md` invariant 7 as amended by 0591;
- decision 0081;
- `core-compatibility-freeze-v1.md` (Core verb list);
- restructure tickets labelled `restructure-2026-11` (V1-4800..4829).

## Agent digest
- Claim: Corvint splits into a core plus bundled apps and plugins, each measured for LLM fit, test isolation, plugin cost, speed and memory.
- Status: accepted (owner, 2026-11-06) / not-started
- Exists: one root module; `corvint` links docs, flows and Tasks; 54 `cmd/` entry points.
- Blocked on: baseline measurements (`CMS-V0-005`, `-006`, `-008`, `-010`, `-011`) before any code moves.
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
- `mcp/tasksbridge`, `mcp/tasksread`, `mcp/docsbridge` and `mcp/bridge` (via `flowcoverage`) link
  the apps into the adapters.

## Units and edges to cut

| Unit | Contents (indicative) | Edges to cut before extracting |
| --- | --- | --- |
| Core `corvint` | index, query/context, impact, `affected`/test selection, review, CEM/dogfood evidence, `corvint-test`, `corvint-test-runner`, `corvint-tests-accept`, `corvint-intake`, `corvint-remote-provider` | n/a (root module) |
| Kernel (leaf packages) | git runner, `gitprobe`, `gitstatus`, language ID, `execfile`, `procgroup`, `secretscreen`, `analyzerenvelope`, `langplugin`, `protocol/**` | upward imports (V1-4809) |
| Tasks `corvint-tasks` | `internal/tasks/**`, `taskman`, `taskprojection`, `taskbarrier`, `localcompletion`, `scopelease`, `workqueue`, `worksource`, `corvint-work-queue` | `tasks`→`contextindex` (V1-4810); `cmd/corvint`→`taskman`; `mcp/tasksbridge`, `mcp/tasksread` |
| Docs `corvint-docs` | doccorpus, doccompiler, docmaintain, doccitations, corpus index/serve/republish, `corvint-corpus-*`, `corvint-docs-mcp` | `cmd/corvint`→`doccorpus`/`doccompiler`/`docmaintain`; `mcp/docsbridge` |
| Flows `corvint-web-flows` | appflows, appmap, funclink, browsercov, flowcoverage, flowdocs, behaviorfalsify, `corvint-playwright-minimize`, `corvint-behavior-falsify` | `cmd/corvint`→`appflows`; `command/breakage`→`appflows`; `mcp/bridge`→`flowcoverage` |
| Adapters | `corvint-mcp`, `corvint-lsp`, `corvint-gateway`, test-validity MCP | bridges call apps through envelopes |
| Plugins | analyzers; test providers only if measured (V1-4824) | registries (V1-4818); `corvint-plugin/1` (V1-4819) |
| Release/update (in manifest) | `corvint-update`, `corvint-release-install` | none |
| Developer tooling (not in manifest, under `tools/`) | release-candidate, public-release-check, companion-release, postmerge-*, readiness-record, dashboard-snapshot, go-toolchain-receipt, cem-experiments | none |
| Console | `corvint-console` | never in the manifest (decision 0081) |

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
| `CMS-V0-002` | release manifest and `corvint-update` | manifest refusal test; interrupted-install test |
| `CMS-V0-003` | Core verb dispatcher | no-app PATH test over every Core verb |
| `CMS-V0-004` | each extraction PR | CLI/wire golden diff; frozen eval parity; benchmarks |
| `CMS-V0-005` | `affected` module mapping | replay before and after, recorded in the build log |
| `CMS-V0-006`, `CMS-V0-011` | release and benchmark harness | recorded size, cold start and RSS per extraction |
| `CMS-V0-007` | module files and release build | release build with `GOWORK=off` |
| `CMS-V0-008` | per-unit `AGENTS.md`; size check | context-pack size and context-eval recall |
| `CMS-V0-009` | per-unit test layout | isolated `go test`; `affected` app-only edit fixture |
| `CMS-V0-010` | `corvint-plugin/1` host | overhead benchmark per plugin boundary |

## Compatibility and rollback

- Each extraction is one PR. Reverting it re-links the unit into `corvint`, and no wire change is
  needed (`CMS-V0-004`).
- Reverting decision 0591 needs a new superseding decision.

## Remaining decisions

None for intent. Each manifest addition still needs its own decision. A plugin boundary for test
providers depends on V1-4824's measurements.
