# Decision 0060 — The parity runner isolates the subject under test, not the toolchain

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-4-plugin-wire.md` after independent review of its cited evidence.

## Context

`conformance/cli-parity-v0` already isolates every execution: fresh process, re-materialised fixture,
rebuilt private cache, home and temp roots, and an allow-listed environment. `docs/reviews/r7-process.md`
O2 proposes parallel replay; an independent reviewer rejected a shared `GOCACHE` for the runner's
candidate build and the specs are silent on build caches.

## Decision

1. Parallel replay is accepted under five conditions: a per-case workspace root (today all executions
   share one `case` path); a bounded worker count from an explicit manifest field or a GOMAXPROCS
   cap, never unbounded fan-out; output buffered per case and flushed in manifest order with the
   `SUMMARY` and the `UNSUPPORTED`/`NOT_RUN` inventory unchanged; the oracle-source-unchanged check
   stays a single post-condition; and acceptance is a full replay before and after with byte-identical
   lines in the same order.
2. The throwaway `GOCACHE` stays until the runner records the candidate binary's SHA-256 and
   `go version -m` build info in the replay transcript. Once that identity is evidence, an explicitly
   opted-in `CORVINT_GOCACHE` may be honoured; a bare inherited `GOCACHE` is never honoured, because it
   is exactly the unrecorded host state at issue.

## Consequences

The gate's claim that this tree's binary matches the frozen oracle acquires a recorded binary
identity. The 5 s cold-build cost is kept until then.

## Rollback

Revert the workspace-root and worker changes; the per-execution isolation is untouched by either.

## Amendment 2026-09-12: `CORVINT_GOCACHE` opt-in not built, on measurement

Section 2's identity precondition landed (`fe18a0f7`, `CANDIDATE-IDENTITY sha256=... go-version-m=...`
before every summary), so the opt-in it gates is now legal to build, but it was left out; the
2026-09-05 `docs/build-log/2026-09-05.md` entry for this decision says so directly ("the optional
cache exit in the decision was not implemented") and its own measurement already answers the open
question. That entry recorded one freshly built candidate replaying the committed corpus in 544 s
at `workers=1` and 178 s at `workers=4`; the throwaway `GOCACHE` this section is about pays for one
candidate build per replay invocation, the ~5 s cold-build cost named in Consequences above, under
1% of either total. The private per-case cache this section defers on does not dominate wall time.
Call: the `CORVINT_GOCACHE` opt-in is not built. Revisit only if a future measured replay shows the
build cost has grown to matter, not on the strength of this note.
