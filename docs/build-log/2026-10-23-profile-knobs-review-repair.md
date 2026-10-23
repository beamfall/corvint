# Review repair of the profile knobs and read-budget harness (PR #1608; GitHub #1600, #1602)

Date: 2026-10-23. Lane: opencode/profiling-bench-gh1600-1602. Follows
`2026-10-23-profile-knobs-and-read-budget.md`. An independent static review found one P1, five P2
and one P3; each fix has a regression test that failed before the fix.

## Findings and decisions

- P1, profile destination inside a repository (`PRB-V0-013`, new): `CORVINT_CPUPROFILE=cpu.pprof`
  in a clean repository created an untracked file before `affected --path` checked cleanliness,
  turning a successful read into `unsupported-affected-mixed-worktree`; naming a tracked file
  truncated it. The bracket now resolves each destination through symbolic links (a dangling final
  link included) and refuses `profile-invalid` before opening either file when any ancestor holds a
  `.git` entry, is a `.git` directory, or has a Git directory's layout. The entry bracket does not
  know which root a command will inspect, so the boundary is every repository, which is stricter
  than the inspected worktree alone.
- P2, late startup failure erased profiles (`PRB-V0-006`): truncation preceded starting the CPU
  profiler. Every fallible step now precedes the first truncation: the profiler starts writing
  into a mutex-gated writer held until truncation ends, each regular file is first truncated to its
  own size as a non-destructive probe, and a non-regular destination (`/dev/null`, a pipe) is not
  truncated. A refused start discards the profiler's final flush instead of writing into the
  preserved file. A truncation to zero that fails after its own probe passed remains a residual
  race, not a reachable startup path.
- P2, CPU write errors lost (`PRB-V0-007`): `runtime/pprof` drops its writer's errors. The CPU
  profile now writes through an error-recording writer whose first error Stop reports as
  `profile-write-failed`, keeping the body's status. The test writes to a pipe whose reader is gone,
  where Close succeeds.
- P2, interrupted builds left descendants (`PRB-V0-037`, new): every harness process now runs
  under `internal/groupreap` containment (own process group, cancellation stops the leader, the
  group is killed before the leader is reaped). `go build` and the harness's Git reads also carry a
  `groupreap.Retirer` owner token where retirement is supported, so a descendant that left the group
  is killed and awaited before the deferred scratch removal. Measured reads use the group sweep
  only, because retirement's process-table scans would inflate measured wall time.
- P2, unstartable MCP executable exit status (`PRB-V0-029`): a process that never started is now a
  `perfbudget.StartError` returned from `Run` (exit 2); a command or tool that starts and refuses
  still fails its scenario (exit 1).
- P2, unreadable fixture directory (`PRB-V0-023`): `Generate` proceeds only when `os.ReadDir`
  proves the directory absent or empty; any other inspection error refuses before writing.
- P3, acceptance markers: every `PRB-V0` definition now leads with the
  `(accepted under the owner's standing auto-accept (2026-10-11), V1-…)` marker used by
  `docs/specs/corvint-tasks-pool-safe-reuse-v0.md`, and `REQUIREMENTS.tsv` was regenerated.
