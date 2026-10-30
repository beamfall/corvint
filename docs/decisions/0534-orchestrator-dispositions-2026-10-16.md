# Decision 0534 — Orchestrator dispositions of the open owner questions, 2026-10-16

Date: 2026-10-16. Status: accepted. Authority: owner delegated to the orchestrator (2026-10-16).
On that date the owner handed every remaining open owner decision to the orchestrator, and the
orchestrator decided them as recorded here. Release publication and 1.0 promotion stay with the
owner (AGENTS.md, "Standing publication approval").

## Context

The queue held owner questions from several lanes. Some asked whether to amend accepted
requirements, some asked about qualification routes, and some asked whether work is in scope for
1.0. Each disposition below names its ticket, the decision and a one-line rationale. The record
amends one accepted requirement, `DCG-V0-001` (V1-1480), and implements it with V1-1490's
documentation in the same change. The other dispositions are carried out by the lanes named.

## Dispositions

1. **V1-0614**: keep `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`. There is no `CEM-V1-050`/`CEM-V1-091`
   amendment, and the frozen S0E qualification values stay as they are. The ticket closes on the
   evidenced narrowing PR #956 delivered. Rationale: the structural tuple has not been qualified
   over the full corpus, so a stable label would claim more than the evidence shows.
2. **V1-1490**: option (a). A sealed dogfood enrollment ends at the bind commit, and the next
   enrollment needs a fresh worktree. `LCP-V0-002`, `LCP-V0-003` and `LCP-V0-020` are not amended.
   Rationale: documenting the boundary fixes the misleading `verify` offer without amending
   accepted requirements. Implemented in this change in `docs/DOGFOOD.md`, "Enrolled local
   completion policy".
3. **V1-1469**: `ARTIFACT-V0-008` covers every `.github/workflows/*.yml`, with a declared per-job
   exception list. Its first entry is the `release-gates.yml` `nightly-failure-signal` job's
   `issues: write`. Rationale: least privilege should hold for every workflow, and the one job
   that needs a write permission is named rather than left unchecked. A separate lane implements
   it.
4. **V1-1480**: `DCG-V0-001`'s document set includes `docs/RELEASE-NOTES-*.md`. Rationale: release
   notes carry pinned line citations, so a line shift in a cited file failed in CI but passed every
   local doc gate. Implemented in this change in `script/check-line-citations.sh` (`scanned_doc`)
   and `docs/specs/documentation-citation-gate-v0.md`.
5. **V1-1475**: no new external receipt profile revision (`/0`..`/3`) for 1.0. The route-stub
   record stays on the internal profile. When the producer is absent, the receipt gets an explicit
   `NOT_PRODUCED` reason, implemented later. Rationale: a public wire revision late in the 1.0
   cycle costs more than it is worth. An explicit absence reason is enough to stop silent omission.
6. **V1-0557**: public `origin/main` is the integration and intent authority, through the
   `CAL-V0-017` upstream reachability path. Rollback is the existing local-main path. The ticket
   closes after the V1-1459 copied-store qualification. Rationale: the public history is the one
   that lands (decision 0331), and `CAL-V0-017` already defines how reachability is read.
7. **V1-0650**: run S0E cases 21 and 22 under `GOTOOLCHAIN=go1.24.13`, the official toolchain,
   verified against the checksum database. Run the Linux lifecycle on CI Linux runners; where that
   is not possible, record it as an explicit exclusion. Rationale: the cases name that toolchain,
   and running them beats leaving them `NOT_RUN`.
8. **V1-1315**: activate the `corvint test` shadow now by setting its trusted pins. Switch the
   execution call site only after the `AFP-V0-029` bar is met, with `AFP-V0-016` consent. V1-0812
   is unchanged: no narrowing before the retained 20/5 bar. Rationale: the shadow collects
   evidence at no risk, and the cutover stays behind its accepted bar.
9. **Deferred past 1.0**: V1-1023 and V1-1024, V1-1450, V1-1451, and V1-0328. V1-0328 is measure
   only, with no rebuild change for 1.0. Rationale: none of them blocks 1.0. Any new or amended
   spec text for them still needs acceptance. (The orchestrator's brief said no spec was drafted
   for V1-1023/V1-1024. In fact accepted contracts exist: TCN-V0 under decision 0457 and
   LPCV-V0-057..070 under decision 0458; see `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-16-v1-1023-1024-console-qualification.md`.
   The deferral covers the work those tickets still have open, such as the `LPCV-V0-070` live
   matrix, which is `NOT_RUN`.)
10. **Not delegated**: V1-0019, V1-0020 and V1-0021 (release publication and 1.0 promotion stay
    with the owner under AGENTS.md), and V1-0014 (external). This record does not decide them.

## Limits

This record does not close any ticket. Each ticket closes when its own acceptance evidence is
retained. Only the V1-1480 and V1-1490 changes are delivered here. `make gate` was not run for this
change (`NOT_RUN`); the focused citation-gate test and the local doc gates were.

## Rollback

Revert this decision and its index row. For V1-1480, also remove `RELEASE-NOTES-[^/]*` from
`scanned_doc` in `script/check-line-citations.sh`, the spec amendment and evidence row in
`docs/specs/documentation-citation-gate-v0.md`, and test case 23 from
`script/check-line-citations_test.sh`, then regenerate `docs/specs/REQUIREMENTS.tsv`. For V1-1490,
remove the bind-commit paragraph from `docs/DOGFOOD.md`. Every other disposition takes effect only
through the lane that implements it, and is rolled back there.
