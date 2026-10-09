# 2026-10-08: Batch L integration (V1-0373, V1-0652, V1-0349, V1-0734)

## Intent

Integrate three finished, independently reviewed lanes as one local batch branch,
`claude/batch-l-2026-10-08`, based on `origin/main` `c8734be3` (PR #691, batch I). Each lane is
merged with `git merge --no-ff`, so its reviewed commits stay intact. V1-0373 and V1-0734 both
rewire `internal/groupreap` and the same Git spawners, so the batch also carries one groupreap
design that keeps both lanes' guarantees:

- every group signal is sent before the leader is reaped (V1-0373, `PGO-V0-008`, proposed), and
  Darwin `waitid` stop/continue reports are not exits;
- a live child group is recorded from its start until its release, made while the leader is still
  unreaped, and `KillLive` kills it at adapter exit (V1-0734, `AHI-048`, proposed);
- `internal/contextindex` keeps its pipe-drain behaviour.

`PGO-V0-008` and `AHI-048` stay proposed; acceptance is human-owned.

## Merges

| Order | Ticket | Lane head | Merge commit | Conflicts |
| --- | --- | --- | --- | --- |
| 1 | V1-0373 + V1-0652 pre-reap group signal | `b7673c38` | `a29f6b7c` | `internal/contextindex/analyzer_schema_test.go` (audit pin) |
| 2 | V1-0349 promisor read coverage | `a3f13679` | `dcba9f80` | none |
| 3 | V1-0734 exit kills live child groups | `68147c39` | `c0b23006` | `internal/groupreap/groupreap_unix.go`, `groupreap_other.go`, `waitid.go`, `internal/contextindex/git.go`, `analyzer_schema_test.go`, `docs/specs/core-compatibility-freeze-v1.md` (line citations) |
| 4 | reconciliation: analyzer digest, citation repins | - | `73323682` | - |
| 5 | reconciliation: `Contain` on a plain command | - | `55cd68b9` | - |

V1-0373's lane was based on `b5616037` (batch H); the other two on `c8734be3`.

## Integration decisions

- One `Contain` (`groupreap_unix.go`): Setpgid, and for a command made by `exec.CommandContext`
  a `Cancel` that calls `Stop`. V1-0734's `Contain` (Setpgid only where groups are recorded) is
  removed. A plain `exec.Command` keeps a nil `Cancel`, because `exec.Cmd.Start` refuses a
  non-nil `Cancel` without a context; V1-0734's exit-kill helper contains such a command
  (`TestContainKeepsPlainCommandStartable`). Elsewhere (Windows) `Contain` is a no-op, so
  `internal/gitstatus` still builds there.
- `WaitPipes` is removed; `Drain` replaces it. `Drain` starts through the live registry (both its
  group path and its plain path, so a start after `KillLive` is refused with `ErrExiting`), keeps
  the group recorded through the pipe drain and the pre-reap sweep, and releases it immediately
  before `command.Wait`. A failed exit observation releases at once and sends no group signal.
  This also closes the V1-0734 review's P2 that a `contextindex` pipe holder was unrecorded while
  `WaitPipes` drained. `contextindex` keeps V1-0373's `startDrained` path and has no post-reap
  `terminateProcessGroup`. `TestAHI048WaitPipesReleasesWithoutKillingTheGroup` becomes
  `TestAHI048DrainKeepsGroupRecordedUntilReap`: it asserts the group is still recorded at the
  pre-reap sweep, released after the reap, swept while the leader is unreaped, and that the pipe
  holder is reported as `exec.ErrWaitDelay` and killed. The old "rest of the group is not killed"
  assertion described `WaitPipes` leaving the kill to `contextindex`'s post-reap
  `terminateProcessGroup`, which `PGO-V0-008` removes; the merged code kills it before the reap.
- `liveTracking` (V1-0734) and `waitidAvailable` (V1-0373) named the same platform fact; only
  `waitidAvailable` remains. `wait` releases a recorded group after its pre-reap sweep (or after a
  failed observation) and before the reap; `Owner` keeps V1-0734's release before its reap.
- V1-0734's out-of-scope finding 1 (an exec `Cancel` that signals the group after the reap) is
  the case V1-0373 fixes: every group-kill `Cancel` is now `Stop`, which kills only the leader
  through `os.Process` on Darwin and Linux. `KillLive` signals only recorded groups, under the
  exclusive gate that every release shares, so it never names a reaped leader's group except
  after a foreign reaper (ECHILD), which neither lane could prevent.
- `AHI-048` text and its traceability row now name `Drain` instead of `WaitPipes`, and the
  failure mode about an unrecorded pipe holder is replaced by the foreign-reap release.
- Analyzer schema stays `corvint-analyzer/113` (V1-0373); the audit digest is recomputed on the
  merged sources (`b3e217f1...`).
- Line citations into `internal/contextindex/git.go` were repinned by content, so every anchor
  hash is unchanged (decisions 0158 and 0210, `core-compatibility-freeze-v1.md`,
  `falsifiable-packet-v0.md`). The unhashed `FRONTIER-DECISION-BRIEF` citation `git.go:408`
  already named the wrong line on main; it now pins the `ls-tree ... identity.treeRevision` line
  it describes (`git.go:450@33f5b343`). `REQUIREMENTS.tsv` was regenerated with no change.

## Evidence

All on darwin/arm64 at `55cd68b9`, `GOTOOLCHAIN=local`, `GOMAXPROCS=3 go test -p 1 -count=1
-timeout 30m`:

- `gofmt -l` on touched Go files: empty. `go build ./...`: rc 0. `go vet` on the touched packages,
  and `GOOS=linux` / `GOOS=windows` vet of `groupreap`, `contextindex`, `gokernel`, `cem/gitrun`,
  `gitstatus`, `worksource` and `cmd/corvint`: rc 0.
- Full packages, all `ok`: `groupreap`, `contextindex`, `worksource`, `gokernel`, `cem/gitauth`,
  `cem/gitrun`, `mcp/bridge`, `gitstatus`, `releasegate`, `liveverify/mutate`, `doccompiler`,
  `taskman`, `criterionexperiment`, `testrunner`. `groupreap` under `-race`: `ok`.
- `cmd/corvint` with `-run 'TestAHI044HookAdaptersFailOpen$|TestAHI048|Promisor|TestCore|TestMapCore|TestCEM|TestOCM|Completion'`:
  `ok`. Before `55cd68b9` the same subset failed in `TestAHI048ExitProcessKillsLiveChildGroups`
  (the helper's plain `exec.Command` was refused by `Start` once `Contain` set a `Cancel`).
- Doc gates (`spec-requirements` through `diagnostic-coverage-check`) and
  `go test ./internal/specindex`: pass.
- Independent read-only review (Codex `gpt-6-astra`) of the `c0b23006` resolution, `73323682`
  and `55cd68b9`: no P0-P3 findings. It noted that a foreign reaper can reap a recorded leader
  before `KillLive`, a window already present at `68147c39` and not introduced here.

`NOT_RUN`: Linux and Windows runtime execution (vet only), the full `go test ./...`, `make gate`,
and dogfood/CEM binding for the batch.

## Rollback

Revert the reconciliation commits `55cd68b9` and `73323682`, then the merge commits in reverse
order (`c0b23006`, `dcba9f80`, `a29f6b7c`) with `git revert -m 1`. No stored state, wire format or
pack encoding changes beyond the analyzer schema ID (`/113`), which a revert returns to `/112`.

## Merge with main a4acc6f6 (after batch K2)

Batch K2 re-pinned the analyzer audit digest at `corvint-analyzer/112`, and this batch had bumped the
schema to the unreleased `corvint-analyzer/113`. The merge keeps `/113` and pins the merged input
digest `e6432008…`; `TestAnalyzerSchemaInputs` and the full `internal/contextindex` package pass.
The `core-compatibility-freeze-v1.md` register takes main's rows (including the K2
`governance_refused` rows), with the `git.go` citation moved to line 437 for this batch's edit.
`line-citations-check` and the other doc gates pass.
