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
- Amends: decision 0593 (the rest of the inventory is now accepted); the owner acceptances that
  put the cut surfaces in 1.0; `AGENTS.md` product invariants 4, 7 and 8 and the dogfood contract
  where they name a removed surface.
- Retires: every spec whose only implementation this change deletes. Each such spec carries a
  status line naming this decision and keeps its body and requirement IDs as history. Live specs
  that named a removed surface are amended in place. `docs/specs/README.md` lists both.
- Keeps: the `corvint` command groups adopt, affected, changes, diagnostic, help, impact, index,
  init, profile, repository, runs, runtime, signals, source, taskcontext, test, tests, version,
  workspace and worktree; every `corvint-analyzer-*`, `corvint-*-test-provider` and
  `corvint-test-runner`; the release support binaries; `corvint flows appmap` (`AMAP-V0`), which
  E2E-safe selection reads; and every package those reach through kept code.

## Context

By 2026-11-09 Corvint shipped dozens of binaries and command groups around one useful core,
affected-test selection with an explicit unknown frontier. The rest was experimental, mostly
without adopter evidence, and cost CI time, review attention, catalog entries and dogfood
ceremony on every change. The inventory sorted each package, binary and command group into KEEP,
CUT, RETIRE, EXTRACT or CARVE by whether it serves selection for a diff.

## Decision

1. **CUT-CONTEXT.** Context injection and its delivery surfaces leave: the context, query and
   review-pack command families, the MCP servers and MCP tool catalog, the LSP server, the
   OpenCode qualifier, the native hook observer and the evaluation runner that only scored context
   (decision log: `docs/build-log/2026-11-09-refocus-on-test-selection.md`).
2. **CUT-HOSTED.** The hosted and service surfaces leave: the local console, dashboard snapshot,
   post-merge host, workflow, proof, connector and metrics, the corpus index, HTTP, parity and
   republish servers, intake, gateway and remote provider, with their protocols and conformance
   suites.
3. **CUT-OTHER.** The documentation surfaces leave: the `corvint docs` and `corvint skill-export`
   groups, `corvint-docs-mcp`, `corvint-web-flows` and every `corvint flows` verb except
   `appmap`, with `requestpath`, `docmaintain`, `docviews`, `doccitations` and `skillexport`.
   Orphan packages that no kept binary reaches leave with them.
4. **RETIRE-PROCESS.** The CEM and dogfood ceremony that bound every Corvint change to a sealed
   evidence map leaves, with its command families, make targets, scripts and repository metadata.
   Changes are verified by affected-test selection, focused tests, review and CI.
5. **EXTRACT.** Corvint Tasks (`internal/tasks/**`, `internal/taskman`, `cmd/corvint-tasks`,
   `cmd/corvint-work-queue` and their guides) moves to `beamfall/corvint-tasks`, imported from
   this repository at `4663b4e2b`. Corvint no longer builds or bundles `corvint-tasks`; the
   repository keeps using an installed `corvint-tasks` for its own `.taskman/` store.
6. **CARVE.** Where a kept package imported a cut one, the edge is severed in the kept package:
   `internal/lineanchor` takes the line anchors test discovery needed from `doccorpus`, and
   `cmd/corvint/runs_baseline.go` keeps its own clean-path check instead of importing
   `evidencereach`.
7. The `.github` changes (jobs, `.github/cishards` entries and workflow tests for the removed
   surfaces) go in a separate change, because `github-split-check` (`ASD-V0-706`) refuses a range
   that mixes `.github` with code.

## Rollback

Revert the change. Nothing persisted or published depends on the removed binaries. Old CEMs,
build-log entries and specs that name them stay readable as history, and Tasks history continues
in `beamfall/corvint-tasks`.
