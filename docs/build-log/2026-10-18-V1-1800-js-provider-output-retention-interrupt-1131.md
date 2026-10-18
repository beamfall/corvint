# 2026-10-18 V1-1800, V1-1801, V1-1802: JS provider runner output, clean retention and interrupt-first timeout

GitHub #1131, #1132 and #1133 report three gaps in `corvint-js-test-provider`:

- **#1131.** An external-server Playwright run that printed more than 4 MiB was killed and reported
  as `output-overflow`, even though its report was small. The operator could not see the console or
  the runner's exit status.
- **#1132.** The first `--retain` left untracked files under `.corvint/test-evidence`, so a
  clean-tree check failed after an ordinary run.
- **#1133.** A timed-out runner received `SIGTERM` first, so Playwright could not write its
  interrupted report or stop its own workers. The `--server-ready-url` help also promised the
  owned-server readiness rule in both modes.

The new requirements are accepted under the owner's standing auto-accept of lane requirements
(2026-10-11):

- `PWP-V0-028..031` in `docs/specs/playwright-external-provider-v0.md` (V1-1800, V1-1802).
- `LPCV-V0-084` in `docs/specs/live-proof-carrying-verification-v0.md` (V1-1801).

There is no CAL-V0 change. The IDs were checked against origin/main `70a21da5d`, whose highest are
`PWP-V0-027` and `LPCV-V0-083`.

## Design

- **Console bound (PWP-V0-028).** External mode now captures with the existing
  `procgroup.OverflowTruncate` policy. The overflow flags on the observation still say whether the
  console was truncated, and that is recorded as `outputTruncated`. `externalBoundaryFailure` no
  longer turns truncation into `output-overflow`, so the run reads its report as usual. The report
  file keeps its own 4 MiB bound and `report-output-overflow`. Owned-server and unit modes are
  unchanged.
- **Tee.** procgroup gains the opt-in `Spec.StdoutTee` and `Spec.StderrTee`. The tee writer sees
  every byte before the bounded capture does. Its first write error stops the tee only.
  - `--runner-output PATH` creates or truncates the file with mode 0600.
  - `--runner-output -` writes to the provider's stderr, so the printed document on stdout keeps its
    bytes.
  - A failed tee prints `runner-output-incomplete` to stderr and changes nothing in the receipt.
- **Runner record (PWP-V0-029).** `external.runner` is an optional receipt member. It is closed and
  diagnostic, and it is an additive exception to the profile-revision rule, following the
  `projectReporters` precedent (decision 0461). A reader that predates it fails closed through the
  canonical re-encoding check. Receipts written before it keep their bytes.
  - `stopSignal` is the last group signal procgroup sent while the leader's exit was not yet
    observed. Probes (signal 0) and post-exit straggler kills are not recorded, so a runner that
    exits by itself records no stop signal.
  - The exit-status explanation checks are skipped after a timeout, because a runner that exits 130
    on SIGINT is the expected outcome there.
- **Interrupt-first stop (PWP-V0-030).** procgroup's opt-in `Spec.InterruptGrace` sends SIGINT to
  the owned group after the timeout, then waits for the leader's exit, the grace or cancellation.
  The existing stop (SIGTERM, a 100 ms grace and SIGKILL) follows only if the runner has not exited.
  - The default is 10 s, long enough for Playwright to write an interrupted report on a small
    suite.
  - `0` keeps SIGTERM-first.
  - Every other procgroup caller leaves the field zero and is unchanged.
- **Readiness help (PWP-V0-031).** The help of `e2e` and `negate` now states the rules the code
  applies:
  - Owned server: ready on any status below 500, with redirects followed.
  - External server: ready only on a 2xx answer, with no redirect followed.

  The behaviour is unchanged, and a help test pins the text.
- **Clean retention (LPCV-V0-084).** The new helper `internal/testevidence/gitignore.go`
  (`IgnoreRetained`) runs inside `Retain`, so the Go and JavaScript providers both get it.
  - If the retention creates `.corvint`, it writes `.corvint/.gitignore` containing `*`.
  - If `.corvint` already existed (for example, Corvint's own repository tracks
    `.corvint/changes/*.cem.json`), it writes `.corvint/test-evidence/.gitignore` instead, so no tracked project
    file is hidden.
  - Creation is exclusive, so an existing file is never overwritten.
  - `--discover` lists `*.json` only, so selection is unchanged.
  - The Ruby provider lane (V1-1797) can call `IgnoreRetained` or `Retain`.

## Evidence

All tests use fake runners: a shell `npx` stand-in and an `httptest` readiness server. No browser
or npm install was used. The live Playwright run is `NOT_RUN`. The focused tests are listed in each
spec's traceability row.

## Rollback

1. Remove the two flags, the `runner` member, procgroup's `InterruptGrace` and tee hooks, and the
   `IgnoreRetained` call.
2. Restore fail-on-overflow capture in external mode.

Retained receipts that carry `runner` then fail closed and are neither deleted nor rewritten.
Ignore files that were already written stay in place and are harmless.
