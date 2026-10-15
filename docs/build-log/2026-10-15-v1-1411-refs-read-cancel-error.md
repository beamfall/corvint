# V1-1411: a cancelled refs read reports the cancellation, not the close error

Date: 2026-10-15. Base: `778bfcd3dd1c8ac8a92b73924de37105a448a3da` (origin/main).
Ticket: V1-1411. Governing requirement: `UCL1142-V0-004` in `docs/specs/upgrade-checklist-v0.md`
(cancellation or the deadline closes an open `--refs-file` descriptor; amendment 0519, V1-1155).

## Contract

`readRefsContext` returns `ctx.Err()` whenever the context is done and the read ended in an error.
On cancellation the `context.AfterFunc` callback closes the descriptor, so the stalled read returns
`os.ErrClosed` and the reader sends it on `done`. When both `done` and `ctx.Done()` were ready at
the final `select`, Go chose one at random and the `done` case returned "file already closed". The
`done` case now prefers `ctx.Err()` when the read failed. A successful read whose `done` case wins
still returns its data; if `ctx.Done()` wins instead, the caller gets the cancellation, as before.
The V1-1155 guarantees are unchanged: the callback stays armed
on cancellation, closes the descriptor, and nothing waits for an uninterruptible read.

Both callers (`Compile` refs scan and the PR-state file) already check `ctx.Err()` before the read
error, so the public refusal stays `changes-git-unavailable`. The defect was the internal error and
the CI flake (PR #920, run 38761994872, `iteration 55 error=file already closed`). No requirement
row changes and no new requirement is needed.

## Test

The existing `V1-1155 cancellation cleanup survives delayed callback dispatch` subtest only hits the
race when the caller reaches the `select` after both channels are ready. That is rare, so it could not
show the fix. A replaceable `beforeRefsWait` hook, a no-op in production, runs just before the
`select`. The new subtest `V1-1411 cancellation wins over the close error when both are ready` uses it
to cancel and wait until the read result is buffered. Each of its 200 iterations then has both cases
ready.

## Evidence

All runs used `-run TestCompileRefsReadHonoursContext ./internal/upgradechanges/` on the shared
12-core host. No extra CPU burners were used.

| Tree | Runs | Failed runs | Load (1 min) |
| --- | --- | --- | --- |
| Base, existing test, `-count=50` | 50 | 1 (`iteration 102 error=file already closed`) | 22.7 |
| Base, cancellation subtest only, `-count=200` | 200 | 9, all `file already closed` | 21.5 |
| Fix, existing test, `-count=200` | 200 | 0 | 24 to 33 |
| Fix with the new subtest, `-count=20` | 20 | 0 | 101.9 to 85.8 |
| Fix with the new subtest, `-count=100` | 100 | 0 | 73.9 to 52.4 |
| Mutation (preference disabled), existing test only, `-count=20` | 20 | 0 (shows the old test's weak bite) | 32 to 37 |
| Mutation with the new subtest, `-count=20` | 20 | 20 (new subtest; 0 in the V1-1155 subtest) | 80.9 to 73.9 |
| After review fixes: fix, `-count=20`; `-race -count=3` | 23 | 0 | 34.7 |
| After review fixes: mutation, `-count=20` | 20 | 20 (new subtest) | 35.7 to 37.4 |

## Review

One Codex round (read-only, diff range above) found two issues. Both are fixed. P2: the test hook's
wait for the stalled read had no bound, and its wait for the buffered result ended without failing.
Both waits are now bounded, and the subtest fails unless both cases were ready. P3: this entry said a
successful read always returns its data, which it does only when the `done` case wins. The text is
corrected. Codex also ran the subtest under `-race -count=3`, which passed.
