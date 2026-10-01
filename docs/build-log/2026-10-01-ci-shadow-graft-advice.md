# CI shadow historical misses: graft advice and Git identity

Ticket `ci-shadow-historical-compat` asked why the shadow qualification campaign
(hosted run 36897095402; pair base `d3c8d0f1fceecbc66cf29c375cc16b122e8aa2fe`,
target `29e185103e6927e6c63e2fbf1922c1661300754f`) recorded 11 failed packages,
including the selection misses `internal/frontiernextrepo` and
`internal/tracerecordrepo`. V1-0616 activation depends on that answer.

## Diagnosis

The misses were induced by the harness environment, not by the change. They were
not omissions by the selector. `closedEnv` sets `GIT_GRAFT_FILE=/dev/null` so the
driver refuses grafts (AFP-V0-013). The same environment reached `go test`. Under
current Git, naming a graft file makes commit-walking commands (`git log`, but not
`rev-parse`) print `hint: Support for <GIT_DIR>/info/grafts is deprecated` on
stderr. Tests that read combined Git output then parse that advice as the command
result.

The target was replayed locally (Darwin, Git 2.54.0, no race; hosted CI runs Git 2.55.0)
in the driver's closed environment:

- With `GIT_GRAFT_FILE=/dev/null`, `tracerecordrepo`, `frontiernextrepo`,
  `cem/verify`, `plansnapshot` and `taskman` failed.
- Without that entry and otherwise unchanged, those packages passed. So did
  `dashboard-snapshot-v0`, `frontier-v0`, `contextindex`, `extevidence` and
  `jstestprovider`.

The hosted `jstestprovider` failure is a separate cause: `exec: "node": executable
file not found in $PATH`. The closed PATH is the Go directory plus `/usr/bin:/bin`.
It passed locally only because the local Go directory (`/opt/homebrew/bin`) also
contains `node`. That PATH compatibility gap is still open and is not repaired here.

The frozen runtime identity recorded Go and the compiler but not Git. The behaviour
change therefore could not be detected as runtime drift.

## Decision

- The driver's own Git operations (`capture`, `git`, clone) use `closedEnv`. The
  `go test` process uses `testEnv`, which is `closedEnv` with `GIT_CONFIG_COUNT=0`
  replaced by one setting, `advice.graftFileDeprecated=false`. Both keep
  `GIT_GRAFT_FILE=/dev/null`. PR mode tests the Actions checkout, and a full
  fallback can run after "grafts are unsupported", so a repository graft file can be
  present; tests therefore inherit the graft refusal while Git stays silent about
  it. AFP-V0-013's graft refusal is unchanged and the accepted requirement text is
  not amended.
- The frozen identity gains `Git`: the resolved executable path, its SHA-256 and
  `git --version`. Git drift before execution fails safely, like Go/compiler drift.
  Run-result validation requires the field, so container evidence without it is
  incoherent.
- Existing frozen rows and their identity digests predate this field. V1-0616 needs
  a newly frozen identity and a fresh campaign; old rows are not relabelled.

## Evidence

- `TestTestEnvironmentSilencesGraftAdvice` (AFP-V0-013 graft advice) checks that
  `testEnv` is exactly `closedEnv` plus the advice setting, keeping the graft entry,
  and that a real `git log` under `testEnv` returns exactly the subject. It logs the
  driver environment's output; on Git 2.54.0 that is the deprecation hint. The hint
  is recorded rather than asserted, because its wording depends on the Git version.
- `TestExecuteGivesTestsSilencedGraftEnvironment` runs `execute` over a one-package
  fixture whose test fails unless `GIT_GRAFT_FILE=/dev/null` is set and combined
  `git log` output is exactly the subject. It pins the environment `execute` hands
  to `go test`, not only `testEnv`.
- `selectedFailureAndFallback` asserts that the Git identity is populated and that
  Git drift refuses execution.
- `TestExecuteRequiresGitIdentity` refuses execution when Git cannot be identified
  and no frozen identity exists to compare against, so the drift check does not
  cover it. `TestContainerRunResult` rejects run evidence whose identity lacks Git.

## Review repair (rev-462-r1)

The first revision dropped `GIT_GRAFT_FILE` for tests and claimed the owned clone
carries no graft file. That overstated the case: PR mode runs on the Actions
checkout (`ci.yml`), and a full fallback after "grafts are unsupported" would have
run tests with grafts active. The repair keeps the graft entry for tests and
silences only the advice. The new tests above kill each reviewed mutation: passing
`closedEnv` or the graft-less environment to `go test`, and deleting either Git
identity guard.

The repaired environment was replayed locally (Darwin, Git 2.54.0, no race) on
`tracerecordrepo`, `frontiernextrepo`, `cem/verify`, `plansnapshot`, `taskman`,
`contextindex`, `behaviorfalsify`, `worksource`, `dashboard/repository`, `console`,
`conformance/cli-parity-v0` and `conformance/work-queue-v0`; all passed. Several set
their own `GIT_CONFIG_COUNT`. The control with the graft entry and no advice setting
failed `tracerecordrepo` and `frontiernextrepo` again.

## Limits

- Hosted re-qualification is `NOT_RUN`. The local replay is not race-instrumented
  and used Git 2.54.0, not hosted 2.55.0.
- The `node` PATH gap, the 200-row campaign and reviewed pins remain open under V1-0616.

Rollback: revert this change. The driver then passes `closedEnv` to tests again and
the identity has no `Git` field.
