## 2026-09-26 V1-0373 V1-0391 V1-0362 V1-0349 V1-0361: Git runners signal their group before reaping, run isolated, and never fetch a promisor object

Base `b29d35e53a535723624c692aa98b00692a6a8d04`.

### V1-0373: one pre-reap group kill

`internal/groupreap` now owns the order that `gokernel.waitGroupLeader` used. On darwin and linux it
calls `waitid(P_PID, pid, WEXITED|WNOWAIT)` and then `kill(-pid, SIGKILL)` while the leader is still an
unreaped zombie, so the group ID cannot name a process that reused the leader's PID. Only after that
does it call `cmd.Wait`. On other platforms, or when `waitid` fails, it waits first and signals after.

Adopters:
- `gokernel`
- `doccompiler`
- `liveverify/mutate` (`shared.go` and `gotest.go`)
- `taskman`
- `cem/gitrun` (`gitrun.go` and `session.go`)
- `releasegate`

Each adopter's old post-reap kill is removed: `terminateProcessGroup`, `killDescendants`, and the
unconditional kill that `releasegate` ran after its select. Timeout, cancellation and error
classification are unchanged, because each runner still classifies from its context and from the
`Wait` error. The proving test is `TestGroupIsSignalledBeforeLeaderIsReaped`.

One behavior changes in those runners. When the leader has exited but a descendant in its group still
holds stdout or stderr, that descendant is now killed before `Wait`. The runner therefore returns the
leader's status and the captured output, where it used to return `ErrWaitDelay` or time out. No spec or
test encodes the old outcome, and the new one matches `gokernel`.

Two runners are excluded: `worksource/git.go` and `contextindex/git.go`. Their contracts classify a
pipe-holder that outlives the leader as incomplete capture (`ErrWaitDelay`): `VPO-V0-010` and
`WQO-V0-005`, `WQO-V0-015` and `WQO-V0-032` for worksource, and
`TestBuildWithGitExecutionOwnsPipesAndCancellation` for contextindex. A pre-reap kill closes that pipe
and turns it into accepted output. With the kill in place, `TestWorkSourceGitBoundsAndPipeOwnership`
failed: "pipe-holder was accepted as complete". Keeping that classification would require draining the
pipes before the kill, over pipes the runner owns, which is what `procgroup` does. Moving these two
runners onto `procgroup` is a separate decision for the owner.

With host load between 390 and 500, the descendant-cleanup tests in `gitrun`, `doccompiler` and
`taskman` failed identically at the base and with this change, on PID-handoff timeouts. Run on their
own, they pass with this change, as do the whole `mutate`, `groupreap`, `doccompiler` and
`releasegate` packages.

### V1-0391: pipe-drain bounds outside Git acquisition

V1-0390 raised the `WaitDelay` of `contextindex` and `gokernel` from one second to one minute. The same
one-second value remained in `doccompiler`, `taskman`, `liveverify/mutate` and the work executable
version probe. `os/exec` starts that timer when the process exits. A successful subprocess therefore
failed with `ErrWaitDelay` whenever the goroutine copying its output was not scheduled within one
second, which a loaded host does. All four now use one minute. The bound still detects a pipe held by
a descendant that escaped the group kill; it is not a latency budget.

A scratch check ran `taskman` `runRead` on a script that prints `ok`, then starts a `setsid` descendant
that holds stdout for two seconds, and exits 0. With one minute, it returned `ok` after 2.2 seconds.
With the bound set back to one second, it failed after 1.2 seconds. The existing cancellation,
descendant and group tests of `doccompiler`, `taskman` and `mutate` pass, because cancellation kills
the group and so closes the pipes without waiting out the bound.

### V1-0362: isolated Git runners

These runners now use Core's `gokernel.SanitizedGitEnvironment` with `-c credential.helper=`:
- `dogfoodflow` `flow.git`
- `lspprovider` `rootCommit`
- `releasecandidate` `runSourceGit`
- `tasks/store` `gitOutput`

`tasks/store` restates that environment inline, because Tasks imports no Core package (decision 0397,
`internal/tasks/boundary_test.go`).

`dogfoodflow` `gitPassthrough` deliberately keeps the caller's environment. It runs the seal's `git mv`
and `git commit`, which take author identity, signing and hooks from the user's configuration. This is
recorded in `DCW-V0-015`.

Global configuration is now isolated, so the user's `core.excludesFile` no longer applies to these
reads. That matches Core, whose `gitRaw` already passes `-c core.excludesFile=`.

`touchsurprise` `requireCleanWorktree` now reads status through `gitstatus.Status`, which works on a
private metadata copy and refuses filters. A clean or process filter configured after the loader
finished can therefore never run. That refusal is `unsupported-surprise-git`, recorded in `TSS-V0-003`.

Every changed runner has a test that fails when its change is reverted.

### V1-0349: no promisor fetch

Fixture: a `blob:none --sparse` clone of a local repository with `uploadpack.allowFilter` set. The
clone's upload-pack is `touch sentinel && git-upload-pack`, so any fetch attempt leaves the sentinel.
The source repository is removed.

Results on Git 2.54:

| Environment | `ls-tree -l` | `cat-file blob` | Sentinel |
|---|---|---|---|
| `GIT_NO_LAZY_FETCH=1` (the flows environment before this change) | prints `BAD` as the missing intent blob's size | fails with `bad file` | none |
| Neither guard | fetches the blob | fetches the blob | present |
| Only an empty `GIT_ALLOW_PROTOCOL` | fails with "transport 'file' not allowed" | fails with "transport 'file' not allowed" | none |

So `corvint.flows.*` does not lazily fetch on a Git that honours `GIT_NO_LAZY_FETCH`. On a Git that
ignores it, `flowTree`'s `ls-tree -l` would fetch. That second case is the one the Core freeze tests
model: Git before 2.46 ignores the variable for a diff's blob prefetch.

Changes:
- An empty `GIT_ALLOW_PROTOCOL` is added once, to `SanitizedGitEnvironment`. Every caller of it is a
  read. `lspprovider` and `dogfoodflow` drop the appends that did the same thing.
- A `BAD` size used to be refused as "flow input exceeds byte limit". It is now refused as
  "committed flow input unavailable".
- MCP still maps the refusal to `flows-refused`. No new error code is added; a promisor-specific MCP
  code would change the wire contract.

Core CLI reads were already covered by `TestIndexedCoreVerbsCodeAPromisorObjectWithoutFetching` and
`TestIndexedCoreVerbsRefuseAPromisorFetchGitStartsAnyway`, which expect `repository-object-unavailable`.

New tests:
- `TestSanitizedGitEnvironmentRefusesAPromisorFetch` is deterministic. It passes with the change, and
  fails when `GIT_ALLOW_PROTOCOL` is removed, because the sentinel is written.
- `TestAFUV1034CommittedFlowsReadNeverFetchesAPromisorObject` is an end-to-end regression for a
  normally loaded host.
- `TestMCPReadsRefuseAMissingPromisorObjectWithoutFetching` is an end-to-end regression for a
  normally loaded host.

The two end-to-end tests do not catch the missing guard at this load. The flows Git timeout is
10 seconds, and the unguarded `ls-tree` alone took 27 seconds in a shell. The MCP probe likewise hits
its own 10-second deadline before any flows read runs. Both tests therefore pass with the guard
removed.

### V1-0361: stale temporaries

`isStoreTemporary` also matches `.gitignore-*.tmp`. `evictSnapshotsAt` also sweeps stale `blob-*.tmp`
files inside blob shard subdirectories. This is recorded in the `IDX-SNAP-V0-007` amendment, and the
proving test is `TestEvictSnapshotsRemovesStaleShardAndIgnoreTemporaries`.

### Analyzer schema

The `contextindex` source changes move the schema from `corvint-analyzer/87` to `corvint-analyzer/88`,
and the audited-input digest is now `88b1407c4fd0e82ddd197e047f80be3592905b263e29bfc444ccf16567d735e0`
(`IDX-SNAP-V0-017`). Snapshots rebuild once, and extraction is unchanged.
