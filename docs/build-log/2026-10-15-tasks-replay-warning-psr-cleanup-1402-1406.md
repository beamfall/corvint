# Tasks semantics batch: superseded-replay warning and unproved probe cleanup

Date: 2026-10-15. Base: `d47b0f01e5250aeab69bbf63bf44f9a285aeb3be` (origin/main).
Tickets: V1-1402 (`docs/specs/corvint-tasks-agent-leases-v0.md`, new CAL-V0-236) and V1-1406
(`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`, new PSR-V0-022). Both requirements are new lane
requirements accepted under the owner's standing auto-accept of 2026-10-11. Neither changes the text
of an accepted requirement.

## V1-1402: advisory warning on a superseded-generation replay

Contract question: does the replay contract allow anything new on a replay? TM-V0-006 and CAL-V0-048
require a same-request replay to return the original outcome with `replayed:true` and without a
write. CAL-V0-230 keeps a replay at the original generation while a fresh superseded renew is
`FENCED`. All three constrain the replayed outcome and its item. None constrains the envelope's
separate `warnings` member. Three accepted clauses say a replay does not repeat warnings: CAL-V0-107
and CAL-V0-211 for derivation-time warnings, and CAL-V0-214's "replays add none". Those cover warnings
the original result derived, not an observation made at replay time. Claim delivery already adds
replay-time warnings. The V1-1320 section lists "refusing or warning on a same-request replay" as a
non-goal. That is the scope of that slice, not requirement text. CAL-V0-236 says so explicitly. The
orchestrator should confirm that reading with the owner if they want the earlier slice's non-goal to
stand for later slices too.

Decision: CAL-V0-236 adds one `warnings` entry beginning `advisory, not authoritative` when a replay
served by the lease writer names an attempt whose generation, in the audit that served the replay,
differs from the replayed generation. The entry names both generations and says to use a fresh
request ID. The item, outcome, receipt and original generation stay byte-identical. The value comes
from the proof the writer already holds on both the complete and writer-checkpoint routes, so it
costs no extra read and the two routes agree (CAL-V0-116). It is never an input to fencing or
admission.

Evidence (go1.27.2, darwin, 12 cores, `-count=1`):

- `TestCALV0236_SupersededReplayCarriesAdvisoryWarning` (store, both routes). It covers claim, renew
  and heartbeat replays before (no warning) and after a release and re-claim (exactly one warning
  naming the new generation). The report minus the warning is `reflect.DeepEqual` to the earlier
  replay.
- `TestCALV0236_SupersededReplayWarningLeavesResultBytes` (cli). It covers renew and heartbeat
  replay stdout. With the warning removed, stdout is byte-identical to the replay before the release.
  It also checks the verbose help note.
- BASE: the store test does not compile (no `ReplayWarnings` field). The CLI test FAILs with no
  warning on the superseded renew replay.
- Mutation: dropping the assignment in `leaseOnce`
  (`_ = supersededReplayWarnings(proof, report)`) FAILs the CLI test and both store subtests.
- The CAL-V0-048 replay tests and the CAL-V0-230/231 tests are unchanged and pass.

## V1-1406: unproved Git-probe cleanup classified SOURCE_CHANGED

Finding at the base: `sweepGit` returns a typed bounded error only for a clean TIMEOUT or INTERRUPTED
probe. When the runner cannot prove the probe's own group cleanup, the runner's class is already
`UNKNOWN`. Examples are a pipe holder outliving the wait delay, or a group still live at the end of
the 5-second allowance. `sweepGit` then dropped that class. `sweepSourceClass` and `poolPostClass`
turned it into `SOURCE_CHANGED`, and the pinned-cwd proof turned it into `MISSING_EVIDENCE` "pinned
path is not a readable Git worktree". Both claim facts the probe never observed.

Governing text:

- PSR-V0-012 keeps `SOURCE_CHANGED` for "shown drift".
- PSR-V0-007 requires an "explicit unknown/signal class".
- PSR-V0-004 says bounded probes record TIMEOUT or INTERRUPTED, "not SOURCE_CHANGED", and makes
  "unknown cleanup" terminal, not retried.

The runner's own rule (`pool_process_unix.go`, a not-clean result is `UNKNOWN` even after a timeout)
already ranks unproved cleanup above TIMEOUT. The base gate for `sweepGitBounded` (`result.Clean &&
...`) shows that PSR-V0-004's sentence was written for the clean case.

Decision: do not keep SOURCE_CHANGED, and do not add a new outcome or refusal code. PSR-V0-022
records the existing `UNKNOWN` class (already in both observation codecs) and routes it like the
PSR-V0-004 bounded classes:

- before launch it runs nothing and the member stays quarantined;
- after exit it replaces only a class that could pass;
- at admission or preparation the refusal keeps the probe's own `UNSUPPORTED` error.

The retry planner retries only EXIT_NONZERO and STDOUT_MISMATCH, so quarantine, ownership release and
retry debt are unchanged; only the recorded class and the pinned refusal text change. No encoding,
field or code is added.

Overlap: open PR #918 (`claude/tasks-flakes-1392-1324-1294`) edits the `executePoolCaptured` line two
lines above this change in `sweepGit`, to lift the bound in tests. The hunks do not touch, and both
changes compose: a lifted bound changes when the probe ends, not how an unproved cleanup is
classified.

Evidence:

- `TestPSRV0022_UnprovedProbeCleanupRecordsUnknown` (store). A fake `git` leaves a pipe holder past
  the wait delay. The test checks that `sweepSource` gives `UNKNOWN`, that `poolPostClass` replaces
  only a passing class, and that the pinned-cwd proof refuses `UNSUPPORTED` with class `UNKNOWN`.
- BASE: the test FAILs with `SOURCE_CHANGED`.
- Mutation: removing the `!result.Clean` branch FAILs the test (`SOURCE_CHANGED`).
- `TestPSRSweepSourceBoundClass` is unchanged and passes.

## Verification and residuals

Focused packages: FOCUSED_RESULTS.

Not run: `make gate` and full `go test ./...` (lane charter). The four status replicas of the leases
spec (header line 6, digest line 42, INDEX.json, README) do not yet list CAL-V0-236. They were left
alone to avoid conflicts with open PR #920, which edits all four. This is a follow-up residual. The
PSR spec's replicas list PSR-V0-022.

Codex review: CODEX_RESULTS.
