# Decision 0594: Corvint refocuses on test selection

Owner decision 2026-11-09; base `4663b4e2b`, integration branch `claude/bigcut`. The change was
built in five lanes (context, hosted and other, docs, process, tasks) plus a release-tooling
pass, merged into one branch. Each lane kept its own notes. This entry records what the
integration reviewer needs: what left, what stayed and why, the open calls and how they were
settled.

## What left

- **Context (CUT-CONTEXT).** CONTEXT_LANE_PLACEHOLDER
- **Hosted (CUT-HOSTED).**
  - Console and dashboard: `internal/console`, `internal/dashboard`, `cmd/corvint-console`,
    `cmd/corvint-dashboard-snapshot`.
  - Post-merge: host, workflow, proof, connector and metrics packages and binaries.
  - Corpus servers and republish: `internal/corpusindex`, `corpusserve`, `corpusrepublish`,
    `cmd/corvint-corpus-{mcp,http,parity,republish}`.
  - Intake, gateway and remote provider: `internal/intake`, `modelgateway`, `remoteprovider`
    and their binaries.
  - Protocols and conformance: `protocol/intake`, `protocol/postmerge-host`, and
    `conformance/{dashboard-snapshot-v0,postmerge-runtime-v2}`.
- **Other (CUT-OTHER).**
  - Packages `internal/analyzercap`, `analyzerexec`, `runidentity`, `shaderrunner` and
    `evidencereach`.
  - `corvint runs fresh`, `runs identity` and `runs reusable --fresh`, which now refuse with
    `runs-invalid-arguments`.
- **Docs (CUT-OTHER).**
  - The `corvint skill-export` group with `internal/skillexport`.
  - `tools/docs-blinded-task`.
  - `corvint docs corpus` tests and fixtures that needed the cut corpus index or CEM stable
    fixtures. The `docs corpus cem` and `cem-stable` verbs still exist without them.
- **Process (RETIRE-PROCESS).**
  - The CEM, dogfood, OCM, frontier, witness, LRF, authority, lease, step, work, obligations,
    observations, self, prove, delta and gate command families, with their packages.
  - `internal/cem` keeps only `cemcode`, `coverprofile`, `gitauth`, `gitrun`, `patch`, `sim`,
    `verify` and `wire`.
  - Every `dogfood-*` make target except `dogfood-merge-base`, plus the dogfood and CEM scripts
    and `.corvint/cem-lineage.tsv`.
  - `docs/DOGFOOD.md` is now a retirement stub, because retained skills and checks still cite
    its path.
- **Tasks (EXTRACT).**
  - `internal/tasks/**`, `internal/taskman`, `internal/taskbarrier` and
    `internal/criterionexperiment`.
  - `cmd/corvint-tasks` and `cmd/corvint-work-queue`, the MCP tasks group and the Tasks guides.
  - All of these were imported into `beamfall/corvint-tasks` from `4663b4e2b`.
- **Release tooling.**
  - Removed: `internal/companionrelease` with `currentproof`, `cmd/corvint-companion-release`,
    `cmd/corvint-public-release-check` and `script/corvint-companion-release-gate`. With no
    companions left, nothing remains for these to build or qualify.
  - The Core-only code they held (source export, archive, toolchain and the stable readers)
    moved to the new `internal/corerelease`.
  - `internal/releasecandidate` keeps only the core profile (`PRS-V1-005`).
  - `examples/cem/experiments/` left with its binary.
  - Retired: `PUB-V0-002/003/011/013..016/020`, `ASD-V0-825`, `CTF-V0-008`, `CSD-V0-7517`.
  - Amended in place: `PRS-V1-005`, `UPD-V0-367` and `PUB-V0-022..026`, among others.
  - `readiness.go` still emits the operator rule `gate/companion-release` for wire
    compatibility.

## What stayed and why

- **Documentation and flows (amendment, same day).** The first build cut the `corvint docs`
  group, every flows verb except `appmap`, `corvint-docs-mcp` and `corvint-web-flows`. The
  owner then restated that documentation (MCP, generated docs, flows, E2E mapping) is a
  product goal, so those cuts were reverted on `claude/bigcut-keepdocs` and their spec
  retirements undone. They stay in this repository because they read the selection index. A
  follow-up splits them into a `corvint-docs` binary behind an import rule.

- **Discovery adapter.** `cmd/corvint-cli` and `internal/cliprofile` provide CAP-V0-9300
  discovery for the 16 protocol-only analyzers that stay. Cutting them would hide KEEP roots.
- **Shadow selection code.** `runnerpolicy`, `testrunner/golang`, `testrunner/swiftselection`
  and `appflows/e2erunner` hold live RSP requirements. One is the RSP-V0-057 confirmed-miss
  logic, which is no-miss proof, so all four stay with the product even though nothing imports
  them yet.
- **Packages kept code still reaches.** `appmap` and `appflows` serve E2E-safe selection.
  `doccorpus` keeps aliases over the new `internal/lineanchor` kernel. `releasegate`,
  `perfbudget`, `browsercov`, `closureguard` and `execfile` also stay.
- **The observations ledger writer.** It is still written by kept code. `AGENTS.md` no longer
  grants the ledger exception, so the writer needs its own cut; that is tracked separately.
- **CEM 1.0 as a published protocol.** `protocol/cem-1.0` and `interop/cem01-go` stay, because
  the kept `internal/cem/verify` reads them.
- **Tasks as an operator tool.** The repository still runs an installed `corvint-tasks` against
  its own `.taskman/` store. `corvint-update --component tasks` now resolves releases from
  `beamfall/corvint-tasks`.

## Specs

- Retired specs keep their bodies and requirement IDs. Each has these status lines:
  - `Intent status: retired 2026-11-09 (decision 0594)`
  - `Delivery status: retired`
- Each retirement note names base `4663b4e2b` or, for Tasks, the `beamfall/corvint-tasks` copy.
- Live specs that named a removed surface are amended in place. `docs/specs/README.md` and
  `INDEX.json` are regenerated from them.

## Deferred to a `.github`-only change

`github-split-check` (`ASD-V0-706`) refuses a range that mixes `.github` with code. These items
therefore follow in a separate change:

- the `.github/cishards` entries for every deleted package;
- the `ci.yml` dogfood batch-check, local-authority, web-flows and Tasks jobs;
- the intake-containment, post-merge, console and companion-release workflows;
- the go-macos local-authority entries.

The `.corvint/test-read-scope-literals.json` bridge rows stay until that change lands.

## Open tickets

The tickets for the cut families stay open in this change. They are dispositioned against
decision 0594 after it merges. Friction found while building it is filed as V1-5228 to V1-5232.
