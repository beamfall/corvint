# Performance Profiling and Read Budget V0

Owner: Russell Lewis
Date: 2026-10-23
Requirement prefix: `PRB-V0`
Intent status: accepted (owner standing auto-accept 2026-10-11)
Delivery status: experimental
Authoritative inputs: GitHub #1600 and #1602; GitHub #1598 and #1601 (the latency targets the
budgets encode); GitHub #1406 (host-latency limits need a baseline and a contention record);
`docs/specs/corvint-rebrand-v0.md` `CRB-V0-018` (exact runtime settings);
`docs/specs/task-context-packet-v0.md` operational note V1-0051 (the original CPU profile knob).

## Agent digest
- Claim: Every command and MCP server honours the CPU and heap profile knobs, and `make perf-budget` measures seven read budgets over a generated fixture.
- Status: accepted (owner standing auto-accept 2026-10-11)/experimental; checked by `TestPRBV0008EveryNamedCommandWritesProfiles` and `TestPRBV0024HarnessSmokeRun`.
- Exists: `internal/procprofile`, `cmd/corvint/profile_knobs.go`, each MCP server's `runProfiled`, `internal/perfbudget`, `internal/perfbudget/budgets.json`, `make perf-budget`.
- Blocked on: CI wiring of `make perf-budget` needs owner approval of a `.github/` change; the #1598 and #1601 targets stay pending until those changes land.
- Read next: Requirements; Failure modes; Acceptance.

PRB-V0-001..013 (GitHub #1600; V1-2475) and PRB-V0-020..037 (GitHub #1602; V1-2474) are
accepted under the owner's standing auto-accept (2026-10-11); each definition carries its marker.

## User and boundary

An operator diagnosing a slow read (#1598 measured 20 to 27 s per corpus read) needs Go frames, not
an external sampler's view, for whichever command is slow; before this slice `CORVINT_CPUPROFILE`
reached only `context` and the generic harness-event path. A maintainer changing a read, index or
ranking path needs a repeatable measurement on realistic input, because #1487, #1554 and #1598
were each found by callers on large inputs the small CI fixtures never exercise.

Both surfaces are diagnostics. A profile file and a budget report are local, operator-requested
outputs; neither is evidence, authority, or an input to ranking, and neither changes what a
command reads, returns, or mutates (AGENTS.md invariant 4). The budget fixture is generated content
(`trust: generated`) built only for measurement.

## Requirements

### Profile knobs (GitHub #1600)

- `PRB-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) The `corvint` binary MUST apply one profile bracket at its entry point, around the
  whole dispatch, so every command it dispatches (including `docs corpus *`, `affected`, `context`
  and commands added later) writes a pprof CPU profile to the path in `CORVINT_CPUPROFILE`. No
  command may start its own CPU profile.
- `PRB-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) Both knobs MUST be off when unset or empty and MUST be read only through
  `internal/runtimeenv` (`CORVINT_` prefix; the bare `CPUPROFILE` and `MEMPROFILE` are ignored).
  With both off nothing is created.
- `PRB-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) `CORVINT_MEMPROFILE` MUST write a pprof heap profile (after a collection) at the end
  of the same bracket. `CRB-V0-018`'s exact runtime-settings list names it.
- `PRB-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) The profiles MUST be written when the process body returns and as soon as the
  process's signal context ends on SIGINT or SIGTERM, even if the body is still running; a later
  exit does not write them again. A context that ends for another reason (an adapter's host-kill
  deadline) MUST NOT stop the profiles, and a panicking body still writes them.
- `PRB-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) The token `{pid}` in either path MUST be replaced by the process ID, so a child that
  inherits the environment writes its own file instead of truncating its parent's. Without the
  token the path is used as given; one process writes one CPU and one heap file.
- `PRB-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) A knob that cannot be honoured MUST refuse before the command runs, with status 2:
  `profile-unavailable` when a file cannot be created or the profiler cannot start,
  `profile-invalid` when both knobs name the same file or a value holds a NUL byte. `corvint`
  reports the refusal as its JSON error envelope; servers write one `<server>: <error>` stderr line.
  Two names that reach one file (a relative and an absolute path, a symbolic link) are the same
  file. A refusal MUST NOT truncate an existing file and MUST remove a file it created: every
  fallible startup step (opening both files, the same-file check, starting the CPU profiler, and
  a size-preserving truncation probe of each regular file) precedes the first truncation, and the
  profiler cannot write before truncation ends. A non-regular destination (a device such as
  `/dev/null`, or a pipe) is written without truncation.
- `PRB-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) A profile write that fails at exit MUST be reported (`profile-write-failed`) and MUST
  keep the command's own exit status. The CPU profiler writes through an error-recording writer,
  because `runtime/pprof` drops its writer's errors: a failed CPU profile write is reported even
  when closing the file succeeds.
- `PRB-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) Setting the knobs MUST NOT change any stdout byte or exit status of a command.
- `PRB-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) `corvint help --json` MUST carry a `profiling` member (`corvint-profile-knobs/0`)
  naming both knobs, the default, the `{pid}` token, when files are written, every command that
  honours them (the dispatcher's own top-level command list, and the hidden plumbing verbs
  `native-hook`, `authority-event` and `qualified-event`), every MCP server that honours them,
  and the entries that do not. Until the capability catalog of GitHub #1588 lands, `help --json`
  prints only this member; the catalog absorbs it unchanged.
- `PRB-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) Every MCP server binary (`corvint-mcp`, `corvint-corpus-mcp`, `corvint-docs-mcp`,
  `corvint-test-validity-mcp`) MUST apply the same bracket at its entry point, one profile per
  server process, written when the server exits (end of input) and on SIGINT or SIGTERM.
- `PRB-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) `CRB-V0-018`'s exact runtime-settings list MUST name `CORVINT_CPUPROFILE` and
  `CORVINT_MEMPROFILE`.
- `PRB-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) The internal child workers that `corvint` starts before the bracket (the dogfood
  outcome and verifier workers, and the index refresh worker) MUST NOT be profiled, because their
  startup-only verification precedes every goroutine; `help --json` lists them as not honouring
  the knobs.
- `PRB-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2475) A knob whose destination resolves (through symbolic links, including a dangling
  final link that opening would follow) to a path inside a Git worktree (any ancestor holding a
  `.git` entry) or Git metadata (a `.git` directory, or a directory with a Git directory's
  `HEAD`, `objects` and `refs` layout) MUST be refused with `profile-invalid` before either file is
  opened, so profiling never creates an untracked file that turns a clean read into a refusal and
  never truncates repository content. The check covers every repository, not only the one a
  command inspects, because the inspected root is not known at the entry bracket.

### Read budget harness (GitHub #1602)

- `PRB-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) A generator MUST produce the budget fixture at run time, never committed: a primary
  repository, additional repositories, a previous and a current corpus artifact and their
  manifests, and a Ruby tree. Identical scales MUST produce byte-identical commits, manifests and
  artifacts for one builder (fixed Git identity and dates, no user or system Git configuration).
- `PRB-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The full scale MUST hold 10,000 corpus inputs (7,000 primary Markdown pages and
  three additional repositories of 1,000 pages each), about 96,000 corpus records (under the
  100,000-record artifact bound), 350 pages changed between the previous and current artifacts,
  and a Ruby tree of at least 20,000 files. A smoke scale keeps the same shape at unit-test size.
- `PRB-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The current artifact MUST declare the three additional repositories
  (`DCP-V1-175`); additional-repository records come from adoption providers whose anchors carry
  Git blob identities and span hashes of the generated pages.
- `PRB-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) An invalid scale or a non-empty fixture directory MUST be refused before anything
  is written. Generation proceeds only into a directory proved absent or empty; a directory that
  cannot be listed (for example write and search permission without read permission) is refused.
- `PRB-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The harness MUST measure six scenarios: `cli-cold-read` (`docs corpus search` with
  any `.corvint/` derived state removed before each sample), `cli-warm-read` (the same read after
  one unmeasured run), `mcp-first-call` (corpus MCP process start through the first `docs_search`
  response), `mcp-warm-call` (later `docs_search` and `docs_get` calls on one server),
  `full-delta` (`docs corpus delta` of the previous against the current artifact) and
  `affected-path` (`affected --path` on the Ruby tree after one unmeasured run).
- `PRB-V0-025`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The measured `corvint` binary MUST build the artifacts it reads, because a read
  rederives the artifact and refuses another builder's. A reused fixture whose recorded builder
  SHA-256 differs MUST be rebuilt from its manifests; a reused fixture of another scale or another
  generator version (`corvint-perf-budget-fixture/1`) is refused.
- `PRB-V0-026`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) Each scenario MUST report every sample's wall time, nearest-rank p50 and p95, and
  the maximum peak resident set of the processes that served it. A refused read (non-zero exit or
  MCP error result) MUST record the scenario as failed, never as a fast sample.
- `PRB-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) Peak resident set MUST come from the exited process's own resource usage (bytes on
  Darwin, kilobytes scaled to bytes elsewhere). Where the host reports none, the value MUST be null
  with a `NOT_OBSERVED` reason, never estimated.
- `PRB-V0-028`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) Every report and baseline MUST carry the host-contention record (GitHub #1406):
  OS, architecture, Go version, core count, the 1, 5 and 15 minute load averages before and after
  the run (null with a reason where unavailable), and `contended` when either 1-minute load reached
  the core count.
- `PRB-V0-029`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) `make perf-budget` MUST build the harness and both binaries from the tree, run the
  harness as a built binary (not `go run`, which flattens exit statuses), and print the report
  JSON. The harness MUST exit 0 when every enforced limit holds, 1 when an enforced limit is
  exceeded or unknown, and 2 when it cannot run, including a `--scale` other than the budgets
  file's and a measured executable (`--corvint`, `--corpus-mcp`) that cannot be started (a command
  or tool that starts and refuses fails its scenario instead); `make` fails on either non-zero status and names it in its `Error N` line. `PERF_BUDGET_ARGS`
  passes `--budgets`, `--fixture`, `--scale`, `--corvint`, `--corpus-mcp`, `--out` and
  `--record-baseline`; a baseline is not recorded when any scenario failed or measured nothing.
- `PRB-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The committed budgets file (`internal/perfbudget/budgets.json`,
  `corvint-perf-budget/0`) MUST bound exactly the six scenarios in order with positive limits on
  `p50_wall_ms`, `p95_wall_ms` or `peak_rss_bytes`, each with a reason; a pending limit MUST name
  what it waits on, and every scenario MUST carry the two `PRB-V0-032` ceilings. A file that
  departs from this is refused.
- `PRB-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The budgets MUST encode the GitHub #1598 targets (cold CLI read p95 under 1,000 ms,
  warm CLI read under 200 ms, MCP first call under 1,000 ms, MCP warm call under 100 ms) and the
  GitHub #1601 target (warm `affected --path` under 1,000 ms) at exactly those numbers, as
  `pending` until the owning change lands; promoting one to `enforced` is that change's edit.
- `PRB-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) Every scenario MUST carry enforced regression ceilings on p95 wall time and peak
  resident set, derived from the recorded baseline (3x p95 wall rounded up to 500 ms, 1.5x peak
  resident set rounded up to 64 MiB) and labelled as ceilings, not targets. A ceiling is never
  pending.
- `PRB-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) The budgets file MUST carry the baseline the ceilings derive from: the measured
  revision, time, fixture record, host-contention record and every scenario's measurements.
  `--record-baseline` rewrites only that member.
- `PRB-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) A pending target MUST be reported (`pending-pass`, `pending-exceeded` or
  `pending-unknown`, also listed under `pending`) and MUST NOT fail the run; an exceeded enforced
  limit MUST fail it.
- `PRB-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) An enforced limit whose metric was not observed, or whose scenario failed or was
  not measured, MUST fail the run (`unknown`), never pass it.
- `PRB-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) Measured processes MUST NOT inherit any `CORVINT_` setting, the profile knobs
  included, and reports MUST NOT carry absolute host paths in the measured commands.
- `PRB-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2474) Every process the harness starts (the `go build` of both binaries, Git, the
  measured reads and corpus MCP servers) MUST run as the leader of its own process group under
  `internal/groupreap` containment: cancellation (SIGINT or SIGTERM to the harness) stops the
  leader, and every group member is killed before the leader is reaped. The unmeasured builds and
  harness Git reads also carry a `groupreap.Retirer` owner token where the platform supports
  retirement, so a descendant that left the group is killed and awaited too. The harness removes
  its scratch directory only after that cleanup returns.

### Unchanged-pair delta budget (GitHub #1634)

Status for `PRB-V0-050..053`: proposed, experimental; GitHub #1634. They amend `PRB-V0-024`,
`PRB-V0-030` and `PRB-V0-032` for one added scenario.

- `PRB-V0-050`: The harness MUST measure a seventh scenario after the six of `PRB-V0-024`:
  `unchanged-delta`, `docs corpus delta --previous corpus.json --artifact corpus.json --format
  jsonl` over the current artifact against itself, with no unmeasured run. The budgets file MUST
  bound exactly the seven scenarios in that order.
- `PRB-V0-051`: `unchanged-delta` MUST carry the two enforced `PRB-V0-032` ceilings, except that
  its p95 wall ceiling is 2x its baseline p95 rounded up to 500 ms, so a delta that rederives the
  artifact (`DCP-V1-460`) exceeds it on the baseline host; the margin depends on the host, and it
  has no target, because GitHub #1634 states no number.
- `PRB-V0-052`: The baseline MUST carry the `unchanged-delta` measurement the ceilings derive from.
  Until the next `--record-baseline` run it was recorded in a separate run from the other six,
  whose revision, time and host load are named in its ceiling reasons and in
  docs/build-log/2026-10-23-corpus-delta-verified-read.md.
- `PRB-V0-053`: A baseline measurement taken in another run than the baseline's own MUST carry
  that run's `provenance`: its start time, the measured corvint executable's clean source revision
  and SHA-256 (the same executable built the artifacts it read), and the run's host observation.
  It is attributed to that provenance, never to the baseline's `revision`, `host` or
  `fixture.builder_sha256`; a measurement without one belongs to the baseline's run. The budgets
  file refuses a provenance missing any of those fields, and `--record-baseline` writes none,
  because every measurement then comes from one run.

## Non-goals

- No per-phase timing for `docs corpus` (`--timings` is GitHub #1598's lane) and no profile
  analysis, upload or retention; the knobs write files and nothing reads them.
- No CI job: wiring `make perf-budget` into `.github/` needs owner approval. GitHub #1602 stays open
  for that step.
- No enforced product targets before #1598 and #1601 land, and no loosening of their numbers.
- No profile for the internal child workers (PRB-V0-012), `corvint-lsp`, `corvint-pulse` or the
  other non-MCP binaries.
- No Windows peak resident set (PRB-V0-027 reports it `NOT_OBSERVED`, and an enforced RSS ceiling
  then fails closed); the harness targets Unix hosts.
- No committed large fixture, no network, no new storage root, and no change to any read path.
- The `unchanged-delta` scenario does not measure `--verify-rederive`, a changed pair (that is
  `full-delta`) or classification cost; it bounds wall time, not the read path, so a regression
  smaller than its headroom passes.

## Failure modes

- Unwritable profile path, both knobs naming one file, NUL in a value: refused before the command
  runs (PRB-V0-006); the command neither reads nor writes anything.
- Profile write failure at exit, including a CPU profile write error the profiler drops:
  reported, exit status unchanged (PRB-V0-007).
- A profile destination inside a Git worktree or Git metadata, directly or through a symbolic
  link: refused before either file is opened (PRB-V0-013).
- A startup step that fails after both files are open: refused with every existing profile
  intact (PRB-V0-006).
- A process killed by SIGKILL or a crash writes no complete profile; only SIGINT and SIGTERM are
  caught.
- Fixture generation over a corpus bound (records, artifact bytes, provider bytes, index postings)
  fails the harness with status 2 and the builder's own refusal.
- A refused read, an MCP error result, or a server that exits non-zero fails its scenario, and so
  the run (PRB-V0-026, PRB-V0-035). A measured executable that cannot be started is a harness
  failure, status 2 (PRB-V0-029).
- An unreadable fixture directory: refused before anything is written (PRB-V0-023).
- An interrupted harness: every started process group is swept, and owned descendants retired,
  before the scratch directory is removed (PRB-V0-037).
- A `docs corpus delta` that rederives its current artifact exceeds the `unchanged-delta` wall
  ceiling and fails the run (PRB-V0-051); one that only slows by less than the headroom passes.
- A baseline measurement's provenance missing its start time, clean executable revision,
  executable SHA-256 or host observation: refused when the budgets file is loaded (PRB-V0-053).
  A measurement from another run committed without any provenance cannot be detected by loading;
  `TestPRBV0053BaselineProvenance` pins the one such measurement committed today.
- Contended hosts inflate latency: the run still judges ceilings and records `contended` so a
  reader can discount the verdict; the baseline itself was recorded under heavy contention.

## Acceptance

- #1600: `TestPRBV0008EveryNamedCommandWritesProfiles` runs `docs corpus build`, `info`, `search`,
  `get`, `flow`, `coverage`, `locate`, `delta`, `affected --path`, `context` and `help --json`
  through the entry bracket on a committed fixture, checks both profiles parse as gzip pprof,
  and checks stdout and exit are identical without the knobs.
  `TestPRBV0010CorpusServerHonoursProfileKnobs` serves a corpus MCP session and exits at end of
  input; `TestPRBV0010ServerHonoursProfileKnobs` runs the other three servers' entry function
  (`--version`) under the knobs.
  `TestPRBV0004SignalWritesProfiles` delivers SIGINT and SIGTERM to a real process.
- #1602: `TestPRBV0024HarnessSmokeRun` builds both binaries and runs all six scenarios end to end;
  `TestPRBV0031CommittedBudgets` checks the committed targets, ceilings and baseline. The recorded
  full-scale baseline (build-log entry 2026-10-23-profile-knobs-and-read-budget) measured 96,350
  records over 10,000 inputs on a 12-core host at load 137 (contended).
- #1634: `TestPRBV0050UnchangedDeltaBudget` checks the seventh scenario, its derived ceilings and
  that the before-change delta exceeds the wall ceiling; a harness run with the before-change
  binary fails on `unchanged-delta` (build-log entry 2026-10-23-corpus-delta-verified-read). Its
  rederiving measurements are recorded constants, so it cannot detect a restored rederivation;
  `TestDCPV1460DeltaCLIVerifiedRead`'s build counter (`DCP-V1-461`) is the deterministic guard.
  `TestPRBV0053BaselineProvenance` checks that the separately recorded measurement names its run.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `PRB-V0-001` | `internal/procprofile`, `cmd/corvint/profile_knobs.go`, `cmd/corvint/main.go` | `TestPRBV0001BothKnobsWriteProfiles`, `TestPRBV0008EveryNamedCommandWritesProfiles` |
| `PRB-V0-002` | `internal/procprofile` | `TestPRBV0002KnobsOffByDefault` |
| `PRB-V0-003` | `internal/procprofile` | `TestPRBV0001BothKnobsWriteProfiles`, `TestPRBV0008EveryNamedCommandWritesProfiles` |
| `PRB-V0-004` | `internal/procprofile` | `TestPRBV0004ContextEndWritesProfilesBeforeExit`, `TestPRBV0004SignalWritesProfiles` |
| `PRB-V0-005` | `internal/procprofile` | `TestPRBV0005PIDPlaceholder` |
| `PRB-V0-006` | `internal/procprofile`, `cmd/corvint/profile_knobs.go` | `TestPRBV0006UnusableKnobRefusesBeforeBody`, `TestPRBV0006RefusalWritesNothing`, `TestPRBV0006UnusableKnobRefusesCommand`, `TestPRBV0006LateStartupFailurePreservesProfiles`, `TestPRBV0006DeviceDestinationKeepsOtherProfile` |
| `PRB-V0-007` | `internal/procprofile` | `TestPRBV0007WriteFailureReported`, `TestPRBV0007CPUWriteFailureReported` |
| `PRB-V0-008` | `cmd/corvint/profile_knobs.go`, `cmd/corvint/main.go`, `cmd/corvint/taskcontext.go` | `TestPRBV0008EveryNamedCommandWritesProfiles`, `TestPRBV0008NoNestedProfilerStart` |
| `PRB-V0-009` | `cmd/corvint/profile_knobs.go`, `cmd/corvint/help.go` | `TestPRBV0009HelpJSONListsProfileKnobScope` |
| `PRB-V0-010` | `cmd/corvint-mcp`, `cmd/corvint-corpus-mcp`, `cmd/corvint-docs-mcp`, `cmd/corvint-test-validity-mcp` | `TestPRBV0010ServerHonoursProfileKnobs`, `TestPRBV0010CorpusServerHonoursProfileKnobs`, `TestPRBV0009HelpJSONListsProfileKnobScope` |
| `PRB-V0-011` | `docs/specs/corvint-rebrand-v0.md` | `TestPRBV0011KnobsInExactSettingsList` |
| `PRB-V0-012` | `cmd/corvint/main.go`, `cmd/corvint/profile_knobs.go` | `TestPRBV0009HelpJSONListsProfileKnobScope` |
| `PRB-V0-013` | `internal/procprofile` | `TestPRBV0013DestinationInsideRepositoryRefused`, `TestPRBV0013ProfileInsideWorktreeRefusesBeforeRead` |
| `PRB-V0-020` | `internal/perfbudget/fixture.go` | `TestPRBV0020FixtureIsDeterministicAndShaped` |
| `PRB-V0-021` | `internal/perfbudget/fixture.go` | `TestPRBV0021FullScaleIsRealistic`, `TestPRBV0020FixtureIsDeterministicAndShaped`, `TestPRBV0031CommittedBudgets` |
| `PRB-V0-022` | `internal/perfbudget/fixture.go` | `TestPRBV0020FixtureIsDeterministicAndShaped`, `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-023` | `internal/perfbudget/fixture.go` | `TestPRBV0023InvalidScaleRefused`, `TestPRBV0023UninspectableDirectoryRefused` |
| `PRB-V0-024` | `internal/perfbudget/harness.go`, `internal/perfbudget/measure.go` | `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-025` | `internal/perfbudget/fixture.go`, `internal/perfbudget/harness.go` | `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-026` | `internal/perfbudget/measure.go` | `TestPRBV0026Percentiles`, `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-027` | `internal/perfbudget/rusage_unix.go`, `internal/perfbudget/rusage_other.go` | `TestPRBV0026Percentiles`, `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-028` | `internal/perfbudget/host.go` | `TestPRBV0024HarnessSmokeRun`, `TestPRBV0031CommittedBudgets` |
| `PRB-V0-029` | `Makefile`, `internal/perfbudget/cmd/perf-budget`, `internal/perfbudget/harness.go`, `internal/perfbudget/process.go` | `TestPRBV0029HarnessCannotRunExitsTwo`, `TestPRBV0029UnstartableMCPExitsTwo`, `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-030` | `internal/perfbudget/budget.go` | `TestPRBV0030BudgetsValidation` |
| `PRB-V0-031` | `internal/perfbudget/budgets.json` | `TestPRBV0031CommittedBudgets` |
| `PRB-V0-032` | `internal/perfbudget/budgets.json`, `internal/perfbudget/budget.go` | `TestPRBV0031CommittedBudgets`, `TestPRBV0030BudgetsValidation` |
| `PRB-V0-033` | `internal/perfbudget/budgets.json`, `internal/perfbudget/cmd/perf-budget` | `TestPRBV0031CommittedBudgets` |
| `PRB-V0-034` | `internal/perfbudget/budget.go` | `TestPRBV0034EvaluateVerdicts` |
| `PRB-V0-035` | `internal/perfbudget/budget.go` | `TestPRBV0034EvaluateVerdicts` |
| `PRB-V0-036` | `internal/perfbudget/measure.go`, `internal/perfbudget/harness.go` | `TestPRBV0036MeasuredEnvironmentStripsSettings`, `TestPRBV0024HarnessSmokeRun` |
| `PRB-V0-050` | `internal/perfbudget/budget.go`, `internal/perfbudget/harness.go` | `TestPRBV0050UnchangedDeltaBudget`, `TestPRBV0024HarnessSmokeRun`, `TestPRBV0030BudgetsValidation` |
| `PRB-V0-051` | `internal/perfbudget/budgets.json` | `TestPRBV0050UnchangedDeltaBudget`, `TestPRBV0031CommittedBudgets` |
| `PRB-V0-052` | `internal/perfbudget/budgets.json` | `TestPRBV0050UnchangedDeltaBudget`, `TestPRBV0031CommittedBudgets` |
| `PRB-V0-053` | `internal/perfbudget/measure.go`, `internal/perfbudget/budget.go`, `internal/perfbudget/budgets.json` | `TestPRBV0053BaselineProvenance` |
| `PRB-V0-037` | `internal/perfbudget/process.go`, `internal/perfbudget/measure.go`, `internal/perfbudget/cmd/perf-budget/main.go` | `TestPRBV0037InterruptRetiresBuildDescendants`, `TestPRBV0024HarnessSmokeRun` |

## Rollback

Profile knobs: remove the brackets and `internal/procprofile`; restore the two `context` and
harness-event blocks if the original `CORVINT_CPUPROFILE` scope is wanted, drop
`CORVINT_MEMPROFILE` from `CRB-V0-018`, and drop `help --json` (or the `profiling` member once the
catalog lands). No persistent state, wire format or read output changes. Budget harness: remove
`internal/perfbudget`, the Makefile target and this spec's rows; nothing else depends on them.
