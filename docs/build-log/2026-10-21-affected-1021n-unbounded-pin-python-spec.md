# 2026-10-21: affected-1021n, appmap-only unbounded-reader pin and Python adapter spec

Lane `affected-1021n`, base `58d4ed2d309d5dc1afc21c127647d2e22b1bed3f`. Tickets V1-0999 and V1-2050
were delivered. The rest of the lane's tickets are dispositioned below. AFP-V0-180 to AFP-V0-186 are
accepted under the owner's standing auto-accept (2026-10-11).

## Problem

- **V1-0999.** `corvint affected` selected many `UNBOUNDED_READER` units for a change confined to
  `internal/appmap`. Nothing tied that set to a reviewed reason, and no test pinned it.
- **V1-2050.** `affected/python` is registered in `languages.All()` but had no owning
  requirements, non-goals, failure modes or qualification label. While writing them, a soundness gap
  showed up. `from pkg import sub` gave an edge only to `pkg`. A change to `pkg/sub.py` selected no
  test and reported `NO_SELECTABLE_TEST`. A probe confirmed this, with and without
  `pkg/__init__.py`.

## Decisions

- **AFP-V0-180.** The justification already exists: the AFP-V0-033 ledger
  `.corvint/unbounded-readers.json` records a reason for every unbounded reader, and
  `unbounded-readers-check` enforces it.
  - On 2026-10-21 an appmap-only change selected 56 units. 15 of them were Go `UNBOUNDED_READER`
    units, and every one has a recorded reason: the filesystem root `/`, `.git`, a whole-repository
    walk, or a built binary's import closure. None can be narrowed without hiding a real read.
  - The new live test pins three things: the reader set equals the recorded set not otherwise
    reached, each reader's witness shape, and that every other witness resolves.
  - No selection behaviour changed.
- **AFP-V0-181 to AFP-V0-186.** A new "Python adapter (experimental)" subsection of
  `docs/specs/affected-plan-v0.md` owns the adapter.
  - Existing tests are traced to these requirements.
  - Gaps are listed: the relative-import frontier raise, unreadable source, and the corpus
    narrowing test. That test skips because its `src/` corpus is not tracked on `origin/main`.
- **AFP-V0-183** fixes the gap above. Each name on a `from X import ...` line is also tried as the
  submodule `X.name`. A name that is not a submodule resolves to the same prefix as before, so the
  change only adds edges.

## Evidence

- `TestAFPV0180AppmapOnlyChangeSelectsOnlyRecordedUnboundedReaders` passes. It fails when one
  ledger reason (`internal/flowdocs`) is blanked.
- `TestAFPV0183FromImportNamesReachSubmodules`: all four subtests failed before the parser change
  and pass after it.
- `TestParseImportLineReadsEveryStatementForm` was updated for the added refs. The whole
  `./internal/liveverify/affected/python/` package passes.

## Not delivered

- **V1-2252: already delivered** by e66a5bf30 (AFP-V0-162 tiers and recipe). The remaining
  criteria (charters citing the subset, and the 10-batch measurement) belong to the orchestrator.
- **V1-2162: already delivered** by 8bfc7c4e2 (AFP-V0-156) for its code criterion. The measured
  drop did not happen, because the dependents inherit from `internal/jstestprovider`. That residual
  is V1-2223.
- **V1-2223: skipped; needs a design.**
  - `worksource` and `postmergeconnector` are already declared.
  - `jstestprovider` runs `git rev-parse --show-toplevel`, then `git log`, `rev-list` and `status`
    at that root. Those are real `.git` reads that AFP-V0-023 cannot grant.
  - The `-unresolved` ledger (105 at base, 62 of them via `jstestprovider`) counts packages whatever
    they declare.
- **V1-1803: skipped; needs a design (V1-2163).** Glob-literal readers touch the accepted rule (c)
  lexicon, and an earlier lane judged them unsound without a design.
- **V1-2071: skipped.** The impact half lives in `internal/contextindex`, which is off limits for
  this lane. The affected half already reads AGENTS.md Verify fences without a Makefile.
- **V1-2292: skipped.** Splitting the packages needs a Linux Landlock measurement to declare the
  new package. Accepting them as floor members instead is an owner or orchestrator decision.

## NOT_RUN

- The tier-2 packages from `corvint affected` were not run locally; they are left to batch CI.
- `make gate` was not run, per the lane charter.

## Rollback

- Revert the name loop in `parseImportLine` and the test expectations.
- Delete the two new tests and the AFP-V0-180 to AFP-V0-186 text and rows, then regenerate the
  spec index.
