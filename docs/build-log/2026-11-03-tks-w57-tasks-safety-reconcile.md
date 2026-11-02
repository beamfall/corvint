# Tasks barrier safety and post-merge reconcile — tks-w57

Base: `5d1ed52b17d61147bd8c04b08afa6da39fa80afb` (`origin/main` at dispatch).
Branch: `claude/lane-tks-w57-1103`. Human-owned inputs: lane tickets V1-4261 and V1-3496.
Requirements: PUR-V0-2210, PUR-V0-2211, TCD-V0-2210, TCD-V0-2211, written under standing
acceptance; this entry does not claim owner read-back or validated delivery.

## Decisions and evidence

V1-4261 reuses the base's writer-lock fence (PUR-V0-1920/1921), instead of adding a barrier
expiry that could override operator authority. A compiled qualification child in a linked fixture
lane inherits the forbidden common directory and refuses pause/unpause without touching its
primary's intent or state. Policy optionally declares `barrierWarningSeconds` (1..604800; default
3600). The warning is advisory, strictly older than the limit, and binds its request ID to the
PAUSE receipt's barrier post digest. Missing evidence says `NOT_OBSERVED`.

The initial hook implementation imported Tasks implementation into Core; review against decision
0397 rejected that approach before delivery. The final hook consumes only the companion's
read-only wire through `internal/taskbarrier`: fixed `queue status --fields barrier` argv,
500 ms deadline, 50 ms pipe-settlement bound and 64 KiB output cap. Core still imports no Tasks
implementation. A named buffer avoids promoting `bytes.Buffer.ReadFrom`, which would bypass a
Write-only cap under `io.Copy`; the regression exercises that route. Missing, older or invalid
companions remain unknown. This applies to the Claude/Codex adapters registered by the existing
integration hook manifests; other hosts and actual installed-host qualification are NOT_OBSERVED.

The [operator note](../TASKS-BARRIER-2026-10-30.md) retains the requested October 30/31 build-log
and V1-4261 history searches. The record does not identify how the specific barrier was removed;
its actual recovery transaction, actor and time remain NOT_OBSERVED. No live journal was changed
or inspected through a Tasks command.

V1-3496 reuses `make tasks-reconcile` and the orchestrator's existing first-MERGED watcher call.
Each unpaired direct delivery commit now has an explicit `commitProposals` row, while PR proposals
retain the existing batch-close path and ambiguous/umbrella citations remain review items. An
explicit application first proves its proposal's evidence is on `origin/main`; the fixture
regression proves refusal while the merge object is merely local, then completion after the
tracking ref contains it. Successful application retains a fresh `after.json` debt scan. The
watcher's explicit skip leaves a SKIPPED report and alert, and failure to retain either refuses.
The default remains proposal-only; existing-debt audit and disposition belong to the operator.

## Verification and limits

The final command/result matrix, exact test selectors, acceptance mapping and remaining integration
steps are retained in the worktree-root `tks-w57.handback.md`. Focused verification covers the
native fixture store, compiled child guard, actual reconcile script with Git/gh fixture evidence,
wire consumer, native hook rendering, policy validation and the Core/Tasks import-direction gate.
The landing-watcher test uses the existing fixture group's reconcile-related section without
running unrelated lane harnesses. This is fixture qualification, not a live-store or host claim.

Before adding requirements, the static generator capacity check passed. Exact definition counts
were PUR 7 and TCD 76; after this slice they are 9 and 78, both below 256. The generator continues
to warn about other specs already at 240..256; they were not expanded.

Observed repair failures were retained: an initial compile used the nonexistent `IntentDir`
field; the native old-pause test initially hit the expected backwards-time refusal; a new commit
citation required updating the proposal oracle; transport fixtures initially violated the wire's
canonical sorted-key/single-LF framing. Each was corrected. No failed test is a passing witness.

Sandbox restrictions refuse writes to the real linked-worktree Git index and dogfood private Git
directory. Index-based checks therefore use `/tmp/tks-w57.index`, `/tmp/tks-w57-objects` and the
real objects as alternates. Generated files remain in this worktree. The canonical index remains
unstaged; the integrator must stage normally. Initial Make regeneration also exposed an unwrapped
Go test recipe and an unwritable default cache; subsequent runs explicitly override its test
command through `script/heavy.sh`. Heavy defaults pointed outside the writable roots, and shared
`/tmp` filled during verification; task-owned cache/temp files moved to ignored `.lane-tmp/`.

No task-store command, commit, push, codex execution, background job or polling loop was launched
as a lane action. Tests exercise their own disposable stores and executable fixtures. Native
completion, post-commit CEM binding/check/seal, publication and independent reviewer acceptance
remain the orchestrator's integration work; no ticket is claimed natively completed here. The
repository-wide gate and Darwin qualification are NOT_RUN for this scoped Linux lane.

## Rollback

Revert the code and scripts; remove `barrierWarningSeconds` from policy before using the old reader.
Barriers and receipts keep their stored formats. Reconcile reports remain advisory and existing
native completion receipts are never rolled back by deleting reports.
