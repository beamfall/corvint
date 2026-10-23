# Profile knobs on every command and a read-budget harness for GitHub #1600, #1602

Date: 2026-10-23. Lane: opencode/profiling-bench-gh1600-1602 (base dd0d09db5). Tickets: V1-2475,
V1-2474. Requirements `PRB-V0-001..012` (#1600) and `PRB-V0-020..036` (#1602) in the new
`docs/specs/performance-profiling-and-read-budget-v0.md` are accepted under the owner's standing
auto-accept (2026-10-11). `CRB-V0-018` gains `CORVINT_MEMPROFILE` in its exact settings list.

## Decisions

- #1600 cause: `CORVINT_CPUPROFILE` was started inside two command paths (`context` and the
  harness-event dispatch), so every other command ignored it. The fix removes both and applies one
  bracket (`internal/procprofile.Run`) around `runContext` in `main`, and around `run` in each of
  the four MCP servers. New commands inherit it; `TestPRBV0008NoNestedProfilerStart` keeps a
  command from starting its own profile, which would also collide with the bracket at run time.
- The heap knob writes `pprof.Lookup("heap")` after a collection at the end of the bracket. Both
  files are created before the command runs, so an unwritable path refuses up front
  (`profile-unavailable`, status 2) rather than losing the profile after a 20 s read.
- Signals: the servers and `corvint` already catch SIGINT and SIGTERM through
  `signal.NotifyContext`; the bracket flushes when that context ends, even if the body keeps
  running. `{pid}` in a path gives one file per process, because `corvint` re-executes itself for
  native hooks, dogfood flow and the index refresh worker and those children inherit the
  environment.
- The internal workers dispatched before the bracket stay unprofiled: their startup-only group
  verification must precede every goroutine, and the CPU profiler starts one.
- `help --json` did not exist on main; the capability catalog of #1588 (PR #1590) is still open.
  This lane adds `help --json` printing only `{"profiling": ...}`; when #1590 lands the member moves
  into the catalog. The two changes touch the same lines of `parseHelpInvocation` and `helpText`,
  so whichever merges second resolves a small conflict.
- #1602 fixture: generated at run time through the doccorpus library (manifests) and the
  measured `corvint docs corpus build` (artifacts). A first attempt built artifacts in process and
  every CLI read refused with `artifact cannot be rederived from current builder`, because the
  artifact records the builder's Go version and VCS revision; hence `PRB-V0-025` and the recorded
  builder SHA-256 for fixture reuse. Additional-repository records come from adoption providers
  (64 MiB record bound) so 1,000 pages per repository fit; the plain provider schema stops at 4,096
  records.
- Budgets: the #1598 and #1601 targets are committed at their exact numbers with
  `status: pending`, so the run passes today while reporting `pending-exceeded`. What fails a run
  today is a separate, explicitly labelled regression ceiling per scenario, derived from the
  recorded baseline. Unobserved enforced metrics fail closed.
- CI wiring is not in this change: `.github/` edits need owner approval, so #1602 stays open.

## Baseline (recorded in `internal/perfbudget/budgets.json`)

Full scale: 10,000 inputs, 96,350 records, 3 additional repositories, a 97 MB artifact, and a
20,000-file Ruby tree. Measured on darwin/arm64, 12 cores, go1.27.2, with the tree at dd0d09db5
plus this change (the read paths are unchanged), under `nice -n 15` from the shared host semaphore.
The host was heavily contended: the 1-minute load was 137.6 before and 23.1 after (`contended:
true`), so these numbers are pessimistic.

| Scenario | p50 ms | p95 ms | peak RSS MiB | Target (pending) |
|---|---|---|---|---|
| cli-cold-read | 10,271 | 11,771 | 989 | < 1,000 (#1598) |
| cli-warm-read | 7,551 | 11,645 | 978 | < 200 (#1598) |
| mcp-first-call | 8,552 | 9,434 | 954 | < 1,000 (#1598) |
| mcp-warm-call | 131 | 625 | 951 | < 100 (#1598) |
| full-delta | 6,945 | 7,423 | 1,163 | none stated |
| affected-path | 12,037 | 12,235 | 5,244 | < 1,000 (#1601) |

An earlier, less contended run (load 36 to 44) measured cold reads at 7.8 s p50 and
`affected --path` at 16.8 s p50 with 5.1 GiB peak RSS. That confirms #1598 and shows that #1601
is also a memory problem on a large Ruby tree.

## Findings and limits

- With three to ten samples, p95 is close to the slowest sample. The sample counts are in the
  budgets file and can be raised once CI hosts are known.
- The ceilings are relative to this host. A CI host needs its own `--record-baseline` run before
  its ceilings mean anything; the contention record says how far to trust a verdict.
- A full harness run takes about 5 minutes and several GiB of memory on this host.

## Independent review

One read-only review found no HIGH issues. Every MED finding was fixed in this change:
- `go run` flattened the harness's exit status 2 to 1, so `make perf-budget` now builds the
  harness and runs the binary.
- The adapter host-kill deadline stopped the profiles early. The bracket now watches only the
  signal context.
- A refusal could truncate or leave behind a heap file. Files are now opened without truncation,
  and a file the refusal created is removed.
- A smoke run could be judged or baselined against the full-scale ceilings. A scale mismatch now
  refuses, and a baseline with a failed scenario is not recorded.

LOW findings fixed:
- `help --json` now lists the plumbing verbs.
- Stopping the profiles is deferred, so a panic still writes them.
- The same-file check now compares absolute paths and uses `os.SameFile`.
- `Validate` requires both ceilings.
- Reused fixtures are now refused unless they carry the current generator version.
- An MCP response with no `isError` field is accepted.
- The determinism test now compares every repository's HEAD.

LOW findings left open:
- `TestPRBV0008NoNestedProfilerStart` scans only `cmd/corvint`. No package that `corvint`
  imports starts a profile today; `internal/tasks/scale` does, but it is a separate binary.
- The three non-corpus server tests drive only `--version` through the entry function.
