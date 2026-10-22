# bf-1022q: Beamfall takeover restored to 1.0, consumers fenced, flow-docs Beamfall qualification

Date: 2026-10-22. Tickets: V1-0464, V1-0461. Lane base: origin/main 6e90476ec.

## Problem

The 2026-10-21 amendment moved `PRS-V1-014` and `PRS-V1-016` (the Beamfall roadmap takeover)
post-1.0 because its prerequisite V1-0398 was post-1.0. On 2026-10-22 the owner moved V1-0398 to
milestone v1-0, and V1-0464 already carries v1-0. The release contract therefore misstated the 1.0
scope. In Beamfall core-history, ADR-0227 (PR beamfall/core-history#44) retired the roadmap
dispatcher, but several readers and writers still used the retired `.roadmap` state. A first
flow-docs run on Beamfall also produced false flows and bindings.

## Per-criterion status

| Ticket / criterion | Status | Evidence |
| --- | --- | --- |
| V1-0464 C1 (activation decision) | met | Decision 0544 item 2 (commit 50a9f5318); ADR-0227 in core-history (0f494e72b) |
| V1-0464 C2/C4 (fence legacy writers, migrate consumers) | delivered for review | beamfall/core-history#45, commit 61ba6c364 on `claude/rewire-roadmap-consumers` (base #44) |
| V1-0464 C3/C6 (production native completion, readback, receipt audit) | NOT_PRODUCED | Needs native Tasks mutations and completion; out of this lane's authority |
| V1-0461 C1 (release contract amendment) | delivered | Amendment 2026-10-22 in `docs/specs/corvint-1.0-product-and-release-v1.md` |
| V1-0461 C2 (owning gates on one frozen candidate) | NOT_PRODUCED | Needs a frozen release candidate |
| V1-0461 C3 (Tasks replaces Beamfall writers; owner switch) | partial | Writers fenced in #45; real lifecycle rehearsal and owner switch still required |
| V1-0461 C4 (docs and flow docs on Corvint and Beamfall) | Beamfall run delivered | Section "Beamfall qualification" below; Corvint run retained in `2026-10-21-v1-0461-docs-flows-e2e.md` |
| V1-0461 C5 (final gates, stable promotion) | NOT_PRODUCED | Release gates and owner promotion decision |

## Decisions

- **1.0 scope.** `PRS-V1-014` and `PRS-V1-016` are again stable 1.0 requirements. The digest
  (and the generated `INDEX.json` and `README.md` rows), the binary table row, the requirement text
  and the traceability row were updated. The 2026-10-21 paragraph stays as history. Stable 1.0
  still makes no takeover claim until production completion and the concrete owner authority switch
  are retained.
- **core-history consumers (#45).**
  - `bf_tools.py` roadmap verbs, `ticket-blockers` and `context-packet --ticket-id` now read
    `corvint-tasks` (`queue status`, `roadmap`, `ticket show`) and fail closed unless the envelope is
    `OK`.
  - `release-readiness.sh` reads the queue write barrier and `receipt audit`; a missing store is
    evidence-missing.
  - The legacy writer verbs in `roadmap_lib.py`, `roadmap_pipeline.py`, `apply-allowed-paths.py` and
    `import-to-backlog.py --execute` exit 3 with an ADR-0227 message.
  - The nightly red contract no longer writes `.roadmap/fanout-pauses`.
  - `retired_roadmap_writers_test.sh` passes with the guards and has 31 failures with them reverted.
- **Flow-docs extractor (AFU-V1-043, AFU-V1-045).** A named arrow is a declaration only when the
  arrow is the direct initializer, optionally wrapped in `useCallback`, `useMemo`, `memo` or
  `forwardRef`. An HTTP-verb registration needs a `/`-rooted path literal followed by a handler
  argument. The generator version moved to 3; version 1 and 2 manifests are refused with
  `flowdocs-generator-stale`. The requirement text amendments are accepted under the owner's
  standing auto-accept (2026-10-11).

## Beamfall qualification (scratch clone of core-history origin/main ae9feed41)

- **`docs flows generate`, generator 2 (before the fix).**
  - `src/lib`: 53 flows. `src/components`: 56 flows.
  - False positives included `binding:get:Content-Length` from `response.headers.get(...)` and a
    flow `matched` from `messages.map((message) => ...)`.
- **Generator 3.**
  - `src/lib`: 41 flows. `src/components`: 51 flows. Nothing was added.
  - The 17 removed entries were all member-call values, IIFEs or a header read.
- **`docs flows check` at the base.** Exit 0, clean, 381 bindings.
- **Scratch drift commit 858ebbf1d** (a call added to `resetIdentityEpoch`). `check` exits 1; the
  flow is stale with "evidence anchor inventory changed".
- **Generator-2 manifest.** Exit 2 with `flowdocs-generator-stale`.
- **`docs maintain` on a seeded page for `internal/beam`.**
  - `--preview` wrote nothing.
  - `--apply` wrote the generated block and kept both human paragraphs.
  - A second `--apply` was a no-op.
  - After a source drift commit (38d3b3850), `--preview` proposed a diff naming the new symbol.

## Tests

- **Packages.** Focused tests via `heavy.sh` with `GOTOOLCHAIN=local` go1.27.2: `./internal/flowdocs/`
  (whole package) and `-run 'TestFlowDocs|TestDocsFlows' ./cmd/corvint/`.
- **New and changed tests.** `TestFlowDocsWebMemberCallsAreNotDeclarations` and
  `TestFlowDocsPredecessorGeneratorManifestIsTypedStale` (now covers version 2) both fail on the
  base code.
- **Review.** One Codex read-only review round found two regressions, both fixed with test cases:
  direct arrows with nested parameter parentheses (`(cb: () => void) => cb()`), and route
  registrations with a comment or line break before the handler comma. Rerunning generator 3 on
  Beamfall gave the same 41 and 51 flows, a clean base check and an exit-1 drift check.
- **core-history.** `retired_roadmap_writers_test.sh`, `release_readiness_test.sh`,
  `nightly-freshness_test.sh`, `roadmap_lib_test` and the new `bf_tools_test` check pass locally.
  Beamfall private Actions are billing-blocked, so CI was NOT_RUN.

## NOT_RUN and residuals

- **NOT_RUN.**
  - `make gate`.
  - Beamfall CI (billing-blocked).
  - Real single- and multi-repository Tasks lifecycle and recovery rehearsal on Beamfall.
  - Frozen-candidate qualification.
- **Residuals.**
  - The `docs flows generate` 256-flow refusal surfaces as the untyped `internal-error`.
  - TypeScript generic calls (`useMemo<T>(() => ...)`) make the web lexical mask swallow the rest
    of the statement and the next line.
  - `docs maintain` reports `documentation-limit-exceeded` without naming the limit
    (`internal/auth`).
  - Non-literal HTTP-verb member reads (`params.get(key)`) still add "computed web registration
    unresolved" gaps.
  - core-history consumers not yet rewired (listed in #45).

## Rollback

- Revert this lane's commit. Generator 3 manifests then become unsupported under generator 2, so
  regenerate them.
- Close beamfall/core-history#45 unmerged.
