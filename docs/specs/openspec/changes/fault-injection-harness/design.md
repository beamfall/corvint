# Design: fault-injection-harness

Status: proposed (2026-11-09). Requirement text lives in `specs/fault-injection-harness/spec.md`.

## Decisions

1. A repository tool (proposed: `tools/fault-harness`) generalises `tools/affected-edit-oracle` to
   seven runners, with a pre-registration file, a three-run baseline with quarantine, three-run kill
   confirmation, and a per-runner verdict. Go first (G1).
2. One runner adapter per runner on the single compiled-in runner contract (G23).
3. The harness measures plan recall, the AEO-V0-004 shape, not test strength. It defines its own
   operators per language (G26).
4. N counts narrowed PRs with a confirmed kill; the pre-registration over-samples to about 140
   sealed PRs; below 100 the verdict is `INVALID` (owner decision R19 (2026-11-09), review
   release-tasks 10).
5. Exactly 30 reverts per blocking runner, chosen by a recorded selection query; non-code faults
   are a separate N with their own 3/N bound (review release-tasks 11).
6. The pre-registration names an operator list per language. Proposed families, to be fixed in the
   pre-registration: relational and arithmetic operator swap, boolean negation, constant
   replacement, and removed statement or return value. The Go list may reuse the operators in
   `internal/liveverify/mutate` only under decision 3 of G26.
7. Quarantined tests that a plan omitted for a PR touching their dependencies are counted separately
   (review release-tasks 12).
8. The release wording is the G3 sentence, copied exactly (review minor 20), with the Wilson upper
   bound printed beside 3/N (settled release-tasks S6).
9. Held-out repositories fastapi, solidus and payload (owner decision R21 (2026-11-09)) get N = 100
   faults each, pass with at most 3 misses, a 5 percent excluded-test cap, and per-test durations
   in every receipt (owner decision R19 (2026-11-09)).
10. "Line-level proven" means zero-miss with plans at line granularity (review release-tasks 21).

### Removed packages and whether this change needs them (G26)

G26 removes pymutate and rbmutate and gives the orphaned Go `liveverify/mutate` its own ticket. This
change does not need any of them back. They answer "does this cited test kill a mutant", a measure
of test strength. The harness answers "does the plan contain every test that kills a fault in a
changed line", a measure of plan recall, which is the AEO-V0-004 shape. The Go operator list in
`liveverify/mutate` may be reused only if the G26 ticket re-owns that package under this spec;
otherwise it is removed with no loss here.

## Failure modes

- A pinned input cannot be reproduced: the case is excluded with `pin-unavailable` and counted
  against the pre-registered exclusion budget; over budget the runner's verdict is `INVALID`.
- More than 2 percent of tests fail in some baseline run: the runner is disqualified (G3).
- A mutant does not build, times out or kills nothing: it is not counted (G3, AEO-V0-004); it
  appears in the dropped count of FIH-V1-014.
- Fewer than 100 narrowed PRs with a confirmed kill: `INVALID` (FIH-V1-014).
- The selected run does not execute a confirmed killing test: `MISS`, which blocks and is never
  waived as a flake (G3).
- The pre-registration file changes after the first execution: the harness refuses.
- A held-out set records more than 3 misses in N = 100: the held-out verdict is FAIL.

## Acceptance evidence

- Seven sealed pre-registration files with hashes; one cost-probe report.
- Seven verdict files at the RC SHA with the G3 wording and the four counts, plus three held-out
  verdicts (fastapi, solidus, payload).
- Go tests for every failure mode above, using a small fixture repository per runner.
- Go adapter parity with `affected-edit-oracle` on its existing cases.

## Rollback

Revert the change's commits; the tool and verdict schema go and the IDs are retired. The release
then has no miss proof and cannot claim PROD-TEST-002; runners fall back to full runs with a printed
reason under NMS-V1-007.

## Open questions for the owner

- The revert selection query (which bug-fix commits qualify) is not chosen.
- Owner decision R19 lets a held-out set pass with up to 3 misses, while G3 and G12 say any miss on
  a blocking runner's set blocks. This change keeps "any miss blocks" for blocking-runner sets and
  applies the 3-miss rule to held-out sets only; the owner has not confirmed that split.
- The 5 percent excluded-test cap (R19) and the 2 percent quarantine cap (G3) apply to different
  sets; whether both apply to one baseline is not decided.
- R21 names one held-out repository per language family (Python, Ruby, JS); no held-out Go
  repository is named.
