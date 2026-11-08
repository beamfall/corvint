# Preserve failed terminals across incomplete reports

Requirement: RSP-V0-003, runner-selection-plugins-v0.

The self-review found two ways that incomplete reports could erase an observed
failure before a passing FULL retry: Go discarded all subtest events, and shell
validation stopped at the first invalid parsed terminal.

The repaired class is failure retention independent of report completeness. Go
accepts only the existing decoded action allowlist (start, run, pause, cont,
output, pass, skip, fail). Only top-level pass/fail/skip-with-reason terminals
certify top-level completion. Descendant fail events prove failure of their
owning top-level identity at any depth; descendant success never proves parent
completion. Missing parent terminals remain incomplete, contradictory parent
success is refused, and a parent failure is not duplicated. Unknown actions
retain prior failures and refuse completeness through the existing decoder.

Shell accepts only passed, failed, and skipped-with-nonblank-reason terminals
from the bound source. Validation continues across every parsed test, retaining
valid failures while keeping the first validation refusal and its existing text.
Unknown native forms and terminal states remain refused; no new wire members,
error codes, exit codes, goldens, or relaxed assertions are needed.

Regression witnesses:

- `TestRSPV0003GoChildFailureSurvivesFullRetry`: failed child, nested/multiple
  failures, absent parent, malformed/unknown suffix, package terminal without
  parent, contradictory parent pass, and passing FULL retry; a completed failed
  parent remains a single result.
- `TestRSPV0003ShellReasonlessSkipRetainsLaterFailure`: reasonless skip preceding
  failure and passing FULL retry; unknown native form refuses completeness both
  with and without the invalid skip.

This repair restores the accepted contract; it does not qualify either plugin
or expand supported report syntax. Rollback is reverting this repair commit,
which restores the two known failure-retention gaps. Whole-package outcomes and
binding results are retained in the lane handback. Independent review and
repository-wide verification belong to the orchestrator in this repair round.

Verification observed in this sandbox: both packages' `TestRSPV0003` tests pass.
The required whole-package runs fail only in existing tests outside this repair:
`TestRSPV0007GoUnknownRepositoryBoundaryRefusesEvidence` finds the sandbox's
read-only `/home/russell/lanes/.git` after removing its fixture marker, and
`TestRSPV0040ShellTraceRefusals` reports `invalid execution accepted` in the
unchanged trace-execution path. Neither assertion was altered. The local outcome
is failed, with these retained failures left for the host verifier; targeted
regressions are not represented as whole-package success. Citation repinning
found no moved citation; `make line-citations-check` passes.
