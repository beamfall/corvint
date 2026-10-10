# 2026-10-10: V1-0397 upgrade SessionStart envelope, bounded non-reproduction (RC3 known limit)

## Intent

V1-0397: on 2026-09-27 the Claude Code HLQ upgrade case failed once with `SessionStart:
additionalContext carries no repository-data envelope`. The tuple was Claude Code 2.1.267, adapter
0.2.3 and rc.1 build 154, at host load 81–109 on 12 CPUs. The received text was discarded.
Acceptance criteria 1 and 3 were met by 79b2ef0e (PR #321): the received context is now reported,
and three runs above load 80 passed. Criterion 2 is still open: identify and fix the cause, or record
it with its host-load bound. This entry records an RC3 attempt to reproduce the failure.

## Method

- Source: `origin/main` `64b5d1745b158edf10203257951375d005037560`. Plugin sources came from a
  clean worktree at that commit, adapter 0.3.1.
- Current binary: built from that source, not installed to PATH, `Corvint 1.0.0-rc.3 (build 0)`,
  SHA-256 `4f61eec0bdc469fb97755b6219a752e6042b345da43e79df692dc3707d533098`.
- N-1 binary: the published rc.2 binary, `Corvint 1.0.0-rc.2 (build 360)`, SHA-256
  `a7293de93d52e9e6296ab2eb42ac1b832901df6f0440f61118e84520fdee3e86`.
- Runner: built from the same source, SHA-256
  `13fdcaf8f77a8b26fb654129d65290a5a2e2a4900b68723910954b45773bb291`.
- Host: Claude Code 2.1.293 (the operator's install, unchanged), darwin/arm64, Darwin 25.6.0,
  12 CPUs. Each run uses a private `HOME` and `CLAUDE_CONFIG_DIR` (`HLQ-V1-003`).

Command, run in two concurrent loops of 100:

```sh
TMPDIR=/private/tmp/claude-501/gotmp ./host-lifecycle-v1 --host claude-code \
  --corvint corvint --base-corvint corvint-n1 --source CLEAN_CHECKOUT --report REPORT
```

Load was manufactured by eight `yes >/dev/null` processes at normal priority, plus ambient load
from concurrent RC3 lanes. The one-minute load average was sampled only at each run's start and
end. Continuous load was NOT_OBSERVED.

For each run the loop recorded the exit code, the upgrade case status, and the number of
`SessionStart attempt N` time-bound retries named on the upgrade case line. `HLQ-V1-009` names
every time-bound retry whether the case passes or fails. Any other degradation fails the case. A
zero retry count therefore means neither SessionStart call in the upgrade case degraded.

## Result

- 200 runs, 17:33:48–18:01:39 UTC: the upgrade case passed in all 200, with 0 SessionStart
  time-bound retries.
- Start loads: p10 66.8, median 121.9, p90 202.5, max 258.7.
- 134 runs had a boundary sample of 100 or more, and 27 had one of 200 or more. In 39 runs the
  higher boundary sample fell inside the historical 81–109 window.
- Mean run time was 16.6 s and the longest was 31 s.
- An earlier, aborted burst used 24 burners and three loops, at boundary loads of 122–191. It
  reached the upgrade case 9 times, and all 9 passed with no SessionStart retry.
- Focused tests: `go test -race -count=50 -timeout 30m ./conformance/host-lifecycle-v1` PASS
  (182 s).
- Also PASS: `go test -race -count=20 -timeout 30m -run
  'TestAHI017AdapterHostKillMatchesDeclaredHooks|TestAHI017HostAdapterWatchdogDegradesBeforeHostKill|TestClaudeAdapterForkSessionStartIsResume|TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths|TestDogfoodEventDeadlineBelowDeclaredHostKill|TestAHI047GuidanceIsMainThreadSessionStartOnly'
  ./cmd/corvint`.
- An initial `-count=200` run of the conformance package hit Go's default 10-minute package
  timeout. That timeout bounds total package time, and no test failed. The run was repeated with
  the repository's `-timeout 30m` and a smaller count.

Other cases failed in 18 of the 200 runs, none of them the upgrade case:

- 11 frontier failures: the enrolled Stop failed open on `dogfood-event-deadline` in all three
  attempts. This is the known `V1-0844` shape, observed here at high load.
- 7 uninstall failures. Their reports were not retained, because the loop kept only reports with
  an upgrade retry or failure. They are outside V1-0397.

## Finding and known limit

The V1-0397 failure did not reproduce in 200 runs at loads up to 258.7. No code changed: a fix
without a reproduction would be speculative. The historical cause remains UNKNOWN. Watchdog or
event-deadline degradation of SessionStart remains a hypothesis. It is now retried and named under
`HLQ-V1-009`, and none occurred in this sample.

The sample is bounded and does not prove absence. Zero failures in 200 runs puts the failure rate
below about 1.5% per run at 95% confidence (rule of three), on this host, tuple and source. It does
not cover the historical tuple (Claude Code 2.1.267, adapter 0.2.3, rc.1 build 154), which was not
rerun.

RC3 known limit: the 2026-09-27 upgrade-case missing envelope is unexplained and not reproduced on
the RC3 source. V1-0397 stays open for the orchestrator.
