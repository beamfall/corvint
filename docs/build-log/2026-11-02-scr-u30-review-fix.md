# Lane scr-u30: Codex review fixes (2026-11-02)

Follow-up to `2026-11-01-scr-u30.md`. One independent Codex review of
`35475f860d97bfde9bbaa6cccf1f2ba8f78ea44b..e2451bbb3` returned two P1 findings and one P2 finding,
with verdict FIX. Each finding was checked against the code before it was changed.

## Findings

- P1, `script/heavy.sh`: an offload lost `GIT_CONFIG_GLOBAL`. **Fixed.** This was confirmed: only the
  argv and `CALLER_GMP` crossed ssh, so the remote command ran under the remote login's value.
  - `offload` now writes `CALLER_GCG_SET` and `CALLER_GCG` into `meta.env`.
  - `serve` exports the caller's value, or unsets the variable, just before it runs the command.
    This overrides any value set by the remote environment file.
  - heavy.sh's own remote Git calls still go through the `git` function without the variable.
  - ASD-V0-708 now requires this.
  - The `lane-scripts-test` ASD-V0-708 group adds two offload cases: the caller's directory value
    wins over the env file's, and an unset caller gives an unset command. With the previous
    heavy.sh both cases fail.
  - Limit: the value is a path, and it is read on the remote host.
- P1, `internal/tasks/dispatch/reap_test.go`: a symlink does not rename the process on Darwin.
  **Fixed.** This was confirmed live on the Mac (Darwin 25.6.0).
  - A C binary `realsleepx` was run through a symlink named `dispatch-reaper`.
  - The kernel name was `realsleepx`, in both the sysctl `kinfo_proc.p_comm` and `proc_name`.
    `ps -o comm` showed the symlink path.
  - Run through a hard link, the binary got `p_comm` = the link's name.
  - `ps -o ucomm` reads `p_comm` on Darwin and the kernel comm on Linux.
  - `startDeadMan` now hard-links the test binary as `dispatch-reaper`. When the temporary directory
    is on another file system, it copies the binary instead (`copyExecutable`).
  - The first Darwin run of the fix failed. The reaper was sometimes already a zombie: 2 of 4 runs
    of a perl harness that forks the helper child. Removing the alias as soon as `cmd.Start`
    returned raced the Darwin process start.
  - The reaper now removes its own alias directory once it runs (`deadManAliasEnv`). After that
    change, 0 of 8 harness runs had a zombie reaper and no alias directory was left.
  - `TestATRV0213_FixtureWorkersDieWithTheirTestBinary` now matches the binary's name by argv[0]
    (`observeProcs`) or by kernel name (`kernelNames`, `ps -axo pid=,ucomm=`). It also requires a
    child named `dispatch-reaper` by both. ATR-V0-700 now says this.
- P2, `script/heavy.sh` `env_reasons`: the operand of an env option ended the scan of env words.
  **Fixed.** `-u`/`--unset NAME`, `-C`/`--chdir DIR` and `-P PATH` now skip their operand.
  - ASD-V0-709 and its group add `env -u GOFLAGS HEAVY_WHOLE_REASON=x go test ./x` and
    `env --unset GOFLAGS -C . HEAVY_WHOLE_REASON=x go test ./x`. Both are refused by the previous
    heavy.sh.
  - Non-goal: `-S`/`--split-string`, which is not parsed.

## Tests

- `bash script/lane-scripts_test.sh` on unraid passes all 52 groups. With the previous `heavy.sh`,
  2 groups fail with the 4 new assertions.
- On unraid (Linux), `script/heavy.sh go test -count=3 -run 'TestATRV0213_FixtureWorkersDieWithTheirTestBinary$|TestCALV0353|TestCALV0143' ./internal/tasks/dispatch`
  passes, and `go vet ./internal/tasks/dispatch` passes. No `dispatch-reaper` process was left.
- On the Mac (Darwin 25.6.0), the test ran from a `git archive` of the lane tree, through
  `script/heavy.sh` with `HEAVY_LOCAL_REASON='darwin-only: ...'`.
  - `go test -count=5 -run 'TestATRV0213_FixtureWorkersDieWithTheirTestBinary$|TestCALV0353' ./internal/tasks/dispatch`
    passes. No `dispatch-reaper` process and no `dispatch-dead-man-*` directory was left.
  - With the previous symlink `reap_test.go`, the new test fails on Darwin: `processes named
    "dispatch.test" (kernel name "dispatch.test") under the child: [2 pids]`.
  - The Linux symlink start had passed the old argv-only check.
- `corvint affected --base 35475f860d97bfde9bbaa6cccf1f2ba8f78ea44b` gave scope=UNKNOWN:
  106 selected and 16 unknown (LANGUAGE_FRONTIER 9, NO_SELECTABLE_TEST 7). Only focused tests were
  run.
