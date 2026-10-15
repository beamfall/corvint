# 2026-10-14 TaskContext reader join observed directly, stack check made tolerant (V1-1362)

## Intent

Hosted run 38476173623 (PR #867, job go-product-shard (7)) failed
`TestTaskContextCancellationNeverReturnsAPacket/recency-on/lexical-term` with
`startContextRecency.func still running after TaskContext returned`. The product join was correct:
`startContextRecency` runs `defer close(recency.ready); recency.read()` in a goroutine and `join`
awaits `ready`, so TaskContext returns only after `read` has finished. The goroutine frame can still
appear in `runtime.Stack` for a moment after the close, between `close` and goroutine exit, and
`assertTaskContextReadersJoined` took one snapshot. `startHistory` (`defer
close(compiler.historyReady)`) has the same window; both are fixed together. No other reader is
joined this way under TaskContext.

The guarantee under test (TCP-V0-064: the co-change history and recency readers are joined before
`TaskContext` returns on every path) stays tested, now by direct observation instead of a stack
snapshot.

## Decisions

1. Test hook, existing pattern. Each reader goroutine's `defer close(ready)` becomes `defer
   taskContextReaderDone("history-read"|"recency-read", ready)`; the helper (appended at the end of
   `taskcontext.go`) calls `taskContextStage(stage)` behind the existing `taskContextStage != nil`
   check, then closes the channel. Zero cost when unset (nil outside tests). The hook runs before
   the close that `join` receives, so a hook set before the call and reset after it is race-free.
   The one-line-for-one-line replacement and the end-of-file helper keep every line cited by
   `docs/specs/core-compatibility-freeze-v1.md` and decision 0377 in place (a first version that
   inserted lines shifted 20 citations). No other product change.
2. Direct observation in every cancellation case: the stage hook counts finished readers; when
   TaskContext returns, the count must equal the readers started (history always, since the case
   has a subject; plus recency when on).
3. Deterministic broken-join detection: `held-history-read` and `held-recency-read` subtests
   (recency off runs only the history one) cancel at the first boundary (`pair`, which skips
   compile's own history and recency waits) and block one reader inside its finish hook, before
   its ready channel can close, for 150 ms while the other reader finishes freely. TaskContext
   returning during the hold fails the test. A correct join cannot return during the hold, so
   this never fails on a correct join; a join that does not await the held reader returns during
   it without depending on timing of the reader. Holding one reader at a time is required: holding
   both let a join that awaited only history pass (observed in the first mutation round).
4. `assertTaskContextReadersJoined` stays as a secondary leak guard and now waits up to 2 s
   (1 ms poll) for the reader frames to leave the stack.
5. Analyzer audit: `TestAnalyzerSchemaInputs` hashes every production source of contextindex. The
   change is query-time only (no extraction or encoding change), so only the audit digest is
   repinned and the schema stays `corvint-analyzer/117`, as for the V1-1141 consumer-only repin.

No spec change: TCP-V0-064's test contract text (`docs/specs/task-context-packet-v0.md`) says "no
surviving reader" and does not name the stack check (grep for `runtime.Stack`, `startContextRecency.func`
and `still running` under `docs/specs` found no reference to this test).

## Failure modes

- A reader leaked by a broken join from an earlier subtest can call a later subtest's hook; the
  counter then reads high and the case still fails (seen as "4 of 1" under the mutation).
- A host stalled for more than 150 ms between compile's return and TaskContext's return would let
  a broken join pass one held case (false negative only); a correct join can never fail it.
- The leak guard can still fail if a reader goroutine survives 2 s past its ready close; that is
  a real leak, not the observed window.

## Acceptance evidence

BASE adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f.

All on the final code (after the review fixes), in a plain clone, host load average 40 to 110 (shared
with other lanes):

| Check | Result |
| --- | --- |
| `GOTOOLCHAIN=local go test -timeout 30m -count=200 -run TestTaskContextCancellationNeverReturnsAPacket ./internal/contextindex/` (run as 4 x `-count=50`: the shared host cannot finish 200 inside one 10-minute tool call) | 200/200 pass |
| same with `-race` (4 x `-count=50`) | 200/200 pass, no race report |
| same under synthetic CPU contention (`contend.sh`: 12 `/usr/bin/yes` busy loops = hw.ncpu, trapped and killed on EXIT/INT/TERM; `-count=100`, 1437.7 s at load average about 100) | 100/100 pass; no busy loop left afterwards |
| `go test -count=1 ./internal/contextindex/`, and with `-race` | ok, ok |
| `go vet ./internal/contextindex/`, `go build ./...`, `gofmt -l cmd internal tools .github/cishards .github/testconfine` | clean, ok, empty |
| 12 CI doc gates (staged) | all rc=0 |

The default `go test` 10-minute timeout once ended a `-count=50` chunk at 600.9 s on the pre-final
code at load average 62 (no assertion failed); the chunks above pass `-timeout 30m` (decision 0082).

Mutation (scratch copy of the clone, not committed): `join()` with its awaits removed, three ways,
each run 5 times with `GOTOOLCHAIN=local go test -count=1 -run
TestTaskContextCancellationNeverReturnsAPacket ./internal/contextindex/`:

| Mutation of `join()` | Held cases | Failed |
| --- | --- | --- |
| both awaits removed | `recency-off/held-history-read`, `recency-on/held-history-read`, `recency-on/held-recency-read` | 15/15 |
| recency await removed | `recency-on/held-recency-read` | 5/5 |
| history await removed | `recency-off/held-history-read`, `recency-on/held-history-read` | 10/10 |

Each failure reads `TaskContext returned while <reader> was held in its finish hook`; every
mutated run also failed overall (0 of 15 `ok`). The fifth run of each mutation used `-race`; the
race detector also reports the broken join itself (reader writes after TaskContext returned). The sampled counter check fails too on some
stages (for example `cancelled at pair: 0 of 1 readers had finished when TaskContext returned`)
but is timing-dependent, which is why the held cases exist.

## Independent review

One Codex read-only round (`codex exec -s read-only`) over adf587fdb..HEAD returned CONCERNS with
three findings, all confirmed against the source and fixed:

1. The leak guard's 100 ms grace could still fail on a reader descheduled on its exit path; the
   grace is now 2 s (a passing check returns at the first clean snapshot, so it costs nothing).
2. In the held case, Go's `select` picks at random when the return and the hold timer are both
   ready, so a broken join could pass; a non-blocking recheck of the return after the timer now
   fails it. The residual false negative is only a host stall over the whole 150 ms hold.
3. On the broken-join failure path, the cleanup reset `taskContextStage` while unreleased readers
   could still read it; the failure path now releases the hold and waits (bounded, 5 s) until
   every reader has entered its hook before failing.

Retained limit: in the per-stage counter subtests a broken join can leak readers into later
subtests' hooks; that only happens when the test is already failing.

## Rollback

Revert the change commit: the two deferred `taskContextReaderDone` calls and the helper in
`recency.go` and `taskcontext.go`, the analyzer audit digest repin, and the test changes in
`taskcontext_cancel_test.go`. No data, format or wire contract changes.
