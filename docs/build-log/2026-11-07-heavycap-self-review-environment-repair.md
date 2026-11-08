# Heavycap self-review repair: keep admitted test filters through execution setup

Date: 2026-11-07. Repair starts at `81e35b096a4922eadd9255af0d86af7203a09ef2`;
CEM and affected base remains `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Owner intent: the supplied self-review repair findings, V1-5007 / ASD-V0-2478.

## Findings and decision

The supplied P1 findings identify two ways setup could add exclusions after automatic
source-change admission. Locally, `env GOENV=off go test ./p` passed the guard, but
parallelism setup read the wrapper's GOENV and exported its `-skip=TestExisting`.
Offloading admitted the sender's clean flags, then sourced receiver-only exclusions.
Either path could hide an existing test broken by the source edit.

Parallelism now resolves Go defaults against the child's leading environment assignments.
Automatic and manifest admission are checked again against the effective local flags
after toolchain and parallelism setup. A refused run uses the existing refusal diagnostic,
marker and exit status. Manual reasons keep their existing policy.

For offloads, bind the sender's resolved GOFLAGS as a leading command-local assignment
inside the existing CMD array. The receiver consumes this assignment after its environment
setup and before parallelism setup. No envelope field or JSON shape changes. The receiver
also replaces leading argv GOFLAGS assignments with the bound value. Empty GOFLAGS needs
a nonempty neutral `-p` bound: Go otherwise still consults the receiver's GOENV. The marker
is consumed before execution, preserving the original command and its diagnostics.

The local regression extends `TestASDV02478SourceEditRunsExistingTests` with unset wrapper
GOFLAGS, wrapper-only GOENV exclusions and child GOENV=off. The real Go child must fail the
existing broken test. `TestASDV02478OffloadKeepsValidatedFlags` uses a local SSH fixture
that drops sender environment variables, runs the actual receiver setup and reconstructs
the edited source. Receiver-only -skip, -run and GOENV defaults, including explicit empty
argv GOFLAGS, must each retain the existing failure on the receiver. A local fallback is
an assertion failure. Existing assertions, goldens and frozen output fixtures are unchanged.

Rollback: revert this repair's environment resolution, final validation, command-local flag
binding, regression additions and ASD-V0-2478 clarification. The separate absent pre-commit
entry point still leaves V1-5007 PARTIAL; this repair does not claim external integration.
V1-5040 / ASD-V0-2477, V1-4722 / DTL-V0-017 and V1-4944 / MCJ-V0-025 receive no repair edits.

## Verification and limits

The read-only before/after probe uses `go env`, never direct Go test execution. Before:
guard status 0, child flags `-skip=TestExisting -p=1`, final guard status 1. After: guard
status 0, child flags `-p=1`, final guard status 0. The full-package fixture verifies the
actual failing Go child, rather than treating this probe as execution evidence.

The first full tools/test-commands-check run failed the new offload fixture: immediate
same-size source edits left the sender's copied-index stat cache at the old blob, while
the receiver correctly applied the patch. Its assertion rejected local fallback. Staging
the fixture's edited source before the snapshot pins that blob without weakening any
assertion or changing production tree-identity behavior. The failed evidence is retained.

Raw logs remain under `/home/russell/lanes/heavycap-evidence/`; scratch uses
TMPDIR=/home/russell/lanes/heavycap.tmp, mode 0700. Focused tests run only through heavy.sh,
one package per command, without author/committer identity in their environment.

The existing pre-change capture is reused with its original identity and limitations;
no new dogfood-change capture is run. Actual self-use is the affected plan and final CEM
rebind/check. Broader selection, doc gates, Windows qualification and model-backed review
are NOT_RUN by charter and belong to the host verifier/orchestrator. No Tasks writes,
push, PR or seal. External outcomes, performance savings and unavailable telemetry remain
NOT_OBSERVED. Final report limitations are retained in the external handback.

The initial affected plan selected tools/test-commands-check (DIRECT_TEST_CHANGE), with
scope UNKNOWN, 365 selected / 526 excluded / 69 LANGUAGE_FRONTIER unknowns. The installed
binary reports self-binary-stale against 30 changed Go source paths; its plan is retained
without claiming current-selector qualification. Go version equality against go1.27.3,
`bash -n script/heavy.sh` and diff whitespace checks pass.

`script/gen-spec-requirements.sh --write` reported no generated changes. The required
`script/check-line-citations.sh --repin` found no moved citation, so no second repin was
needed. `make -s spec-requirements-check line-citations-check` passed: whole internal/specindex
1.182 s after a 337.4 s slot wait, focused internal/lrfrepo 0.098 s. Both existing target
commands ran through heavy.sh, with one package per command and no commit identity.

Raw evidence SHA-256 (files in the external evidence directory):

- self-repair-local-probe.log: `ccd2f57f3b83066eaa4e1104921ff519b3e4cd1ff228e132e04ae63ec46703b2`
- self-repair-affected.json: `11e6c3467d24f66a9e74c1a176ace85f80567e76300a44968a294ee35cebff4e`
- self-repair-affected.stderr: `0ee4ade61bddb0b8c027b78fa8737c82aace500a40af0cd6dfa11e8abed5f83e`
- self-repair-package-test.log: `2ea1c920324ad0dac417c9a337ba7cfeca9aabbe7cafca17f353560150157402`
- self-repair-spec-generate.log: `574000ff552503675798e941126791a2bd9028c1ae10acfa0799c446b1be1e19`
- self-repair-repin.log: `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133`
- self-repair-spec-lines.log: `6565a7ea2f03e6a64a4c6d3472bc6f0ad3d9c7549693860d36538b771b4aa8bb`

The whole tools/test-commands-check retry passed, including every existing test and both
new environment regressions. Command (no -run filter):

```sh
TMPDIR=/home/russell/lanes/heavycap.tmp HEAVY_WHOLE_REASON='affected selected touched package; self-review repair requires existing whole-package tests' script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/test-commands-check
```

- ok  	github.com/Beamfall/corvint/tools/test-commands-check	53.250s
- heavy.sh: done queue=567.6s (load=0s slot=567.6s) run=53.9s rc=0
- self-repair-package-retry.log SHA-256: `4415c0cbede56a8f1d5da81e913de71d60b29a0e65428807b3ea8714f53c415f`

Shared-host slot saturation delayed the retry and the required spec check; the configured
lock directory was never overridden. No lock-dir refusal occurred. The queue-entry identity
predates the final documentation entry; this is final-code execution evidence, not a claim
that tests ran at a later CEM commit. Final binding/check results belong to the handback.
