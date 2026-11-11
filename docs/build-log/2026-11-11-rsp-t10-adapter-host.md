# RSP T10: run-level adapter host and measured protocol overhead

Date: 2026-11-11. Ticket: V1-5112. Requirements: RSP-V0-031, RSP-V0-063 (new, proposed,
experimental); overhead under CMS-V0-010.

## Decision

The T10 slice on main had the session, discovery, store and conformance kit, but nothing that
turned a failed or missing plugin into a run outcome: a `hello` failure returned an error and no
session, so no runner line named it, and the PR #143 rule that an unavailable external-plugin
runner makes the run FAIL `runner-adapter-unavailable` was unimplemented. `RSP-V0-063` adds one
host per run (`internal/runnerprotocol/host.go`). It takes the required runners, the discovery plan
(trusted pins in CI), the store and release entries and the core's pre-plugin paths, launches only
pins a required runner names, and emits one line per required external runner on close. Mode is
always `full` because the host never admits qualification (RSP-V0-030). `run` is `ok` only for a
clean session or a runner with a pre-plugin path. Every other cause is `fail` with
`runner-adapter-unavailable`: unpinned, malformed discovery, unsupported schema, a resolution
failure, or any RSP-V0-031 session failure. A runner the candidate's pins changed still runs FULL
from its trusted pin. Reasons keep the stable code before the first `:`, so wrapped OS detail such
as store paths does not reach plan output. Orchestrator-made under the 2026-11-02 delegation; the
owner may override.

## Evidence

Run on the Unraid lane host (i7-8700K, 12 CPUs, load about 4, heavy.sh `-p 2`, GOMAXPROCS 2).

- The whole `internal/runnerprotocol` package passed on origin/main `b55635262` before the change
  (13.6 s), as did `internal/runnerpolicy`, `tools/runner-plugin` and `tools/runner-reference`.
  The tests the T10 and T7 build-only entries left NOT_RUN have now run.
- `TestRSPV0063CleanTwin`, `TestRSPV0063FailureLines` (20 subtests: crash, timeout, oversize,
  protocol error, duplicate member, schema violation at `hello` and mid-session, `error` response,
  version mismatch, undeclared runner, untracked and late repository mutation, digest mismatch,
  missing executable, not installed, launch failure, unpinned, malformed discovery, unsupported
  schema, pre-plugin path) and `TestRSPV0063CandidatePinsChangedRunsTrustedPin` pass. They did not
  compile before `host.go` existed. A mutant that turns every `fail` line into `ok` fails 19 of
  them (every subtest except the pre-plugin path, which expects `ok`).
- The conformance kit over the compiled-in reference and its out-of-process twin passes through
  the host (`TestRSPV0063CleanTwin`) as well as through a bare session (`TestRSPV0025TwinParity`).

Per-run overhead (`go test -run '^$' -bench`, 3 x 10 iterations unless noted):

| Measurement | Result |
|---|---|
| `BenchmarkRSPV0063HostRun`: one run on a one-file fixture repository: resolve and digest the test binary, launch, `hello`, inventory, three commands, parse, staged evidence, shutdown, reap, two status snapshots | 77.1 to 79.3 ms/op (30x run: 73.7 ms, 2.08 MB, 8,440 allocs) |
| `BenchmarkRSPV0063CompiledInRun`: the same corpus through the compiled-in twin | 0.18 ms/op (30x) |
| `BenchmarkRSPV0029RepositorySnapshot`: one RSP-V0-029 status snapshot of this checkout (10,241 tracked files, 4 dirty) | 686 to 734 ms/op, 117 MB and 394k allocs per op |
| `corvint affected --base b55635262` on the same checkout (lane binary, 3 runs) | 903 to 978 ms |
| `corvint index --if-stale` (already fresh) | 2,154 ms |

A session takes two snapshots, so one out-of-process run on a repository this size costs about
1.5 s, and the snapshot dominates. The selector never calls a plugin (invariant 7 note in the
spec), so `index` and `affected` pay nothing. Against the runner's own wall time, which is where
the spec places the cost, 1.5 s is under 5% only for runs of about 30 s or more. A narrowed run of
one small Go package, about 2.4 s here, would pay roughly 60%. The CMS-V0-010 bound is therefore
not met for short narrowed runs on large repositories. This agrees with the accepted choice to keep
Go, shell and make, and JavaScript compiled-in. Making the out-of-process form acceptable on hot
paths would mean cutting the snapshot cost (for example hashing only dirty paths against a cached
index, which is a separate design). Frozen-repository measurement is NOT_RUN: no frozen corpus run
was in scope for this lane.

## Remainder (not done here)

- No production run path (`corvint test`, CI shard planner) opens the host or any runner plugin.
  Wiring it in needs the test-planning code under `internal/liveverify/affected/**`, which another
  lane owns.
- The RSP-V0-028 digest-pinned declaration file (`.corvint/runner-plugins/<plugin>.declaration.json`)
  is still unimplemented. Its pin member, the declaration schema and the strict validator need their
  own requirement IDs and a wire decision.

## Rollback

Delete `host.go`, its tests and the `RSP-V0-063` text. The session, discovery and store APIs it
composes are unchanged, and nothing in production calls it.
