## 2026-10-04 V1-0711 V1-0712 V1-0713: lessons from claude-mem's hook and observer failures

Human-owned intent: the owner asked what Corvint can learn from claude-mem
(`https://github.com/thedotmack/claude-mem`, Apache-2.0), then asked for the lessons to be filed as
tickets and recorded here. This entry is an external-design finding. It makes no claim about Corvint
runtime behaviour, and none of the three tickets records a confirmed Corvint defect.

Source: a shallow clone at commit `bfc50259f64a4e0c9e81df849095683608e87ac8` (committed 2026-10-03).
claude-mem captures every tool call through host hooks, has a background worker ask an LLM observer
for typed observations, stores them in SQLite with FTS5 and a Chroma vector index, and injects a
summary at session start. Its design documents under `plans/14` to `plans/23` group several hundred
user reports into defect families, each with a root cause and a fix sequence. Those documents, not
the README, are the evidence for this entry. Nothing was copied into Corvint.

### Findings that apply to Corvint

1. Hooks must be cheap and unable to block (`plans/17-hook-wrapper-contract.md`). More than 25
   reports trace to a login-shell PATH probe on every invocation (250 to 420 ms on macOS, 1 to 10 s
   on Git Bash), exit 2 on an unreachable worker turning the Stop hook into an endless wake loop, a
   cross-session failure counter that escalated to blocking, writes to a closed pipe, and empty stdin
   silently disabling capture. Their remedy is a contract: an availability failure can never return a
   blocking exit code. Filed as V1-0711, a cross-host conformance matrix for Corvint's adapters.
2. Separate liveness from duration (`plans/2026-10-03-liveness-over-deadlines.md`). They count about
   238 guessed timeouts, each mixing "is the peer alive?" with "how long should the work take?", and
   replace them with liveness signals plus idle timeouts. This session's own hook reported
   `corvint-event-rejected:dogfood-event-deadline` on every prompt. That is already tracked by V1-0607
   and V1-0396, so no new ticket was filed; the framing is offered as input to V1-0607.
3. One verified liveness authority (`plans/15-worker-port-and-liveness-authority.md`). Five
   inconsistent "is the worker alive?" signals produced ghost listeners and spawn loops. Their remedy
   is an owner record with a start token that the health endpoint echoes. Corvint has no permanent
   daemon; the lesson applies to the optional local console and to any lock or ledger owner. Not filed.
4. Child processes die with their owner (`plans/14-child-process-ownership.md`). Leaked sidecars
   reached 759 processes and about 71 GB. Corvint already requires owned-process cleanup; this is
   corroboration, not a new requirement. Not filed.
5. Classify model output before trusting it (`plans/18-observer-response-pipeline.md`). Every
   non-conforming LLM reply fell into one bucket that confirmed the batch, so prose, auth errors,
   context overflow, empty replies and schema drift were silent data loss, with unbounded history and
   a hard-coded `max_tokens` truncating 76% of responses. Filed as V1-0712, a spike to list every path
   where host- or model-produced output enters Corvint evidence, evaluation or learning. V1-0369 is a
   known instance.
6. Injected context must be byte-stable (`plans/20-project-identity-and-injection-scope.md`).
   Minute-granularity timestamps changed the injected bytes every 60 s and defeated prefix caching,
   and file context was re-injected on every read of an unchanged file. Filed as V1-0713. Whether any
   Corvint packet field varies across identical invocations is NOT_OBSERVED.
7. One project-identity resolver for writes and reads (same plan). Divergent rules misattributed 26%
   of their store. Corvint pins evidence to Git content, which avoids most of this; their test matrix
   (worktree, submodule, monorepo subdirectory, temp-directory subagent, case-variant checkouts) stays
   a useful checklist for root and store resolution. Not filed.

### Rejected as models for Corvint

- LLM-authored memory with no source binding conflicts with product invariants 1 and 2; claude-mem's
  own near-duplicate and misattribution families show how such a store degrades.
- The Bun, uv/Python, Chroma, HTTP-daemon and hosted-tier stack produces most of their process and
  port defect families, which supports invariant 7's single-binary, no-daemon boundary.
- The README promotes a third-party crypto token. That bears on how far to trust the project, and
  is not a design input.

### Evidence and limits

The three tickets were created in the native store (head sequence 2599 to 2602, `receipt audit`
CONSISTENT and AGREES) and their exports are carried by the same pull request as this entry.
Figures quoted above are claude-mem's own reports and were not reproduced. Its claimed roughly
10x token saving from index-first retrieval is unverified. No Corvint code changed, so no CEM,
evaluation or gate applies.

Rollback: delete this file and the three ticket exports. The tickets themselves can be archived
in the native store.
