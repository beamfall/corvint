# 2026-10-10: HLQ failures under heavy host load (V1-1119, V1-1120) and failed-report retention (V1-1121)

## Intent

RC3 lane E (PR beamfall/corvint#741, `2026-10-10-v1-0397-upgrade-sessionstart-nonrepro`) ran the
Claude Code HLQ tuple 200 times at load1 52–259. The `uninstall` case failed 7 times (V1-1119) and
the `frontier` case failed open 11 times (V1-1120). Its loop deleted the report of every run whose
target case passed, so no failure text was retained (V1-1121). Contracts: `HLQ-V1-002`,
`HLQ-V1-007`, `HLQ-V1-009`, `LCP-V0-008`, `AHI-017`. No owner acceptance was given in this lane, so
every requirement text below is proposed.

## V1-1121: retention keyed on every case

The runner gains `--failed-reports DIR`, specified by the proposed `HLQ-V1-010`. A run with any
non-`PASS` case, or any time-bound hook retry, writes a new copy of its report into `DIR`. The file
is named for the host, the UTC time and the pid. It is opened with `O_EXCL` and mode 0600. It is
at most 64 KiB, including the explicit omitted-byte marker. The report text is already the bounded,
quoted `HLQ-V1-007` text, and it holds nothing from outside the private workspace.

A probe file proves `DIR` writable before any case runs. Retention is attempted even when the
`--report` write fails. `TestFailedReportRetention` pins all of this: the selection, the no-replace
naming, the content, the mode, the truncation bound, retention after a failed `--report` write, and
the setup error for a file or read-only `DIR`. It passes under `-race`. A mutation that drops the
retry criterion fails it. The lane G loops below used the retention.

## Load runs

The setup is darwin/arm64 with 12 CPUs, `corvint` 1.0.0-rc.3 (build 0, from `bf04e8a0`), N-1
1.0.0-rc.2 (build 360), adapter 0.3.1, and Claude Code 2.1.293. `yes` burners and two concurrent
HLQ loops ran per arm (`arm.sh` and `loop.sh`). Load is machine-wide; other lanes ran at the same
time. Runs are bucketed by the higher of the two boundary load1 values (`runs.tsv`):

| Arm | Burners | Runs | Max boundary load1 | frontier FAIL | uninstall FAIL |
|---|---|---|---|---|---|
| A | 8 | 12 | < 50 | 0 | 1 |
| B | 8 | 80 | 22–131 (34 under 50, 23 at 50–99, 23 at 100–149) | 0 | 0 |
| C | 16 | 3 | 150–199 | 2 | 0 |
| C | 16 | 17 | ≥ 200 | 14 | 2 |

Arm C was stopped at 20 of its 100-run cap once both shapes had recurred. Arm C used
`diag.patch`, a scratch runner patch that was never committed. It logs every exec with pid and
time. On residue, it also logs the writer pid, the file, how long the file persists, and the
`.claude` listing. The redacted results are in `uninstall-diag.txt`.

## V1-1120: frontier fail-open is the specified deadline, a known limit

All 16 arm C frontier FAIL lines are the V1-0844 shape: "failed open with time-bound degradation
corvint-event-rejected:dogfood-event-deadline on all 3 attempts". `frontier-attempts.tsv` extracts
every retried Stop attempt from the 20 reports that `--failed-reports` retained. All 58 attempts
are `dogfood-event-deadline`, at 1.53–2.07 s as the runner measures from spawn. That is
the visible fail-open that `LCP-V0-008` specifies on expiry. `runLocalCompletionEvent` bounds the
event by the nominal 1.6 s `dogfoodEventDeadline`, derived from the adapter's own context.
That context already expires at the adapter work bound, about 1.5 s from process start: the 2 s
declared host kill, less the 400 ms reserve and the 100 ms grace (`AHI-017`). The work bound
therefore governs, and both expiries surface as `dogfood-event-deadline`. Neither can be widened
without changing the hook timeout that the package declares to the host.

This is not a regression of V1-0844. That change fixed the runner's reporting and retry, and its
known gap predicted that a slowdown of roughly 5x would reach the bound.

Under load, the enrolled incomplete Stop does little CPU work but waits for about a dozen
sequential git subprocesses. An unretained probe of one enrolled Stop on this host measured about
0.37 s CPU against about 1.5 s wall time. Some of its snapshot calls repeat: `rev-parse`, `status`
and `ls-tree`.

Disposition: this is a known limit with a load envelope.

- With maximum boundary load1 below 150 on 12 CPUs, 0 of 92 runs failed open, and no run needed a
  retry.
- At 150–199, 2 of 3 runs failed.
- At 200 or more, 14 of 17 runs failed.
- Lane E saw 11 of 200 runs fail, at a median load of 122.

The product decision stays fail-open; no deadline was changed. Removing the duplicated git calls
would shift the envelope, but it is out of scope here and proposed as a separate ticket.

## V1-1119: uninstall residue is Claude Code's own `pluginUsage` bookkeeping

All three uninstall failures flagged content in Claude Code's config `.claude/.claude.json` (run
c1-5) or in its atomic-write temp file `.claude.json.tmp.<pid>.<hex>` (runs c2-6 and a1-1). For c1-5
and c2-6, the flagged content is the host's usage record
`"pluginUsage": {"corvint@corvint": {"usageCount": 0, "lastUsedAt": …}}`. Run a1-1 ran without the
diagnostic patch, so only its file name is retained. Corvint writes neither file.

Read from the Claude Code 2.1.293 bundle, not observed at run time: the host builds and deletes
`pluginUsage` entries itself, through a deferred config save that is also registered to run at exit
(`flushAtExit`). In run c2-6, `.claude.json` kept the mtime it got at install time.

In run c2-6, the temp file's writer pid 76364 is the `claude plugin enable` command. That process
had exited, and the file still existed 3 s later. In run c1-5, the record was committed to
`.claude.json` at the end of `enable`, and a `.claude.json.lock` was left beside it. The following
`uninstall` never rewrote the file. That this lock blocked the uninstall's cleanup is an inference,
not an observation. Run a1-1 had the same temp-file shape, without the diagnostic patch.

Observed: the residue is host-owned plugin-usage state, left by the `claude plugin enable` step,
and Corvint writes none of it. Inferred, not observed: under load, the host's deferred or at-exit
save of that record races its own process exit, or a stale lock blocks the uninstall's cleanup.
Either way the defect is in host behaviour that the harness observes. It is not a Corvint product
defect.

The `HLQ-V1-002` predicate correctly reports it, because the host really does leave text naming the
plugin in the user's `HOME`. No runner change is made. The predicate is not weakened without an
owner decision. A classification amendment is proposed in the lane report only.

## Retained evidence

The files are in `evidence/v1-1119-1120/`:

- `runs.tsv`: one row per run, with load and failed cases.
- `frontier-attempts.tsv`: each time-bound Stop retry in the retained arm C reports.
- `uninstall-diag.txt`: diagnostics for the three uninstall failures. Host user ids are redacted.
- `arm.sh` and `loop.sh`: the load and loop scripts.
- `diag.patch`: the scratch runner patch.

The full kept reports and stderr were scratch and were removed after these extracts.
