# 2026-10-10: V1-0397 upgrade SessionStart envelope, bounded non-reproduction (RC3 known limit)

## Intent

V1-0397: on 2026-09-27 the Claude Code HLQ upgrade case failed once with `SessionStart:
additionalContext carries no repository-data envelope`. The tuple was Claude Code 2.1.267, adapter
0.2.3 and rc.1 build 154, at host load 81–109 on 12 CPUs. The received text was discarded.
Acceptance criteria 1 and 3 were met by 79b2ef0e (PR #321): the received context is now reported,
and three runs above load 80 passed. Criterion 2 is still open: identify and fix the cause, or record
it with its host-load bound. This entry records an RC3 attempt to reproduce the failure.

## Method

Source: `origin/main` `64b5d1745b158edf10203257951375d005037560`. `SOURCE` below is a clean worktree
at that commit, which also supplies the plugin package (adapter 0.3.1).

Host:

- Apple M2 Max, 12 CPUs, darwin/arm64, Darwin 25.6.0 (macOS 26.6.2).
- Claude Code 2.1.293, the operator's existing install, unchanged.
- Go 1.27.1 (`GOTOOLCHAIN=local`).
- Each run uses a private `HOME` and `CLAUDE_CONFIG_DIR` (`HLQ-V1-003`).

Binaries, all in one work directory `W`:

```sh
cd "$SOURCE"
GOTOOLCHAIN=local GOMAXPROCS=3 go build -o "$W/corvint" ./cmd/corvint
GOTOOLCHAIN=local GOMAXPROCS=3 go build -o "$W/hlq" ./conformance/host-lifecycle-v1
cp "$RC2_BUILD360" "$W/corvint-n1"   # the operator's retained rc.2 build 360 binary
```

| Binary | Version | SHA-256 |
|---|---|---|
| `corvint` | `Corvint 1.0.0-rc.3 (build 0)` | `4f61eec0bdc469fb97755b6219a752e6042b345da43e79df692dc3707d533098` |
| `corvint-n1` | `Corvint 1.0.0-rc.2 (build 360)` | `a7293de93d52e9e6296ab2eb42ac1b832901df6f0440f61118e84520fdee3e86` |
| `hlq` | runner | `13fdcaf8f77a8b26fb654129d65290a5a2e2a4900b68723910954b45773bb291` |

The `corvint` binary was not installed to PATH.

The two scripts that ran are retained under `evidence/v1-0397/`:

- `loop.sh` runs the runner N times. Each run's command is
  `"$W/hlq" --host claude-code --corvint "$W/corvint" --base-corvint "$W/corvint-n1" --source
  "$SOURCE" --report "$W/rep-LANE-I.tsv"`.
- `burn.sh` starts K `yes >/dev/null` processes at normal priority and reaps them on exit.

Retained SHA-256: `loop.sh`
`41d60cb46f10f466989103c3e9018e2f28561f223fcd06d3f6e4b6cccb67c5a0`, `burn.sh`
`f833c8f8e66501a4f1f625183f6b12bd1433e0f322d94d463be66ee970ce6f8c`. The retained copies replace the session's absolute work, source and `TMPDIR` paths with the
variables `W`, `SOURCE` and `TMPDIR`. Nothing else differs. The run was:

```sh
export W=... SOURCE=...
./burn.sh 8 3600 & ./loop.sh a 100 & ./loop.sh b 100 & wait
```

Load came from the eight burners, the two concurrent loops, and ambient work from concurrent RC3
lanes. The one-minute load average was sampled only at each run's start and end. Continuous load
was NOT_OBSERVED.

For each run, `loop.sh` writes one row to `evidence/v1-0397/runs.tsv` (SHA-256
`c208e106e1f9bdc7a2f4f16356676748ae2c48c536e476821f92e291153f6bdd`, header added). The row records:

- the exit code;
- the upgrade case status;
- the count of `SessionStart attempt N` time-bound retries on the upgrade case line;
- any other failed cases.

A passing case line still names every time-bound retry (`HLQ-V1-009`; `step` in
`conformance/host-lifecycle-v1/main.go`). A non-time-bound fallback output carries no envelope, so
it fails the upgrade case's predicate. A zero retry count on a passing upgrade line therefore means
neither SessionStart call in the case degraded.

`loop.sh` kept a full runner report only when the upgrade case failed or retried. Every other
report was deleted, so no runner reports are retained. The table is the record.

## Result

From `evidence/v1-0397/runs.tsv`:

- 200 runs, 17:33:48–18:01:38 UTC: the upgrade case passed in all 200, with 0 SessionStart
  time-bound retries.
- Start loads (linear-interpolated quantiles): min 52.0, p10 67.3, median 122.1, p90 202.5,
  max 258.7.
- 134 runs had a boundary sample of 100 or more, and 27 had one of 200 or more. In 39 runs the
  higher boundary sample fell inside the historical 81–109 window.
- Run times, from the start and end columns: mean 16.6 s, longest 31 s.

Other cases failed in 18 runs, none of them the upgrade case:

- 11 frontier failures (`otherFailedCases`).
- 7 uninstall failures.

Their reports were deleted under the retention rule above, so their text is unretained. In the
aborted burst below, every frontier failure text read showed the enrolled Stop failing open on
`dogfood-event-deadline` in all three attempts, the known `V1-0844` shape (unretained observation).
That the 11 frontier failures here share it is an inference. The cause of the 7 uninstall failures
was not observed.

Unretained observation: the aborted burst used 24 burners and three loops, at boundary loads of
122–191. It reached the upgrade case 9 times, and all 9 passed with no SessionStart retry. Its rows
are not in the table.

Focused tests, at the same source, during the loop (unretained observations; their output was not kept):

- PASS: `go test -race -count=50 -timeout 30m ./conformance/host-lifecycle-v1` (182 s).
- PASS: `go test -race -count=20 -timeout 30m -run
  'TestAHI017AdapterHostKillMatchesDeclaredHooks|TestAHI017HostAdapterWatchdogDegradesBeforeHostKill|TestClaudeAdapterForkSessionStartIsResume|TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths|TestDogfoodEventDeadlineBelowDeclaredHostKill|TestAHI047GuidanceIsMainThreadSessionStartOnly'
  ./cmd/corvint`.
- An initial `-count=200` run of the conformance package hit Go's default 10-minute package
  timeout. That timeout bounds total package time, and no test failed. The run was repeated with
  the repository's `-timeout 30m` and a smaller count.

## Finding and known limit

The V1-0397 failure did not reproduce in 200 runs at loads up to 258.7. No code changed: a fix
without a reproduction would be speculative. The historical cause remains UNKNOWN. Watchdog or
event-deadline degradation of SessionStart remains a hypothesis. It is now retried and named under
`HLQ-V1-009`, and none occurred in this sample.

The sample is bounded and does not prove absence. If the runs were independent trials with one
common failure probability, zero failures in 200 would bound that probability below about 1.5% at
95% one-sided confidence (exact binomial 1.49%, rule of three 1.5%). This experiment does not
establish those assumptions: runs were concurrent, shared one host, and saw widely varying load.
The figure is only an order-of-magnitude guide, not a measured rate. It also does not cover the
historical tuple (Claude Code 2.1.267, adapter 0.2.3, rc.1 build 154), which was not rerun.

RC3 known limit: the 2026-09-27 upgrade-case missing envelope is unexplained and not reproduced on
the RC3 source. V1-0397 stays open for the orchestrator.
