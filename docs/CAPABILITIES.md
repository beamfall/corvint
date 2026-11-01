# Corvint capability catalog

This is the one page for an agent that knows nothing about Corvint. It lists every `corvint`
command, every binary built from `cmd/`, and every MCP tool. For each one it says what question
it answers, when to use it, the smallest call, what comes back, how mature it is, and when it
refuses or abstains. Each claim is taken from the code and `--help` text on the revision that
holds this file. The [capability catalog spec](specs/capability-catalog-v0.md) (CAP-V0) owns the
page, and `tools/capabilities-check` fails `go test ./...` when a command, binary or MCP tool
has no row, when a row names something that no longer exists, when a code-declared subcommand is
missing from its command's row, when a command's maturity disagrees with its root help group, or
when a listed refusal code no longer appears in the source.

The binary carries two more views of the same surface. `corvint help --json` prints a
machine-readable command catalog (`corvint-capability-catalog/0`) derived from the command
registry: fields the registry cannot answer are `null` and named under `unknowns`; `corvint help
--json COMMAND` prints one command's entry, under 5 KB (`corvint-capability-command/0`,
CAP-V0-950). `corvint help guide` prints a task-indexed guide with runnable examples and the
non-capabilities each tracked by an issue (CAP-V0-015 to CAP-V0-021).

For a dated, counted snapshot of this page by maturity, with the first promotion batch and its
unmet gates, see the [Core promotion inventory](build-log/2026-10-29-core-promotion-inventory.md)
(V1-3180). It is a snapshot, not a second catalog: when the two differ, this page is right.

## Command trees

Every subcommand at every depth, with its flags, required inputs and an example, is in one
`corvint-command-tree/0` document per binary, derived from the help that binary prints
(CAP-V0-040 to CAP-V0-048): `corvint help --tree`, `corvint-tasks help --json` and `corvint-test
help --json` (both also answer `help --tree`, CAP-V0-950), committed as `internal/capcatalog/trees/BINARY.json` and linked from the
`commandTrees` member of `corvint help --json`. Any node answers `--help` and `-h` with its own help
and exits 0. `tools/capabilities-check` fails when a binary dispatches a subcommand or option its
tree lacks, when a node has no usage, example or help, and when a new `cmd/` binary has neither a
tree nor a row in the shrink-only `internal/capcatalog/CLI-RATCHET.tsv`. The repository's root
`llms.txt` links all of these.

How to read maturity:

- **core**: the verbs `corvint help` lists as Core (decision 0332). Only the modes and profiles
  the [Core compatibility freeze](specs/core-compatibility-freeze-v1.md) lists are frozen. Other
  modes of the same verb can be experimental; the row says so.
- **experimental**: no stability promise. The owning spec governs the behaviour.
- **implemented**: the owning spec records delivery. This is still not a 1.0 stability promise.
- **internal**: CI, release or qualification tooling, not for everyday agent use.

Every Corvint read reports what it could not see. An `UNKNOWN`, `NOT_RUN`, `NOT_OBSERVED`,
`NOT_PRODUCED` or `unsupported-*` result is the answer, not a crash. Keep it and do not replace it
with a guess. Commands are local-only, with no network or telemetry. `init`, `adopt`, `query`,
`feature` and `impact` are qualified on Darwin and Linux only.

## Start here

| Task | Command sequence |
|---|---|
| Orient on an unfamiliar repository | `corvint init`, then `corvint query --task "what governs work here?"` for the governing instructions and minimum context |
| Find the files to read for one task | `corvint context --task "TEXT" --subject PATH`; for a symbol, `corvint context defs NAME` or `corvint context refs NAME` |
| Check what a change reaches | `corvint impact PATH...` for paths, or `corvint impact --base FULL_SHA` for the committed range to `HEAD` |
| Choose the tests for a change | `corvint affected --base FULL_SHA` for the plan; `corvint test --base FULL_SHA --dry-run` to see the experimental Go run without running it |
| Check the evidence before trusting it | `corvint prove --task "TEXT"` or `corvint prove --base FULL_SHA`: each claimed row gets a falsifier verdict |
| Bind a change to evidence | `corvint cem prepare --base REV --target REV`, then `cem cite` or `cem mark` each hunk, then `cem verify` and `cem report` ([guide](CHANGE-EVIDENCE-MAP.md)) |
| Prove declared obligations are closed | `corvint ocm prepare --target REV --intent SPEC.md`, then `ocm link` with test claims, then `ocm verify` |
| Order a review | `corvint frontier --cem MAP --ocm MAP --expected-base REV --target REV --artifacts DIR` |
| Get advice on a branch | `corvint review --base FULL_SHA`: compact affected advice and overlap with local branches (experimental) |
| Look up documentation | `corvint docs corpus build --manifest M > corpus.json`, then `corvint docs corpus search --artifact corpus.json --query TERM`, or serve it with `corvint-corpus-mcp` ([guide](DOCUMENTATION-CORPUS.md)) |
| Inspect application flows | `corvint flows --manifest FILE`, then `flows coverage` and `flows gaps` ([flow coverage](FLOW-COVERAGE.md)) |
| Work the ticket queue | `corvint-tasks queue status`, `corvint-tasks roadmap`, then claim, submit, gate and complete ([external agents](TASKS-EXTERNAL-AGENTS.md)) |
| Give an agent host Corvint tools | `corvint mcp --root /abs/repo --tool-profile task-review` over stdio, or install a host plugin ([MCP](MCP-SERVER.md), [extensions](EXTENSION-ECOSYSTEM.md)) |
| Read retained live-test evidence | `corvint test-validity --discover`, or the `corvint.test_validity` tool of `corvint-test-validity-mcp` |
| Run this repository's change workflow | `make dogfood-change BASE=FULL_SHA` (it wraps `corvint dogfood change`); then follow [DOGFOOD](DOGFOOD.md) §4 |

## Commands

One row per top-level command in `corvint help`. Run `corvint COMMAND --help` for the complete
contract; the subcommands named here are the ones the help lists.

Columns: Q = the question it answers; Use / not = when to use it and when not; Min = the smallest call; Out = what it prints.

### Repository orientation and context

<!-- capability-gate:commands -->
| Command | Q | Use / not | Min | Out | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|---|---|
| `corvint init` | What does this repository contain, mechanically? | First contact with a repository. Not for task context (use `context`). | `corvint init` | Bounded sealed JSON summary; `--full-receipt` prints the canonical receipt | core | Platforms other than Darwin or Linux; revisions the bounded index refuses | [CCF-V1](specs/core-compatibility-freeze-v1.md) |
| `corvint adopt` | Same inventory as `init`, recoverable for an adopting repository | Adopting Corvint in an existing repository; same flags as `init`. Not a write: both are read-only. | `corvint adopt` | Same as `init` | core | Same as `init` | [CCF-V1](specs/core-compatibility-freeze-v1.md) |
| `corvint index` | Is there a fresh index snapshot for the packet commands? | Before batches of reads, after `HEAD` moves. Not needed for correctness: reads build what they need. | `corvint index --if-stale` | JSON receipt naming the snapshot store and any evicted snapshots; writes `corvint/index/*.gob` under the Git common directory | core | Repositories the bounded index refuses | [Index snapshot](specs/index-snapshot-v0.md) |
| `corvint query` | What governs work here, and what minimum context does this task need? | The start of a task: the authority-start profile returns the governing instruction file first. Not for symbol lookup. | `corvint query --task "TEXT"` | One canonical JSON receipt with ranked evidence, budget, freshness, omissions and uncertainty | core | `unsupported-query-trace-state` (recovery: `context --task TASK --limit 5`); other unsupported profiles exit 2 with a structured error | [CCF-V1](specs/core-compatibility-freeze-v1.md) |
| `corvint context` | Which files should I read for this task? | You know the task and maybe the file you are changing (`--subject`). Subcommands `defs`, `refs`, `grep` look up one symbol or term. Help labels it Go-only; see Language support. | `corvint context --task "TEXT"` | JSON packet: each file with the relation that admitted it (pair, mentioned, definition, reverse-import, cochange, sibling, lexical); state `READY` or `NO_CANDIDATES` | core | `unsupported-context-lookup-identifier`; a malformed learned slot-weights file refuses | [Task context packet](specs/task-context-packet-v0.md) |
| `corvint feature` | What context belongs to this known feature ID? | You have an explicit feature ID. Not for discovery (use `features`). | `corvint feature FEATURE_ID --limit 1` | Bounded feature packet | experimental | `unsupported-feature-repository` (non-Go ranking with limit above 1); unknown IDs are `OUT_OF_SCOPE` | [GPK-V0](specs/go-production-kernel-migration-v0.md) |
| `corvint features` | Which feature candidates can be inferred? | Exploring an unfamiliar codebase at a clean `HEAD`. Not accepted feature IDs. | `corvint features` | Heuristic candidates with blob and line evidence | experimental | Missing sources and index data stay unknown; clean `HEAD` only | [RGV-V0](specs/repository-guidance-v0.md) |
| `corvint overview` | What is the inferred shape of this repository? | A first map with suggested next commands. The suggested argv are inert and never run. | `corvint overview` | Guidance JSON with suggested argv | experimental | Same as `features` | [RGV-V0](specs/repository-guidance-v0.md) |
| `corvint batch` | Can I answer several reads from one loaded snapshot? | Several independent `query`, `context` or `impact` reads at once. Go-only. | `corvint batch < request.json` | `snapshot-batch/0` with one answer per operation (at most 16) | experimental | `unsupported-batch-snapshot`; oversized or duplicate-id requests | [SBQ-V0](specs/snapshot-batch-v0.md) |
| `corvint depsource` | What is the exact source of this Go dependency? | Citing third-party Go code you cannot see in the repository. | `corvint depsource MODULE` | Verified module file list or one excerpt (`--file`) | experimental | `module-not-required`, an absent cache entry or a hash mismatch: abstains with a named reason; never fetches | [DSE-V0](specs/dependency-source-evidence-v0.md) |
| `corvint answerability` | Do two independent retrieval channels agree on this task? | Judging whether a context packet is likely to be enough. | `corvint answerability --task "TEXT"` | JSON agreement signal (top-K overlap) between the lexical and structural channels | experimental | Signal only; it never states that an answer is right | [RDS-V0](specs/retriever-disagreement-v0.md) |
| `corvint necessity` | Which packet files are load-bearing? | Trimming or auditing a context packet. | `corvint necessity --task "TEXT"` | Each included file labelled load-bearing, supporting or unlabelled | experimental | An abstaining packet is returned unchanged with no labels | [NEC-V0](specs/necessity-labels-v0.md) |
| `corvint kernel` | What must survive a context compaction? | Before or after compacting an agent context; `kernel verify` checks recovered text. | `corvint kernel --requirements ID1,ID2` | Envelope pinning governing authority and named requirements to committed blobs; verify reports intact, stale, missing or corrupt | experimental | `unsupported-kernel-authority` (exit 2) when the index pins no governing authority (run `corvint index` first); `verify` exits 1 when not intact (a verdict, not an error) | [CKN-V0](specs/compaction-kernel-v0.md) |
<!-- /capability-gate:commands -->

### Impact, tests and verification

<!-- capability-gate:commands -->
| Command | Q | Use / not | Min | Out | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|---|---|
| `corvint mcp` | How does an agent host reach Corvint tools with only `corvint` installed? | Serving the MCP stdio server the Claude Code and Codex plugins launch (AHI-373); same server as `corvint-mcp`, not a separate CLI query. | `corvint mcp --root /abs/repo` | MCP `tools/list` and tool results; see MCP tools | experimental | Invalid arguments; `--require-clean`; a global `--root` before the verb; per-tool refusals | [MCP server](specs/mcp-server-2026-07-28-v0.md), [AHI-373](specs/agent-harness-integration-v0.md) |
| `corvint graph` | Which file import edges resolve across a pinned tree? | `imports` streams every indexed syntax edge, literal Angular component resource edge and single-candidate Zeitwerk constant edge (`authority: convention`), plus uncertainty, without impact's limit; no runtime closure claim. | `corvint graph imports --revision FULL_COMMIT` | `graph-imports/1` JSONL; optional `--kinds` subset of `import,type-import,angular-template,angular-style,ruby-constant` (edges only), `--format jsonl` | experimental | `graph-invalid-revision`, `graph-unknown-revision`, `graph-invalid-kinds`, `graph-invalid-arguments`; index exclusions remain unknown | [Non-Go impact](specs/non-go-impact-v0.md) |
| `corvint impact` | What does changing these paths, or this committed range, reach? | Before editing a file or reviewing a range. `--working-tree-untracked` covers untracked `.go` files only. Evidence is syntax, never complete runtime impact; reverse imports are one hop, so a `READY` receipt is not a closed test selection (use `affected --path`). | `corvint impact PATH` | JSON receipt with evidence rows, uncertainty and freshness; `--provider` adds external evidence | core | `unsupported-impact-path-suffix`, `unsupported-platform`; `--require-clean` on a mixed worktree gives an empty `NEEDS_WIDENING` receipt | [Non-Go impact](specs/non-go-impact-v0.md), [CCF-V1](specs/core-compatibility-freeze-v1.md) |
| `corvint affected` | Which tests should run for this worktree or branch? | Before running tests; `--base FULL_SHA` plans the whole branch, and `--path PATH` plans a listed change at a clean HEAD. Batch several changed paths into one call (repeat `--path`, or `--paths-from FILE`) rather than one call per path; a private cache under the user cache directory (outside the repository) reuses the verified graph across calls on an unchanged checkout with byte-identical output; `--cache DIR` moves it, `--no-cache` turns it off, and `affected cache` reports its location and size; `--format text` prints the same plan as one line per selection, unknown and check. It plans and never runs tests. | `corvint affected --base FULL_SHA` | `affected-plan/1` (compact; `--full` for `affected-plan/0`; `--format text` for lines): scope, selected units, unknown frontier, mandatory checks and advisory command; subcommands `discovery` for Playwright and `cache` for the cache status | core | `unsupported-affected-mixed-worktree` with `--require-clean`; paths no language claims widen the scope to `UNKNOWN` | [Affected plan](specs/affected-plan-v0.md) |
| `corvint test` | Run the affected Go test set, sharded | Executing the selection in this repository's CI shape; `--dry-run` plans only, and `--dry-run --summary` prints a text summary of at most 4 KiB; `--go-parallel N` (local profile) replaces the declared `-p 1` so a focused run fits a bounded offload. Subcommands `shards`, `verify-shards`. | `corvint test --base FULL_SHA --dry-run` | `corvint-test-run/0` receipt; exit 0 PASS, 1 FAIL, 3 INCOMPLETE | experimental | Exit 3 `INCOMPLETE` when a package declares partial coverage; an unreadable plan | [STR-V0](specs/session-test-runner-v0.md) |
| `corvint tests` | What do the committed JS/TS and RSpec test sources say? | Linting tests, finding duplicates, selecting setup, checking teardown or validity. Subcommands `setup-select`, `lint`, `duplicates`, `agreement`, `validity`, `teardown`, `unselected`, `citations`, `env`, `role-locators`. `lint` and `duplicates` take `--advisory`, `--compare FILE` and `--log FILE` (agreement records that `agreement` summarises per rule); `duplicates` takes `--decisions FILE` caller decisions; `validity --legacy` adds advisory `assertionParity`. | `corvint tests unselected` | JSON or lines per subcommand | experimental | `tests-invalid-arguments`, `tests-invalid-revision`, `tests-source-unavailable`, `tests-declaration-invalid`, `tests-decisions-invalid`, `agreement-verdicts-invalid`, `agreement-log-inside-repository`, `agreement-log-unavailable`; helpers it cannot resolve abstain by name | [TSA-V0](specs/test-source-analysis-v0.md), [GVA-V0](specs/gate-verdict-agreement-v0.md) |
| `corvint test-validity` | Is this live-test result fresh, associated and strong? | Reading a retained `corvint-js-test-provider` receipt; `negate` runs step negation. | `corvint test-validity --discover` | `corvint-test-validity/0`: five independent axes per test (association, hygiene, freshness, execution, strength) | experimental | `UNPROVEN`, `STALE`, `NOT_PRODUCED` axes; strength stays `NOT_MEASURED` unless measured | [MTV-V0](specs/mcp-test-validity-profile-v0.md) |
| `corvint test-plan` | Can these declared test variations be consolidated? | Planning fewer e2e tests over the same variations. Subcommands `consolidate`, `check`, `prefixes`, `page-state`. | `corvint test-plan consolidate --input FILE` | Ordered candidate tests (table or JSON) | experimental | Variations with a missing anchor abstain; `check` exits 1 on `test-plan-mismatch` or `test-plan-incomplete` | [TCN-V0](specs/test-consolidation-planner-v0.md) |
| `corvint runs` | What do these Playwright JSON reports say across runs? | Flake, comparison, reuse, redundancy and freshness questions over report files. Subcommands `stability`, `compare`, `reusable`, `redundancy`, `fresh`, `baseline`, `response-wait`. | `corvint runs stability --runs a.json,b.json` | JSON per subcommand; `--summary` for counts only | experimental | `runs-invalid-arguments`, `runs-report-unavailable`, `runs-baseline-unavailable`, `runs-revision-unresolved`; incomplete shard sets refused | [PWR-V0](specs/playwright-run-evidence-v0.md) |
| `corvint prove` | Can each claimed row of the packet be falsified? | Before trusting a context, change or CEM packet; `--mutate` adds bounded Go and pytest mutation checks. | `corvint prove --task "TEXT"` | `falsifiable-packet/0`: per-row falsifier verdicts PASS, FAIL or NOT_RUN; state `CITED`, `UNPROVEN` or `REJECTED` | core | `unsupported-version`, `attest-public-key-unavailable`; a dirty path is `NOT_RUN` | [FPK-V0](specs/falsifiable-packet-v0.md) |
| `corvint prove-observe` | Record a proof's verdict counts locally | After `prove`, to feed the falsification rate in `observations`. | `corvint prove --task "TEXT" \| corvint prove-observe` | Nothing on success; appends one row to `.corvint/self-observations.jsonl` | experimental | `invalid-proof-document` (exit 2, ledger unchanged) | [SOL-V0](specs/self-observation-ledger-v0.md) |
| `corvint surprise` | How far did the actual change differ from the predicted touch set? | After a task is committed, to measure the packet. | `corvint surprise --task "TEXT" --base REV --target REV` | JSON with predicted-only, actual-only paths and a surprise ratio | experimental | `unsupported-surprise-dirty-worktree` | [TSS-V0](specs/touch-set-surprise-v0.md) |
| `corvint witness` | Which part of this committed range has no witness? | Reviewing a range for unexercised surface. | `corvint witness --base REV` | Conservative unwitnessed report; `corvint-witness-stable/0` with an accepted cem/1.0 map; a base-tree `.corvint/witness-paths.json` declares inert or data paths (AGW-V0-041..045) | experimental | A cem/1.0 map without `--stable-artifact-root`, or one the verifier does not accept, exits 2; never invents a witness | [AGW-V0](specs/corvint-witness-v0.md) |
| `corvint breakage` | Which declared cross-repository API relations does this change touch? | Several checked-out repositories declared in a manifest. | `corvint breakage --manifest FILE --api REPO:PATH:SYMBOL --repository REPO=DIR` | Relation report; scope `INCOMPLETE` | experimental | Missing, unsupported and ambiguous relations stay unknown; no discovery, fetch or execution | [BKM-V0](specs/cross-repository-breakage-map-v0.md) |
| `corvint migration-ratchet` | Did migration evidence move forward between two snapshots? | Gating a migration with a frozen profile. | `corvint migration-ratchet --profile FILE` | `corvint-migration-evidence-ratchet-receipt/1`; with `--log` one `corvint-gate-agreement/0` record per run | experimental | Exit 1 fail or unknown, exit 2 refused input; `--advisory` exits 0 instead of 1 | [MER-V0](specs/migration-evidence-ratchet-v0.md), [GVA-V0](specs/gate-verdict-agreement-v0.md) |
<!-- /capability-gate:commands -->

### Change evidence and review

<!-- capability-gate:commands -->
| Command | Q | Use / not | Min | Out | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|---|---|
| `corvint cem` | Does every hunk of this change cite evidence, and does the map verify? | Binding a delivered diff to evidence. Subcommands include `begin`, `prepare`, `cite`, `mark`, `status`, `verify`, `report`, `verify-stable`, `export`, `bundle`, `cover`, `discriminate`, `gate-verdict`, and the experimental `anchor`, `provenance` and `defer` (a seal-bound intent-scope deferral record). | `corvint cem prepare --base REV --target REV` | cem/1.0 map, status, verification and report artifacts; `begin`, `prepare`, `cite`, `mark`, `defer` and `report` write local artifacts | core | `unsupported-spec` for retired cem/0.x maps; unverifiable hunks stay unknown; opt-in `--path-policy base` (prepare, status) reports base-tree declared-inert hunks beside the result (CAX-V0-060..065) | [CEM-V1](specs/cem-stable-v1.md), [guide](CHANGE-EVIDENCE-MAP.md) |
| `corvint ocm` | Is each declared obligation closed by a test claim bound to the CEM? | After a CEM, against an intent spec. Subcommands `prepare`, `link`, `mark`, `status`, `verify`, `report`. | `corvint ocm prepare --target REV --intent SPEC.md` | `ocm/0.1-experimental` map and reports | core | `unsupported-spec`; unlinked obligations stay open | [OCM-V0](specs/ocm-v0-dogfood.md) |
| `corvint frontier` | What should a reviewer look at first? | You have a verified CEM and its OCM. | `corvint frontier --cem MAP --ocm MAP --expected-base REV --target REV --artifacts DIR` | Deterministic review queue (`frontier-stable-view/0` for cem/1.0) | core | `unsupported-spec` for cem/0.x maps, `invalid-frontier-input` for a missing or malformed input (exit 2) | [Change Frontier](specs/change-frontier-v0.md) |
| `corvint dogfood` | Did this change follow the enrolled evidence workflow? | Corvint's own change workflow and enrolled sessions. Subcommands `change`, `check`, `seal`, `finish-lane`, `batch-check`, `propose-citations` (draft plan; `--refresh PLAN` carries an earlier plan's rows to re-hunked hunks, DCW-V0-442), `begin`, `status`, `verify`, `finish`, `review`, `cancel`, `handoff`. | `corvint dogfood change FULL_SHA` | One line per step, then every `NOT_PRODUCED` row with its reason | core | Missing evidence stays incomplete; `NOT_PRODUCED` and `NOT_OBSERVED` rows name their reason | [DOGFOOD](DOGFOOD.md) |
| `corvint dogfood-ocm` | Do the private dogfood OCM maps verify together? | Checking the ordered per-scope OCM set. | `corvint dogfood-ocm status --expected-base REV --target REV` | Fail-closed aggregate JSON | experimental | Any scope that does not verify fails the aggregate | [OCM-V0](specs/ocm-v0-dogfood.md) |
| `corvint obligations` | Which external obligations does each CEM hunk reach? | You have an accepted cem/1.0 map and a saved `impact --provider` receipt. | `corvint obligations --cem FILE --impact FILE --repository DIR --expected-base REV --target REV --stable-artifact-root DIR` | `external-frontier-obligations-stable/0` | experimental | Maps the stable verifier does not accept; unrelated rows are `unknown` associations | [EFO-V0](specs/external-frontier-obligations-v0.md) |
| `corvint lrf` | Does the lexical relevance floor admit this map? | Historical only: lrf/0 is frozen over retired cem/0.x maps. | `corvint lrf --cem MAP` | No LRF output for any current map | experimental | `unsupported-spec` for every cem/0.x and cem/1.0 map (exit 2) | [LRF-V0](specs/lexical-relevance-floor-v0.md) |
| `corvint review` | What does a branch change, and which local branches overlap it? | Advice before review or merge. Not a verdict. | `corvint review --base FULL_SHA` | Compact affected advice (`affected-plan/1`) and local overlap hints | experimental | Missing sources and bounded branch paths stay unknown; clean `HEAD` only | [RGV-V0](specs/repository-guidance-v0.md) |
| `corvint delta` | What is a source-free record of exactly this change? | Feeding an immutable change record to another system. | `corvint delta --base FULL_SHA --head FULL_SHA` | `corvint-delta/0` canonical JSON | experimental | Incomplete coverage keeps uncertainty and `runFullSuite` | [DLT-V0](specs/immutable-delta-v0.md) |
| `corvint changes` | What changed between two builds, and which workarounds may now go? | Upgrading Corvint; `check-shared`, `split-shared` and `behind` police shared test paths. | `corvint changes --from BUILD_SHA --to BUILD_SHA` | Offline upgrade checklist JSON | experimental | `changes-invalid-arguments`, `changes-build-unavailable`, `changes-shared-config-unavailable`; GitHub state is `NOT_OBSERVED` without a supplied export | [UCL1142-V0](specs/upgrade-checklist-v0.md) |
| `corvint step` | Did this authoring step stay inside its declared scope? | A host that declares a scope before an edit. Subcommands `snapshot`, `verify`, `env-check`. | `corvint step snapshot --declaration FILE --host FILE` | Scope receipt; exit 0 PASS, 1 violation, 2 unavailable | experimental | Exit 2 on unavailable, unsupported or malformed evidence | [ASS-V0](specs/authoring-step-scope-v0.md) |
| `corvint repro` | Can this reported behaviour be reproduced and checked on another build? | Attaching an executable reproduction to an issue (`record`), then checking a build against it (`run`); not for stdin-driven or non-repository inputs. | `corvint repro record --output b.json -- impact --path README.md` | `corvint-repro/0` bundle; `corvint-repro-run/0` PASS/FAIL per expectation with both build identities | experimental | `repro-inputs-unpinned` (dirty tree, missing input), `repro-inputs-unavailable` (HEAD, tree or input differs; nothing runs), `repro-bundle-invalid` | [RPB-V0](specs/reproduction-bundle-v0.md) |
<!-- /capability-gate:commands -->

### Documentation, flows and work

<!-- capability-gate:commands -->
| Command | Q | Use / not | Min | Out | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|---|---|
| `corvint docs` | What does the source say, as pinned documentation? | Drafting docs from owner prose and Go declarations, building or searching a documentation corpus, maintaining generated blocks. Subcommands `draft`, `consume`, `corpus` (manifest, build, search, render and more), `flows`, `maintain`, `nav`. `draft --dry-run` previews a first draft over an untracked or stdin owner, marked NOT_ADMISSIBLE. | `corvint docs corpus search --artifact corpus.json --query TERM` | Markdown drafts, corpus JSON and search results | experimental | `corpus-refused` for absolute or unsafe paths; unclosed fences; `nav` build `NOT_RUN` | [SDD-V0](specs/source-documentation-draft-v0.md), [corpus](DOCUMENTATION-CORPUS.md) |
| `corvint flows` | Which application flows and assertions exist, and what covers them? | Web applications with a flow manifest. Subcommands include `coverage`, `gaps`, `impact`, `map`, `navigate`, `record`, `appmap`, `export`, `import`, `ingest`, `ingest-coverage`, `docs`, `stability`, `request-path`. | `corvint flows --manifest FILE` | `application-flow-report/0` and related `/1` documents | experimental | Gaps exit 1, refusals exit 2; static assertions are candidates, not passing tests; launches no browser | [AFU-V0](specs/application-flow-understanding-v0.md), [guide](FLOW-DOCUMENTATION.md); `flows appmap build` starts the [E2E adoption chain](E2E-ADOPTION.md) |
| `corvint work` | Which tickets could run together without path clashes? | A repository adopted with `work init`. Proposals authorize nothing. | `corvint work observe` | `work-command-result/0` | experimental | `SOURCE_UNQUALIFIED` without the committed adoption files; `MALFORMED_INPUT` | [WQO-V0](specs/work-queue-observation-v0.md) |
| `corvint lease` | Who holds a scope lease over these paths? | Coordinating local agents on overlapping paths. Subcommands `acquire`, `renew`, `release`, `status`, `list`. | `corvint lease list` | Lease JSON naming whether it mutated | experimental | An overlapping acquisition exits 1 (a result) | [SCL-V0](specs/scope-lease-v0.md) |
<!-- /capability-gate:commands -->

### Agent hosts, learning and local ledgers

<!-- capability-gate:commands -->
| Command | Q | Use / not | Min | Out | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|---|---|
| `corvint harness event` | What context should a host inject for this lifecycle event? | Called by host adapters (Claude Code, Codex, Gemini CLI, OpenCode), not by hand. | `corvint harness event --host codex --host-version V --surface S --adapter-version V --event session-start --input -` | `corvint-harness-event/0` | experimental | Unknown host or event; budget overflows report omissions | [AHI](specs/agent-harness-integration-v0.md) |
| `corvint adapter` | Run one native host adapter step | Called by host plugins: `codex`, `claude-code EVENT`, `claude-source-handoff`, `source-view`. | `corvint adapter codex < event.json` | One bounded adapter response with no repository authority | experimental | Oversized or malformed stdin | [AHI](specs/agent-harness-integration-v0.md) |
| `corvint record` | Record one local task outcome for learning | After a task finishes, at a clean worktree. | `corvint record --task "TEXT" --changed PATH --verify "CMD" --outcome passed` | Writes one local trace row | experimental | Dirty worktree, unsafe paths, credentials in commands | [LTPM-V0](specs/local-trace-producer-migration-v0.md) |
| `corvint migrate-traces` | Can the legacy trace store be migrated? | Once per old trace store; `--dry-run` first. | `corvint migrate-traces --dry-run` | Plan with digest; `--apply` publishes and quarantines | experimental | `--apply` with a digest that does not match the dry run | [LTPM-V0](specs/local-trace-producer-migration-v0.md) |
| `corvint eval` | How well does retrieval do on the frozen corpus? | Measuring ranking; `--learn-slot-weights --admit` is the only learning step that can reach ranking. | `corvint eval` | Evaluation report with trace counts by producer | experimental | A learned weight file the held-out gate does not admit | [REC-V0](specs/retrieval-eval-comparability-v0.md), [LTA-V0](specs/learned-trace-admission-v0.md) |
| `corvint skill-export` | Export admitted learned traces as Agent Skills | Sharing admitted traces with an agent host. | `corvint skill-export --out DIR` | One `SKILL.md` directory per admitted trace; evaluation reads `NOT_RECORDED` | experimental | Rows that do not re-validate are not exported | [LTA-V0](specs/learned-trace-admission-v0.md) |
| `corvint calibrate` | Did the packet's stance match the outcome? | Reviewing recorded outcomes against packet stances. | `corvint calibrate --window 50` | JSON or table of agreements and disagreements | experimental | Records without a stance are reported, not guessed | [OCL-V0](specs/outcome-calibration-v0.md) |
| `corvint observations` | What has Corvint noticed about its own failures here? | Reading the local self-observation digest. | `corvint observations` | Up to 120 proposal lines, falsification rate, adapter degradations | experimental | Empty or absent ledger gives an empty digest | [SOL-V0](specs/self-observation-ledger-v0.md) |
| `corvint reads` | How often did agents read files the packet did not carry? | Opt-in measurement of packet misses. `enable` and `disable` toggle the marker. | `corvint reads` | Digest JSON | experimental | Nothing is recorded without `.corvint/unplanned-reads.enabled` | [URE-V0](specs/unplanned-read-events-v0.md) |
<!-- /capability-gate:commands -->

## Binaries

Every directory under `cmd/` that builds a program. Companions are separate executables, not
`corvint` subcommands, and are qualified on their own. They do not inherit the Core promise.

<!-- capability-gate:binaries -->
| Binary | Class | Q | Min | Out | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|---|---|
| `corvint` | user-facing, agent-facing | Every command above | `corvint help` | See Commands | core | See Commands | [CCF-V1](specs/core-compatibility-freeze-v1.md) |
| `corvint-mcp` | agent-facing | Context, impact, status, CEM report and flow tools over MCP stdio | `corvint-mcp --root /abs/repo` | MCP `tools/list` and tool results; see MCP tools | experimental | Invalid arguments; per-tool refusals | [MCP server](specs/mcp-server-2026-07-28-v0.md), [multi-root](specs/mcp-multi-root-v0.md) |
| `corvint-docs-mcp` | agent-facing | Source-pinned documentation drafts over MCP | `corvint-docs-mcp --root /abs/repo` | `corvint.docs_draft`, `corvint.docs_consume` | experimental | Invalid arguments; drafts that no longer match source | [AHI](specs/agent-harness-integration-v0.md), [MCP guide](MCP-SERVER.md) |
| `corvint-corpus-mcp` | agent-facing | Read tools over one documentation corpus artifact | `corvint-corpus-mcp --root /abs/repo --artifact corpus.json` | Capability-gated `corvint.docs_*` tools | experimental | `invalid-root`, `corpus-unavailable`; tools the artifact lacks are not listed | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint-test-validity-mcp` | agent-facing | Retained live-test evidence over MCP | `corvint-test-validity-mcp --root /abs/repo` | `corvint.test_validity` | experimental | Same axes as `corvint test-validity` | [MTV-V0](specs/mcp-test-validity-profile-v0.md) |
| `corvint-lsp` | agent-facing | Go, TS/JS and Ruby definitions and task context inside an editor | `corvint-lsp --experimental --gopls /abs/gopls --root /abs/repo`; add `--tsserver /abs/bin`, `--ruby-lsp /abs/bin` or `--server SUFFIXES=ABSOLUTE_ARGV` | LSP over stdio; `corvint/context` experimental method | experimental | Non-open targets unavailable; drift refused with `--workspace-drift-guard`; wrapped results are server observations; fake-server tests only | [LSE-V0](specs/lsp-stdio-experiment-v0.md), [LED-V0](specs/lsp-editor-definition-v0.md), [LEC-V0](specs/lsp-editor-context-v0.md), [LWB-V0](specs/lsp-wrapped-backends-v0.md) |
| `corvint-tasks` | agent-facing | Ticket store, roadmap, claims, gates and completion | `corvint-tasks queue status` | JSON envelopes per command | experimental | Closed codes per command (see `corvint-tasks help`) | [Tasks](plans/AGENT-TASK-MANAGER-2026-09-04.md), [external agents](TASKS-EXTERNAL-AGENTS.md) |
| `corvint-work-queue` | agent-facing | Work-queue adapter snapshot, details and verification | `corvint-work-queue snapshot` | Adapter JSON | experimental | `unsupported operation` for unknown verbs | [WQO-V0](specs/work-queue-observation-v0.md) |
| `corvint-intake` | agent-facing | Validate a closed-vocabulary change candidate | `corvint-intake validate --root PATH --base FULL_OID --head FULL_OID < candidate.json` | Validation JSON; never grants authority | experimental | Invalid candidates and unbound revisions | [CVI-V0](specs/closed-vocabulary-intake-v0.md) |
| `corvint-pulse` | agent-facing | Snapshot lifecycle coordinator over stdio | `corvint-pulse --root ROOT serve --stdio` | JSON frames | experimental | `invalid-arguments`; leases and deltas are not delivered | [PSL-V0](specs/pulse-snapshot-lease-v0.md) |
| `corvint-console` | user-facing | Loopback admin console: snapshot, tickets, specs, code | `corvint-console --repo /abs/repo --tasks PATH --snapshot PATH` | Local web UI at 127.0.0.1:7777 | experimental | Non-loopback addresses refused | [LAC-V0](specs/local-admin-console-v0.md) |
| `corvint-dashboard-snapshot` | user-facing | What Corvint data exists for a repository, and what is stale | `corvint-dashboard-snapshot snapshot --root /abs/repo` | Read-only snapshot JSON; `roadmap` subcommand | experimental | `DASHBOARD_INVALID_ARGUMENT`; unavailable denominators reported as unavailable | [LOD-V0](specs/local-observability-dashboard-v0.md) |
| `corvint-update` | user-facing | Check, apply or roll back an official Core or Tasks update | `corvint-update check --component core` | JSON with freshness and qualification limits | experimental | No network without `--allow-network`; failed verification refuses | [UPD-V0](specs/operator-update-v0.md) |
| `corvint-web-flows` | user-facing | Parse JS/TS web flows and optionally observe a trusted local app | `corvint-web-flows --help` | Flow documents | experimental | Execution needs explicit trust flags | [AFU-V0](specs/application-flow-understanding-v0.md), [NEX-V0](specs/application-flow-navigation-execution-v0.md) |
| `corvint-behavior-falsify` | user-facing | Plan and run a bounded browser behaviour falsification | `corvint-behavior-falsify plan --experimental < input.json` | Plan, execution and receipt JSON | experimental | Missing `--experimental`; unapproved plans | [BBF-V0](specs/browser-behavior-falsification-v0.md) |
| `corvint-playwright-minimize` | user-facing | Minimize a Playwright suite's interactions | `corvint-playwright-minimize plan --experimental < input.json` | Plan and execution JSON | experimental | Missing `--experimental`; unapproved plans | [PSM-V0](specs/playwright-suite-interaction-minimizer-v0.md) |
| `corvint-corpus-http` | user-facing | Serve a documentation corpus index over HTTP | `corvint-corpus-http --help` | HTTP responses | experimental | Remote listening needs `--allow-remote`; external qualification `NOT_OBSERVED` | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint-corpus-parity` | internal | Compare corpus query results between producers | `corvint-corpus-parity --help` | Parity JSON | experimental | Mismatched inputs | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint-corpus-republish` | user-facing | Rebuild or queue a trusted local corpus republish | `corvint-corpus-republish build --trusted-local --root ROOT --request FILE --policy FILE --expected-policy-sha256 SHA` | Republish receipt | experimental | Missing trust flags or policy digest mismatch | [RCP-V0](specs/corpus-republish-v0.md) |
| `corvint-cem-candidate` | internal | Assemble or verify a stable CEM candidate | `corvint-cem-candidate verify --experimental ...` | Candidate artifacts | experimental | Missing `--experimental`; verifier rejection | [CEM-V1](specs/cem-stable-v1.md) |
| `corvint-cem-experiments` | internal | Plan, run and verify CEM criterion experiments | `corvint-cem-experiments plan --experimental ...` | Experiment receipts | experimental | `experiment refused` with a reason | [CEX-V0](specs/cem-criterion-experiments-v0.md) |
| `corvint-remote-provider` | user-facing | Serve external evidence from a configured remote | `corvint-remote-provider --allow-network --config FILE` | External-evidence provider records | implemented | No network without `--allow-network` | [EEP-REMOTE](specs/external-evidence-provider-remote-v0.md) |
| `corvint-go-test-provider` | test provider | Foreground Go test session with receipts | `corvint-go-test-provider --experimental --trusted-local ...` | Discovery, plan, run and observation receipts | experimental | `IDENTITY_MISMATCH` and other `go-live-error/0` codes | [GLTP-V0](specs/go-live-test-provider-v0.md) |
| `corvint-js-test-provider` | test provider | Run Vitest (`unit`) or Playwright (`e2e`) and print a receipt | `corvint-js-test-provider unit --help` | Provider receipt JSON; `--retain` keeps it under `.corvint/test-evidence` | experimental | Infrastructure failures separated from test failures | [JS live tests](specs/js-live-test-provider-v0.md), [PWP-V0](specs/playwright-external-provider-v0.md) |
| `corvint-ruby-test-provider` | test provider | Run RSpec once and print a receipt | `corvint-ruby-test-provider rspec --dir DIR` | Provider receipt JSON | experimental | Missing project directory or runner | [LPCV](specs/live-proof-carrying-verification-v0.md) |
| `corvint-test-runner` | test provider | Plan and run an approved test request | `corvint-test-runner plan --experimental ...` | Plan and run receipts | experimental | `independent executable identity required` | [TRE-V0](specs/test-runner-execution-v0.md) |
| `corvint-test` | test provider | Compile and prove change-scoped e2e tests; `e2e discriminate` mutates changed Ruby/TS/JS hunks against an external suite (CCE-V0-170..176, 236..238) | `corvint-test e2e catalog` | Catalog, obligations, proof and `e2e-external-discrimination/0` documents | experimental | Unknown subcommands; incomplete obligations; `e2e-map-unavailable` names `corvint flows appmap build`; `discriminate-app-dir-invalid` for a dirty or wrong-revision app worktree | [CCE-V0](specs/change-compiled-e2e-v0.md), [adoption guide](E2E-ADOPTION.md) |
| `corvint-tests-accept` | test provider | Plan and accept new e2e tests with repeats | `corvint-tests-accept plan --plan FILE` | Plan and acceptance receipts | experimental | Unapproved plan digests | [NEA-V0](specs/new-e2e-test-acceptance-v0.md) |
| `corvint-analyzer-go` | analyzer plugin | Go facts with byte spans and digests | `corvint-analyzer-go < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason such as `NONCANONICAL_REQUEST` | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-analyzer-python` | analyzer plugin | Python facts | `corvint-analyzer-python < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [PNC](specs/analyzer-python-native-candidate.md) |
| `corvint-analyzer-js` | analyzer plugin | JavaScript and TypeScript facts | `corvint-analyzer-js < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-analyzer-java` | analyzer plugin | Java facts through the native bridge | `corvint-analyzer-java < request.json` | Native-bridge facts JSON | experimental | `REJECTED` with a reason | [NJB](specs/native-jvm-bridge-analyzer-candidates-v0.md) |
| `corvint-analyzer-kotlin-android` | analyzer plugin | Kotlin and Android facts | `corvint-analyzer-kotlin-android < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [KA-V0](specs/kotlin-android-candidate-v0.md) |
| `corvint-analyzer-swift` | analyzer plugin | Swift facts | `corvint-analyzer-swift < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [SAC](specs/swift-apple-analyzer-candidate-v0.md) |
| `corvint-analyzer-objective-c` | analyzer plugin | Objective-C facts through the native bridge | `corvint-analyzer-objective-c < request.json` | Native-bridge facts JSON | experimental | `REJECTED` with a reason | [NJB](specs/native-jvm-bridge-analyzer-candidates-v0.md) |
| `corvint-analyzer-c-jni` | analyzer plugin | C and JNI facts through the native bridge | `corvint-analyzer-c-jni < request.json` | Native-bridge facts JSON | experimental | `REJECTED` with a reason | [NJB](specs/native-jvm-bridge-analyzer-candidates-v0.md) |
| `corvint-analyzer-dotnet` | analyzer plugin | .NET facts | `corvint-analyzer-dotnet < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-analyzer-ruby` | analyzer plugin | Ruby facts | `corvint-analyzer-ruby < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-analyzer-rust` | analyzer plugin | Rust facts | `corvint-analyzer-rust < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [RAC](specs/rust-analyzer-candidate-v0.md) |
| `corvint-analyzer-shell` | analyzer plugin | Shell facts | `corvint-analyzer-shell < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-analyzer-sql-native` | analyzer plugin | SQL facts under the SQLite source profile | `corvint-analyzer-sql-native < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [SQL ratchet](specs/sql-native-ratchet-gate-v0.md) |
| `corvint-analyzer-shader` | analyzer plugin | GLSL and Metal shader facts | `corvint-analyzer-shader < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [SHA-V0](specs/shader-glsl-metal-analyzer-candidate-v0.md) |
| `corvint-analyzer-html-css` | analyzer plugin | HTML and CSS facts | `corvint-analyzer-html-css < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-analyzer-structured-data` | analyzer plugin | JSON, YAML and other structured-data facts | `corvint-analyzer-structured-data < request.json` | Candidate facts JSON | experimental | `REJECTED` with a reason | [SDA](specs/structured-data-analyzer-candidate-v1.md) |
| `corvint-companion-release` | internal | Assemble the companion bundle from clean checkouts | `corvint-companion-release --help` | Bundle directory, manifest, checksums | internal | Dirty or mismatched checkouts | [Public release](specs/public-release-v0.md) |
| `corvint-public-release-check` | internal | Qualify one retained companion bundle | `corvint-public-release-check --help` | Qualification record | internal | Mismatched commit, tree or pins | [Public release](specs/public-release-v0.md) |
| `corvint-release-candidate` | internal | Assemble a Core and companion release candidate | `corvint-release-candidate --help` | Candidate directory | internal | Unverified inputs | [1.0 release](specs/corvint-1.0-product-and-release-v1.md) |
| `corvint-release-gate` | internal | Offline release evidence gate | `corvint-release-gate --root DIR --commit SHA --manifest PATH --git /abs/git --git-sha256 HEX --policy FILE --policy-sha256 HEX` | Gate verdict | internal | Unknown arguments; missing evidence | [1.0 release](specs/corvint-1.0-product-and-release-v1.md) |
| `corvint-release-install` | internal | Install a qualified candidate into a store | `corvint-release-install --candidate DIR --store DIR` | Install record | internal | Unqualified candidates | [1.0 release](specs/corvint-1.0-product-and-release-v1.md) |
| `corvint-readiness-record` | internal | Write or verify the stable readiness record | `corvint-readiness-record --help` | Readiness record | internal | Unverified candidates | [Readiness record](specs/stable-readiness-record-v1.md) |
| `corvint-go-toolchain-receipt` | internal | Digest a GOROOT tree into a receipt | `corvint-go-toolchain-receipt GOROOT` | Toolchain receipt | internal | Unreadable GOROOT | [Candidate profiles](specs/analyzer-candidate-profiles.md) |
| `corvint-postmerge-connect` | internal | Post-merge connector plan, intake and write | `corvint-postmerge-connect plan --experimental --root REPO ...` | Connector records | internal | `experimental-and-absolute-root-required` | [PMC-V0](specs/postmerge-connectors-v0.md) |
| `corvint-postmerge-host-launcher` | internal | Launch the paired post-merge CI host | (invoked by the CI host) | Host records | internal | `invalid paired host invocation` | [PCH-V0](specs/postmerge-ci-host-v0.md) |
| `corvint-postmerge-metrics` | internal | Post-merge metrics under a policy | `corvint-postmerge-metrics report --policy FILE --records FILE --from FROM --until UNTIL` | Metrics JSON | internal | `invalid-input` | [PMM-V0](specs/postmerge-metrics-v0.md) |
| `corvint-postmerge-workflow` | internal | Replay a post-merge workflow as a dry run | `corvint-postmerge-workflow replay --change ID --dry-run --fixture FILE --policy FILE --root REPO` | Replay record | internal | Missing fixture or policy | [PMR-V0](specs/postmerge-replay-v0.md) |
<!-- /capability-gate:binaries -->

## MCP tools

Each server speaks MCP over stdio, is bound to one repository root, and is read-only. Tool names
are the `corvint.*` names in `tools/list`. The [MCP guide](MCP-SERVER.md) covers setup and the
protocol version: both servers speak MCP `2026-07-28` (no `initialize`; every request carries
metadata) unless started with `--protocol-version 2025-11-25` for an `initialize`-first client.

`corvint-mcp` profiles: the default lists `corvint.help`, `corvint.query`, `corvint.impact` and
`corvint.status`.
`--tool-profile task-review` and `task-review-lsp` add `corvint.context` and `corvint.cem.report`. The separate `--tool-profile import-graph` adds `corvint.graph.imports`.
`--tool-profile flows` adds the five `corvint.flows.*` tools. `corvint-docs-mcp`,
`corvint-corpus-mcp` and `corvint-test-validity-mcp` also list `corvint.help`. `--tool-profile tasks` adds
the five read-only `corvint.tasks.*` tools, which run the installed `corvint-tasks` read verbs (`TMR-V0`). `corvint-corpus-mcp` lists only the
tools that the artifact's capabilities support; `corvint.map_plan` and `corvint.map_knowledge` also
need `--map`, `corvint.consolidate_tests` needs `--consolidation`, and `corvint.docs_search_cases`
needs `--legacy-cases`.

<!-- capability-gate:mcp-tools -->
| Tool | Server and profile | Q | Maturity | Abstains or refuses on | Spec |
|---|---|---|---|---|---|
| `corvint.query` | `corvint-mcp` (all profiles) | Same receipt as `corvint query` | experimental | Same codes as the CLI | [MCPV0](specs/mcp-server-2026-07-28-v0.md) |
| `corvint.graph.imports` | `corvint-mcp --tool-profile import-graph` | Pinned whole-tree import JSONL pages; required full `revision`, optional `kinds` and `offset`; concatenate `jsonl` following `nextOffset` until `done` | experimental | Named graph revision/kind refusals; invalid arguments/offset; per-record `OUTPUT_BUDGET_EXCEEDED`; no runtime closure | [Non-Go impact](specs/non-go-impact-v0.md) |
| `corvint.impact` | `corvint-mcp` (all profiles) | Same receipt as `corvint impact`, path requests for Go, Ruby, JS and TS | experimental | Other suffixes and range requests | [MCPV0](specs/mcp-server-2026-07-28-v0.md) |
| `corvint.status` | `corvint-mcp` (all profiles) | Repository status receipt | experimental | Unavailable repository state | [MCPV0](specs/mcp-server-2026-07-28-v0.md) |
| `corvint.help` | `corvint-mcp` (all profiles), `corvint-docs-mcp`, `corvint-corpus-mcp`, `corvint-test-validity-mcp` | Same bytes as `corvint help guide` (default), `corvint help --json COMMAND` (`command`) or `corvint help --json` (`topic` `catalog`) | experimental | Arguments other than `topic` `guide`, `command` or `catalog` and a catalogued `command` | [MCPV0](specs/mcp-server-2026-07-28-v0.md) |
| `corvint.context` | `corvint-mcp` task-review, task-review-lsp | Same packet as `corvint context` | experimental | Same codes as the CLI | [MCPV0](specs/mcp-server-2026-07-28-v0.md) |
| `corvint.cem.report` | `corvint-mcp` task-review, task-review-lsp | Read a CEM report | experimental | Maps the verifier does not accept | [CEM-V1](specs/cem-stable-v1.md) |
| `corvint.flows.coverage` | `corvint-mcp` flows | Flow variation coverage | experimental | Missing inputs stay unknown | [Flow coverage](FLOW-COVERAGE.md) |
| `corvint.flows.gaps` | `corvint-mcp` flows | Flow gaps | experimental | Missing inputs stay unknown | [Flow coverage](FLOW-COVERAGE.md) |
| `corvint.flows.impact` | `corvint-mcp` flows | Flows a change reaches | experimental | Missing inputs stay unknown | [Flow coverage](FLOW-COVERAGE.md) |
| `corvint.flows.map` | `corvint-mcp` flows | Application flow map | experimental | Missing inputs stay unknown | [Flow coverage](FLOW-COVERAGE.md) |
| `corvint.flows.navigate` | `corvint-mcp` flows | Navigation within the flow map | experimental | Missing inputs stay unknown | [Flow coverage](FLOW-COVERAGE.md) |
| `corvint.docs_draft` | `corvint-docs-mcp` | Source-pinned documentation draft of a module directory in any language the native index extracts declarations from; `dry_run` previews an untracked or inline owner as NOT_ADMISSIBLE (SDD-V0-019) | experimental | Paths outside the root, unsupported sources, languages the index reads only as text (named) | [MCP guide](MCP-SERVER.md) |
| `corvint.docs_consume` | `corvint-docs-mcp` | Recheck a draft's exact bytes against source | experimental | Drafts that no longer match | [MCP guide](MCP-SERVER.md) |
| `corvint.tasks.queue_status` | `corvint-mcp --tool-profile tasks` | Same envelope as `corvint-tasks queue status` | experimental | `CLI_REFUSED` with the CLI's codes | [TMR-V0](specs/tasks-mcp-read-profile-v0.md) |
| `corvint.tasks.roadmap` | `corvint-mcp --tool-profile tasks` | One `offset`/`limit` page of `corvint-tasks roadmap` | experimental | `CLI_REFUSED` with the CLI's codes | [TMR-V0](specs/tasks-mcp-read-profile-v0.md) |
| `corvint.tasks.ticket_search` | `corvint-mcp --tool-profile tasks` | One page of `corvint-tasks ticket search` by status, kind, priority, owner, milestone, label or text | experimental | `CLI_REFUSED` with the CLI's codes | [TMR-V0](specs/tasks-mcp-read-profile-v0.md) |
| `corvint.tasks.ticket_show` | `corvint-mcp --tool-profile tasks` | Same envelope as `corvint-tasks ticket show ID` | experimental | An unknown ticket is `CLI_REFUSED` | [TMR-V0](specs/tasks-mcp-read-profile-v0.md) |
| `corvint.tasks.receipt_audit` | `corvint-mcp --tool-profile tasks` | Same envelope as `corvint-tasks receipt audit` | experimental | `CLI_REFUSED` with the CLI's codes | [TMR-V0](specs/tasks-mcp-read-profile-v0.md) |
| `corvint.test_validity` | `corvint-test-validity-mcp` | Five-axis projection of retained test evidence | experimental | `UNPROVEN`, `STALE`, `NOT_PRODUCED` axes | [MTV-V0](specs/mcp-test-validity-profile-v0.md) |
| `corvint.docs_inventory` | `corvint-corpus-mcp` (`/2` artifacts) | Page through the corpus inventory | experimental | Capability absent from the artifact | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_info` | `corvint-corpus-mcp` | Corpus header, revision and capabilities | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_validate` | `corvint-corpus-mcp` | Validate the corpus artifact | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_concept` | `corvint-corpus-mcp` | One concept's record | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_claims` | `corvint-corpus-mcp` | Claims about a path, claims ranked by text, or supplied status claims by state, qualifier and run; each claim carries a `claim_kind`, filterable by `kind` / `exclude_kind`, and run records rank below behavioural claims ([DCS-V1-591..593](specs/documentation-corpus-search-ranking-v1.md)) | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_flow` | `corvint-corpus-mcp` | One documented flow | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_dependencies` | `corvint-corpus-mcp` | Dependencies of a subject | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_recommend_tests` | `corvint-corpus-mcp` | Tests the corpus links to a subject, plus the subjects documenting the path and their test links ([DCS-V1-595](specs/documentation-corpus-search-ranking-v1.md)) | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_navigation` | `corvint-corpus-mcp` | Navigation structure of one flow `id`, or of the flows a `query` selects by name or linked test ([DCS-V1-596..597](specs/documentation-corpus-search-ranking-v1.md)) | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_vocabulary` | `corvint-corpus-mcp` | Corpus vocabulary | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_intent` | `corvint-corpus-mcp` | Intent records of one subject, or every intent comparison | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_search` | `corvint-corpus-mcp` | Search the corpus; compact receipt by default, `receipt: "full"` for the full one | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_get` | `corvint-corpus-mcp` | One subject and its links, by ID or legacy case ID | experimental | `legacy-case-unlinked` gap; capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_locate` | `corvint-corpus-mcp` | Where a path is documented, including the subjects documenting it through one `documents` hop, or module and symbol subjects ranked by a query, with line anchors ([DCS-V1-594](specs/documentation-corpus-search-ranking-v1.md)) | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_find_related` | `corvint-corpus-mcp` | Related subjects | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_coverage` | `corvint-corpus-mcp` | Coverage counts per outcome | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_gaps` | `corvint-corpus-mcp` | Producer gap claims | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_get_journey` | `corvint-corpus-mcp` | One user journey | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_get_stability` | `corvint-corpus-mcp` | Stability records | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_trace` | `corvint-corpus-mcp` | Walk relations to a depth | experimental | Capability absent | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.docs_search_cases` | `corvint-corpus-mcp` with `--legacy-cases` | Search imported legacy test cases | experimental | Server started without the flag | [DCP-V1](specs/documentation-corpus-v1.md) |
| `corvint.map_plan` | `corvint-corpus-mcp` with `--map` | Scenario plan over an application map | experimental | Server started without the flag | [AMSP-V0](specs/application-map-scenario-planner-v0.md) |
| `corvint.map_knowledge` | `corvint-corpus-mcp` with `--map` | Site knowledge (page objects, steps, setup, static links, candidate flows) mined from committed E2E sources | experimental | Server started without the flag; unread sources are named | [SKH-V0](specs/site-knowledge-howto-v0.md) |
| `corvint.consolidate_tests` | `corvint-corpus-mcp` with `--consolidation` | Test consolidation plan | experimental | Server started without the flag | [TCN-V0](specs/test-consolidation-planner-v0.md) |
<!-- /capability-gate:mcp-tools -->

## Editor and LSP

`corvint-lsp` is an operator-started experiment. It serves Go definitions between open overlays
through an explicitly supplied local gopls, and the experimental `corvint/context` method returns
the task-review context packet for an open document. Operator-named TypeScript/JavaScript and
Ruby servers (`--tsserver`, `--ruby-lsp`, `--server SUFFIXES=ABSOLUTE_ARGV`) are wrapped by file
suffix; their results are labelled as that server's observations, never Corvint authority
([LWB-V0](specs/lsp-wrapped-backends-v0.md)). It installs no editor configuration and
changes no Core defaults. `corvint context --lsp gopls|off` selects the separate gopls
cross-check for the CLI packet. See [LSP](LSP.md) and the [VS Code extension](../extensions/vscode/README.md),
which is deferred from 1.0.

## Tasks

`corvint-tasks` is the local ticket store and roadmap: tickets, priorities, claims with leases and
heartbeats, submit, gates, completion, receipts, dispatch and supervision. It is a separate binary,
never a `corvint` subcommand, and it has its own developer release channel. Start with
`corvint-tasks help`, `corvint-tasks queue status` and `corvint-tasks roadmap`. Agents follow
[TASKS-EXTERNAL-AGENTS](TASKS-EXTERNAL-AGENTS.md); supervised and continuous dispatch is in
[TASKS-SUPERVISION](TASKS-SUPERVISION.md). The `corvint-tasks-*` specs in the
[spec index](specs/README.md) own each feature.

Before filing, `corvint-tasks ticket similar --text|--payload-stdin` ranks open tickets with the
context compiler's BM25 and names the matched terms; `ticket create` refuses a likely duplicate
(`DUPLICATE_ID`) unless `--not-duplicate-of` names it; `ticket digest` prints a one-line friction
digest that `ticket similar --digest FILE` matches without a store; `ticket recur` records a
recurrence on the existing ticket. `queue snapshot` exports a verified intent snapshot that
`CORVINT_TASKS_SNAPSHOT=FILE` serves read-only on a lane host without a journal, refusing every
other verb (experimental; [TCL-V0](specs/corvint-tasks-cli-v0.md), [lane use](LANE-ORCHESTRATION.md)).

The basic external-agent lifecycle (create, plan preview, claim, renew, release, submit, gate run,
complete, readback and receipt audit on one store) has a named profile,
[TBS-V1](specs/corvint-tasks-basic-stability-v1.md). It fixes the profile's commands, refusal codes
and supported tuples. `script/qualify-tasks-basic.sh OUT_DIR` requalifies it from source on a
disposable non-fixture store. The profile was qualified on darwin/arm64 and linux/amd64
([evidence](build-log/2026-10-29-tasks-basic-stability.md)). The binary stays experimental until the
owner accepts the profile.

## Language and path support

Corvint reads syntax, never full runtime behaviour. Where a rule is missing, the answer carries
uncertainty instead of a guess.

| Analysis | Supported | What falls back |
|---|---|---|
| `impact` path and range | Go `.go` (package imports); Ruby `.rb` (markers, advisory autoload constant references up to two hops, imports unknown); JavaScript `.js .jsx .mjs .cjs` and TypeScript `.ts .tsx` (relative, profile and tsconfig imports). Non-Go uses the `non-go-syntax-v0` profile, which always reports dynamic-dispatch unknowns. | Other suffixes the index admits get positional evidence and missing-rule uncertainty. Suffixes the index does not admit refuse with `unsupported-impact-path-suffix`. `--working-tree-untracked` is `.go` only. MCP `corvint.impact` admits Go, Ruby, JS and TS for path requests only. |
| `affected` | Eleven language providers ([languages.go](../internal/liveverify/affected/languages/languages.go)): Go; JavaScript and TypeScript (Vitest, Jest, AVA, `node:test`, Bun, Deno, Playwright, Cypress, WebdriverIO, TestCafe, Nightwatch, Detox, Storybook); Python (pytest, unittest); Ruby (RSpec, Minitest, Rails); Rust (Cargo); Swift (SwiftPM, Xcode, XCTest, Swift Testing); Kotlin and Java (Gradle, JUnit, kotlin.test); C# and .NET (VSTest in SDK-style projects); C and C-family; shell (`.sh .bash .bats .zsh .ksh`). Playwright also has a config-driven profile. | An unindexed source path, a dirty path no provider claims, a language frontier or a change with no selectable test widens the plan to scope `UNKNOWN`. Dynamic configuration, ambiguous frameworks and external runtime or device inputs stay explicit unknowns. |
| `context` | Help labels the packet Go-only. Path, name, co-change, sibling and lexical relations work on any tracked path; `reverse-import` follows impact's Go, Python and web rules. | Paths without a rule get only the relations that apply; `NO_CANDIDATES` when nothing does. |
| `feature`, `batch` | Go only (`feature --limit 1` returns ledger-only context for any repository). | `unsupported-feature-repository`; `batch` is Go-only. |
| `prove` | Import rows for Go, Python, JavaScript and TypeScript; same-package reference rows for Go and web; mutation (`--mutate`) for Go and pytest. | Other rows get no falsifier (`none`); dirty paths are `NOT_RUN`. |
| Analyzer plugins | Sixteen candidate analyzers (see Binaries). | Each is admitted to the product only through its own accepted profile; none is on by default. |

## Deeper guides

[Agent routes](AGENT-ROUTES.md), [install](INSTALL.md), [V1 workflows](V1-WORKFLOWS.md),
[automation](AUTOMATION.md), [change evidence map](CHANGE-EVIDENCE-MAP.md),
[documentation corpus](DOCUMENTATION-CORPUS.md), [flow documentation](FLOW-DOCUMENTATION.md),
[external evidence providers](EXTERNAL-EVIDENCE-PROVIDERS.md),
[test claim qualification](TEST-CLAIM-QUALIFICATION.md), [self-development](SELF-DEVELOPMENT.md),
and the [spec index](specs/README.md).
