# V1-1969: OpenCode native qualification latency re-sample

Issue 1218 reports that the OpenCode native qualification campaign failed 14 of about 45 tries on
the `latency` check alone (lifecycle p95 260 to 299 ms against the 250 ms AHI-012 limit), with
elevated early samples that then dropped to the passing baseline, and that the failure line did not
name the failed check. `AHI-056` in `docs/specs/agent-harness-integration-v0.md` (accepted under the
owner's standing auto-accept of 2026-10-11) amends `AHI-032`.

Decisions:

- Keep the absolute AHI-012 limits and minimum sample counts. Widening the limit or adding a
  `PASS_WITH_LATENCY_WARNING` result would weaken the gate for real regressions; a same-run host
  baseline would need a new probe callback and still measures a different path.
- When the first timing campaign is complete, delivered and fault-free but misses only a p95 limit,
  run exactly one more timing campaign in the same run; the re-sample decides. A steady regression
  misses both and fails. Short, undelivered or faulted samples are never re-sampled.
- Retain every attempt under `metrics.latencyAttempts` and the method under
  `metrics.latencyMethod`. `metrics.query`/`metrics.lifecycle` stay the decisive attempt, so the
  consumer record check in `record.go` re-derives the same verdict; it now shares the limit
  constants.
- The campaign error names each failed check, sorted, and for `latency` every attempt's measured p95
  against its limit.

Evidence: `TestAHI056LatencyResample` (loaded-host sample re-sampled and passes, steady regression
fails both attempts, short sample not re-sampled, exact failure message). A live OpenCode campaign
on this change is `NOT_RUN`; the measured flake rate after the change is `NOT_OBSERVED`.

Rollback: revert the re-sample so the first timing campaign decides alone.
