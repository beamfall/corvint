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

Decision: CAL-V0-236 adds one `warnings` entry beginning `advisory, not authoritative` when a
`claim`, `claim --next`, `renew` or `attempt heartbeat` replay of a `COMPLETED` original names an
attempt whose generation, in the audit that served the replay, differs from the replayed generation.
A completed original held the live generation, so a difference is a supersession; a refused
original (a `FENCED` renew naming a generation the attempt never held) and other verbs (`release`,
`gate run`, which also calls the lease writer) add nothing. The entry names both generations and says to use a fresh
request ID. The item, outcome, receipt and original generation stay byte-identical. The value comes
from the proof the writer already holds on both the complete and writer-checkpoint routes, so it
costs no extra read and the two routes agree (CAL-V0-116). It is never an input to fencing or
admission.

Evidence (go1.27.2, darwin, 12 cores, `-count=1`):

- `TestCALV0236_SupersededReplayCarriesAdvisoryWarning` (store, both routes). It covers claim, renew
  and heartbeat replays before (no warning) and after a release and re-claim (exactly one warning,
  compared as the exact expected text on both routes). The report minus the warning is
  `reflect.DeepEqual` to the earlier replay. Controls: a fenced future-generation renew replays
  without a warning before and after the re-claim, and a release replay after the re-claim carries
  none.
- `TestCALV0236_SupersededReplayWarningLeavesResultBytes` (cli). It covers renew and heartbeat
  replay stdout. With the warning removed, stdout is byte-identical to the replay before the release.
  It also checks the verbose help note.
- BASE: the store test does not compile (no `ReplayWarnings` field). The CLI test FAILs with no
  warning on the superseded renew replay.
- Mutations, each FAILing the store test on both routes: dropping the assignment in `leaseOnce`
  (also FAILs the CLI test); dropping the `COMPLETED` condition (the fenced replay warns); dropping
  the verb scope (the release replay warns).
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
records the existing `UNKNOWN` class (already in both observation codecs). A probe that also hit its
bound keeps the PSR-V0-004 `TIMEOUT` or `INTERRUPTED`, so PSR-V0-004's sentence is not amended: the
runner now keeps that run-context class in an unencoded `Bound` field that its cleanup override
used to discard. Unproved cleanup is terminal under PSR-V0-004 ("unknown cleanup ... not retried"):

- before launch the probe runs nothing and the member stays quarantined;
- after exit its class replaces the command's exit class, including a retryable `EXIT_NONZERO` or
  `STDOUT_MISMATCH`, so the retry planner (which retries only those two) quarantines the member;
- an expired attempt deadline still records PSR-V0-004 `TIMEOUT` or `INTERRUPTED`, also terminal;
- at admission or preparation the refusal keeps the probe's own `UNSUPPORTED` error.

At the base the same probe became `SOURCE_CHANGED`, which was also terminal, so quarantine and
retry outcomes match the base; only the recorded class and the pinned refusal text change. No
encoding, wire field or code is added.

Overlap: PR #918 (`claude/tasks-flakes-1392-1324-1294`) edits the `sweepGit` bound line next to
this change to lift the bound in tests. It merged during this lane and was merged in with
`git merge origin/main` (no conflict there; `REQUIREMENTS.tsv` was regenerated). The changes compose:
a lifted bound changes when the probe ends, not how an unproved cleanup is classified.

Evidence:

- `TestPSRV0022_UnprovedProbeCleanupRecordsUnknown` (store). A fake `git` leaves a pipe holder past
  the wait delay. The test checks that `sweepSource` gives `UNKNOWN`; that `poolPostClass` replaces
  a passing class and the retryable `EXIT_NONZERO` and `STDOUT_MISMATCH`; the classifier table
  (unclean with a TIMEOUT or INTERRUPTED bound keeps the bound class and is terminal after exit; a
  clean bound keeps the command's failure; a plain failure stays `SOURCE_CHANGED`); that the runner
  records `Bound` on a timeout; and that the pinned-cwd proof refuses `UNSUPPORTED` with `UNKNOWN`.
- BASE: the test FAILs with `SOURCE_CHANGED`.
- Mutations, each FAILing the test: removing the `!result.Clean` branch (`SOURCE_CHANGED`); making
  unproved cleanup replace only a passing class (`EXIT_NONZERO` kept); not recording `Bound` in the
  runner; ignoring `Bound` in the classifier (`UNKNOWN` instead of `TIMEOUT`).
- `TestPSRSweepSourceBoundClass` is unchanged and passes.

## Verification and residuals

Focused packages: FOCUSED_RESULTS.

Not run: `make gate` and full `go test ./...` (lane charter). PR #920 merged during this lane, so the four
status replicas of the leases spec (header, digest, INDEX.json, README) now list CAL-V0-236; the PSR
spec's replicas list PSR-V0-022.

Codex review (one round, `codex exec -s read-only`, diff `d47b0f01e..bc065a2da`), six findings, all
addressed in this change:

1. HIGH: an unproved probe after a failed command kept the retryable `EXIT_NONZERO`, so the planner
   could launch another attempt, which SOURCE_CHANGED had prevented. Fixed: unproved cleanup always
   replaces the exit class (mutation-tested).
2. MED: an expired attempt deadline overwrote the new class with TIMEOUT/INTERRUPTED. Kept as
   PSR-V0-004 behaviour (terminal either way) and stated in PSR-V0-022 instead of preserving
   `UNKNOWN`.
3. MED: a replayed `FENCED` renew naming a future generation warned without any supersession.
   Fixed: only a `COMPLETED` original warns (mutation-tested control).
4. MED: `gate run` replays pass through the lease writer, contrary to the stated non-goal, and
   lane-untouched release replays return before the warning. Fixed: the warning is limited to
   claim, claim --next, renew and heartbeat, and the non-goals name release, lane-untouched
   release and `gate run` (mutation-tested release control).
5. MED: `UNKNOWN` for a timed-out unclean probe qualified PSR-V0-004's TIMEOUT/INTERRUPTED sentence.
   Fixed without amending it: a probe that hit its bound keeps that class (runner `Bound` field).
6. LOW: route coverage used substring checks. Fixed: the exact warning text is compared on both
   routes. Not done: the test does not observe which route served the replay; it uses the same
   `LowerWriterCheckpointForTest` switch as the accepted CAL-V0-230/231 route tests.
