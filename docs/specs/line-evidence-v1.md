# Line Evidence V1

Owner: Russell Lewis
Date: 2026-11-18
Intent status: accepted
Delivery status: experimental
Health: KEEP

## Agent digest
- Claim: Private tree-bound line evidence augments file selection with explicit widening reasons.
- Status: accepted / experimental
- Exists: Shared additive reader, refresh collectors, consumer integration and argument limits.
- Blocked on: Frozen default output migration, Ruby proof, FIH records and Action integration.
- Read next: Requirements; Acceptance; Traceability; Rollback.

## Intent and baseline

The owner accepted the line-evidence proposal, design and delta in PR #235 on 2026-11-18.
The affected user is a developer selecting tests for a local edit. The baseline is the existing
file-level plan. Evidence may add or rank tests and never subtract from that baseline.
The explicit Go `--line-index FILE` contract remains unchanged.

## Requirements

- `LEV-V1-001`: (accepted 2026-11-18, PR #235) One writing verb for line evidence.
  `corvint line-evidence refresh` MUST be the only command that collects and writes line evidence.
  `corvint affected`, `corvint test`, `corvint watch` and MCP MUST only read it and MUST NOT run a
  collector.

- `LEV-V1-002`: (accepted 2026-11-18, PR #235) Private storage per runner instance.
  Line evidence MUST be stored per runner instance (TIV-V1-003) under `<git-dir>/corvint/line-evidence/`
  in the `linecov-index/4` encoding (GLS-V0-001) with the instance's language, runner family, command
  and config recorded. Tests MUST be named by canonical test ID. The store MUST NOT be committed.

- `LEV-V1-003`: (accepted 2026-11-18, PR #235) Evidence bound to the tree.
  A reader MUST compare the evidence commit, runner identity, config and per-file blob OIDs with the
  planned tree. A file whose blob is not in the evidence MUST widen to tier `file`; a commit that is
  not an ancestor of the planned base, or a different runner identity or config, MUST widen the
  instance to tier `full`. Both MUST print reason `line-evidence-stale`.

- `LEV-V1-004`: (accepted 2026-11-18, PR #235) Missing or damaged evidence widens.
  Missing evidence for an instance MUST widen reached files to tier `file` with reason
  `line-evidence-absent`. A damaged section MUST use the `index-section-invalid` fallback (GLS-V0-002)
  and widen. Stale or missing evidence MUST NOT narrow.

- `LEV-V1-005`: (accepted 2026-11-18, PR #235) Line-exact runners.
  Go, Vitest, Jest, RSpec and Minitest MUST produce per-test line evidence through
  `corvint line-evidence refresh`: Go through the existing Go collector, Vitest and Jest through the
  JavaScript collector, RSpec and Minitest through the Ruby hooks.

- `LEV-V1-006`: (accepted 2026-11-18, PR #235) pytest selects by file with its tier label.
  pytest MUST select whole test files and every pytest selection MUST carry tier `file`. The release
  acceptance record MUST list pytest file-level selection as a release exception, and the Python line
  collector MUST be tracked as a 1.0.x ticket.

- `LEV-V1-007`: (accepted 2026-11-18, PR #235) Line evidence never removes a browser E2E test.
  A browser E2E test MUST be removed from a selection only by accepted flow links plus static
  reachability at file and symbol level. Line evidence MAY add a browser E2E test to a selection or
  order selected tests, and MUST NOT remove one. An ordering MUST be shown as `rank` and MUST NOT be
  presented as narrowing.

- `LEV-V1-008`: (accepted 2026-11-18, PR #235) Per-test E2E line narrowing is opt-in and experimental.
  Per-test line narrowing of browser E2E tests MUST run only under an explicitly named opt-in profile
  labelled experimental. Promotion of that profile MUST require a fault-injected zero-miss proof and a
  measured net wall-clock win.

- `LEV-V1-009`: (accepted 2026-11-18, PR #235) One tier vocabulary.
  Every selected unit in a plan MUST carry `tier` with one of `line`, `file`, `flow`, `full`, and a
  separate boolean `exact`, true only when the runner ran exactly the selected tests. Tier `flow` MUST
  NOT count as narrowed in the no-missed-test claim.

- `LEV-V1-010`: (accepted 2026-11-18, PR #235) One-line reason per selection.
  Every selection MUST print one line naming the runner instance, the tier and the reason.

- `LEV-V1-011`: (accepted 2026-11-18, PR #235) Tier line only after fault-injection proof.
  A runner's evidence MUST produce tier `line` in plans only after that runner is line-level proven by
  the fault-injection harness (FIH-V1-015); the harness computes its own line plans for that proof.
  Before that, fresh evidence MUST be used at tier `file` with reason
  `line-evidence-unqualified`. Go keeps its qualification check (GLS-V0-034).

- `LEV-V1-012`: (accepted 2026-11-18, PR #235) One reader for every consumer.
  `corvint affected`, `corvint test`, `corvint watch` and the MCP affected tool MUST use one reader for
  line evidence. `corvint watch` MUST accept evidence for every supported language and MUST NOT refuse
  a non-Go index.

- `LEV-V1-013`: (accepted 2026-11-18, PR #235) The Action builds and caches evidence.
  The GitHub Action MUST run `corvint line-evidence refresh` on the default branch and cache the store
  keyed by commit and runner instance. A PR job MUST restore evidence only from an ancestor commit; a
  cache miss MUST widen with `line-evidence-absent`.

- `LEV-V1-014`: (accepted 2026-11-18, PR #235) Argument limits.
  When the selected names for one invocation exceed the runner's argument limit, Corvint MUST pass them
  in an argument file where the runner accepts one, and otherwise MUST widen the invocation to file
  level with reason `argv-limit`.

- `LEV-V1-015`: (accepted 2026-11-18, PR #235) One-shot commands plan the working tree.
  One-shot `corvint affected`, `corvint test` and the MCP plan tool MUST use the `corvint watch` rule
  of decision 3: the planned change is the working tree and index against HEAD, with HEAD checked to
  descend from the evidence commit. A changed file with no matching blob MUST widen to `file` with
  reason `line-evidence-stale`; HEAD not descending from the evidence commit MUST give FULL.

## Boundaries and failure modes

Reads never collect or write evidence. Collection is explicit, private and bounded. Missing,
stale, corrupt and unqualified evidence retain the baseline with a named reason. An instance
identity mismatch requires its full suite. No runner can use tier `line` without its FIH proof.
Python has no collector in 1.0; file-level pytest remains a release exception and a G4 risk.
Playwright evidence cannot remove a test. Action integration and FIH admission are separate tasks.

The refresh CLI reports `line-evidence-invalid-arguments` for invalid invocation or unresolved
repository/Git inputs, and `line-evidence-unavailable` when collection fails. Both use the existing
CLI diagnostic envelope and exit 2. These codes belong to the LEV-V1-001 writing command; they do
not replace the reader's widening reasons.

## Partial integration boundary

Default annotation of an entirely absent private store is NOT_PRODUCED. The base's frozen-output
assertions reject the new fields and reason lines. Under the lane's freeze rule those outputs and
assertions remain unchanged. The overlay activates after refresh initializes the private store,
or for an explicit watch index; absent instance files within that store print the named fallback.
The canonical requirements above remain accepted intent, not a claim that this gap is delivered.
See [the freeze finding](../build-log/2026-11-18-line-evidence-freeze-boundary.md).

## Acceptance

The change tasks require repeatable collection, freshness and corruption fixtures, runner fixtures,
read-only consumer tests, working-tree cases and an argument-limit fixture. Each completed task
must cite its executed proof. Unobserved or blocked proof remains explicit in the trace.

Collector proof is recorded in
[the lane evaluation](../build-log/2026-11-18-line-evidence-evaluation.md).
Task 5 remains partial because the host has no Ruby or RSpec runtime; its native fixtures are
retained and skipped explicitly. This is not a line-level qualification result.

## Traceability

| Requirements | Implementation and evidence |
| --- | --- |
| LEV-V1-001 | `TestLEVV1001RefreshAndConsumers` (`cmd/corvint`); consumers leave index bytes unchanged |
| LEV-V1-002 | `TestLEVV1002PrivateDeterministicInstances` (`internal/lineevidence`); canonical IDs and repeat refresh in `TestLEVV1001RefreshAndConsumers` |
| LEV-V1-003 | `TestLEVV1003Freshness` (`internal/lineevidence`); `TestLEVV1003StaleInstanceExecution` (`cmd/corvint`) |
| LEV-V1-004 | Default absent-store output PARTIAL (freeze boundary); `TestLEVV1004AbsentDamagedReadOnly` (`internal/lineevidence`) |
| LEV-V1-005 | `TestLEVV1005NativeCollectors` (`internal/lineevidence`): Vitest and Jest observed; RSpec and Minitest NOT_RUN (Ruby unavailable). Go: `TestLEVV1001RefreshAndConsumers`. Synthetic per-runner fallback: `TestLEVV1005EveryRunnerUnqualified` |
| LEV-V1-006 | `TestLEVV1006PytestFileTier` (`cmd/corvint`) |
| LEV-V1-007 | `TestLEVV1007PlaywrightOnlyAddsAndRanks` (`cmd/corvint`) |
| LEV-V1-008 | `TestLEVV1008ExperimentalProfile` (`cmd/corvint`); promotion NOT_PRODUCED (no FIH or wall-clock proof) |
| LEV-V1-009 | Default absent-store output PARTIAL (freeze boundary); `TestLEVV1009TierReasonGolden` (`internal/lineevidence`); `TestLEVV1007PlaywrightOnlyAddsAndRanks` |
| LEV-V1-010 | Default absent-store output PARTIAL (freeze boundary); `TestLEVV1009TierReasonGolden`; `TestLEVV1001RefreshAndConsumers` |
| LEV-V1-011 | `TestLEVV1005EveryRunnerUnqualified`; tier `line` admission NOT_PRODUCED (task 11 outside lane) |
| LEV-V1-012 | Initialized stores and explicit watch indexes only; default absent-store annotations PARTIAL; `TestLEVV1001RefreshAndConsumers`, `TestLEVV1012VitestWatch` (`cmd/corvint`) |
| LEV-V1-013 | NOT_PRODUCED (task 10 outside lane; separate Action change) |
| LEV-V1-014 | `TestLEVV1014GoNamesWidenToPackages`, `TestLEVV1014FileOverflowWidens` (`internal/testrunner/contract`); `TestLEVV1014GoSliceArgumentLimit`, `TestLEVV1014PytestArgumentFile` (`internal/sessiontest`) |
| LEV-V1-015 | `TestLEVV1001RefreshAndConsumers` (staged and untracked paths on affected, test and MCP); `TestLEVV1003Freshness` (blob and ancestry failures) |

## Rollout and rollback

No line-level promotion is claimed. Task 10 (Action refresh/cache) and task 11 (FIH admission)
remain outside this lane. Keep the existing standalone collector tooling until all users migrate.
Revert this change and delete the private store to return to file-level selection. Requirement
IDs remain stable if retired. Independent review is supplied by the host orchestrator.
