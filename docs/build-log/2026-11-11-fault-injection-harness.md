# Fault-injection harness, tasks 2 to 5 and 7 (V1-5405)

Date: 2026-11-11 (America/Toronto)
Intent: change `docs/specs/openspec/changes/fault-injection-harness`, accepted automatically under
G14 (decision 0596) as it implements PROD-TEST-002; FIH-V1-001 to FIH-V1-015 in the new spec
`docs/specs/fault-injection-harness-v1.md`.
Outcome: tasks 1 to 5 and 7 ticked; harness tests pass. Tasks 6, 8, 9 and 10 are not built: they
need the pinned proof repositories, the RC SHA, the candidate tag and the Tasks gate. No release
evidence (sealed sets, cost reports, verdicts) is produced here, and nothing in this entry is a
miss proof for any runner.

## Design decisions

- One new repository tool, `tools/fault-harness`, as the proposal named. The process, Git and
  receipt plumbing (`command`, `git`, `plan`) is copied from `tools/affected-edit-oracle` rather
  than moved into a shared package, so the oracle stays byte-for-byte unchanged and the parity test
  compares the harness against the oracle as it is. The harness imports no mutation package
  (FIH-V1-013): `internal/liveverify/mutate` has not been re-owned by a G26 ticket, so the harness
  defines its own operator families, the five the design proposes.
- The runner contract is one interface, run a named test set in a tree and return per-test
  outcomes, with the dependents query the omitted-quarantined count needs. Only the Go adapter
  exists; any other runner is refused with `runner-adapter-missing` (the rest of FIH-V1-008 is
  task 6).
- A case is a PR: the faults are put into the tree at `headSha` and the plan is computed against
  `baseSha`. For a revert, both are the pin and the fault reverse-applies the fix's source-only
  diff; it counts as restoring the pre-fix lines only if the fix then applies forward again
  (FIH-V1-003). The baseline runs once per distinct head, which is the first cost measure of
  FIH-V1-010 when every case shares the pin.
- The plan and dependent units are computed for every fault, killed or not, so the
  omitted-quarantined count covers every sealed PR that touched a quarantined test's dependency.
  For Go the dependents come from `go list -test -deps`, independent of the planner.
- A selected run is executed only for a NARROWED plan. The planner receipt has no FULL scope (its
  scopes are `BOUNDED` and `UNKNOWN`), so the harness classifies a plan as UNKNOWN when it is not
  bounded or names unknowns, FULL when it runs every unit of the baseline suite whole, and
  NARROWED otherwise. A FULL or UNKNOWN plan is the third count of FIH-V1-014.
- Verdict precedence is DISQUALIFIED, then MISS, then INVALID, then ZERO_MISS, so a miss is never
  hidden behind an invalid measurement. The G3 sentence is printed only for ZERO_MISS, and the
  Wilson upper bound is printed beside it in parentheses so the sentence stays exact. The
  non-code section has its own N and bound; it is INVALID only at N = 0, since FIH-V1-002 sets
  its minimum (30) on the sealed set, not on N. That floor is a lane choice, open to the owner.
- The pre-registration's `overSample` must be at least 100 and the sealed set must hold at least
  that many code PRs; "about 140" is the value a real pre-registration states, not a constant
  the tool enforces.
- The A17 rule is a committed JSON file with its words in `statement` and the decided numbers
  (12, 24, 30, 2, 3) as fields; a rule file with other numbers is refused rather than obeyed, so
  changing the rule needs a new decision and a new hash. The facts it reads about each PR (fix
  signal, files, revert applies, builds, fails 3/3) are an input file, observed by task 8.

## Review fix round 1 (P2-1, P2-2, P3)

An independent review found two fail-open defects; both are fixed under G3, which exempts only a
fault-induced build failure or crash, so every other failed measurement makes the verdict
`INVALID`, never `ZERO_MISS`.

- P2-1, verdict not bound to the seal. `verdict` read only the pre-registration and results, so a
  results file with its MISS and ERROR cases deleted and at least 100 KILLED cases left read
  `ZERO_MISS`. It now requires `--set` and `--seal` (the seal record carries counts, not the case
  list) and refuses a results fault-set hash other than the seal's (`fault-set-changed`), seal
  counts that differ from the set (`fault-set-invalid`), a set with fewer than 30 non-code cases
  (`non-code-faults-below-minimum`, FIH-V1-002 at verdict time), and any case list that is not the
  sealed one (`results-not-sealed`): each sealed case exactly once with its kind, PR and head, and
  no other. The same check covers one level down, which the review did not name: a counted or
  dropped case must hold its sealed faults in their sealed order, each `KILLED` or `NO_KILL`, with
  the case status agreeing, so a fault with a miss cannot be dropped from a case or its case
  relabelled `NO_KILL`. `ERROR` and `EXCLUDED` cases may carry partial faults; they are outside N
  and an `ERROR` already makes the verdict `INVALID`. `run` shares the same seal loader.
- P2-2, a stopped faulted run counted as dropped. Any `Adapter.Run` error during the faulted runs
  became `NO_KILL`. Now cancellation (`ctx.Err()`) or the harness's own wall bound
  (`errWallTimeout`, which `command` returns when its own deadline, not the caller's, stops the
  process group) makes the fault `ERROR` and the case a failed measurement; the Go adapter returns
  such an error before parsing the partial output. The runner's own `go test -timeout` panic stays a
  fault-induced crash and is dropped: the baseline passed under the same bound, and the design's
  failure modes drop a mutant that times out. A clone checkout stopped the same way is now a failed
  measurement instead of `pin-unavailable`, which would have spent the exclusion budget.
- P3, misses per case. The Wilson bound now uses `missedCases`, the cases in N with at least one
  miss, so it is per case like N; the misses are still listed per fault. New wire field
  `missedCases` in each verdict section.

Tests: `TestVerdictBoundToSeal_FIH_V1_002_007_014` (removed MISS case, removed ERROR case, added
unsealed case, duplicated case, fault-set hash mismatch, kind, PR and head changes, removed missed
fault, a missed case relabelled `NO_KILL`, edited seal counts, 29 non-code cases) and
`TestFaultedRunStopped_FIH_V1_005_014` (a cancelled context and a real wall timeout from `command`
end in `ERROR` and an `INVALID` verdict; a build failure is still dropped). With the P2-2 check
disabled by hand, `TestFaultedRunStopped_FIH_V1_005_014` failed with the cancelled case `NO_KILL`.

## Differences from the proposal's tree (9ec3a29d0)

The proposal cites `tools/affected-edit-oracle/main.go` `compare` :124, `testObservations` :475
and `runMutation` :538; those lines are unchanged here. `internal/liveverify/mutate`, `pymutate`,
`rbmutate` and `internal/testrunner/golang` still exist; retiring them stays out of scope. Two
gaps the proposal did not name: the planner receipt carries no FULL scope (handled above) and no
R16 `line`/`file` tier. The harness reads `plan.tier` when present, so until the planner emits it
every real verdict's granularity is `not proven` (FIH-V1-015), never an unearned label.

## Evidence

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 ./tools/fault-harness/`: ok, 16 tests.
- `TestGoParityWithOracle_FIH_V1_011` builds the unchanged oracle and runs one AEO-V0 mutation
  case through both tools on a synthetic two-package repository, for a planner that selects both
  killed units, one, and none: the safety statuses and missing units match. With the harness's
  miss check disabled by hand, this test failed, so it does depend on the harness's own misses.
  The parity run over the checked-in AEO corpus (Corvint and Beamfall repositories) was NOT_RUN.
- `TestRunSealedSet_FIH_V1_004_005_006` runs `generate-go` and `run` end to end on a synthetic
  repository: three mutants on the PR's changed line, an always-failing test quarantined (and
  the runner disqualified at 1 of 3), three-run kills, a narrowed plan's misses, the omitted
  quarantined test named, no clone left behind, and an edited seed refused before any clone.
- `openspec validate fault-injection-harness`: NOT_RUN, the `openspec` command is not installed
  on this host.
- Independent review: NOT_RUN in this lane; the orchestrator runs it.

## Not built

Task 6 (six adapters, PRR-V1 manifest pins and `pin-mismatch`, both cost-check steps), task 8
(observing PR facts on the seven pinned repositories, sealing, running, verdicts at the RC SHA),
task 9 (held-out sets after the PRR-V1-004 draw) and task 10 (ticket completion) remain open.
The spec marks FIH-V1-008 and FIH-V1-002 partly delivered and FIH-V1-009, 010 and 012 not yet
built.
