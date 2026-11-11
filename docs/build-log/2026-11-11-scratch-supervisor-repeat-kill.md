# Scratch-process supervisor repeats the group kill while a member is live (V1-4694)

Batch review finding (P1) on `tools/test-commands-check/scratch_process_unix_test.go`, the test
supervisor that DTL-V0-018 added: it started bash through `groupreap.Owner` with
`RepeatKillWhileLive` off. `PGO-V0-017` records that one Darwin group SIGKILL can miss a member
that a fork running alongside it adds to the group (3 of 300 stops in the V1-2103 probe), and bash
forks for a command substitution. A missed member would stay live until the 30-second retirement
bound, fail the test and could outlive it.

- Disposition: accepted. `scratchHost` and the recording host now set `RepeatKillWhileLive`.
  The ordering test no longer requires exactly one kill; it requires a kill before the single
  reap and nothing but signal-0 probes after it, which is the rule DTL-V0-018 states.
- DTL-V0-018 names the setting and cites `PGO-V0-017`.
- Evidence: `go test -count=5 -run 'TestDTLV0018|TestDTLV0014' ./tools/test-commands-check/`
  passes on darwin/arm64. The race itself was not reproduced before the change, so the finding
  rests on the `PGO-V0-017` measurement, not on an observed failure here. Linux was not rerun
  for this change; a group kill there restarts a fork in progress, and the repeat only acts
  while a member is observed live.
