# Affected Edit Oracle V0

Owner: Russell Lewis
Date: 2026-11-08
Requirement prefix: `AEO-V0`
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner request V1-5181 (2026-11-08), `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/affected-plan-v0.md`, `docs/specs/affected-zero-unknown-v0.md`.

## Agent digest
- Claim: A pinned controlled-edit corpus measures affected-plan recall and over-selection, with opt-in observed mutation killers.
- Status: accepted / experimental
- Exists: `tools/affected-edit-oracle`, its versioned corpus and synthetic Git tests.
- Blocked on: corpus completeness beyond these pins, independent review, and universal recall qualification.
- Read next: Requirements; Expectations; Acceptance; Rollback.

## Intent and current state

The owner wants every change to run only tests it can affect, and never miss one. Controlled,
repeatable interventions are needed to distinguish selection quality from incidental PR contents.
The existing `tools/affected-zero-unknown-replay` and `tools/selection-baseline` replay historical
changes; `tools/beamfall-shadow` measures external analyzer execution; `tools/gate-affected-select`
applies CI policy. None authors and commits a planned edit with an independent expected set.
A new private developer tool is the smallest adequate home. It shares the receipt wire and local
clone convention, not selection code. It is not a released Core command or a gate that repairs
or changes planner policy.

## Requirements

- `AEO-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) The `affected-edit-oracle/0` corpus SHALL name unique cases with repository, full observed base SHA, edit class, deterministic edit, required and permitted unit IDs, and an independent `why`. Edits SHALL support no-op, append-line, unique replace-literal, new file and deletion. Empty diffs SHALL require zero selections with no allowances.
- `AEO-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) Each case SHALL clone locally with `--quiet --no-hardlinks --shared`, detach at its pin, edit and commit only in that clone, run the selected installed binary with `affected --base HEAD~1 --format json`, and remove the owned clone. Source worktrees and their dirty state SHALL remain untouched. Commands SHALL have time/output bounds, local-only heavy execution and process-group cancellation. Invalid paths, symlinks and preexisting destinations SHALL be refused. Committed paths SHALL equal the requested edit path (none for an empty case); an ignored addition SHALL fail instead of becoming a vacuous pass.
- `AEO-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) Missing required unit IDs SHALL be FAIL; selections outside required/permitted IDs or explicit `witness:KIND` allowances SHALL be OVER. Reports SHALL retain each excess unit's witness kind, dirty path and via length, and the planner summary/unknown count. Unknowns SHALL never be described as proof of safety.
- `AEO-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) Opt-in mutations SHALL replace the same pinned literal site, first prove the command passes on the base, then observe assertion failures on the committed mutant. Every failing test unit SHALL occur in that mutant's selected plan; a miss SHALL be SAFETY_FAIL. Build/setup/timeouts and surviving mutants SHALL be failed measurements, not kills. Corvint Go mutation commands SHALL use `script/heavy.sh env GOTOOLCHAIN=local go test`, one package per command.
- `AEO-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) Output SHALL be sorted and deterministic, carry corpus/binary SHA-256 identities, per-case selected/required-hit/excess counts and totals by witness kind. Exit SHALL be nonzero for FAIL, measurement errors, unqualified mutation results and OVER; `--allow-over` SHALL waive only OVER.
- `AEO-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) The checked-in corpus SHALL cover Corvint, beamfall-web, beamfall-cloud, beamfall-plugin-sdk and beamfall at observed pins with at least twelve applicable classes each. It SHALL include at least four small Corvint mutations and one per Beamfall repository. Expectations SHALL precede observation and derive from source/test relationships, never current selected sets.
- `AEO-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) Fast CI SHALL exercise materialization, comparison, mutation failure attribution and cleanup in a tiny synthetic local Git repository with no network or Beamfall dependency, and independently test comparison/report exit policy. A deliberately broken comparator SHALL be detected before restoration.
- `AEO-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5181) The dated build log SHALL retain every full-corpus precision result and mutation outcome, including failures and execution limits. A failed quality evaluation SHALL not be relabelled a harness test failure or a passed planner qualification.

## Expectations and limits

`mustSelect` is a lower bound of demonstrated dependencies. `maySelect` enumerates justified
additional consumers or names an explicit witness-kind allowance. The initial corpus uses exact
IDs, without blanket witness allowances. Go importer allowances come from pinned source imports,
not `affected`. Exported API cases require those importer test packages; body cases require the
direct tests and permit those consumers. A source dependency is not proof that every assertion
can fail. Conversely, missing undeclared readers can make an OVER a corpus error. Review each
`why` before changing an expectation; never bless observed extras merely to obtain green output.

Comment/documentation probes use semantic expectations. Source-reading contract tests may
legitimately observe those bytes; excess selections remain visible for investigation. Drivers are
execution units too: one changed recipe or workflow step requires its target/job. Unit IDs follow
the public provider naming conventions; no implementation import calculates the expected set.
Mutation recall is package/file-unit inclusion, not selected `-run` filter completeness, branch
coverage, all platform variants, external consumers or exhaustive mutation adequacy.

## Non-goals and simpler baseline

No selector changes, PR experiment traffic, network writes, dependency installation, CI execution,
repo policy rewrites, Tasks writes or universal safety claim. Historical replay remains useful but
cannot replace independent controlled expectations. Shell and driver commands are measured, not
executed. Go/Node mutation commands are explicit trusted corpus code, never planner-supplied argv.

## Trust boundary, resources and failure modes

The operator supplies trusted corpus commands and a trusted binary. This is process-group
containment, not a security sandbox: a malicious command can access the host or escape its group.
Git environment pointers/configuration and hooks are neutralized. Edits refuse Git metadata,
nonlocal paths, ambiguous literals and symlinks. Clone destinations must be fresh and caller-owned;
only the individual case path is removed. No source checkout cleanup runs.

Limits: 256 cases, 4 MiB corpus, 256 MiB binary, 32 MiB command output, two minutes per command,
five seconds for process retirement. The shipped Go commands add a 90-second package timeout.
A missing pin/binary/toolchain, dirty fixture setup, unparseable receipt, missing summary, unsupported
process ownership or failed cleanup is ERROR, not a zero-selection pass. Missing test dependencies
remain errors; no package manager installation is attempted. A command that exits without an
assertion failure is not a killer. Interrupted execution retires the current group and unwinds clone
cleanup. SIGKILL/power loss cannot run cleanup; the named private destination can be removed manually.

## Acceptance and traceability

| Requirements | Implementation | Evidence |
| --- | --- | --- |
| AEO-V0-001, AEO-V0-006 | `validate`, corpus | `TestCorpus_AEO_V0_001_006`, `TestEditValidation_AEO_V0_001_002` |
| AEO-V0-002 | `executeCase`, `apply`, `command` | `TestSyntheticClone_AEO_V0_001_002_003_004`, `TestCommandCancellation_AEO_V0_002` |
| AEO-V0-003, AEO-V0-005 | `compare`, `summarize`, `rejected` | `TestCompare_AEO_V0_003_005` |
| AEO-V0-004 | `runMutation`, `testObservations` | `TestObservations_AEO_V0_004`, `TestSyntheticClone_AEO_V0_001_002_003_004` |
| AEO-V0-007 | synthetic Git fixture and fail-before experiment | `tools/affected-edit-oracle/main_test.go`, dated build log |
| AEO-V0-008 | deterministic report | `docs/build-log/2026-11-08-affected-edit-oracle.md` |

Run the focused package test/vet and repository scope/command/spec ratchets. Run precision on all
cases and mutation on all named mutation cases using the installed binary. Retain the result table,
not merely its exit status. Fast CI uses a protocol stub; the full run is the installed-binary check.

## Rollout, rollback and maintenance

This is opt-in developer tooling. Remove the tool, corpus and spec registration to roll back; no
product state or planner behavior needs reverting. Pins and expectations are versioned together.
When source changes, inspect the new pinned dependencies and replacement sites before admitting a
new corpus version. Retain old result identities. Do not silently update pins or expectations at run
time. The profile must change for incompatible corpus/report semantics.

Promotion requires independent review and reproducible measurements. Universal recall remains an
open goal. Kill or revise an expectation when original source evidence disproves it; retain the old
observation and the reason. External acceptance and cross-platform mutation coverage are unobserved.
