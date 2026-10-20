# 2026-10-19: V1-2007 appflows interruption test waits on published readiness and names what is missing

## Intent

Ticket V1-2007 (suspected flake, not a confirmed bug). Hosted run 39357611388, attempt 1, job
118416710971 (`go-product-shard (4)`, ubuntu, `go test -race -p 1` under test-confine) failed
`TestAFUV0NoDescendantsOnInterruption` (`AFU-V0-010`, `docs/specs/application-flow-understanding-v0.md`)
with `observer descendants never started` after 10.55 s. Attempt 2 of the same run passed it in
0.76 s. V1-1363 (`2026-10-14-v1-1363-appflows-descendant-reap.md`) covered a different mode, where
descendants were reported as survivors after interruption.

BASE: `origin/main` `c27d54e9245f0a15d581062b83a0452c03508703`. Test-only change: no requirement,
product code, wire contract or error code changes.

## What the hosted log shows

- `=== RUN` at 02:19:02.802, the fatal readiness message at 02:19:12.843 (the 10 s bound), and
  `--- FAIL` at 02:19:13.351. The 0.51 s between the fatal message and the end matches `runObserver`'s
  500 ms `BeforeStop` grace (`internal/appflows/observer.go`). The deferred join therefore waited for
  a running observer. Inference: `Observe` had not returned, so the Node leader was alive for the
  whole 10 s, but `child.pid` and `grandchild.pid` were not both parseable. The log does not say
  which file was missing.
- The appflows tests in that attempt ran about 1.3 to 1.6 times slower than in attempt 2 (for
  example `TestFlowDocMaintenanceOversizedProposal` 104.17 s against 64.99 s), so the runner was
  loaded. Nothing else in the log shows a 10 s stall.
- `gh run view <run> --job 118416710971 --log-failed` returned attempt 2's log, in which this test
  passed. The attempt-1 log needs `--attempt 1`.

## Cause (NOT_REPRODUCED on the hosted mode)

Two mechanisms produce the hosted signature: a leader still running and no parseable pair of pids
after 10 s.

1. Slow Node start-ups. The readiness chain needs two Node start-ups, first the leader and then the
   child. On a loaded runner these could exceed the 10 s bound. Not observed locally.
2. A child that dies, or whose grandchild spawn fails, before `grandchild.pid` is written. The BASE
   fixture had no `error` handler. It wrote `String(c.pid)` straight after `spawn`, which is
   `undefined` when the spawn fails, and the unhandled error then killed the child. The leader kept
   running on its `setInterval`, so the test polled until its bound. This was reproduced locally by
   fault injection (below): it gives exactly the hosted message and duration.

The retained log cannot separate the two. The fix handles each one, and the next failure, if any,
will name which one it was.

Non-atomic writes are not the cause. BASE wrote both pid files with `writeFileSync`, so a
reader could see an empty file. The poller retried, though, so this could not last 10 s. Publication
now renames the file into place.

## Change

`internal/appflows/observer_unix_test.go` only:

- The fixture provider publishes each pid only on Node's `spawn` event, so the pid names a process
  that has actually exec'd. Publication writes `<name>.tmp` and renames it into place. A spawn
  `error` is published as `child.error` or `grandchild.error`, and the child's `exit` as
  `child.exit`.
- `awaitObserverDescendants` replaces the inline poll. It returns as soon as both pids parse. It
  fails at once, naming each pid file's state, when the observer returns (with its error), when a
  descendant reports a spawn error, or when the child exits before it is ready. The deadline is now
  60 s (`descendantReadinessBound`). Because every failure that is not a stall now fails at once,
  the bound only catches a provider that never becomes ready. A passing run pays nothing for the
  wider bound. Interruption cleanup, which is what the test proves, keeps its own bounds: 10 s to
  join and 2 s per descendant retirement.
- `TestAFUV0NoDescendantsReadinessDiagnostics` covers four cases without Node: the observer
  returned, the grandchild spawn failed, the child exited, and the bound elapsed with an
  unparseable `undefined` pid. It also covers the ready path. The test is added to the AFU-V0-010
  traceability row.

The test's guarantee is unchanged. After readiness, it cancels the observation, requires an error,
and requires both descendants to be absent or zombie within 2 s.

## Measurements (macOS arm64, 12 cores, host load about 15, plus 24 `yes` burners, `-race -cpu 1`)

| Run | Result |
| --- | --- |
| BASE test, `-count=100` | 0/100 failed, slowest 1.94 s |
| Scratch copy of the BASE readiness wait, with per-file timing, `-count=60` | 0/60 failed, slowest grandchild readiness about 1 s |
| Changed tests, `-count=100` | 0/100 failed, slowest 3.36 s |
| Fault: grandchild spawn of `/nonexistent/node`, BASE test | FAIL after 10.79 s, `observer descendants never started` (the hosted signature) |
| The same fault, changed test | FAIL after 1.11 s, `observer descendants failed before ready (grandchild.error "Error: spawn /nonexistent/node ENOENT"; observer still running): child.pid N, grandchild.pid missing` |
| Fault: grandchild spawned `detached:true` (its own session escapes the owned group), changed test | FAIL, `descendant N survived interruption (state Ss)`: the guarantee still fails a survivor |

The fault injections were temporary edits that were reverted before the commit.

## Evidence

- Focused: `go test -race -count=3 -run TestAFUV0NoDescendants ./internal/appflows/` passed. The
  whole `./internal/appflows/` package passed. `go vet ./internal/appflows/` and `gofmt -l` were
  clean.
- Repository guards selected by `corvint affected`: `./tools/fixture-maintenance-check/`,
  `./tools/corvint-pr-tests/` and `./internal/diagnostic/` passed.

## NOT_RUN

- A Linux reproduction. No local Linux image with Node was available without downloading packages,
  so the hosted mode remains NOT_REPRODUCED.
- The 43 `UNBOUNDED_READER` units and the remaining `DECLARED_READ_SCOPE` and
  `PATH_LITERAL_READER` units that `corvint affected` selected because one `_test.go` file changed
  (for example `cmd/corvint` and `internal/tasks`). The change touches no file they read
  semantically. The CI shards run them.
- `make gate`. The owner's standing preference is focused tests.

## Rollback

Revert the commit. The BASE test and the spec row return unchanged. No corpus, output or identity
depends on this test file.
