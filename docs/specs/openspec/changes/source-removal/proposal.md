# Change proposal: remove the source that 1.0 no longer ships

Change name: `source-removal`
Tickets: V1-0014 and V1-1344 to V1-1348 close as superseded under G8 once the companion decision is
accepted. Each removal batch below gets its own ticket when it is dispatched; none is filed by this
proposal.
Product requirements served: PROD-TEST-002 (never leave out a required test), PROD-INT-009
(self-use), PROD-INT-010 (every spec serves a requirement or is retired) in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: a new `docs/specs/source-removal-v1.md` (prefix `RMV-V1`), written when this
proposal is accepted. It is not written by this change.
Companion decision: `docs/decisions/PROPOSED-supersede-0594-keeps-line.md`, which lands with
the removal branch, not with the plan (R42, F1)
Acceptance record: `docs/decisions/PROPOSED-accept-product-plan-1.0.md`
Requirement IDs this change adds: RMV-V1-001 to RMV-V1-037 (no `RMV-` row exists in
`docs/specs/REQUIREMENTS.tsv` and no other change folder uses the prefix; no collision)
Files in this change (G14 layout): `proposal.md` (this file: why, what changes, non-goals,
impact), `design.md` (owner decisions applied, failure modes, batches, open items, verification,
rollback), `tasks.md` (ordered build tasks and acceptance evidence),
`specs/source-removal/spec.md` (ADDED, MODIFIED and REMOVED Requirements) and `path-verdicts.md`
(a verdict for every top-level path the batches do not list, RMV-V1-031).
Owner decisions applied: R14, R28, R29, R32, R33, R34 and R36 (2026-11-09), and the decisions
made 2026-11-09 under R42: D11, D14, D15, E1, E5, E6, E9, M6 and N11; see `design.md`.
Removals finish by 2026-12-01 (owner plan line, before the 2026-12-15 freeze), except the two
batches whose gates fall later: B8b (after the layering merge, about 2026-12-08; by 2026-12-10
or it moves to 1.0.x, E5) and B11 (action cut-over, by 2026-12-15).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No removal lane has been started.

Code facts below were checked read-only in the integration checkout `claude/layers-integ` at
`9ec3a29d0`. Each "no importer" claim is marked VERIFIED (a `git grep` of the import path
`github.com/Beamfall/corvint/<path>` outside the package found nothing, or found only the importers
named) or NOT_VERIFIED. A VERIFIED import check does not cover non-Go references (Makefile, scripts,
`.github`, `.corvint/*.json`, fixtures); each batch's zero-hit `git grep` covers those.

## Why

The owner decided on 2026-11-09 that 1.0 is three products (testing, documentation, task
management) and that the release-blocking runners are Go, Vitest, Jest, Playwright, pytest, RSpec
and Minitest (G1). The owner then answered the source keep-or-remove audit (G22 to G27) and chose to
replace the CEM, OCM and dogfood ceremony fully (G8). Large parts of the tree serve none of that: 16
language analyzers, six non-seven-runner language providers, an out-of-process runner protocol,
test-quality rule sets, the CEM verbs, a VS Code extension and lane tooling. They cost build time,
test time, CI shards and review attention, and several CI files already reference code that is gone.

This proposal orders the removal so each batch compiles, each batch is proven by the same cheap
checks, `.github` edits never mix with other edits, and every batch can be reverted from Git.

## What Changes

- One `.github`-only batch (B0) first repairs references to code that no longer exists.
- Eleven further batches (B1 to B11) remove code in dependency order. Each batch names the exact
  paths removed, the files edited to keep the build compiling, the tests rewritten first, its proof
  and its rollback.
- Before any language provider is removed, `corvint test` must run every test, with a named
  reason, for a change no runner owns or a change for a removed or unavailable runner (RMV-V1-014,
  owner decision R32 (2026-11-09)). Today it does not (see "Open item: a repository that declares
  a removed runner" in `design.md`).
- Code left with no importer by a batch (orphaned code) is deleted in that batch and listed in the
  batch's orphan table (owner decision R33 (2026-11-09); RMV-V1-031).
- Each removal batch is recorded by one `docs/build-log/YYYY-MM-DD-<slug>.md` entry; no numbered
  decision per batch (owner decision R34 (2026-11-09)).
- The per-language test-provider binaries are retired after the single compiled-in runner
  contract (G23) lands and their code is folded into `corvint` (owner decision R14 (2026-11-09)).
- Shared packages under `internal/cem` move before the CEM verbs are removed (RMV-V1-020).
- Retired specs are archived in place with status lines (G27). Nothing is moved out of
  `docs/specs` except the Tasks specs after `beamfall/corvint-tasks` PR #4 merges.

## Capabilities

- Modified: `corvint affected`, `corvint test` (a change no runner owns, or a change for a removed
  or unavailable runner, runs every test with a named reason; owner decision R32 (2026-11-09)).
- Removed: `corvint test-plan`, `corvint tests lint`, `corvint tests duplicates`,
  `corvint tests validity`, `corvint test-validity`, `corvint runs`, `corvint cem`, every
  `corvint-analyzer-*` binary, `corvint-test-validity-mcp`, `corvint-go-toolchain-receipt`, the
  `test-validity` MCP group, the out-of-process runner protocol, the VS Code extension (moved to the
  new public repository `beamfall/corvint-vscode`; owner decision R33 (2026-11-09)).
- Retired after G23 lands, each folded into `corvint` and not shipped as a separate 1.0 artifact
  (owner decision R14 (2026-11-09)): `corvint-go-test-provider`, `corvint-js-test-provider` and
  `corvint-ruby-test-provider` (`cmd/` directories and `tools/release-binary/binaries.json` rows,
  integ 9ec3a29d0). Their retirement is part of the G23 runner-contract change, not a batch here.
- Kept and unchanged: `corvint affected`, `corvint test`, `corvint test-evidence` (labelled
  preview), `corvint flows`, `corvint docs`, `corvint how`, the remaining `corvint tests`
  subcommands (`setup-select`, `teardown`, `unselected`, `role-locators`, `env`, `citations`,
  `example-join`; `cmd/corvint/tests.go:316-334`), the compiled-in `registry.RunnerPlugin`
  interface, `runnerselection`, `testrunner`, `perfbudget`.
- Deferred to the G25 revival proposals: `context`, `impact`, `review`, `feature`, `features`,
  `overview`, `index`. `corvint context` ships redesigned and non-blocking, with no claim of
  measured benefit; its measurement runs in 1.0.x (owner decision R28/R29 (2026-11-09)). No batch
  here removes code that the redesigned context needs before its proposal decides (RMV-V1-023).

## Impact

- Code: about 16 `cmd/corvint-analyzer-*` directories, 14 `internal/analyzer*` packages, six
  `internal/liveverify/affected` providers, the registry plugin files for .NET, JVM and Rust, the
  `internal/cem` packages that are not shared, `interop/cem01-go`, `protocol/cem-*`,
  `extensions/vscode` (moved to `beamfall/corvint-vscode`), `tools/runner-*`, lane scripts,
  `internal/docviews` and every other orphan listed in `path-verdicts.md`. File counts are NOT_VERIFIED; the panel
  estimate for G24 alone (about 60 files deleted, 15 edited) was not checked by a build.
- Release: `tools/release-binary/binaries.json` loses the analyzer rows (`:26-41`),
  `corvint-test-validity-mcp` (`:20`) and `corvint-go-toolchain-receipt` (`:43`).
- CI: `.github/workflows/cem-release-gate.yml`, `intake-containment.yml`, `vscode-extension.yml`
  and `spec-toolchain-conformance.yml` (or its OCM step) go; `ci.yml`, `go-macos.yml`,
  `release-gates.yml` and `.github/cishards/*` are edited. The required-check list in the
  repository ruleset is checked before each `.github` batch lands.
- Adopters: beamfall-apple (Swift) and beamfall-android (Kotlin) run their full declared suites
  until a provider is rebuilt after 1.0 (G26 accepted cost). That only holds once RMV-V1-014 lands.
- Docs: `docs/CAPABILITIES.md`, `AGENTS.md` (through the companion decision), retired specs gain
  status lines, `LICENSING.md` notes.

## Non-goals

- No directory moves or layer restructuring. Removals land first on the integration branch, before
  directory moves (G27).
- No change to provider output formats (G24).
- No design of the G25 revived verbs, the single compiled-in runner contract (G23), the corvint
  action (G17, G18) or the corvint-agents packages (G20). Each has its own proposal.
- No removal of `internal/liveverify/mutate` here; it gets its own ticket (G26).
- No deletion of retired spec files, build-log entries, decisions or benchmark result history.
- No release, tag or publication. No edit of `docs/specs/README.md` or `REQUIREMENTS.tsv` by this
  proposal; when a batch archives a spec, `REQUIREMENTS.tsv` is regenerated with
  `make spec-requirements` in that batch, and retired IDs stay listed as history rows (RMV-V1-026).
- No numbered decision per batch: each batch's verdicts and proof go in one build-log entry
  (owner decision R34 (2026-11-09)).
- The `make windows-vet` exception (once per batch, before that batch's push) covers removal lanes
  only; every other change keeps the every-push rule (owner decision R36 (2026-11-09)).
