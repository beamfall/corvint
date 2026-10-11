# V1-0622 Gradle per-run user home and daemon ownership

Date: 2026-10-11. Base: `10efc02d16e6069d73426e6d6abd860e71ae83ab`. New requirements
`TRE-V0-042` and `TRE-V0-043`, proposed (V1-0622). No decision file.

## Finding

Java takes `user.home` from the account database, not `HOME`. The fresh executor HOME therefore
did not scope `gradle-junit`. Every run read the account `~/.gradle` (properties, init scripts,
caches) and registered its single-use daemon in the shared registry. A live Gradle 9.6.1 run of a
hanging test showed that the daemon and its test worker leave the client's process group, so on
timeout or interruption the group kill alone cannot retire them.

## Change

- The profile now passes `--gradle-user-home <ReportDir>/.home/.gradle` next to `--offline
  --no-daemon` and sets `retireDetachedDescendants`.
- No `--stop`, killall or registry-wide cleanup is issued.
- Historical plans keep their identity. The exact historical invocation (no flag, no
  `--gradle-user-home` pair) is refused for new execution with `plan predates per-run Gradle user
  home; re-plan it`; other drift keeps the generic mismatch refusal. An independent review of the
  first commit found that the historical form fell through to the generic refusal; this is fixed.
- `TestPinnedGradleManifestFIFORefusedBeforeExecution` now expects the retirement-unsupported
  refusal where `groupreap.RetirementSupported` is false.

## Evidence

- These tests failed before the change and pass after it: `TestGradlePerRunUserHomeAndDaemonOwnership`
  and `TestPreIsolationGradlePlanRefusedForExecution`.
- Opt-in `TestGradleLiveOwnership` PASS (108.6 s) through `testrunner.Execute`. Tuple: Gradle 9.6.1
  launcher sha256 `3cd5d18e…ef7ef7`, OpenJDK 17.0.20.1, macOS 26.6.2 arm64, and JUnit Jupiter/Platform
  6.1.3 jars as project-local pinned libraries. The jars came from the existing local module cache;
  each SHA-1 matched its cache path.
- Outcomes: exit 1, complete; `proof.ProofTest#passes()` PASSED, `#fails()` FAILED,
  `#skipped()` SKIPPED; empty retirement.
- No-match filter: exit 1, `report pattern has no matches`, incomplete.
- 90 s timeout and interruption after test-body readiness: exit `-1`, incomplete; two processes
  retired by `ANCESTRY`; no `unretired` entry; no recorded test-body ancestor alive.
- Every run created `<ReportDir>/.home/.gradle/daemon`.
- Ambient preservation:
  - A sentinel daemon under a separate scoped home survived all runs and was then stopped with a
    `--stop` scoped to that home only.
  - The new entries in the account daemon directory during the window came from an unrelated
    concurrent process (their logs name its own temp paths), not from these runs.
- The retained JSON evidence is local and uncommitted.

## Limits

- Known limit, accepted for now: Linux and other non-Darwin platforms refuse `gradle-junit`
  execution, because retirement is unproven there. Linux retirement is a separate follow-up, and
  Linux live qualification is NOT_RUN.
- Toolchain: all tests here ran with local Go 1.27.2, not the 1.27.1 named in `AGENTS.md`.
- External module dependencies cannot resolve in the empty offline home. `GRADLE_RO_DEP_CACHE` is a
  non-goal.
- Full `go test ./...`, `make gate` and `cmd/corvint` package tests are NOT_RUN. Independent review
  is NOT_RUN in this lane.
