## 2026-09-26 V1-0351, V1-0355, V1-0347, V1-0353, V1-0387: rc1 test flakes and local gate build root

Base `b29d35e5`. Checks ran on a host with load averages between 35 and 200.

- V1-0351: no new change. The fixture Git auto-maintenance race is already closed by `adafe83f`
  and `39e00111`. `cmd/corvint/work_materialization_test.go:193` sets `GIT_CONFIG_PARAMETERS` to
  disable `maintenance.auto` and `gc.auto` for the package, and `materializationQuiesce` (line
  258) sets both in each fixture repository. In a Trace2 capture, a fixture commit without the
  override spawns `git maintenance run --auto --quiet --detach`; with it, the commit spawns none.
  `TestObserveWorkUsesOnlyTargetMaterialization` passed `-count=20` alone (74s) and `-count=20`
  again (199s) while `-race -count=20` of `tools/gate-ledger` ran at the same time.
- V1-0355: no new change. The retained hang dump from the #241 tree (`3c1b894c`) shows git
  itself not exiting at `tools/gate-ledger/main.go:646`, called from `main_test.go:326` after the
  test sets `core.fsmonitor=true`. A live git holding its own output pipe also leaves the
  `writerDescriptor` copy blocked, so the dump needs no grandchild. `26612df8` already adds
  `-c core.fsmonitor=false -c core.ignorestat=false` to every gate-ledger git call
  (`main.go:923-926`). This review found no subprocess that leaves a grandchild on the pipe. A
  Trace2 capture of the package shows every fixture auto-maintenance process detaching, which
  closes its stdio, and shows no fsmonitor spawn. The Go telemetry child inherits neither pipe.
  `-count=20` passed (197s, 180 PASS) and `-race -count=20` passed under parallel load (586s).
- V1-0347: `internal/companionrelease/exported_current_test.go` read `CORVINT_PROOF_TASKS_ROOT`
  at lines 12 and 67. Both proofs now export the in-tree checkout at `../..` and pass
  `tasksExport` of that one export to the corvint-tasks build and the queue/policy generator, as
  `companionrelease.go:93` does. The VSIX proof stays opt-in on `CORVINT_PROOF_NPM_CACHE`. The
  smoke-generator proof now has no skip gate, so it runs by default (15s). With a local npm
  cache, `TestCurrentExportedTasksAndVSIX` built corvint-tasks and the VSIX twice each and found
  21 members. It then failed at its `legacy icon accepted` check, which refused
  `extension/media/corvint.svg`, the same member the proof requires, so it could never pass. The
  proof now requires exactly the `vsixMembers` set (the count plus every current member), which
  admits no legacy member, and still requires the configuration and icon members to be non-empty.
  With `CORVINT_PROOF_NPM_CACHE=$HOME/.npm` the proof passed (1394s at host load above 500).
- V1-0353: `script/local-console-release-gate:50` ran `go build ./cmd/corvint-companion-release`
  in the caller's working directory. The script now changes to its own resolved checkout root
  before building, as `script/corvint-companion-release-gate` does. Relative path arguments
  therefore resolve against that root. `TestLocalConsoleGateBuildsFromRepositoryRoot` runs the
  script from a temporary directory with a fake `go` on `PATH`. The test failed on the old
  script and passes on the new one. The PUB-V0-011 traceability row names the test.
- V1-0387: `TestPUBV0026InterruptedInstallReapsDescendant` gave its `ps` observations one
  second (`internal/releasecandidate/operations_posix_test.go`, identity and cleanup) and two
  seconds (`assertFixtureProcessGone` in `operations_test.go`). At host load near 450 the `ps`
  child was killed at the deadline, and the package failed with `fixture identity unavailable`.
  Each observation now has 30 seconds, which is still shorter than the fixture child's 60-second
  sleep. The PUB-V0-026 cancellation and cleanup bounds are unchanged.

NOT_RUN: `go test ./...`, `make gate`, the dogfood CEM steps, and the combined companion bundle
qualification.
