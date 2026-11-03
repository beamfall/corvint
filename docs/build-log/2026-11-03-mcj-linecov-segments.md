# MCJ slices 1, 4 and 7: linecov segments, join, shadow and qualification

Base: `bb729132b` (`claude/map-coverage-join-spec`, stacked on PR #79).
Tickets: V1-4383 (slice 1), V1-4387 (slice 4), V1-4390 (slice 7). Requirements: MCJ-V0-001,
003..006, 014, 016..019 in `docs/specs/map-coverage-join-v0.md`. Scope: `internal/linecov` and
the `collect` path of `tools/linecov`. The tickets stay open for the orchestrator.

## Decisions

- **Format.** `linecov-index/3` adds `Index.Segments` (grouped by test, in run order),
  `Block.Segs` (the segments whose reach holds the block) and `Index.Divergent`. An index whose
  records carry no segments stores none: each test reads as one unbound whole-test segment, and
  `Block.Segs` stays nil. That keeps the Go index the same size, reads a `/2` index unchanged
  (MCJ-V0-003), and leaves the `Merge` of PR #77 valid without changes: an index it rebuilds
  without segments reads as one segment per test, which joins no equivalence class. That fails
  closed. A `/2` reader refuses `/3` through its existing format check. `Block.Tests` is still
  written exactly as before, so `Analyze` and `Narrow` select the same tests with or without
  segments.
- **Residual segment.** Blocks a segmented record ran outside every segment form one trailing
  unbound segment. A change reaching them keeps the test. Because condition 4 needs every later
  segment bound, the residual segment also keeps the test out of every earlier segment's class.
- **Duplicates.** `Build` now refuses a test recorded twice; before, it kept the last record
  silently. `Merge` (PR #77) keeps its own refusal.
- **Equivalence key.** A class is a SHA-256 over the identity, the prior state (the bound setup
  record or the identities of earlier segments), the identities of every later segment through the
  end of the test (condition 4, added at `bb729132b`), and the exact block set. A segment that is
  unbound or empty, whose test is `Incomplete`, that belongs to an in-order suite record, or that has
  an unbound earlier or later neighbour joins no class. The reach digest and block count are
  reported as the block-set-equality evidence (MCJ-V0-016).
- **Representative.** The representative is the selected member with the smallest recorded
  elapsed time. A member with a recorded time comes before one without, and ties go to the test
  identity, which is the index order of package then name. A test is reported as would-skip only
  when all of these hold: the change reached it through segments alone, its own source did not
  change (`Effect.Direct`), and every reached segment's class has a selected representative other
  than itself that is not divergent.
- **Shadow only.** `Index.Shadow` takes no plan and returns a report. `Index.Report` shows a
  would-skip test with its real reason and a `WouldSkip` entry, and counts it separately in
  `ShadowCounts`. Nothing in the package can remove a test from a plan (MCJ-V0-017).
- **Misses.** `DetectMisses` compares the report with the full run. An excluded test or a
  representative that did not run is `Unverified`, never passed. A miss drafts a `bugs` ticket as
  data (filing it is the caller's task-store write). `CarryDivergent` keeps a class divergent while
  every file of its reach keeps its blob. `linecov collect -prior INDEX -misses FILE` carries
  divergent classes into the next index. `Qualification.Promotable` refuses on any miss, divergent
  class or unverified exclusion, on fewer than 1,000 changes, or on a segment kind without a
  passing fault-injection keep case and exclude case.
- **Join API for the functionality-link lane.** `Index.Join(Effect)` returns per-test reach,
  segment identities, units (bound segments only, MCJ-V0-002), the changed blocks and the
  `Uncovered` blocks no test ran. `Index.BlocksAt(path, oid, lo, hi)` answers false for a file the
  index does not record at that blob, so the caller treats it as having no record (MCJ-V0-021).

## Evidence

- `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/linecov/... ./tools/linecov/...`:
  pass. `go vet` on both: pass.
- Condition 4 is load-bearing: with the later-identity term removed from the class key,
  `TestSharedStepEquivalence` fails three subtests, among them the case where two tests share a
  step from the same setup but check different things afterwards. The term was then restored.
- MCJ-V0-019 plan time (`TestPlanTimeBudgetWithSegments`). Synthetic reference index: 300 files ×
  60 blocks, 2,000 E2E tests × 8 bound segments × 12 blocks each, 40 shared step sequences, 2.0 MB
  encoded. A 6-hunk change selects 650 tests, and shadow would skip 637 of them. Each run decodes
  the index, then analyses, joins, runs shadow, narrows and reports. Over 40 runs on a 4-core
  development container: p50 42 ms, p95 55 ms, within the 100 ms budget. The p95 on the reference
  host is `NOT_RUN`. `LINECOV_PLAN_P95_BUDGET` overrides the budget on other hosts.

## Limits

- No segment collector exists. Go records are unbound whole-test segments, so on real indexes the
  shadow analysis reports nothing until slice 2 (Playwright) lands. The MCJ-V0-014 matrix runs on
  a synthetic index. Each collector must still run it on its own fixture.
- The join and the report are library APIs. The `select` and `ci` JSON outputs do not carry them,
  because `select` was outside this lane's scope and `ci` lands in PR #77.
- The CI shadow run, its change count and the owner's promotion acceptance (MCJ-V0-017) are
  `NOT_RUN`.
