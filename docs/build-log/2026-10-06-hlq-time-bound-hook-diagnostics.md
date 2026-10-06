# 2026-10-06: HLQ hook diagnostics and time-bound retry (V1-0844, V1-0397)

## Intent

V1-0844: the `1.0.0-rc.2` candidate `da78c157` (build 358) Claude Code HLQ run reported
`case frontier FAIL enrolled incomplete Stop returned decision <nil>` at load average about 245 on
12 CPUs; three reruns passed, and the runner had discarded the Stop hook output. V1-0397: the
upgrade case's intermittent missing envelope; its received-context diagnostic was already
delivered by 79b2ef0e (PR #321), and only root-cause qualification remained. Contract:
`HLQ-V1-009` in `docs/specs/host-lifecycle-qualification-v1.md`.

## Change

- A failed case line names the case's last hook invocation: event, exit code, elapsed wall time,
  degradation code or `none`, and stdout and stderr each quoted to at most 512 bytes.
- A hook output whose degradation frame names a time bound (`dogfood-event-deadline`,
  `dogfood-event-index-snapshot-stale`, `adapter-host-kill-deadline`) is rerun, at most three runs,
  and every such run is named in the case line on PASS and FAIL. Exhausting the runs FAILs the case
  naming the code. Other degradations are not retried.
- Nothing in the product changed. No deadline is widened; the enrolled incomplete Stop still passes
  only on `decision=block`.

## Finding

Proven from code and `TestClaudeAdapterStopDeadlineFailsOpenVisibly`: an enrolled Stop whose
dogfood event deadline expires returns only a `systemMessage` naming
`corvint-event-rejected:dogfood-event-deadline`, with no `decision`. The previous runner reported
that as exactly `decision <nil>`. The adapter work bound is process start + 2 s declared kill −
400 ms reserve − 100 ms grace; within it the Stop read runs two repository probes and two
local-completion evaluations, with no index build. The watchdog path returns the same shape naming
`adapter-host-kill-deadline`.

Hypothesis, not observed: the rc.2 frontier failure was that time-bound fail-open. It fits the
evidence (identical product source passed on candidate 1, reruns passed, and the other no-decision
outputs have no evident load dependence), but the output was discarded. For V1-0397, the plugin
fixture has no index snapshot, so every SessionStart builds in memory inside the same bound, and
an expiry is named `dogfood-event-index-snapshot-stale` or `adapter-host-kill-deadline`. The
historical upgrade failure's cause remains UNKNOWN.

Retrying instead of widening: the deadline is tied to the host's declared 2 s kill, so widening it
would only move the failure to the host. Hook events are reads, so a rerun is safe. A pass after a
retried time bound qualifies lifecycle semantics, not latency (`HLQ-V1-004`).

## Evidence

- `TestHookTimeBoundDegradation`, `TestDegradationCode`: fake-host frontier runs that fail open
  once (PASS, retry named), on every run (FAIL naming the code and the hook streams, no
  `decision <nil>`), and with a non-time-bound code (not retried, named).
- Real Claude Code HLQ qualification under `HLQ-V1-009`: `NOT_RUN` (release gate running on the
  host).

## Rollback

Revert this change's commit: the runner, its tests, the `cmd/corvint` test, the spec requirement and
the regenerated `REQUIREMENTS.tsv`. No state needs repair.
