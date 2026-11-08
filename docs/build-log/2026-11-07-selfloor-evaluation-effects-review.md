<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor self-review repair: include and discarded evaluation effects

V1-5000, AFP-V0-3121/3122/3128. Owner repair charter of 2026-11-07;
base `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`, incoming head
`83ba40f943dab146f66330aae8e16da8475514d0`.

## Findings and decision

The four review findings can omit a target and its callers when an otherwise
unnamed repository input changes. Include imports preserve targets but do not
evaluate included assignments in order; command-local environment assignments
can execute startup hooks; path normalization mis-evaluates `notdir`; and an
over-limit `foreach` can hide body effects inside a discarded condition.

The repaired certification classes and closed boundaries are:

- Include effects: no include forms are certified until ordered included
  evaluation is modeled. Every parsed include retains the including file's
  targets as frontier sites, even when the included file declares no targets.
  This deliberately chooses conservative selection over a new make evaluator.
- Command-local environment effects: only GOCACHE (cache location) and empty
  GOFLAGS are admitted. Unknown names, startup hooks, nonempty/unresolved flags
  and other assignment forms fail closed through direct assignments, env and
  shell wrapping. Effects become sticky immediately, including standalone forms.
  Existing GOCACHE exclusion assertions remain unchanged.
- Path function values: `notdir` uses literal last-slash slicing on each
  definite word, without filesystem normalization. A trailing slash yields an
  empty word; joining two such results leaves a space, which make treats as a
  true condition. Unknown function forms still retain effects under the existing
  closed function allowlist; there is no new function admission.
- Bounded loop effects: only resolved variable/list forms with at most 256
  fully visited items can certify body effects. Over-limit and unresolved forms
  retain effect uncertainty independently of branch values; unknown functions
  inside a bounded body also fail closed. Other expansion-bound exits already
  retained uncertainty and were not changed.

## Acceptance and rollback

New regressions are `TestFrontierSiteIncludeEffects_AFP_V0_3121`,
`TestFrontierSiteCommandEnvironmentEffects_AFP_V0_3128`,
`TestFrontierSiteNotdirCondition_AFP_V0_3121`, and
`TestFrontierSiteForeachLimitEffects_AFP_V0_3121`. Each class includes the
reported form and an unknown-form case. They exercise target-free includes,
direct/env startup hooks reading HIDDEN_INPUT, trailing/repeated/root slash
paths, and 257-item or unresolved loops. Hidden-input/CEM changes select check,
its make caller and its CI caller. Existing assertions and goldens are intact.

The pre-fix focused run failed all four reported forms (0.129 s). A local GNU
make probe established that two empty notdir results retain a space; the new
multi-word test was corrected to put the reader in that actual true branch.
After correction, all frontier tests passed (1.266 s). This corrects a newly
written test oracle, not an existing contract assertion.

Rollback reverts this repair's code, regressions and spec clarifications
together, restoring the known omissions. No wire shape, field, error/exit code,
stderr or core-freeze output changes. No aggregate qualification or precision
claim. The orchestrator owns independent review, host integration and native
ticket completion. No Tasks writes, publication, pre-change recapture or seal.

Raw logs and final binding reports remain outside the repository at
`/home/russell/lanes/selfloor-evidence/review8/`; the external handback records
commands, outcomes, unproduced evidence and final CEM checking.

## Final package evidence

Go 1.27.3; all Go tests ran through script/heavy.sh using its configured semaphore,
with identity variables unset. The affected selection retained 406 units and 70
unknowns (scope UNKNOWN); affected was selected by DIRECT_SOURCE_CHANGE and the
inherited gateselect test change by DIRECT_TEST_CHANGE. The installed selector
is stale, so its receipt is not a current-tree parity claim. Whole-package runs
without -run, including all existing tests:

- ok   github.com/Beamfall/corvint/internal/liveverify/affected 102.402s (PASS).
- ok   github.com/Beamfall/corvint/internal/gateselect 1.778s (PASS).

Focused spec-requirements-check and line-citations-check passed. Staged citation
repinning found no moved citations. The orchestrator owns the prohibited broad
gates and model-backed review; these are NOT_RUN here. Darwin coverage remains
NOT_OBSERVED on this Linux host. No test source changed after whole-package runs.

| External log | SHA-256 |
|---|---|
| `regressions-before.log` | `633ed7d22f608fbfd61230d0bcd5374a3a5711a5b06a098ae8a7aa3f7c66ec7f` |
| `frontier-after.log` | `e1dfc23077c32c73bf5bc09dcdf95550c2263584d2c3ecd207d8e67499a50186` |
| `frontier-final.log` | `8a46548886524ad2bd8f0a32d15142c871ee3f2290af4da78601e1fb9a57f8f6` |
| `affected-final.log` | `ee4f11c05461e1873807643d0a0a2b31f02572ca6c28b1685dda0236f494be40` |
| `gateselect-final.log` | `db9f45ef48b33cf38d34c14e37e2c040d5752224e2c137b7d0db0fefbc391e0d` |
| `spec-citations.log` | `56254813b78db1620a353d6f1282aa35c1b25127eac8158c82744a0fc539e6ce` |
| `repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
