# Decision 0594: Refocus Corvint on test selection

- Date: 2026-11-09
- Status: accepted (owner, 2026-11-09).
- Authority:
  - Owner chat, 2026-11-09, after decision 0593 and the published Corvint Focus Inventory: "I feel
    we can just swing a hammer and get a lot of the Corvint Focus Inventory done in one go." The
    owner then selected every remaining family: "Context + hosted + other", "Retire process
    ceremony", "Extract Tasks" and "CARVE splits".
  - This accepts the inventory's product sentence that decision 0593 left as a proposal: given a
    diff, Corvint names exactly the tests that must run and why, and proves none were missed.
  - Owner chat, 2026-11-09, amendment before the change was published: Corvint must still do
    documentation ("mcp, auto docs generation, flows, E2E mapping"), testing and task
    management, and "if there are other ways this could be extracted into new repos that is
    fine". The owner then approved the layout in items 1, 3 and 5 below ("Yes, amend and
    rebuild").
  - Owner chat, 2026-11-09, on repository layout: "I'd like it to be optimized for development,
    test and release speed", then "Yes, layers" to the layout in item 8.
- Amends: decision 0593 (the rest of the inventory is now accepted); the owner acceptances that
  put the cut surfaces in 1.0; `AGENTS.md` product invariants 4, 7 and 8 and the dogfood contract
  where they name a removed surface.
- Retires: every spec whose only implementation this change deletes. Each such spec carries a
  status line naming this decision and keeps its body and requirement IDs as history. Live specs
  that named a removed surface are amended in place. `docs/specs/README.md` lists both.
- Keeps: the `corvint` command groups adopt, affected, cem (item 4), changes, diagnostic, help, impact, index,
  init, profile, repository, runs, runtime, signals, source, taskcontext, test, tests, version,
  workspace and worktree, with docs, flows and how (item 3); every `corvint-analyzer-*`, `corvint-*-test-provider` and
  `corvint-test-runner`; the release support binaries; `corvint-mcp`, `corvint-test-validity-mcp`,
  `corvint-docs-mcp` and `corvint-web-flows`; and every package those reach through kept code.

## Context

By 2026-11-09 Corvint shipped dozens of binaries and command groups around one useful core,
affected-test selection with an explicit unknown frontier. The rest was experimental, mostly
without adopter evidence, and cost CI time, review attention, catalog entries and dogfood
ceremony on every change. The inventory sorted each package, binary and command group into KEEP,
CUT, RETIRE, EXTRACT or CARVE by whether it serves selection for a diff.

## Decision

1. **CUT-CONTEXT.** Context injection and its delivery surfaces leave: the context, query and
   review-pack command families, the MCP context and query tools, the LSP server, the OpenCode
   qualifier, the native hook observer and the evaluation runner that only scored context
   (decision log: `docs/build-log/2026-11-09-refocus-on-test-selection.md`). MCP itself stays:
   each product owns its tools, and `corvint-mcp` serves impact, test validity, help and status.
2. **CUT-HOSTED.** The hosted and service surfaces leave: the local console, dashboard snapshot,
   post-merge host, workflow, proof, connector and metrics, the corpus index, HTTP, parity and
   republish servers, intake, gateway and remote provider, with their protocols and conformance
   suites.
3. **CUT-OTHER.** `corvint skill-export` with `skillexport` leaves, and orphan packages that no
   kept binary reaches leave with it. Documentation and flows stay as a product: the
   `corvint docs` group (generation, corpus and maintenance), every `corvint flows` verb,
   `corvint how`, `corvint-docs-mcp` and `corvint-web-flows`, with `requestpath`, `docmaintain`,
   `docviews` and `doccitations`. They stay in this repository, because flows and E2E mapping
   read the same index that selection reads. A follow-up moves the documentation verbs and
   tools into a separate `corvint-docs` binary here, behind a rule that core packages do not
   import documentation packages. The corpus servers stay cut under item 2.
4. **RETIRE-PROCESS.** The CEM and dogfood ceremony that bound every Corvint change to a sealed
   evidence map leaves, with its command families, make targets, scripts and repository metadata.
   Changes are verified by affected-test selection, focused tests, review and CI. The product
   commands that create and check a change evidence map stay (owner, 2026-11-09: "isn't it
   still useful?", then "do it"): `corvint cem` keeps begin, prepare, cite, mark, cover,
   status, verify, report and verify-stable, on `internal/cem`. Its anchor, provenance,
   discriminate, export, defer, bundle and gate-verdict actions leave, because they need
   removed packages.
5. **EXTRACT.** Corvint Tasks (`internal/tasks/**`, `internal/taskman`, `cmd/corvint-tasks`,
   `cmd/corvint-work-queue` and their guides) moves to `beamfall/corvint-tasks`, imported from
   this repository at `4663b4e2b`. Corvint no longer builds or bundles `corvint-tasks`; the
   repository keeps using an installed `corvint-tasks` for its own `.taskman/` store. Tasks
   owns its MCP tools. Before this change merges, the core directories that still import
   Tasks packages are checked and severed or moved.
6. **CARVE.** Where a kept package imported a cut one, the edge is severed in the kept package:
   `internal/lineanchor` takes the line anchors test discovery needed from `doccorpus`, and
   `cmd/corvint/runs_baseline.go` keeps its own clean-path check instead of importing
   `evidencereach`.
7. The `.github` changes (jobs, `.github/cishards` entries and workflow tests for the removed
   surfaces) go in a separate change, because `github-split-check` (`ASD-V0-706`) refuses a range
   that mixes `.github` with code.
8. **Layout direction: layers, not more repositories.** The engine stays one repository,
   `beamfall/corvint`, organised as layers with an enforced import direction; Tasks stays in
   `beamfall/corvint-tasks`; `protocol/**` may later move to its own repository. A separate
   repository per layer was rejected, because every change to the shared layer would then need
   a tag and a version-bump change in each consumer. The layers are:
   - shared: Git pinning, the index, flows and E2E mapping, CLI and MCP plumbing (library only);
   - analyzers: one layer, pluggable per language, one binary each;
   - tests: selection, running and review (`corvint`);
   - docs: generation and corpus reads (`corvint-docs`, item 3);
   - release tooling, which serves the others and is not shipped.

   Shared imports no other layer, and analyzers, tests and docs do not import each other. This
   item records direction only and changes no code. Follow-up tickets measure per-layer test
   time and cross-layer imports first, then enforce the import rule, split the `cmd/corvint`
   test package, shard CI by layer and release each binary independently. `AGENTS.md`
   invariant 7 is reworded in the change that ships the first separate product binary.

## Rollback

Revert the change. Nothing persisted or published depends on the removed binaries. Old CEMs,
build-log entries and specs that name them stay readable as history, and Tasks history continues
in `beamfall/corvint-tasks`.
