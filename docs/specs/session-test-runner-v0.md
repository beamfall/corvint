# Session Test Runner V0

Owner: Russell Lewis
Date: 2026-10-11
Requirement prefix: `STR-V0`
Intent status: proposed (owner-directed 2026-10-11; V1-0040)
Delivery status: experimental (slice 1: Go adapter, dry-run, receipts, universe placement, verify-shards)
Authoritative inputs: V1-0040 owner request (2026-09-19 recommendation, widened 2026-10-11);
`docs/specs/affected-plan-v0.md` (AFP-V0-011..013, 022, 023, 028..030, 041);
`docs/specs/live-proof-carrying-verification-v0.md` (LPCV-V0-021..026);
`docs/specs/go-live-test-provider-v0.md`; AGENTS.md invariants 2, 3, 4, 7 and 8.

## Agent digest
- Claim: `corvint test` plans from the diff, runs only selected tests through runner adapters, shards deterministically, and falls back alike locally and in CI.
- Status: proposed (owner-directed 2026-10-11; V1-0040)/experimental (slice 1: Go adapter, dry-run, receipts, universe placement, verify-shards)
- Exists: the pieces, under separate drivers: `corvint affected` (plan), `script/gate-affected.sh` (local driver), `tools/corvint-pr-tests` and `tools/gate-affected-select` (CI driver), `.github/cishards` (complete-universe partition and test slices), `cmd/corvint-go-test-provider` (experimental Go execute provider).
- Blocked on: owner acceptance; driver parity evidence (STR-V0-013) before either call site moves; AFP-V0-029 shadow bar for any CI narrowing, which this spec does not relax.
- Read next: Requirements; Boundaries and failure modes; Rollout and rollback.

## User and verified baseline

Affected user: an engineer or agent who wants the tests a change needs, run once, quickly, with
the same answer on their machine and in CI. Measurable job: for one session (one base, one
revision, one worktree state) produce and execute a deterministic run set, record exactly what ran,
and never present an omission as a pass.

Verified state at `645304a2fb71065f9061d8d8d866600d703e84c6`:

- `corvint affected --base` emits `affected-plan/1` with selected units across eight language
  plugins, an unknown frontier, and runnable argv only for Go (`provider.go.packages`); the
  Playwright profile emits file commands separately. It executes nothing (AFP-V0-001).
- Two drivers plan the same selection through different code: `make gate-affected`
  (`script/gate-affected.sh`, AFP-V0-011/012) locally, and `tools/corvint-pr-tests` with
  `tools/gate-affected-select` (AFP-V0-013) in CI. Both run Go only.
- `.github/workflows/ci.yml` runs eight `go-product-shard` jobs. Its trusted tool pins are empty,
  so the race-test invocation never narrows today; it is skipped only in the `DOCS` and `REUSE`
  modes, and a `ci:batched` constituent (AFP-V0-027) skips the shard jobs. When pinned, each
  shard would intersect its selection with the AFP-V0-022 complete-universe partition, leaving
  shards idle when the selection clusters, so full and narrowed shards cannot omit a package. `tools/corvint-pr-tests` places whole packages
  and refuses sharded execution while AFP-V0-041 slices are active, as they are for
  `internal/tasks/store`.
- `.github/cishards` already places packages by advisory cost and splits allowed packages into
  test-name slices (AFP-V0-041); placement never changes membership.
- LPCV (decision 0047) specifies runtime providers with discovery, plan, execute and cancel, but is
  `not-started`; the Go live-test provider is experimental and its containment is unqualified.

Simpler baseline: keep the two drivers and run the full suite. That baseline remains the fallback
of every requirement below and the binding gate until this spec is promoted.

## Requirements

- `STR-V0-001`: `corvint test --base FULL_COMMIT_ID [--shard I/N] [--placement universe|balanced]
  [--dry-run] [--receipt PATH]` SHALL plan in process through the same code and profile as
  `corvint affected --base` and record that plan's graph and plan digests and the STR-V0-017
  source identity. It is an executing
  command, never classified as a read command: it MUST NOT write tracked files, the index, trace
  stores or the `.corvint/` ledgers, and writes only its receipt and runner-owned caches.
  `--dry-run` prints the run set, argv and verdict and executes nothing.
- `STR-V0-002`: The run set SHALL be, per language, the union of the plan's selected units and the
  AFP-V0-012 attribution for every dirty path, expanded into execution units. An execution unit is
  a canonical (unit, slice) pair: slice `whole`, or one AFP-V0-041 slice, the catch-all included,
  taken from the slice table in the run's inputs. A split unit is complete only when every one of
  its expected slices is complete. Advice strings MUST NOT execute. Repository-declared
  mandatory checks (`advice.checks`) are reported as still required and never counted as run.
- `STR-V0-003`: The command SHALL run the complete relevant suite (`FULL`), naming the reason as
  `corvint-test: FALLBACK <reason>`, under every AFP-V0-011 fallback condition, and also when a
  module-level frontier or any `plan.unknown` entry outside the AFP-V0-028 reviewed allowlist is
  present. `FULL` covers every language with selected or unknown units, never only the languages
  that narrowed. When no plan can be produced, `FULL` SHALL run every full-suite command the
  repository declares (the Makefile `go-test` target and each declared language full command),
  read independently of the planner; when that declaration cannot be read or names nothing, the
  run is `INCOMPLETE` and exits nonzero. The verdict is a pure function of the plan, the declared
  full scope and the source identity, so every host and shard reaches it from the same inputs.
- `STR-V0-004`: Execution SHALL go through registered runner adapters, one per runner, each with a
  versioned capability manifest (the discover, plan, execute and cancel subset of LPCV-V0-021) and
  the exact runner version captured before execution. The first adapter is Go. A selected or
  unknown unit in a language with no qualified adapter SHALL run that language's
  repository-declared full command; when none is declared, the run is `INCOMPLETE` and exits
  nonzero. A selected unit is never silently skipped.
- `STR-V0-005`: An adapter SHALL receive only its selected targets and invoke the runner's native
  target filter (for example `go test <packages>`, `cargo test -p <crate>`, `swift test --filter`,
  `dotnet test <project>`, `gradle :<module>:test`, or Playwright files reconciled by discovery as
  `typescript-javascript-affected-adapter-v0.md` requires), so the runner compiles only those
  targets' dependency closure. Corvint SHALL NOT run a separate whole-repository build first, and
  SHALL NOT claim build or isolation guarantees stronger than the runner supplies (LPCV-V0-025).
- `STR-V0-006`: Adapter argv SHALL be typed with no shell. Environment, AFP-V0-023 `test-confine`
  wrapper and cache policy SHALL equal the repository's full-suite invocation; only the target set
  may differ between narrowed and `FULL` runs (as AFP-V0-028 requires in CI).
- `STR-V0-007`: `--shard I/N` with `--placement universe` (the default) SHALL use the AFP-V0-022
  complete-universe partition intersected with the run set, unchanged. An empty intersection runs
  nothing and says so. Every shard records the partition digest; a digest mismatch fails the shard.
- `STR-V0-008`: `--placement balanced` (experimental, off by default) SHALL partition the run
  set's execution units themselves by the advisory cost table, so a narrowed run uses every shard.
  Every execution unit SHALL be placed exactly once. Because membership then depends on the plan,
  every shard of one run SHALL reproduce the same STR-V0-018 manifest digest; a shard that cannot
  fails rather than falling back alone. `FULL` runs use the complete-universe partition in either
  mode. Balanced placement MAY execute only in the local profile. In CI it SHALL only be computed
  and retained beside the executing universe placement, because AFP-V0-028 requires the
  complete-universe intersection; executing it in CI needs an accepted AFP-V0-028 amendment.
- `STR-V0-009`: `corvint test verify-shards --manifest PATH RECEIPT...` SHALL read the expected run
  from the STR-V0-018 manifest, never from the receipts. It reports `PASS` only when there is
  exactly one receipt for each shard index `0..N-1` of the manifest's shard count, empty shards
  included; every receipt carries the manifest digest and source identity and records result
  `PASS`; the union of executed
  execution units equals the manifest's with none twice; every execution unit has exactly one
  passing terminal outcome; and every adapter invocation exited zero. A missing, extra, duplicate
  or foreign receipt, a receipt whose result is `INCOMPLETE` (drift, cancellation or an exceeded
  bound), or an execution unit without an outcome, is `INCOMPLETE`; any failed, timed
  out or build-failed outcome, or any nonzero exit, is `FAIL`. Both exit nonzero, `INCOMPLETE`
  taking precedence in the report. A CI required check SHALL depend on this fan-in, not on
  per-shard exits alone, and an unsharded run applies the same rules to its single receipt.
- `STR-V0-010`: Each shard, including an empty one, SHALL write one bounded `corvint-test-run/0`
  receipt (at most 1 MiB) with tool version, base, revision, source identity, plan and manifest
  digests, scope (`NARROWED`, `FULL` with reason, `NOTHING`), result (`PASS`, `FAIL`,
  `INCOMPLETE`), placement mode, partition digest, shard index and count, and per adapter the
  runner identity and version, exact argv, environment digest, exit, elapsed time and per
  execution unit terminal outcome. A narrowed run SHALL summarise as "selected units passed: N of M"
  and name where the full suite runs, never "suite passed".
- `STR-V0-011`: Receipts are evidence only. They MUST NOT feed ranking, selection, learning or
  authority; a later CEM citation needs its own accepted requirement.
- `STR-V0-012`: Local and CI SHALL run the same binary code path. The CI profile adds only
  admission: planner and runner built outside the PR checkout from the reviewed pinned source
  (AFP-V0-013/028), the merge-queue backstop and the forward-qualification trip and kill switch
  (AFP-V0-029/030). Every AFP-V0-028 FULL condition still applies in CI, and CI executes only
  universe placement (STR-V0-008). A local narrowed PASS
  SHALL NOT be reported as a gate PASS; `make gate` and the required CI checks stay binding.
- `STR-V0-013`: Neither `make gate-affected` nor `tools/corvint-pr-tests` SHALL switch to
  `corvint test` until a parity check over the 50 first-parent pairs of the AFP-V0-011 measurement
  and the retained AFP-V0-029 shadow rows shows the same Go package set, argv, environment and
  fallback verdict as the driver it replaces, with every difference explained in a build-log
  entry. Switching the CI call site is a `.github/` change under AFP-V0-016 consent and SHALL
  advance `CORVINT_PR_FORWARD_EPOCH`, because forward qualification binds the driver's identity.
- `STR-V0-014`: A hangup, interrupt or termination SHALL kill every owned process group, exit
  nonzero and write an `INCOMPLETE` receipt; it MUST NOT start the fallback.
- `STR-V0-015`: The run SHALL be bounded: the plan's existing walk and status bounds, at most 4096
  execution units, the receipt bound, the runner's native per-unit timeout unchanged from the
  profile's current call site (for Go, a per-package hang detector: the Makefile `GO_TEST_TIMEOUT`
  locally, and the `50m` that `tools/corvint-pr-tests` and the full CI branch pass in CI), and a
  separate
  total wall-clock budget per adapter invocation from the repository declaration (for Go, default
  70 minutes, the current `tools/corvint-pr-tests` budget). Exceeding a bound yields `INCOMPLETE`
  and nonzero, never a narrower run.
- `STR-V0-016`: Adapters execute repository code with the invoking user's authority. The local
  profile claims no containment or network denial. The CI profile keeps read-only workflow
  permissions, no persisted credentials or secrets, and no `pull_request_target` execution.
- `STR-V0-017`: The source identity SHALL bind the bytes that execute: the HEAD commit and tree,
  plus, for every dirty path, its SHA-256 or a deletion marker, captured before planning. The run
  SHALL recompute it before each adapter launch and after the last one; any difference is drift,
  and the shard is `INCOMPLETE`. In CI the AFP-V0-013 captured merge commit with a clean tree is
  the source identity.
- `STR-V0-018`: Before execution the command SHALL derive one canonical `corvint-test-manifest/0`
  from the plan, declared full scope, slice table, cost table, source identity, placement mode and
  shard count, holding the expanded execution units and their shard assignment, at most 1 MiB. Its
  digest is recorded in every receipt. `--dry-run` writes it; in CI one job SHALL compute it and
  pass it to every shard and to the fan-in, and each shard SHALL recompute and match it.

## Non-goals

- A watch daemon, IDE integration, or per-keystroke runs; that is LPCV's live loop.
- Learned or cost-based suppression of any selected unit; costs change placement only.
- Replacing vet, format, build, interop, documentation or artifact checks; they stay full.
- Retry and flake policy (AFP-V0-029 owns reruns), remote execution, or a hosted service.
- Test-level selection inside a package beyond the existing AFP-V0-041 slices.
- Relaxing any AFP-V0-028..030 admission condition or the shadow bar.

## Boundaries and failure modes

| Condition | Result |
|---|---|
| Plan cannot be produced, base unresolved, module-level frontier, unallowed unknown | `FULL` with named reason |
| Language with selected units but no adapter and no declared full command | `INCOMPLETE`, nonzero |
| Runner version cannot be read | that adapter `INCOMPLETE`, nonzero |
| Partition, manifest or source identity differs across shards | shard fails; fan-in `INCOMPLETE` |
| Worktree bytes change during the run | drift, shard `INCOMPLETE` |
| Missing shard receipt, duplicate execution unit, unit without terminal outcome | fan-in `INCOMPLETE` |
| Any failed outcome or nonzero adapter exit | `FAIL`, nonzero |
| No plan and no readable declared full scope | `INCOMPLETE`, nonzero |
| Bound exceeded | `INCOMPLETE`, nonzero |
| Signal | owned groups killed, `INCOMPLETE`, no fallback |
| CI admission fails (pins, queue rules, `.github/` change, trip, kill switch) | `FULL` |

Every failure fails closed toward a larger run or a nonzero exit. No path narrows on missing
evidence (invariant 2), and repository declarations outrank the planner (invariant 3).

## Acceptance evidence

Testing matrix (all `NOT_RUN` until implementation):

- fixture repositories per verdict (`NARROWED`, `FULL` for each fallback reason, `NOTHING`,
  `INCOMPLETE`), each asserting the exact receipt;
- `--dry-run` executes no process other than Git;
- universe placement equals the current `go-product-shard` membership for the same inputs;
- balanced placement places every execution unit once for 1 to 16 shards, a split package's
  catch-all slice included;
- `verify-shards` reports `INCOMPLETE` for a dropped, duplicated, extra or foreign-digest receipt
  and a missing empty-shard receipt, and for a shard that recorded `INCOMPLETE` after drift while
  its outcomes all passed with zero exits, and `FAIL` for a failed outcome or nonzero exit with every
  digest correct;
- editing an already-dirty file between planning and launch, or during the run, yields drift;
- planning failure runs the declared full scope, and with no declaration is `INCOMPLETE`;
- a language without an adapter runs its declared full command, or is `INCOMPLETE`;
- signal tests show killed groups and no fallback start;
- the STR-V0-013 parity report over the frozen pairs and shadow rows.

Promotion from experimental requires the parity report, 20 merged PRs through the queue on the new
CI call site with complete forward rows and no trip, and median and p90 PR wall time and
runner-minutes per merged PR measured before and after, retained in a build-log entry. Unavailable
hosted measurements stay `NOT_OBSERVED`. Balanced placement is promoted separately. Locally it
qualifies only if executed runs over the same frozen pairs, with the same shard count, lower the
p90 of the slowest shard's wall time by at least 20% against universe placement, with every
execution unit placed once; otherwise it stays unpromoted. In CI it qualifies only after an accepted AFP-V0-028 amendment defines an executed
comparison, and only if that lowers p90 narrowed-PR wall time over 20 PRs. Computed placement
estimates never count as observed timing.

Kill criteria: an unexplained parity difference that cannot be fixed; any queue `MISS` caused by the
runner rather than the selector; or balanced placement failing the local 20% bar or the CI p90 bar
above, which removes balanced placement only.

## Rollout and rollback

1. Go adapter, `--dry-run`, receipts and universe placement, used by hand alongside both drivers.
2. Parity report (STR-V0-013); then `make gate-affected` calls `corvint test`.
3. CI call site behind AFP-V0-016 consent with a new forward epoch; shadow first, as AFP-V0-029.
4. Balanced placement executes locally; in CI it is computed and retained while universe
   placement executes, until an accepted AFP-V0-028 amendment says otherwise.
5. Further adapters, each with its own fixture qualification, before its language may narrow.

Rollback: restore the previous call site (`script/gate-affected.sh`, `tools/corvint-pr-tests`),
which this spec leaves in place until step 3 is promoted; `CORVINT_PR_NARROWING=off` restores full
CI immediately (AFP-V0-030). Removing the command removes no other capability.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| STR-V0-001 | `cmd/corvint/test.go` (`compileAffectedGraph` shared with `affected`) | `cmd/corvint/session_test_command_unix_test.go` TestSTRV0018CLIDryRunLaunchesNoRunner, TestSTRV0010CLIRunWritesReceipt, TestSTRV0001CLIRefusesProtectedOutputPaths |
| STR-V0-002 | `internal/sessiontest/plan.go`, `internal/gateselect` | `internal/sessiontest/plan_test.go` TestSTRV0003BuildVerdicts, TestSTRV0007UniversePlacementEqualsCIPlan |
| STR-V0-003 | `internal/sessiontest/plan.go`, `internal/sessiontest/declared.go` | TestSTRV0003BuildVerdicts, TestSTRV0003ReadDeclared, TestSTRV0003CLINoPlanRunsDeclaredScope, TestSTRV0003CLINoPlanRunsDeclaredLanguageCommand |
| STR-V0-004 | `internal/sessiontest/run.go` (Go adapter; declared `make <lang>-test` fallback) | TestSTRV0004ParseTestEvents, TestSTRV0004RunnerDiscoveryRefusals; no versioned LPCV-V0-021 capability manifest yet (partial) |
| STR-V0-005 | `Invocations` in `run.go` | TestSTRV0006Invocations, TestSTRV0010CLIRunWritesReceipt |
| STR-V0-006 | `Invocations`; `ReadDeclared` types the Makefile `go-test` recipe | TestSTRV0006Invocations, TestSTRV0003ReadDeclared, TestSTRV0006ReaderGrammar, TestSTRV0006RepositoryMakefileTypesGoTest, TestSTRV0006RepositoryMakefileDependsOnConditional |
| STR-V0-007 | `placeGo` over `cishards.PlanFrom` | TestSTRV0007UniversePlacementEqualsCIPlan, TestSTRV0009CLIShardsAndVerify |
| STR-V0-008 | refused as experimental and not implemented | `NOT_RUN`; TestSTRV0008CLIUsageRefusals covers only the refusal |
| STR-V0-009 | `internal/sessiontest/verify.go`, `corvint test verify-shards` | TestSTRV0009VerifyShardsMatrix, TestSTRV0009CLIShardsAndVerify |
| STR-V0-010 | `Receipt`, `Finish`, `testWriteReceipt` | TestSTRV0011FinishResultAndSummary, TestSTRV0018ManifestStrictRoundTrip, TestSTRV0010CLIRunWritesReceipt |
| STR-V0-011 | no reader other than `verify-shards` | by construction; no test asserts absence of other readers |
| STR-V0-012 | `--profile ci` argv and environment on the same code path | TestSTRV0006Invocations; CI admission (pinned build, queue backstop, trip) is the call site's: `NOT_RUN` |
| STR-V0-013 | not started | `NOT_RUN` |
| STR-V0-014 | `internal/sessiontest/procgroup_unix.go`, `testExecute` | TestSTRV0014RunnerCancelAndBudgetKillGroup, TestSTRV0014CLISignalKillsRunAndExits |
| STR-V0-015 | `MaxUnits`, `MaxDocumentBytes`, `DefaultBudget` | TestSTRV0015UnitBound, TestSTRV0014RunnerCancelAndBudgetKillGroup |
| STR-V0-016 | local profile claims no containment (help text) | help text only; CI permissions are the call site's: `NOT_RUN` |
| STR-V0-017 | `internal/sessiontest/identity.go`, drift rechecks in `Execute` | TestSTRV0017Identify, TestSTRV0017RunnerDrift |
| STR-V0-018 | `internal/sessiontest/manifest.go`, `--dry-run --manifest`, shard recompute match | TestSTRV0018ManifestStrictRoundTrip, TestSTRV0018CLIDryRunLaunchesNoRunner, TestSTRV0009CLIShardsAndVerify |

## Open decisions

- Command name: `corvint test` (free today) or a subcommand of `affected`.
- Whether the Playwright profile's file commands become the first non-Go adapter or stay separate.
- Adapter order after Go, likely by the languages adopters run most.
- Whether balanced placement should ever become the default.
