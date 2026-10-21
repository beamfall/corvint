# 2026-10-21 flake-1021j: two V1-0849 load flakes repaired, V1-1499 measured

Lane `flake-1021j` on base `8def072e08e9730a6102657c887655f62993b31d`. Test-only changes; no
requirement or wire contract changed, so no requirement ID was added. Every number was taken on
the shared 12-CPU host with other lanes running, and the 1-minute load average is recorded.

## V1-0849 child V1-0638: `TestWorkActualProducerInterruptedScratchCleanup`

Problem. CI shard 6 of #1444 failed with "interruption witness disappeared before observer
cleanup". The test polled for the producer's `corvint-work-source-*` scratch, sent `SIGSTOP` to
the producer group and then re-checked the scratch. Signal delivery is asynchronous, so a producer
already inside its `RemoveAll` could finish removing the scratch and exit before the stop took
effect, and the witness vanished.

Decisions.
- The witness is decided after the reap: on observing scratch, the helper sends `SIGKILL` to the
  group, waits for the producer to exit, and only then globs the acquisition scratch that remains.
  An attempt whose producer removed its own scratch witnesses nothing and is retried, bounded by
  an attempt count (8), not a timing budget (decision 0082).
- `SIGSTOP` was removed. A stress run of an intermediate version that kept it hung: a stop landed
  on a git child between `fork` and its own `setpgid` (worksource runs git through
  `groupreap.Contain`), so the group `SIGCONT` never reached it.
- Each run first forces the interleaving CI lost (the producer is let finish before the kill) and
  asserts that such an attempt reports no witness. What the test proves is unchanged: a producer
  killed holding acquisition scratch leaves scratch that `run.Close` removes, with the run root.

Evidence.
- Before, natural: 50/50 pass, `-count=50 -race`, load 14 to 20 (does not reproduce on darwin).
- Before, forced (old code plus a hook letting the producer finish first): 3/3 fail with the CI
  message.
- After: 50/50 pass `-count=50 -race` in 156 s at load 21 to 33; 60/60 pass as 6 concurrent
  `-race` copies of `-count=10` at load 56 to 65; no natural retry was logged.

## V1-0849 child V1-0738: `TestGH494PreparationAdmission` one-budget case

Problem. The case gave preparation acquisition a 100 ms budget and two 60 ms delays, at
publication and at the final gate, to prove both consume one original budget. On a loaded host the
phases before publication alone exceeded 100 ms, so it timed out in `phase=registry` with
`published=false`.

Decision. Scale the budget to 1 s and each delay to 60 % of it, so each delay stays below the
budget and their sum above it, as before; the run-time bound stays at 10 budgets. The assertions
(LOCK_TIMEOUT, both delays reached, `phase=gate`) are unchanged.

Evidence.
- Forced: a one-time 150 ms delay before the first registry lock fails the old case 3/3 and passes
  the new one 3/3.
- Saturating, 12 concurrent `-race` copies of `-count=10`: before 44/120 fail (all the one-budget
  case) at load 28 to 52; after 120/120 pass at load 52 to 67.

## V1-0849 child V1-0455: `TestExperimentalKernelAdapterContext`

Not reproduced: 240/240 pass as 12 concurrent `-race` copies of `-count=20` at load 48 to 54. No
change.

## V1-1499: store program_* tests

Measured, no change delivered. The serial program_* subset (27 tests) took 219.6 s at load 48 to
31, of which 36 s user and 85 s system CPU: the tests wait on host, git and supervisor processes.
Slowest: CAL-V0-197 30.3 s, CAL-V0-089 checkpoint restart 29.8 s, turn caps 28.8 s, Codex
continuation 27.9 s (25.0 s real, 4.5 s user, 11.4 s system alone at load 17 to 29). They stay
serial for the reasons already recorded: the continuation fixture's 4 s stage wall failed under
parallel load (2026-10-10 entry), `t.Setenv` (CAL-V0-197, CAL-V0-215) forbids `t.Parallel`, and
process-global test seams. The supervisor's waits are event-driven selects, not idle sleeps. No
cut found that keeps every program_* assertion and its wall-clock margin. The acceptance runs
(three full-package runs at load 60 or more) exceed this lane's 600 s command bound: NOT_RUN.

## Verification and NOT_RUN

Run: `internal/tasks/authority` full package; `internal/tasks/mutation` full package;
`TestImportDirection` and `TestCALV0131_LiveFormatsCoverEveryDecodedProfile`; the `cmd/corvint`
`TestWork|TestExperimentalKernelAdapterContext` set; `go vet` and `gofmt -l` on both packages.
NOT_RUN: the full `cmd/corvint` and `internal/tasks/cli` packages (tier 0/1; the changes are
`_test.go` files, which importers do not compile) and the 65 tier-2 units.

Rollback: revert the lane's two test commits; no state, contract or production code changed.
