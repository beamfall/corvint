# Dogfood fixture helper closure, stale CORVINT_BIN and verify-line bound

Lane friction-dogfood-w5, tickets `V1-3242`, `V1-3154` and `V1-3107`, plus four already-fixed P1
tickets; immutable base `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`. Requirements `ASD-V0-086` to
`ASD-V0-088`.

## V1-3242: fixtures broke whenever a wrapper gained a helper

Four dogfood shell tests built their fixture repository by copying a hand-kept list of
`script/*.sh` files. A wrapper that began to source or run a new helper failed in every fixture
until each list was edited. Decision: one sourced helper, `script/dogfood-fixture-scripts.sh`,
copies the named wrappers and, transitively, every non-test `script/*.sh` they reference outside
comments. A reference to a missing script is refused with its name and a `fix:` line. Deliberate
stub fixtures, which replace a wrapper with a fake, keep their own copy step. The helper test runs
inside the already registered `lane-scripts_test.sh`, so no new gate registration was needed.

## V1-3154: a CORVINT_BIN built before the change ran its own tooling

`CORVINT_BIN` passes the version probe whenever `VERSION` matches, so a binary built before a
change to `internal/dogfoodflow/` ran that change's dogfood steps with the old code. Decision:
`script/dogfood-finish.sh` refuses `corvint-bin-predates-tree-tooling` before any build when
`CORVINT_BIN` names anything but the lane-built tree binary and the change touches
`internal/dogfoodflow/`, `internal/cem/` or `cmd/corvint/dogfood_*.go`. The fix line is to unset
it and rerun the same target. A change outside those paths keeps the override. `docs/DOGFOOD.md`
§4 now says to draft such a plan with the tree binary. The path list is a fixed allowlist, not a
dependency closure: a dogfood behavior change made only in another package is not caught.

## V1-3107: the verify-line bound

The ticket asked that `dogfood-inputs-check` refuse a verify line over the 512-character recorder
bound with its line number before closeout spends a try. On this base it already does (`DCW-V0-092`
validation in the pre-flight with the `DCW-V0-379` line locator). The observed lane ran closeout
without running inputs-check first. The missing piece was the fixture test, now
`TestASDV0088InputsCheckRefusesOverlongVerifyLine`.

## V1-3248: aggregate-schema-invalid named no field

Every `aggregate-schema-invalid` return now carries an `AggregateSchemaError` naming the field (task,
verification, candidates, admitted, receipt or a report member), and its `Error()` text stays the bare
code, so `AggregateWorkerFailureCode` and every wire classifier see no change (`ASD-V0-089`). The
recorder's stderr, `dogfood-check` and the `local-outcome` fix line repeat the field and its recovery;
an older recorder's bare code still gets a recovery line. Not changed: `PrepareAggregateReport`'s own
returns, which only the local-completion finish reads.

## V1-3249: batch-check kept no report

`script/dogfood-batch-check.sh` writes `.git/corvint/batch-check-report.txt` (header with base, head,
verdict and status, then both streams) and prints its path (`ASD-V0-090`). The streams print after the
check exits rather than live: a tee per stream would need process substitution that bash 3.2 cannot
wait on.

## V1-3250: two wrapper fixtures ran in no gate

`dogfood-cache-test` and `dogfood-bin-reuse-test` join `GATE_STEPS` and `shell-test`, which CI's shell
shard runs, so no `.github` change was needed (`ASD-V0-091`). Evidence: a wrapper edited to pass a wrong
flag made `make dogfood-cache-test` exit 2; restored, it exits 0.

## V1-3254: the requirements-section hint required every intent at BASE

The fix line now checks whether the failed `ocm-prepare-N` intent exists at BASE and names the
bootstrap-intent route of `DCW-V0-449` when it does not (`ASD-V0-092`).
