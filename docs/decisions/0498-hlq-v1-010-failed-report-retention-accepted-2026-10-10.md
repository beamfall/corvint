# Decision 0498 — Failed-report retention (HLQ-V1-010) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept both", answering the request to accept HLQ-V1-010 and CAL-V0-211).

## Context

V1-1121 reported that a repeated host lifecycle qualification run kept no failure text when the
case its caller studied passed, so the cause of a failure in another case was lost. The HLQ load
lane (V1-1119, V1-1120, V1-1121) proposed `HLQ-V1-010` in
`docs/specs/host-lifecycle-qualification-v1.md`. AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts `HLQ-V1-010` as written:

- With `--failed-reports DIR`, a run in which any case is not `PASS`, or any case line names a
  time-bound hook retry (`HLQ-V1-009`), also writes its report to a new file in `DIR` named for the
  host, the UTC time and the process id, and never replaces an existing file there. A run whose
  nine cases all pass with no retry writes nothing there.
- The retained copy is the `HLQ-V1-007` report, at most 64 KiB including an explicit omitted-byte
  marker, written with mode 0600, holding nothing read from outside the private workspace.
- A `DIR` that cannot be created, or in which a file cannot be created, is a setup error (exit 2)
  before any case runs. A failed `--report` write does not prevent the retained copy, and either
  failure exits 2.

## Limits

This decision settles intent only. The evidence is `TestFailedReportRetention` under `-race` and
the load runs recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-hlq-load-failures.md`. The V1-1119 and
V1-1120 dispositions and the spec's Known-gaps notes are descriptive and are not settled here.
The full suite, `make gate` and a Linux run are `NOT_RUN`.

## Rollback

Revert this decision and return the `HLQ-V1-010` status text to proposed. To withdraw the behavior,
also revert the V1-1121 runner change in `conformance/host-lifecycle-v1` (`retainReport`,
`keepReport` and the `--failed-reports` flag), then regenerate `docs/specs/REQUIREMENTS.tsv`.
Retained report files are operator-owned and are not removed.
