# 2026-10-08: Runner document admission refuses FIFOs before blocking (V1-0624)

## Intent

Ticket V1-0624 records that `corvint-test-runner` opened its request, plan and tools documents
with a blocking `os.Open` and checked the file type only afterwards. A FIFO without a writer
blocked the command before any validation, and SIGINT could not interrupt the open. The change
adds proposed TRE-V0-024 to `docs/specs/test-runner-execution-v0.md`.

## Decisions

- **Refuse before the open.** The companion `read` stats the path, refuses a nonregular file,
  opens it with `O_NONBLOCK` and requires the opened descriptor to be the same regular file. The
  stable refusal is `runner refused: regular document required`, exit 1, as before for
  nonregular documents that did open. `os.Stat` keeps the historical symlink-following behaviour
  for document paths.
- **Executor pins open nonblocking.** `checkTool` and `checkPinnedFile` already refused a FIFO
  through their no-follow `Lstat` checks. Their opens now add `O_NONBLOCK|O_NOFOLLOW|O_CLOEXEC`,
  and `checkTool` requires the opened file to be the checked one, so a FIFO swapped in after the
  check refuses instead of blocking. This covers the independently pinned Gradle build manifest
  (`Config`/`ConfigSha256`) and the pinned Gradle and Java tools. The private
  `checkGradleToolchain` reader named in the ticket is not on `origin/main`; the reader that
  exists is `checkPinnedFile`.
- **No new error-code token.** The refusal reuses the existing message; no wire, plan or receipt
  bytes change.

## Evidence

Base `2a93b5a1`, Go 1.27.1, macOS arm64. Scratch under `/private/tmp/claude-501/v0624-td/repro`.

- Failing before: the base companion (`runner-base`, SHA-256 `5b3bca37…6a9c56`) run as
  `plan --request <fifo>` and `plan --request /nonexistent --tools <fifo>` blocked for 2 s, was
  still blocked 2 s after an actual SIGINT, and exited 137 only after SIGKILL from the
  reproduction script, which removed the FIFO.
- Failing before (test): with the two source files reverted, the new
  `TestNonregularRunnerDocumentsRefusedBeforeBlockingOpen` failed at its 20-second child deadline
  ("companion blocked on a nonregular document"). `TestPinnedGradleManifestFIFORefusedBeforeExecution`
  passed on the base too, because the no-follow `Lstat` already refused; it is a regression
  guard for the manifest path.
- Passing after: the fixed companion refuses both FIFO documents immediately with
  `runner refused: regular document required`, exit 1. Both new tests pass in under 0.1 s each.
  `go test` over `./cmd/corvint-test-runner ./internal/testrunner/...` passes; `go vet` passes for
  the touched packages and for the companion under `GOOS=windows`.

## Non-goals

No hostile filesystem authority: parent directories of a document path are trusted as before.
No change to approved-plan semantics, historical plan/receipt bytes, report inventory reading or
Gradle profile integration status. SDK retirement is not granted.

## Failure modes

A document replaced between `Stat` and the open by another regular file refuses on the
same-file check. A FIFO swapped in for a pinned file after its `Lstat` refuses on the
nonblocking open or the regular-file `Stat`. A FIFO or special file at a document path that
platforms open without `O_NONBLOCK` support (non-Unix) keeps the `Stat` refusal.

## Rollback

Revert the commit. The blocking opens return; no stored state or wire bytes change.

## NOT_RUN

- No deterministic regression test for the swap race between the pinned-file check and its open.
- No live Gradle execution; the manifest test uses stand-in pinned tools and refuses before launch.
- `make gate` and `go test ./...` were not run (shared host; lane policy).
