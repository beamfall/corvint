# Tasks reads wait out an in-flight writer — issue 433

Owner intent: Russell Lewis reported in issue 433 (2026-10-01) that read-only
`corvint-tasks queue status` and `plan preview` fail `NOT_RUN`/`REDO_PENDING`, and sometimes
`SNAPSHOT_MOVED`, while about ten agents write concurrently, and that a retry two seconds later
succeeds; the owner asked for the condition to be treated as retryable rather than a failed read.
Spec amendment CTS-V0-006 in `docs/specs/corvint-tasks-store-init-v0.md`. Base
`ff3da727e95a1999cbc4a58a471cdd498693f902`.

## Decision

Root cause: `snapshot.Reader.Read` reported a probe-level `REDO_PENDING` at once and retried a
moved snapshot three times back to back with no pause. `REDO_PENDING` at probe time is the §5.2
window between a writer's receipt link-in and its head rename (post-file fsyncs; milliseconds to
hundreds of milliseconds on a loaded host), so a read that lands in it, or straddles consecutive
commits, failed although the store was healthy. The issue's `outcome: ERROR` came from the owner's
external dispatcher; `outcomeFor` already maps both codes to `NOT_RUN`.

The reader now treats both as a writer in flight: it pauses with backoff (25 ms doubling to
400 ms, reset whenever a probe succeeds) and probes again within a two-second patience budget,
then reports the unchanged code with the wait named and the read called retryable. A moved
snapshot is re-read, uncounted, while the budget lasts; the TM-V0-008 attempt bound applies to the
unpaused re-reads after it, so test binaries, which disable the budget through
`internal/tasks/fixture`, keep the exact attempt counts of the existing tests. Reads still write
and lock nothing and never redo a receipt (product invariant 4). The wire envelope is unchanged
(closed profile); the `withStore` and `archive export` reader literals are untouched because the
budget is the reader's default, which keeps this change disjoint from the issue 446 read-cost work
and PR 441. `archive export` owns its own four-attempt loop for movement its body detects, so
`readArchive` gives each attempt only the time left on one shared deadline and names the wait on
its own final `SNAPSHOT_MOVED` (repair r1: the first version gave every attempt the full budget,
up to four times the patience, and reported the moved failure without the wait).

Rejected: a longer budget (hides a crashed writer longer), reader-side redo (a read would mutate),
a new envelope key (closed profile with many decoders), and an environment override (no request;
`Reader.Patience` is the seam if one is ever needed).

## Evidence and limits

`TestCTSV0006_ReadVerbsUnderConcurrentWriter` runs `queue status` and `plan preview` in a loop
against a writer committing twelve receipts with a 10 ms pending window; with the budget disabled
it reproduces the issue (NOT_RUN/REDO_PENDING on both verbs), with the budget it passes. Unit tests
cover the writer finishing during the pause, the budget expiring with the store byte-identical,
pause sizes, and the unpaused attempts after the budget.
`TestCTSV0006_ArchiveExportUnderConcurrentWriter` does the same for `archive export` (29 of 82
exports failed with the budget disabled), and `TestCTSV0006_ArchiveAttemptsShareOnePatience` fails
on the pre-repair loop (20 pauses totalling 415 ms against a 120 ms budget, no wait named). Limits: the reproduction is in-process
and single-writer; the owner's ten-agent host was not re-run. A writer that commits continuously
for longer than the budget still ends `SNAPSHOT_MOVED`, by design.
