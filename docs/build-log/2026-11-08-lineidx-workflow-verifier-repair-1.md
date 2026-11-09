<!-- SPDX-License-Identifier: Apache-2.0 -->
# Lineidx verifier repair 1

The host verifier at `6f9158e407a9e8ec6b6c92fa640d3cdbd7758693` failed
`TestLinecovMissConfirmationReproducesRecordedRunner`: the sizing job and shard
job used the same wrapper-build step name. The test requires exactly one named
shard step before comparing its commands with the miss-confirmation workflow.

Give the shadow sizing build its own descriptive step name. The shard build,
execution commands, and every existing assertion remain unchanged. This repairs
the V1-4366 runner-reproduction verification (`GLS-V0-046`, `GLS-V0-009`);
it changes no selection rule or frozen contract. The defect is a workflow step
identity collision, not an analysis soundness gap requiring a new form allowlist.
Rollback is the one-line workflow name revert, which restores the collision.

`corvint affected --base 16ba0021ab0aa92d244ad1b8d54b607d12bc4ed3`
selected `tools/ci-reuse-plan` by `DIRECT_TEST_CHANGE`. It retained scope UNKNOWN
and a stale-installed-binary notice; this is not current-source parity evidence.
With Go 1.27.3, the complete existing package passed through `script/heavy.sh`
using `go test -count=1 -timeout 30m ./tools/ci-reuse-plan/` in 12.497 seconds.
Whole-package admission used the explicit owner mandate and affected-plan reason.
No identity environment was exported for this test.

Raw test evidence stays outside Git at
`/home/russell/lanes/lineidx-evidence/verifier-repair-1/ci-reuse-plan-test-rerun.log`.
SHA-256: `8311c67fc37afd9ce2f6c4218ba287ef99a5d97065bbf5f86d6b8b151069853c`.
Independent review and broader verification remain with the host orchestrator.
